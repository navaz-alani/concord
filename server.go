package voip

import (
	"encoding/json"
	"log"
	"net"
	"sync"
)

type UDPVoipImpl struct {
	ip       []byte
	port     int
	buffSize int
	conn     *net.UDPConn
	rooms    map[string]Room
	users    map[string]*User // map ip to user
	dist     chan *Packet
}

func (svr *UDPVoipImpl) Listen() error {
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: svr.ip, Port: svr.port})
	if err != nil {
		return err
	}
	defer conn.Close()
	// begin read/write loops
	var wg sync.WaitGroup
	wg.Add(2)
	go svr.read(&wg)
	go svr.write(&wg)
	wg.Wait()
	return nil
}

func (svr *UDPVoipImpl) read(wg *sync.WaitGroup) {
	defer wg.Done()
	readBuff := make([]byte, svr.buffSize)
	for {
		if n, senderAddr, err := svr.conn.ReadFromUDP(readBuff); err != nil {
			log.Fatalln("udp connection read error")
		} else {
			var sender *User
			if usr, ok := svr.users[senderAddr.String()]; !ok {
				sender = &User{
					addr:        senderAddr.String(),
					memberships: make(map[string]chan *Packet),
				}
			} else {
				sender = usr
			}
			// decode bytes to Packet type and process packet
			var pkt Packet
			if err := json.Unmarshal(readBuff[:n], &pkt); err != nil {
				svr.notifyError("packet decode fail", sender.addr)
			}
			// processl payload
			if room, ok := svr.rooms[pkt.Dest]; !ok || pkt.Dest == "-" {
				svr.notifyError("invalid packet destination", sender.addr)
			} else {
				// handle any special message types for the server
				if pkt.Type == "svr.addRoom" {
					var req struct {
						Type string `json:"type"`
						Name string `json:"name"`
					}
					if err := json.Unmarshal(pkt.Raw, &req); err != nil {
						svr.notifyError("packet decode fail", sender.addr)
					} else {
						// create a new room
						if _, ok := svr.rooms[req.Name]; ok {
							svr.notifyError("room exists", sender.addr)
						} else {
							// todo: different types of rooms based on the req.Type field
							// these rooms are different implementations of Room i`face.
							svr.rooms[req.Name] = NewBasicRoom(req.Name, svr, sender)
						}
					}
				} else {
					// no special actions on message, send to room
					room.Send() <- &pkt
				}
			}
		}
	}
}

func (svr *UDPVoipImpl) write(wg *sync.WaitGroup) {
	defer wg.Done()
	var pkt *Packet
	for {
		pkt = <-svr.dist
		if data, err := json.Marshal(pkt.Raw); err != nil {
			// todo: handle packet decode fail
		} else {
			dst, _ := net.ResolveUDPAddr("udp", pkt.Dest)
			// todo: handle destination address resolution fail
			if _, err := svr.conn.WriteToUDP(data, dst); err != nil {
				// todo: handle connection write fail
			}
		}
	}
}

func (svr *UDPVoipImpl) notifyError(msg string, dest string) {
	svr.dist <- &Packet{
		Dest: dest,
		Type: "svr.error",
		Raw:  []byte(msg),
	}
}

func (svr *UDPVoipImpl) Distribute() chan<- *Packet {
	return svr.dist
}
