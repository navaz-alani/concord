package main

import (
	"encoding/json"
	"log"
	"net"

	"github.com/navaz-alani/voip/client"
	"github.com/navaz-alani/voip/packet"
)

func main() {
	svrAddr := &net.UDPAddr{
		IP:   []byte{127, 0, 0, 1},
		Port: 10000,
	}

	pc := packet.JSONPktCreator{}
	req := pc.NewPkt("", svrAddr.String())
	reqComposer := req.Writer()
	reqComposer.Meta().Add(packet.KeyTarget, "app.echo") // set packet target
	var pkt struct {
		Msg string `json:"msg"`
	}
	pkt.Msg = "Hello from client"
	if bin, err := json.Marshal(pkt); err != nil {
		log.Fatalln("Failed to encode request")
	} else {
		log.Println("sending payload: ", string(bin))
		reqComposer.Write(bin) // set packet data
	}
	reqComposer.Close() // commit changes to req packet

	client, err := client.NewUDPClient(svrAddr, 4096, &pc)
	if err != nil {
		log.Fatalln("Failed to instantiate client")
	}

	respCh := make(chan packet.Packet)
	client.Send(req, respCh) // send request
	resp := <-respCh         // wait till response arrives
	log.Println("Got response: ", string(resp.Data()))
}
