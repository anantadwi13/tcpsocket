package tcpsocket

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

type TcpChannel struct {
	chanId       string
	connPool     chan *TcpConn
	incomingData chan []byte

	messages map[int]*Message

	isClosed bool
	rwMu     sync.RWMutex
}

func NewTcpChannel(chanId string) *TcpChannel {
	return &TcpChannel{chanId: chanId, connPool: make(chan *TcpConn, 10), incomingData: make(chan []byte, 10)}
}

func (c *TcpChannel) AddConn(conn *TcpConn) error {
	c.rwMu.RLock()
	if c.isClosed {
		c.rwMu.RUnlock()
		return errors.New("channel is closed")
	}
	c.rwMu.RUnlock()

	select {
	case c.connPool <- conn:
	case <-time.After(30 * time.Millisecond):
		return errors.New("connection pool is full")
	}
	go c.readDaemon(conn)
	return nil
}

func (c *TcpChannel) Close() error {
	c.rwMu.Lock()
	c.isClosed = true
	c.rwMu.Unlock()
	close(c.connPool)
	for conn := range c.connPool {
		conn.Close()
	}
	// todo wrap error
	return nil
}

func (c *TcpChannel) Send(data []byte) error {
	c.rwMu.RLock()
	if c.isClosed {
		c.rwMu.RUnlock()
		return errors.New("channel is closed")
	}
	c.rwMu.RUnlock()

	conn := <-c.connPool
	err := conn.WriteData(data)
	c.connPool <- conn
	if err != nil {
		return err
	}
	return nil
}

func (c *TcpChannel) Receive(timeout *time.Duration) ([]byte, error) {
	if timeout == nil {
		return <-c.incomingData, nil
	} else {
		select {
		// todo handle on closing
		case data := <-c.incomingData:
			return data, nil
		case <-time.After(*timeout):
			return nil, errors.New("timeout exceeded")
		}
	}
}

func (c *TcpChannel) readDaemon(conn *TcpConn) {
	for {
		c.rwMu.RLock()
		if c.isClosed {
			c.rwMu.RUnlock()
			return
		}
		c.rwMu.RUnlock()

		data, err := conn.ReadData()
		if err != nil {
			fmt.Println(err)
			//continue
			break
		}
		select {
		case c.incomingData <- data:
		case <-time.After(30 * time.Millisecond):
		}
	}
}
