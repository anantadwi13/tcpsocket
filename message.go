package tcpsocket

import (
	"github.com/google/uuid"
)

type message struct {
	Id     messageId
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

type messageId []byte

func (m messageId) equal(other interface{}) bool {
	o, ok := other.(messageId)
	if !ok {
		arrBytes, ok := other.([16]byte)
		if !ok {
			return false
		}
		o = arrBytes[:]
	}

	if len(o) != len(m) {
		return false
	}

	for i, v := range m {
		if v != o[i] {
			return false
		}
	}

	return true
}

func (m messageId) fixed() (b [16]byte) {
	copy(b[:], m)
	return
}
