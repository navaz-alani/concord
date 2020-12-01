package main

import (
	"log"
	"net"

	"github.com/navaz-alani/concord/client"
	throttle "github.com/navaz-alani/concord/internal/throttle"
	"github.com/navaz-alani/concord/packet"
)

var svrAddr = &net.UDPAddr{
	IP:   []byte{127, 0, 0, 1},
	Port: 10000,
}

func main() {
	// instatiate client which encodes/decodes JSONPkt packets with a 4096 byte
	// read buffer and throttles packet reads/writes over the network at maximum
	// of 10K packets per second.
	pc := packet.JSONPktCreator{}
	client, err := client.NewUDPClient(svrAddr, nil, 4096, &pc, throttle.Rate10k)
	if err != nil {
		log.Fatalln("Failed to instantiate client")
	}

	// compose packet to send to server
	req := pc.NewPkt("", svrAddr.String())
	reqComposer := req.Writer()
	reqComposer.Meta().Add(packet.KeyTarget, "app.echo") // set packet target
	reqComposer.Write([]byte(`{"msg":"hello"}`))         // write JSON payload
	reqComposer.Close()                                  // commit changes to req packet

	// send packet and wait for response
	respCh := make(chan packet.Packet) // create chanel on which to receive response
	client.Send(req, respCh)           // send packet
	resp := <-respCh                   // wait till response arrives
	log.Println("Got response: ", string(resp.Data()))
}
