package packet

import "sync"

// JSONPktCreator implements a PacketCreator for the JSONPkt type.
type JSONPktCreator struct {
	pool *sync.Pool
}

// NewJSONPktCreator initializes and returns a JSONPktCreator with an underlying
// packet pool size of `numPkts`.
func NewJSONPktCreator(numPkts int) PacketCreator {
	pc := &JSONPktCreator{
		pool: &sync.Pool{
			New: func() interface{} {
				pkt := &JSONPkt{
					jsonPkt: &jsonPkt{
						Meta: make(map[string]string),
					},
					mu:   sync.RWMutex{},
					meta: NewKVMeta(),
				}
				pkt.meta.setMeta(pkt.jsonPkt.Meta)
				return pkt
			},
		},
	}
	pc.Warmup(numPkts)
	return pc
}

func (pc *JSONPktCreator) Warmup(numPackets int) {
	for i := 0; i < numPackets; i++ {
		pc.pool.Put(pc.pool.New())
	}
}

func (pc *JSONPktCreator) PutBack(pkt Packet) {
	pc.pool.Put(pkt)
}

func (pc *JSONPktCreator) NewPkt(ref, dest string) Packet {
	pkt := pc.pool.Get().(*JSONPkt)
	pkt.meta.clear()
	pkt.buff.Reset()
	pkt.dest = dest
	pkt.Meta().Add(KeyRef, ref)
	return pkt
}

func (pc *JSONPktCreator) NewErrPkt(ref, dest, msg string) Packet {
	pkt, _ := pc.NewPkt(ref, dest).(*JSONPkt)
	// set reponse's error metadata
	pkt.Meta().Add(KeySvrStatus, "-1")
	pkt.Meta().Add(KeySvrMsg, msg)
	return pkt
}
