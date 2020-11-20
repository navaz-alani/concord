package voip

type Packet interface {
	Type(string) string       // get/set packet type
	From(string) string       // get/set sender ip
	Dest(string) string       // get/set destination ip
	Raw() []byte              // raw payload
	Marshal() ([]byte, error) // serialize packet to binary for wire
	Unmarshal([]byte) error   // deserialize packet from binary
	Clone() Packet
}
