package voip

type BasicRoom struct {
	name     string
	addr     string
	members  map[*User]bool
	incoming chan Packet
	srvc     *UDPVoipImpl
}

func NewBasicRoom(name string, service *UDPVoipImpl, creator *User) Room {
	room := &BasicRoom{
		name:    name,
		members: make(map[*User]bool),
		srvc:    service,
	}
	room.members[creator] = true
	return room
}

func (r *BasicRoom) AddMember(u *User) error {
	r.members[u] = true
	return nil
}

func (r *BasicRoom) RemoveMember(u *User) {
	delete(r.members, u)
}

func (r *BasicRoom) Name() string {
	return r.name
}

func (r *BasicRoom) Addr() string {
	return r.addr
}

func (r *BasicRoom) Send() chan<- Packet {
	return r.incoming
}

func (r *BasicRoom) Listen() {
	var pkt Packet
	for {
		pkt = <-r.incoming
		if pkt.Dest("") != r.name {
			continue
		}
		// forward payload to each user
		for user := range r.members {
			pkt := pkt.Clone()
			pkt.Dest(user.addr)
			r.srvc.Distribute() <- pkt
		}
		r.srvc.dist <- pkt
	}
}
