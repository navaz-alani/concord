package voip

type Room interface {
	Addr() string
	Name() string
	AddMember(*User) error
	Send() chan<- *Packet
}
