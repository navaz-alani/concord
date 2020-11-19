package voip

// Packet is the Go type for the underlying wire type (JSON). Applications may
// make use of the `t` JSON field to categorize messages and deal with the raw
// `d` JSON field appropriately.
type Packet struct {
	// packet type
	Type string `json:"t"`
	// packet destination - room id
	Dest string `json:"to"`
	// raw packet data
	Raw []byte `json:"d"`
}
