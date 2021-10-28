package tcpsocket

import (
	"errors"
	"log"
	"sync"
	"time"
)

type TcpChannel struct {
	chanId             Id
	incomingData       chan []byte
	outgoingMessageIds chan []byte

	connPool     map[*tcpConn]*tcpConn
	connPoolLock sync.RWMutex

	messages     map[FixedId]*message
	messagesLock sync.RWMutex

	listener     map[FixedId]ChannelListener
	listenerLock sync.RWMutex

	isClosed     bool
	isClosedLock sync.RWMutex
}

type ChannelListener func(data []byte, err error)

func newTcpChannel(chanId Id) *TcpChannel {
	c := &TcpChannel{
		chanId:             chanId,
		incomingData:       make(chan []byte, 256),
		outgoingMessageIds: make(chan []byte, 256),
		connPool:           map[*tcpConn]*tcpConn{},
		messages:           map[FixedId]*message{},
		listener:           map[FixedId]ChannelListener{},
	}
	go c.receiverDaemon()
	return c
}

func (c *TcpChannel) addConn(conn *tcpConn) error {
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

func (c *TcpChannel) AddListener(l ChannelListener) (listenerId FixedId, err error) {
	if c.IsClosed() {
		err = errors.New("channel is closed")
		return
	}

	listenerId = newId().Fixed()
	c.listenerLock.Lock()
	c.listener[listenerId] = l
	c.listenerLock.Unlock()
	return
}

func (c *TcpChannel) RemoveListener(listenerId FixedId) (err error) {
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

func (c *TcpChannel) Close() error {
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

func (c *TcpChannel) IsClosed() bool {
	c.isClosedLock.RLock()
	defer func() {
		c.isClosedLock.RUnlock()
	}()
	return c.isClosed
}

func (c *TcpChannel) Send(data []byte, timeout *time.Duration) error {
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
	c.messages[msg.Id.Fixed()] = msg
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

func (c *TcpChannel) receiverDaemon() {
	for !c.IsClosed() {
		data := <-c.incomingData
		c.listenerLock.RLock()
		for _, listener := range c.listener {
			go listener(data, nil)
		}
		c.listenerLock.RUnlock()
	}
}

func (c *TcpChannel) writeDaemon(conn *tcpConn) {
	for {
		if c.IsClosed() {
			return
		}

		var msgId FixedId
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

func (c *TcpChannel) readDaemon(conn *tcpConn) {
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

		if msgId.Equal(AckId) {
			var id FixedId
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
