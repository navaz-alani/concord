package main

import (
	"encoding/json"
	"log"
	"net"
	"time"

	"github.com/navaz-alani/voip/packet"
	"github.com/navaz-alani/voip/server"
)

func main() {
	// instantiate server
	addr := &net.UDPAddr{
		IP:   []byte{0, 0, 0, 0},
		Port: 10000,
	}
	svr := server.NewUDPServer(&packet.JSONPktCreator{}, addr, 4096)

	// configure target on server
	svr.AddTarget("app.echo", func(ctx *server.ServerCtx, pw packet.Writer) {
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
		log.Println("Pkt contents: ", pkt.Msg)
		pw.Write([]byte("Received at " + time.Now().String()))
		pw.Close()
	})

	// run server
	log.Println("Listening on port ", addr.Port)
	svr.Serve()
}
