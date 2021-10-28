package tcpsocket

import (
	"errors"
	"net"
	"sync"
)

type Client struct {
	isRunning      bool
	isRunningLock  sync.RWMutex
	tcpChannel     *TcpChannel
	tcpChannelLock sync.RWMutex
}

func (c *Client) IsRunning() bool {
	c.isRunningLock.RLock()
	defer func() {
		c.isRunningLock.RUnlock()
	}()
	return c.isRunning
}

func Dial(address string) (c *Client, err error) {
	c = &Client{isRunning: true}

	defer func() {
		if err != nil {
			c.Shutdown()
		}
	}()

	chanId := newId()

	c.tcpChannelLock.Lock()
	c.tcpChannel = newTcpChannel(chanId)
	c.tcpChannelLock.Unlock()

	current := 5
	for current > 0 {
		var (
			conn  net.Conn
			msgId Id
			data  []byte
		)
		conn, err = net.Dial("tcp", address)
		if err != nil {
			return
		}
		tcpConn := newTcpConn(conn)
		err = tcpConn.writeData(JoinReqId[:], chanId)
		if err != nil {
			return
		}

		msgId, data, err = tcpConn.readData()
		if err != nil {
			return
		}

		if !msgId.Equal(JoinOkId) || !c.tcpChannel.chanId.Equal(data) {
			err = errors.New("unable to dial connection")
			return
		}
		c.tcpChannelLock.RLock()
		err = c.tcpChannel.addConn(tcpConn)
		c.tcpChannelLock.RUnlock()
		if err != nil {
			return
		}
		current--
	}

	return
}

func (c *Client) Shutdown() error {
	c.isRunningLock.Lock()
	c.isRunning = false
	c.isRunningLock.Unlock()
	c.tcpChannelLock.Lock()
	defer func() {
		c.tcpChannel = nil
		c.tcpChannelLock.Unlock()
	}()
	err := c.tcpChannel.Close()
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) Channel() (*TcpChannel, error) {
	if !c.IsRunning() {
		return nil, errors.New("client is not running")
	}

	c.tcpChannelLock.RLock()
	defer func() {
		c.tcpChannelLock.RUnlock()
	}()
	return c.tcpChannel, nil
}
