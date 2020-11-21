package main

import (
	"encoding/json"
	"log"
	"net"

	"github.com/navaz-alani/voip/packet"
)

func main() {
	svrAddr := &net.UDPAddr{
		IP:   []byte{127, 0, 0, 1},
		Port: 10000,
	}
	conn, err := net.ListenUDP("udp", &net.UDPAddr{
		IP:   []byte{127, 0, 0, 1},
		Port: 10001,
	})
	if err != nil {
		log.Fatalln("Error connecting")
	}
	defer conn.Close()

	pc := packet.JSONPktCreator{}
	req := pc.NewPkt(svrAddr.String())
	reqComposer := req.Writer()
	reqComposer.SetTarget("app.echo") // set packet target
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
	// can set additional metatdata
	reqComposer.Close() // commit changes to req packet

	// encode request packet to binary and write it to server over connection.
	if raw, err := req.Marshal(); err != nil {
		log.Fatalln("Failed to encode Packet.")
	} else if _, err := conn.WriteToUDP(raw, svrAddr); err != nil {
		log.Fatalln("Failed to write request to connection\n", err)
	}

	// read and decode server resoponse
	respBuff := make([]byte, 4096)
	if n, sender, err := conn.ReadFromUDP(respBuff); err != nil {
		log.Fatalln("Failed to read response")
	} else {
		log.Printf("Read %d bytes from %s", n, sender.String())
		// decode packet
		respPkt := pc.NewPkt("")
		if err := respPkt.Unmarshal(respBuff[:n]); err != nil {
			log.Fatalln("Failed to decode server response")
		}
		log.Printf("Server says: %s", string(respPkt.Data()))
	}
}
