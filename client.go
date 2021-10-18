package tcpsocket

import (
	"errors"
	"net"
	"sync"
)

type Client struct {
	clientId    string
	isRunning   bool
	runningLock sync.RWMutex
	tcpChannel  *TcpChannel
	chanLock    sync.RWMutex
}

func NewClient(clientId string) *Client {
	return &Client{clientId: clientId}
}

func (c *Client) Dial(address string) error {
	c.runningLock.RLock()
	if c.isRunning {
		c.runningLock.RUnlock()
		return errors.New("client is already running")
	}
	c.runningLock.RUnlock()

	conn, err := net.Dial("tcp", address)
	if err != nil {
		return err
	}
	c.runningLock.Lock()
	c.isRunning = true
	c.runningLock.Unlock()

	tcpConn := NewTcpConn(conn)
	err = tcpConn.WriteData([]byte(c.clientId))
	if err != nil {
		return err
	}

	c.chanLock.Lock()
	c.tcpChannel = NewTcpChannel(c.clientId)
	c.tcpChannel.AddConn(tcpConn)
	c.chanLock.Unlock()

	return nil
}

func (c *Client) Shutdown() error {
	c.runningLock.Lock()
	c.isRunning = false
	c.runningLock.Unlock()
	c.chanLock.Lock()
	defer func() {
		c.tcpChannel = nil
		c.chanLock.Unlock()
	}()
	err := c.tcpChannel.Close()
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) Channel() (*TcpChannel, error) {
	c.runningLock.RLock()
	if !c.isRunning {
		c.runningLock.RUnlock()
		return nil, errors.New("client is not running")
	}
	c.runningLock.RUnlock()

	c.chanLock.RLock()
	defer func() {
		c.chanLock.RUnlock()
	}()
	return c.tcpChannel, nil
}
