package voip

type Room interface {
	Addr() string
	Name() string
	AddMember(*User) error
	RemoveMember(*User)
	Send() chan<- *Packet
}
