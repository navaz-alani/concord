package main

import (
	"log"
	"net"

	"github.com/navaz-alani/concord/client"
	crypto "github.com/navaz-alani/concord/core/crypto"
	throttle "github.com/navaz-alani/concord/core/throttle"
	"github.com/navaz-alani/concord/packet"
	"github.com/navaz-alani/concord/server"
)

var (
	rate throttle.Rate = throttle.Rate10k
	pc                 = packet.NewJSONPktCreator(int(rate) / 2)

	svrAddr = &net.UDPAddr{
		IP:   []byte{127, 0, 0, 1},
		Port: 10000,
	}
	clientA_Addr = &net.UDPAddr{
		IP:   []byte{127, 0, 0, 1},
		Port: 10001,
	}
	clientB_Addr = &net.UDPAddr{
		IP:   []byte{127, 0, 0, 1},
		Port: 10002,
	}
)

func createSecureClient(listenAddr *net.UDPAddr) (cl client.Client, cr *crypto.Crypto) {
	var err error
	if cl, err = client.NewUDPClient(svrAddr, listenAddr, 4096, pc, rate); err != nil {
		log.Fatalf("client init err: %s", err.Error())
	}
	if cr, err = crypto.ConfigureClient(cl, svrAddr.String(), pc.NewPkt("", svrAddr.String())); err != nil {
		log.Fatalf("crypto err: %s", err.Error())
	}
	return cl, cr
}

func makeRelayPkt(to string, msg string) packet.Packet {
	pkt := pc.NewPkt("", svrAddr.String())
	writer := pkt.Writer()
	writer.Meta().Add(packet.KeyTarget, server.TargetRelay)
	writer.Meta().Add(server.KeyRelayTo, to) // set server target to "relay"
	writer.Write([]byte(msg))
	writer.Close()
	return pkt
}

func main() {
	// instantiate secure clients
	clientA, crA := createSecureClient(clientA_Addr)
	clientB, crB := createSecureClient(clientB_Addr)

	// perform key-exchange between clients
	if err := crA.ClientKEx(clientA, clientB_Addr.String(), pc.NewPkt("", svrAddr.String())); err != nil {
		log.Fatalf("clientA kex fail: %s\n", err.Error())
	} else if err = crB.ClientKEx(clientA, clientA_Addr.String(), pc.NewPkt("", svrAddr.String())); err != nil {
		log.Fatalf("clientB kex fail: %s\n", err.Error())
	}

	// initiate clientB misc packet listener
	go func() {
		var pkt packet.Packet
		pkt = <-clientB.Misc()
		sender := pkt.Meta().Get(server.KeyRelayFrom)
		if err := crB.DecryptE2E(sender, pkt); err != nil { // decrypt incoming data
			log.Printf("clientB: got relay pkt decrypt-e2e err: %s", err.Error())
		} else {
			log.Printf("clientB: got relay pkt with data: %s", string(pkt.Data()))
			respPkt := makeRelayPkt(clientA_Addr.String(), "super-secret-msg-from-B")
			respPkt.Meta().Add(packet.KeyRef, pkt.Meta().Get(packet.KeyRef))
			crB.EncryptE2E(clientA_Addr.String(), respPkt)
			clientB.Send(respPkt, nil) // send and discard reponse
		}
	}()

	// send message from clientA to clientB
	pkt := makeRelayPkt(clientB_Addr.String(), "super-secret-msg-from-A")
	crA.EncryptE2E(clientB_Addr.String(), pkt) // end-to-end encrypt packet data
	respCh := make(chan packet.Packet)
	clientA.Send(pkt, respCh)
	log.Printf("clientA: sent message to clientB")
	resp := <-respCh
	crA.DecryptE2E(clientB_Addr.String(), resp)
	log.Printf("clientA: response from clientB: %s\n", string(resp.Data()))
}
