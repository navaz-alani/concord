package main

import (
	"encoding/json"
	"log"
	"net"
	"time"

	"github.com/navaz-alani/concord/core"
	throttle "github.com/navaz-alani/concord/core/throttle"
	"github.com/navaz-alani/concord/packet"
	"github.com/navaz-alani/concord/server"
)

func main() {
	// instantiate server
	addr := &net.UDPAddr{
		IP:   []byte{0, 0, 0, 0},
		Port: 10000,
	}
	var rate throttle.Rate = throttle.Rate10k
	svr, err := server.NewUDPServer(addr, 4096, packet.NewJSONPktCreator(int(rate)/2), rate)
	if err != nil {
		log.Fatalln("Failed to initialize server")
	}

	var requestsServed int

	// configure target on server
	svr.PacketProcessor().AddCallback("app.echo", func(ctx *core.TargetCtx, pw packet.Writer) {
		log.Println("got packet")
		// decode packet data, which in this case is JSON.
		var pkt struct {
			Msg string `json:"msg"`
		}
		if err := json.Unmarshal(ctx.Pkt.Data(), &pkt); err != nil {
			// packet data is malformed - cannot process
			// modify server context to prevent further execution of callback queue
			ctx.Stat = -1
			ctx.Msg = "malformed packet data"
			return
		}
		requestsServed++
		log.Printf("Served %d", requestsServed)
		pw.Write([]byte("Received at " + time.Now().String()))
		pw.Close()
	})

	// run server
	log.Println("Listening on port ", addr.Port)
	svr.Serve()
}
