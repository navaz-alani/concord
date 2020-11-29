package main

import (
	"crypto/ecdsa"
	"crypto/rand"
	"log"
	"net"

	"github.com/navaz-alani/concord/client"
	crypto "github.com/navaz-alani/concord/internal/crypto"
	"github.com/navaz-alani/concord/packet"
	"github.com/navaz-alani/concord/throttle"
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
	client, err := client.NewUDPClient(svrAddr, 4096, &pc, throttle.Rate10k)
	if err != nil {
		log.Fatalln("Failed to instantiate client")
	}
	// generate private key
	privKey, err := ecdsa.GenerateKey(crypto.Curve, rand.Reader)
	if err != nil {
		log.Fatalln("Failed to generate public key: " + err.Error())
	}
	// initialize Crypto extension
	cr, err := crypto.NewCrypto(privKey)
	if err != nil {
		log.Fatalln("Cryto extenstion error: " + err.Error())
	}
	// perform key-exchange with server
	svrKexPkt := pc.NewPkt("", svrAddr.String())
	cr.ConfigureKeyExServerPkt(svrKexPkt.Writer())
	kexResp := make(chan packet.Packet)
	client.Send(svrKexPkt, kexResp)
	kexRespPkt := <-kexResp
	if err := cr.ProcessKeyExResp(svrAddr.String(), kexRespPkt); err != nil {
		log.Fatalln("Crypto server handshake error: ", err.Error())
	}
	// install extension on server pipelines to provide transport encryption
	cr.Extend("client", client)

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
