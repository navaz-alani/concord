package server

import (
	"fmt"
	"net"

	"github.com/navaz-alani/concord/internal"
	throttle "github.com/navaz-alani/concord/internal/throttle"
	"github.com/navaz-alani/concord/packet"
)

type writePacket struct {
	data []byte
	addr *net.UDPAddr
}

// UDPServer is an implementation of the Server type. As suggested by the name,
// it uses UDP for underlying Packet transfer in the transport layer. It is also
// concurrent, by default - each incoming packet is processed in its own
// go-routine.
type UDPServer struct {
	addr      *net.UDPAddr
	conn      *net.UDPConn
	th        throttle.Throttle
	pipelines struct {
		data   *internal.DataPipeline
		packet *internal.PacketPipeline
	}
	pc         packet.PacketCreator
	sendStream chan packet.Packet
	distStream chan writePacket
	shutdown   chan string
	rBuffSize  int
}

func NewUDPServer(addr *net.UDPAddr, rBuffSize int, pc packet.PacketCreator,
	throttleRate throttle.Rate) (*UDPServer, error) {
	// initialize connection
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return nil, err
	}
	svr := &UDPServer{
		addr: addr,
		conn: conn,
		th:   throttle.NewUDPThrottle(throttleRate, conn, rBuffSize),
		pipelines: struct {
			data   *internal.DataPipeline
			packet *internal.PacketPipeline
		}{
			data:   internal.NewDataPipeline(),
			packet: internal.NewPacketPipeline(),
		},
		pc:         pc,
		sendStream: make(chan packet.Packet),
		distStream: make(chan writePacket),
		shutdown:   make(chan string),
		rBuffSize:  rBuffSize,
	}
	svr.pipelines.packet.AddCallback(TargetRelay, svr.relayCallback)
	return svr, nil
}

func (svr *UDPServer) DataProcessor() internal.DataProcessor {
	return svr.pipelines.data
}

func (svr *UDPServer) PacketProcessor() internal.PacketProcessor {
	return svr.pipelines.packet
}

// Serve initiates the server's underlying read/write routines over the
// unerlying connection. It blocks until there is an error in reading over the
// connection, which is then returned.
func (svr *UDPServer) Serve() error {
	defer func() {
		close(svr.distStream) // close writePkts routine
		close(svr.sendStream) // close sendPkts routine
	}()
	svr.pipelines.data.Lock()
	// fire off routines
	go svr.sendPkts()  // pre-process packets before writing
	go svr.writePkts() // write packets to connection
	go svr.readPkts()  // read packets from connection
	// return shutdown message when read fails (readPkts routine will exit)
	return fmt.Errorf(<-svr.shutdown)
}

// relayCallback implements packet forwarding
func (svr *UDPServer) relayCallback(ctx *internal.TargetCtx, pw packet.Writer) {
	sendStream := svr.send() // send-only access to svr.sendStream
	ref := ctx.Pkt.Meta().Get(packet.KeyRef)
	relayAddr := ctx.Pkt.Meta().Get(KeyRelayTo)
	// create a new packet to be forwarded and send it
	fwdPkt := svr.pc.NewPkt(ref, relayAddr)
	fwdPkt.Meta().Add(KeyRelayFrom, ctx.From)
	fwdPkt.Writer().Write(ctx.Pkt.Data())
	fwdPkt.Writer().Close()
	sendStream <- fwdPkt
	//can stop processing of packet here, no more actions needed
	ctx.Stat = internal.CodeStopNoop
	ctx.Msg = "packet forwarded"
}

func (svr *UDPServer) dist() chan<- writePacket   { return svr.distStream }
func (svr *UDPServer) send() chan<- packet.Packet { return svr.sendStream }

func (svr *UDPServer) fmtMsg(msg string) string {
	return fmt.Sprintf("[UDPServer@%s] %s", svr.addr.String(), msg)
}

// processIncoming runs the given data through the server's data pipelines.
func (svr *UDPServer) processIncoming(data []byte, senderAddr net.Addr) {
	sendStream := svr.send() // send-only access to svr.sendStream
	// pre-processing data buffer
	var err error
	transformCtx := &internal.TransformContext{
		PipelineName: "_in_",
		From:         senderAddr.String(),
	}
	if data, err = svr.pipelines.data.Process(transformCtx, data); err != nil {
		sendStream <- svr.pc.NewErrPkt("", senderAddr.String(), "data pipeline error: "+err.Error())
		return
	} else if transformCtx.Stat == internal.CodeStopNoop {
		return
	}

	pkt := svr.pc.NewPkt("", "")
	if err := pkt.Unmarshal(data); err != nil { // decode packet
		sendStream <- svr.pc.NewErrPkt("", senderAddr.String(), "malformed packet")
		return
	}
	// execute packet target callback queue
	ref := pkt.Meta().Get(packet.KeyRef)
	resp := svr.pc.NewPkt(ref, senderAddr.String())
	ctx := &internal.TargetCtx{
		PipelineCtx: internal.PipelineCtx{
			Pkt: pkt,
		},
		TargetName: pkt.Meta().Get(packet.KeyTarget),
		From:       senderAddr.String(),
	}
	// execute callback queue
	if err := svr.pipelines.packet.Process(ctx, resp.Writer()); err != nil {
		sendStream <- svr.pc.NewErrPkt(ref, senderAddr.String(), "packet pipeline error: "+err.Error())
	} else if ctx.Stat != internal.CodeStopNoop {
		resp.Writer().Close()
		sendStream <- resp
	}
}

// processOutgoing runs the given `pkt` through the client pipelines and when
// done, sends the final data to be written to the connection (through the
// server `distStream`).
func (svr *UDPServer) processOutgoing(pkt packet.Packet) {
	if bin, err := pkt.Marshal(); err == nil {
		if addr, err := net.ResolveUDPAddr("udp", pkt.Dest()); err == nil {
			// pre-processing data buffer
			transformCtx := &internal.TransformContext{
				PipelineCtx: internal.PipelineCtx{
					Pkt: pkt,
				},
				PipelineName: "_out_",
			}
			if bin, err := svr.pipelines.data.Process(transformCtx, bin); err != nil {
				svr.send() <- svr.pc.NewErrPkt(pkt.Meta().Get(packet.KeyRef),
					pkt.Dest(), "pipeline error: "+err.Error())
			} else if transformCtx.Stat == internal.CodeStopNoop {
			} else {
				svr.dist() <- writePacket{
					data: bin,
					addr: addr,
				}
			}
		}
	}
}

// read is a routune which reads and decodes packets from the underlying
// connection and spawns a routine to process each packet read. It is the only
// writer to the server's `shutdown` channel.
func (svr *UDPServer) readPkts() {
	for {
		if data, senderAddr, err := svr.th.ReadFrom(); err != nil {
			// send shutdown signal to end write routine
			svr.shutdown <- svr.fmtMsg("read fail - connection error")
			break
		} else {
			go svr.processIncoming(data, senderAddr)
		}
	}
}

// write is a routine which distributes packets by writing them over the
// underlying UDP connection. Any encoding/write errors are ignored. It is the
// only consumer of distStream. It also serves the purpose of throttling the
// packet-write-rate of the server.
func (svr *UDPServer) writePkts() {
	var pkt writePacket
	for pkt = range svr.distStream {
		svr.th.WriteTo(pkt.data, pkt.addr) // throttled write operation
	}
}

// sendPkts is a routine which processes packets before they are written over
// the conecction. It is the only consumer of sendStream.
func (svr *UDPServer) sendPkts() {
	var pkt packet.Packet
	for pkt = range svr.sendStream {
		go svr.processOutgoing(pkt)
	}
}
