package tcpsocket

import (
	"errors"
	"net"
	"sync"
)

type Client struct {
	clientId      string
	isRunning     bool
	isRunningLock sync.RWMutex
	tcpChannel    *tcpChannel
	chanLock      sync.RWMutex
}

func NewClient(clientId string) *Client {
	return &Client{clientId: clientId}
}

func (c *Client) IsRunning() bool {
	c.isRunningLock.RLock()
	defer func() {
		c.isRunningLock.RUnlock()
	}()
	return c.isRunning
}

func (c *Client) Dial(address string) error {
	if c.IsRunning() {
		return errors.New("client is already running")
	}

	conn, err := net.Dial("tcp", address)
	if err != nil {
		return err
	}
	c.isRunningLock.Lock()
	c.isRunning = true
	c.isRunningLock.Unlock()

	tcpConn := newTcpConn(conn)
	err = tcpConn.writeData(JoinReqId[:], []byte(c.clientId))
	if err != nil {
		return err
	}

	msgId, data, err := tcpConn.readData()
	if err != nil {
		return err
	}

	if !msgId.equal(JoinOkId) || string(data) != c.clientId {
		return errors.New("unable to dial connection")
	}

	c.chanLock.Lock()
	c.tcpChannel = newTcpChannel(c.clientId)
	c.tcpChannel.addConn(tcpConn)
	c.chanLock.Unlock()

	return nil
}

func (c *Client) Shutdown() error {
	c.isRunningLock.Lock()
	c.isRunning = false
	c.isRunningLock.Unlock()
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

func (c *Client) Channel() (*tcpChannel, error) {
	if !c.IsRunning() {
		return nil, errors.New("client is not running")
	}

	c.chanLock.RLock()
	defer func() {
		c.chanLock.RUnlock()
	}()
	return c.tcpChannel, nil
}
