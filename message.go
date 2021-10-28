package tcpsocket

import (
	"github.com/google/uuid"
)

type message struct {
	Id     Id
	Data   []byte
	IsSent chan bool
}

var (
	JoinReqId = [...]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	JoinOkId  = [...]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}
	PingId    = [...]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0}
	AckId     = [...]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2, 0}
)

func newMessage() *message {
	id := uuid.New()
	return &message{Id: id[:], IsSent: make(chan bool, 1)}
}
