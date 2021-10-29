package tcpsocket

import (
	"errors"
	"log"
	"sync"
	"time"
)

type TcpChannel struct {
	chanId                Id
	incomingData          chan []byte
	outgoingMessageIds    chan []byte
	rescheduledMessageIds chan []byte

	connPool     map[*tcpConn]*tcpConn
	connPoolLock sync.RWMutex
	closedConn   chan *tcpConn

	messages     map[FixedId]*message
	messagesLock sync.RWMutex

	readListener     map[FixedId]ChannelReadListener
	readListenerLock sync.RWMutex

	isClosed            bool
	isClosedLock        sync.RWMutex
	receiverCloseFunc   closeFunc
	writeCloseFuncs     map[*tcpConn]closeFunc
	writeCloseFuncsLock sync.Mutex
}

type ChannelListener func(channel *TcpChannel, err error)
type ChannelReadListener func(data []byte, err error)
type closeFunc func()

func newTcpChannel(chanId Id) *TcpChannel {
	c := &TcpChannel{
		chanId:                chanId,
		incomingData:          make(chan []byte, 256),
		outgoingMessageIds:    make(chan []byte, 256),
		rescheduledMessageIds: make(chan []byte, 256),
		connPool:              map[*tcpConn]*tcpConn{},
		closedConn:            make(chan *tcpConn, 256),
		messages:              map[FixedId]*message{},
		readListener:          map[FixedId]ChannelReadListener{},
		writeCloseFuncs:       map[*tcpConn]closeFunc{},
	}
	c.receiverCloseFunc = c.receiverDaemon()
	return c
}

func (c *TcpChannel) addConn(conn *tcpConn) error {
	if c.IsClosed() {
		return errors.New("channel is closed")
	}

	c.connPoolLock.Lock()
	c.connPool[conn] = conn
	c.connPoolLock.Unlock()

	c.writeCloseFuncsLock.Lock()
	c.writeCloseFuncs[conn] = c.writeDaemon(conn)
	c.writeCloseFuncsLock.Unlock()
	c.readDaemon(conn)
	return nil
}

func (c *TcpChannel) ChanId() Id {
	return c.chanId
}

func (c *TcpChannel) AddReadListener(l ChannelReadListener) (listenerId FixedId, err error) {
	if c.IsClosed() {
		err = errors.New("channel is closed")
		return
	}

	listenerId = newId().Fixed()
	c.readListenerLock.Lock()
	c.readListener[listenerId] = l
	c.readListenerLock.Unlock()
	return
}

func (c *TcpChannel) RemoveReadListener(listenerId FixedId) (err error) {
	if c.IsClosed() {
		return errors.New("channel is closed")
	}

	c.readListenerLock.Lock()
	if listener, ok := c.readListener[listenerId]; listener != nil && ok {
		delete(c.readListener, listenerId)
	} else {
		err = errors.New("readListener not found")
	}
	c.readListenerLock.Unlock()
	return
}

func (c *TcpChannel) Close() error {
	if c.IsClosed() {
		return nil
	}

	c.isClosedLock.Lock()
	c.isClosed = true
	c.isClosedLock.Unlock()

	c.readListenerLock.Lock()
	for id := range c.readListener {
		delete(c.readListener, id)
	}
	c.readListenerLock.Unlock()

	c.connPoolLock.Lock()
	for conn := range c.connPool {
		delete(c.connPool, conn)
		conn.Close()
	}
	c.connPoolLock.Unlock()

	c.writeCloseFuncsLock.Lock()
	for conn, f := range c.writeCloseFuncs {
		f()
		delete(c.writeCloseFuncs, conn)
	}
	c.writeCloseFuncsLock.Unlock()

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
	msg.data = data

	c.messagesLock.Lock()
	c.messages[msg.id.Fixed()] = msg
	c.messagesLock.Unlock()

	c.outgoingMessageIds <- msg.id

	timeoutAfter := time.After(*timeout)
	for {
		select {
		case isSent := <-msg.isSent:
			if isSent {
				c.messagesLock.Lock()
				delete(c.messages, msg.id.Fixed())
				c.messagesLock.Unlock()
				return nil
			}
		case <-timeoutAfter:
			c.messagesLock.Lock()
			delete(c.messages, msg.id.Fixed())
			c.messagesLock.Unlock()
			return errors.New("timeout")
		}
	}
}

func (c *TcpChannel) receiverDaemon() (f closeFunc) {
	signalChan := make(chan bool, 1)

	f = func() {
		signalChan <- true
	}

	go func() {
		for !c.IsClosed() {
			select {
			case data := <-c.incomingData:
				c.readListenerLock.RLock()
				for _, listener := range c.readListener {
					go listener(data, nil)
				}
				c.readListenerLock.RUnlock()
			case <-signalChan:
				return
			}
		}
	}()

	return
}

func (c *TcpChannel) writeDaemon(conn *tcpConn) (f closeFunc) {
	signalChan := make(chan bool, 1)

	f = func() {
		signalChan <- true
	}

	go func() {
		defer func() {
			if c.IsClosed() {
				return
			}

			c.connPoolLock.Lock()
			if _, ok := c.connPool[conn]; ok {
				delete(c.connPool, conn)
				conn.Close()
				c.closedConn <- conn
			}
			c.connPoolLock.Unlock()
		}()
		for {
			if c.IsClosed() {
				return
			}

			var (
				msgId FixedId
				mId   []byte
			)

			select {
			case mId = <-c.rescheduledMessageIds:
				copy(msgId[:], mId)
			case mId = <-c.outgoingMessageIds:
				copy(msgId[:], mId)
			case <-signalChan:
				return
			}

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

			err := conn.writeData(msgId[:], msg.data)
			if err != nil {
				c.rescheduledMessageIds <- msgId[:]
				return
			}
		}
	}()

	return
}

func (c *TcpChannel) readDaemon(conn *tcpConn) {
	go func() {
		defer func() {
			if c.IsClosed() {
				return
			}

			c.connPoolLock.Lock()
			if _, ok := c.connPool[conn]; ok {
				delete(c.connPool, conn)
				conn.Close()
				c.closedConn <- conn
			}
			c.connPoolLock.Unlock()
		}()
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

			switch {
			case msgId.Equal(AckId):
				var id FixedId
				copy(id[:], data)
				c.messagesLock.RLock()
				if msg, ok := c.messages[id]; msg != nil && ok {
					msg.isSent <- true
				}
				c.messagesLock.RUnlock()
			default:
				err = conn.writeData(AckId[:], msgId)
				if err != nil {
					continue
				}
				c.incomingData <- data
			}
		}
	}()
}
