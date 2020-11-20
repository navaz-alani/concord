package voip

import (
	"encoding/json"
	"fmt"
	"net"
	"sync"
)

// Packet types
const (
	// server packet types
	SvrPing  = "svr.ping"
	SvrError = "svr.err"

	// room packet types
	SvrMkRoom   = "svr.rm.mk"
	SvrJoinRoom = "svr.rm.join"
	SvrExitRoom = "svr.rm.ex"
)

// Room types
const (
	RoomBasic   = "rm.basic"
	RoomPrivate = "rm.priv"
	RoomGroup   = "rm.grp"
)

type UDPVoipImpl struct {
	ip       []byte
	port     int
	buffSize int
	conn     *net.UDPConn
	rooms    map[string]Room
	users    map[string]*User
	dist     chan Packet
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
			panic("udp connection read error")
		} else {
			var sender *User
			if usr, ok := svr.users[senderAddr.String()]; !ok {
				sender = &User{
					addr: senderAddr.String(),
				}
				// cache sender ip
				svr.users[sender.addr] = sender
			} else {
				sender = usr
			}

			var pkt Packet
			if pkt.Unmarshal(readBuff[:n]) != nil { // decode packet
				svr.notifyError("packet decode fail", sender.addr)
			}
			// process packet
			if room, ok := svr.rooms[pkt.Dest("")]; !ok || pkt.Dest("") == "-" {
				svr.notifyError("invalid packet destination", sender.addr)
			} else {
				// handle any special message types for the server
				switch pkt.Type("") {
				case SvrMkRoom:
					{
						var req struct {
							Type string `json:"type"`
							Name string `json:"name"`
						}
						if err := json.Unmarshal(pkt.Raw(), &req); err != nil {
							svr.notifyError("packet decode fail", sender.addr)
						} else {
							// create a new room
							if _, ok := svr.rooms[req.Name]; ok {
								svr.notifyError("room exists", sender.addr)
							} else {
								// todo: different types of rooms based on the req.Type field
								// these rooms are different implementations of Room i`face.
								switch req.Type {
								case RoomBasic:
									svr.rooms[req.Name] = NewBasicRoom(req.Name, svr, sender)
								case RoomPrivate:
								case RoomGroup:
								default:
									svr.rooms[req.Name] = NewBasicRoom(req.Name, svr, sender)
								}
							}
						}
					}
				case SvrJoinRoom:
					if err := room.AddMember(sender); err != nil {
						svr.notifyError("failed to join room", sender.addr)
					}
				case SvrExitRoom:
					room.RemoveMember(sender)
				default:
					// not a server message type
					room.Send() <- pkt
				}
			}
		}
	}
}

func (svr *UDPVoipImpl) write(wg *sync.WaitGroup) {
	defer wg.Done()
	var pkt Packet
	for {
		pkt = <-svr.dist
		if data, err := pkt.Marshal(); err != nil {
			continue // ignore packet encode fail
		} else {
			dst, err := net.ResolveUDPAddr("udp", pkt.Dest(""))
			if err != nil {
				continue
			} // ignore malformed destinations
			if _, err := svr.conn.WriteToUDP(data, dst); err != nil {
				continue // ignore connection write fail
			}
		}
	}
}

func (svr *UDPVoipImpl) notifyError(msg, dest string) {
	svr.dist <- &JSONPacket{
		PktDest: dest,
		PktType: SvrError,
		PktRaw:  []byte(fmt.Sprintf("error: %s", msg)),
	}
}

func (svr *UDPVoipImpl) Distribute() chan<- Packet {
	return svr.dist
}
