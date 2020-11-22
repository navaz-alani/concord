package packet

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"sync"
)

// jsonPkt is the underlying type which is encoded and decoded to and from bytes
// over the wire.
type jsonPkt struct {
	Target string            `json:"t"`
	Meta   map[string]string `json:"m"`
	Data   string            `json:"d"`
}

// JSONPkt wraps the underlying wire type to provide concurrency support and
// manage access to the underlying packet data. It encodes and decodes to
// `jsonPkt`. The JSON for JSONPkt should use field names as specified in the
// `jsonPkt` struct tags.
type JSONPkt struct {
	*jsonPkt
	mu   sync.RWMutex
	buff bytes.Buffer
	meta *KVMeta
	dest string
}

// JSONPktCreator implements a PacketCreator for the JSONPkt type.
type JSONPktCreator struct{}

func (pc *JSONPktCreator) NewPkt(ref, dest string) Packet {
	pkt := &JSONPkt{
		jsonPkt: &jsonPkt{
			Meta: make(map[string]string),
		},
		mu:   sync.RWMutex{},
		meta: NewKVMeta(),
		dest: dest,
	}
	pkt.meta.setMeta(pkt.jsonPkt.Meta)
	pkt.Meta().Add("_ref", ref)
	return pkt
}

func (pc *JSONPktCreator) NewErrPkt(ref, dest, msg string) Packet {
	pkt, _ := pc.NewPkt(ref, dest).(*JSONPkt)
	// set packet error flags
	pkt.Meta().Add("_stat", "-1")
	pkt.Meta().Add("_msg", msg)
	return pkt
}

func (p *JSONPkt) Dest() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.dest
}

func (p *JSONPkt) Writer() Writer { return p }

func (p *JSONPkt) Target() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.jsonPkt.Target
}

func (p *JSONPkt) Meta() Metadata {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.meta
}

func (p *JSONPkt) Data() []byte {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.buff.Bytes()
}

func (p *JSONPkt) SetTarget(t string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.jsonPkt.Target = t
}

func (p *JSONPkt) Marshal() (bin []byte, err error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if bin, err := json.Marshal(p.jsonPkt); err != nil {
		return nil, err
	} else {
		return bin, nil
	}
}

// Unmarshal decodes the binary data into the packet `p`.
func (p *JSONPkt) Unmarshal(bin []byte) (err error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if err := json.Unmarshal(bin, p.jsonPkt); err != nil {
		return err
	}
	// unpack metadata from packet
	p.meta.setMeta(p.jsonPkt.Meta)
	// decode received base64 data
	if dec, err := base64.StdEncoding.DecodeString(p.jsonPkt.Data); err != nil {
		return err // malformed base64 data
	} else {
		p.buff = *bytes.NewBuffer(dec)
	}
	return nil
}

func (p *JSONPkt) Write(data []byte) (n int, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.buff.Write(data)
}

// Close commits the data written to the buffer to the underlying packet.
func (p *JSONPkt) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.jsonPkt.Data = base64.StdEncoding.EncodeToString(p.buff.Bytes())
	return nil
}
