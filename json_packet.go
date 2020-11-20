package voip

import "encoding/json"

type JSONPacket struct {
	// packet type
	PktType string `json:"t"`
	// packet destination - room name
	PktDest string `json:"to"`
	// source room name = room/ip
	PktFrom string `json:"s"`
	// raw packet data
	PktRaw []byte `json:"d"`
}

func (jp *JSONPacket) Type() string { return jp.PktType }
func (jp *JSONPacket) Dest() string { return jp.PktDest }
func (jp *JSONPacket) From() string { return jp.PktFrom }
func (jp *JSONPacket) Raw() []byte  { return jp.PktRaw }

func (jp *JSONPacket) Marshal() ([]byte, error) {
	if b, err := json.Marshal(jp); err != nil {
		return nil, err
	} else {
		return b, nil
	}
}

func (jp *JSONPacket) Unmarshal(raw []byte) error {
	if err := json.Unmarshal(raw, jp); err != nil {
		return err
	}
	return nil
}
