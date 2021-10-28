package tcpsocket

import (
	"errors"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
)

type tcpChannel struct {
	chanId             string
	incomingData       chan []byte
	outgoingMessageIds chan []byte

	connPool     map[*TcpConn]*TcpConn
	connPoolLock sync.RWMutex

	messages     map[[16]byte]*message
	messagesLock sync.RWMutex

	listener     map[[16]byte]ChannelListener
	listenerLock sync.RWMutex

	isClosed     bool
	isClosedLock sync.RWMutex
}

type ChannelListener func(data []byte, err error)

func newTcpChannel(chanId string) *tcpChannel {
	c := &tcpChannel{
		chanId:             chanId,
		incomingData:       make(chan []byte, 256),
		outgoingMessageIds: make(chan []byte, 256),
		connPool:           map[*TcpConn]*TcpConn{},
		messages:           map[[16]byte]*message{},
		listener:           map[[16]byte]ChannelListener{},
	}
	go c.receiverDaemon()
	return c
}

func (c *tcpChannel) addConn(conn *TcpConn) error {
	if c.IsClosed() {
		return errors.New("channel is closed")
	}

	c.connPoolLock.Lock()
	c.connPool[conn] = conn
	c.connPoolLock.Unlock()

	go c.writeDaemon(conn)
	go c.readDaemon(conn)
	return nil
}

func (c *tcpChannel) AddListener(l ChannelListener) (listenerId [16]byte, err error) {
	if c.IsClosed() {
		err = errors.New("channel is closed")
		return
	}

	listenerId = uuid.New()
	c.listenerLock.Lock()
	c.listener[listenerId] = l
	c.listenerLock.Unlock()
	return
}

func (c *tcpChannel) RemoveListener(listenerId [16]byte) (err error) {
	if c.IsClosed() {
		return errors.New("channel is closed")
	}

	c.listenerLock.Lock()
	if listener, ok := c.listener[listenerId]; listener != nil && ok {
		delete(c.listener, listenerId)
	} else {
		err = errors.New("listener not found")
	}
	c.listenerLock.Unlock()
	return
}

func (c *tcpChannel) Close() error {
	c.isClosedLock.Lock()
	c.isClosed = true
	c.isClosedLock.Unlock()

	c.listenerLock.Lock()
	for id := range c.listener {
		delete(c.listener, id)
	}
	c.listenerLock.Unlock()

	c.connPoolLock.Lock()
	for conn := range c.connPool {
		delete(c.connPool, conn)
		conn.Close()
	}
	c.connPoolLock.Unlock()
	// todo wrap error
	return nil
}

func (c *tcpChannel) IsClosed() bool {
	c.isClosedLock.RLock()
	defer func() {
		c.isClosedLock.RUnlock()
	}()
	return c.isClosed
}

func (c *tcpChannel) Send(data []byte, timeout *time.Duration) error {
	if c.IsClosed() {
		return errors.New("channel is closed")
	}

	if timeout == nil {
		t := 30 * time.Second
		timeout = &t
	}

	msg := newMessage()
	msg.Data = data

	c.messagesLock.Lock()
	c.messages[msg.Id.fixed()] = msg
	c.messagesLock.Unlock()

	c.outgoingMessageIds <- msg.Id

	select {
	case isSent := <-msg.IsSent:
		if !isSent {
			return errors.New("data is not sent")
		}
		return nil
	case <-time.After(*timeout):
		return errors.New("timeout")
	}
}

func (c *tcpChannel) receiverDaemon() {
	for !c.IsClosed() {
		data := <-c.incomingData
		c.listenerLock.RLock()
		for _, listener := range c.listener {
			go listener(data, nil)
		}
		c.listenerLock.RUnlock()
	}
}

func (c *tcpChannel) writeDaemon(conn *TcpConn) {
	for {
		if c.IsClosed() {
			return
		}

		var msgId [16]byte
		copy(msgId[:], <-c.outgoingMessageIds)

		if c.IsClosed() {
			return
		}

		c.messagesLock.RLock()
		msg := c.messages[msgId]
		if msg == nil {
			c.messagesLock.RUnlock()
			continue
		}
		c.messagesLock.RUnlock()

		err := conn.writeData(msgId[:], msg.Data)
		if err != nil {
			return
		}
	}
}

func (c *tcpChannel) readDaemon(conn *TcpConn) {
	for {
		if c.IsClosed() {
			log.Println("channel is closed")
			return
		}

		msgId, data, err := conn.readData()
		if err != nil {
			//log.Println(err)
			return
		}

		if c.IsClosed() {
			log.Println("channel is closed")
			return
		}

		if msgId.equal(AckId) {
			var id [16]byte
			copy(id[:], data)
			c.messagesLock.RLock()
			if msg, ok := c.messages[id]; msg != nil && ok {
				msg.IsSent <- true
			}
			c.messagesLock.RUnlock()
		} else {
			err = conn.writeData(AckId[:], msgId)
			if err != nil {
				continue
			}
			c.incomingData <- data
		}
	}
}
