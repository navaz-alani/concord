package main

import (
	"encoding/json"
	"flag"
	"log"
	"math/rand"
	"net"
	"sync"
	"time"

	"github.com/navaz-alani/voip/client"
	"github.com/navaz-alani/voip/packet"
)

var (
	requests = flag.Int("request-count", 1000, "number of requests to send to server")

	bytesRead = 0
)

func main() {
	flag.Parse()
	rand.Seed(time.Now().Unix())
	svrAddr := &net.UDPAddr{
		IP:   []byte{127, 0, 0, 1},
		Port: 10000,
	}

	pc := packet.JSONPktCreator{}

	completed := 0
	completeChan := make(chan bool)
	wg := &sync.WaitGroup{}

	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		for i := 0; i < *requests; i++ {
			<-completeChan
			completed++
			log.Printf("Competed request %d of %d\n", completed, *requests)
		}
		wg.Done()
	}(wg)

	client, err := client.NewUDPClient(svrAddr, 4096, &pc)
	if err != nil {
		log.Fatalln("Failed to instantiate client")
	}

	start := time.Now()
	for i := 0; i < *requests; i++ {
		go func() {
			wg.Add(1)
			req := pc.NewPkt("", svrAddr.String())
			reqComposer := req.Writer()
			reqComposer.SetTarget("app.echo") // set packet target
			var pkt struct {
				Msg string `json:"msg"`
			}
			pkt.Msg = "Hello from client"
			if bin, err := json.Marshal(pkt); err != nil {
				log.Fatalln("Failed to encode request")
			} else {
				reqComposer.Write(bin) // set packet data to pkt JSON repr
			}
			// can set additional metatdata ...
			reqComposer.Close() // commit changes to req packet

			respCh := make(chan packet.Packet)
			client.Send(req, respCh)
			<-respCh // wait till response arrives
			completeChan <- true
			wg.Done()
		}()
	}

	wg.Wait()
	end := time.Now()
	log.Printf("%d requests in %v", completed, end.Sub(start))
}
