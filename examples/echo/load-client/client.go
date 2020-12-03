package main

import (
	"flag"
	"log"
	"math/rand"
	"net"
	"sync"
	"time"

	"github.com/navaz-alani/concord/client"
	throttle "github.com/navaz-alani/concord/core/throttle"
	"github.com/navaz-alani/concord/packet"
)

var (
	requests = flag.Int("request-per-client", 1000, "number of requests to send to server")
	clients  = flag.Int("num-clients", 1, "number of concurrent clients")

	bytesRead = 0

	svrAddr = &net.UDPAddr{
		IP:   []byte{127, 0, 0, 1},
		Port: 10000,
	}
)

func main() {
	flag.Parse()
	totalRequests := *requests * *clients
	rand.Seed(time.Now().Unix())

	completeChan := make(chan bool) // channel over which request completions will be reported
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		for completed := 1; completed <= totalRequests; completed++ {
			<-completeChan
			log.Printf("Competed request %d / %d\n", completed, *requests)
		}
		wg.Done()
	}(wg)

	// instatiate client which encodes/decodes JSONPkt packets with a 4096 byte
	// read buffer and throttles packet reads/writes over the network at maximum
	// of 10K packets per second.
	pc := packet.JSONPktCreator{}
	client, err := client.NewUDPClient(svrAddr, nil, 4096, &pc, throttle.Rate1h)
	if err != nil {
		log.Fatalln("Failed to instantiate client")
	}

	start := time.Now()
	for c := 0; c < *clients; c++ {
		wg.Add(1)
		go func() {
			clientWg := &sync.WaitGroup{}
			// compose packet to send to server
			for r := 0; r < *requests; r++ {
				clientWg.Add(1)
				go func() {
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
					completeChan <- true
					clientWg.Done()
				}()
			}
			// wait till all requests completed
			clientWg.Wait()
			wg.Done()
		}()
	}
	wg.Wait()
	log.Printf("%d requests in %v", totalRequests, time.Now().Sub(start))
}
