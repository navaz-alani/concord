package voip

type User struct {
	memberships map[string]chan *Packet // maps room names to their recv channels
	addr        string
}
