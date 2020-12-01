package main

import (
	"crypto/ecdsa"
	"crypto/rand"
	"fmt"
	"log"
	"net"

	"github.com/navaz-alani/concord/client"
	crypto "github.com/navaz-alani/concord/internal/crypto"
	throttle "github.com/navaz-alani/concord/internal/throttle"
	"github.com/navaz-alani/concord/packet"
	"github.com/navaz-alani/concord/server"
)

var (
	pc = packet.JSONPktCreator{}

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

func createSecureClient(addr *net.UDPAddr) (client.Client, *crypto.Crypto) {
	client, err := client.NewUDPClient(svrAddr, addr, 4096, &pc, throttle.Rate10k)
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
	return client, cr
}

// performs key exchange with given addr
func kexWith(client client.Client, cr *crypto.Crypto, addr *net.UDPAddr) error {
	pkt := pc.NewPkt("", svrAddr.String())
	cr.ConfigureKeyExClientPkt(addr.String(), pkt.Writer())
	pkt.Writer().Close()
	respChan := make(chan packet.Packet)
	client.Send(pkt, respChan)
	if err := cr.ProcessKeyExResp(addr.String(), <-respChan); err != nil {
		return fmt.Errorf("Client-Client kex failed with: %s\nerror: %s", addr.String(), err.Error())
	}
	return nil
}

func main() {
	// instantiate clients
	clientA, crA := createSecureClient(clientA_Addr)
	clientB, crB := createSecureClient(clientB_Addr)

	// perform key-exchange between clients
	if err := kexWith(clientA, crA, clientB_Addr); err != nil {
		log.Fatalf("clientA kex fail: %s\n", err.Error())
	} else if err = kexWith(clientB, crB, clientA_Addr); err != nil {
		log.Fatalf("clientB kex fail: %s\n", err.Error())
	}

	// initiate clientB misc packet listener
	go func() {
		var pkt packet.Packet
		pkt = <-clientB.Misc()
		sender := pkt.Meta().Get(server.KeyRelayFrom)
		if err := crB.DecryptE2E(sender, pkt); err != nil {
			log.Println("clientB: got relay pkt decrypt-e2e err: " + err.Error())
		} else {
			log.Println("clientB: got relay pkt with data: " + string(pkt.Data()))
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

func makeRelayPkt(to string, msg string) packet.Packet {
	pkt := pc.NewPkt("", svrAddr.String())
	writer := pkt.Writer()
	writer.Meta().Add(packet.KeyTarget, server.TargetRelay)
	writer.Meta().Add(server.KeyRelayTo, to) // set server target to "relay"
	writer.Write([]byte(msg))
	writer.Close()
	return pkt
}
