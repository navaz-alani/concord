package voip

type Packet interface {
	Type() string             // packet type
	From() string             // sender ip
	Dest() string             // destination ip
	Raw() []byte              // raw payload
	Marshal() ([]byte, error) // serialize packet to binary for wire
	Unmarshal([]byte) error   // deserialize packet from binary
}
