package main

import (
	"encoding/json"
	"flag"
	"log"
	"net"
	"sync"
	"time"

	"github.com/navaz-alani/voip/packet"
)

var (
	requests = flag.Int("request-count", 1000, "number of requests to send to server")

	bytesRead = 0
)

func main() {
	flag.Parse()

	svrAddr := &net.UDPAddr{
		IP:   []byte{127, 0, 0, 1},
		Port: 10000,
	}

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
	raw, err := req.Marshal()
	if err != nil {
		log.Fatalln("Failed to encode Packet.")
	}

	completed := 0
	completeChan := make(chan bool)
	wg := &sync.WaitGroup{}

	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		for i := 0; i < *requests; i++ {
			<-completeChan
			completed++
			//log.Printf("Competed request %d of %d\n", completed, *requests)
		}
		wg.Done()
	}(wg)

	conn, err := net.ListenUDP("udp", &net.UDPAddr{
		IP:   []byte{127, 0, 0, 1},
		Port: 10001,
	})
	if err != nil {
		log.Println("Error connecting: ", err)
		return
	}

	start := time.Now()
	for i := 0; i < *requests; i++ {
		wg.Add(1)
		go func(wg *sync.WaitGroup) {
			if _, err := conn.WriteToUDP(raw, svrAddr); err != nil {
				log.Println("Failed to write request to connection", err)
			} else {
				// read and decode server resoponse
				respBuff := make([]byte, 4096)
				if n, _, err := conn.ReadFromUDP(respBuff); err != nil {
					log.Println("Failed to read response")
				} else {
					bytesRead += n
					respPkt := pc.NewPkt("") // decode packet
					if err := respPkt.Unmarshal(respBuff[:n]); err != nil {
						log.Println("Failed to decode server response")
					}
				}
			}
			completeChan <- true
			wg.Done()
		}(wg)
	}

	go func() {
		for {
			time.Sleep(1 * time.Second)
			log.Printf("Read %d bytes\n", bytesRead)
			log.Printf("Completed %d of %d\n", completed, *requests)
			if completed == *requests {
				break
			}
		}
	}()

	wg.Wait()
	end := time.Now()
	log.Printf("Read %d bytes\n", bytesRead)
	log.Printf("%d requests in %v", completed, end.Sub(start))
}
