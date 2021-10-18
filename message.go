package tcpsocket

import (
	"github.com/google/uuid"
)

type Message struct {
	id      [16]byte
	Request []byte
	Reply   []byte
}

func NewMessage() *Message {
	return &Message{id: uuid.New()}
}

func (m *Message) Id() [16]byte {
	return m.id
}
