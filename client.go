package tcpsocket

import (
	"errors"
	"net"
	"sync"
)

type Client struct {
	chanId         Id
	address        string
	isRunning      bool
	isRunningLock  sync.RWMutex
	tcpChannel     *TcpChannel
	tcpChannelLock sync.RWMutex
	closeFunc      closeFunc
}

func (c *Client) IsRunning() bool {
	c.isRunningLock.RLock()
	defer func() {
		c.isRunningLock.RUnlock()
	}()
	return c.isRunning
}

func Dial(address string) (c *Client, err error) {
	c = &Client{chanId: newId(), address: address, isRunning: true}

	c.closeFunc = c.closedConnectionHandler()

	defer func() {
		if err != nil {
			c.Shutdown()
		}
	}()

	c.tcpChannelLock.Lock()
	c.tcpChannel = newTcpChannel(c.chanId)
	c.tcpChannelLock.Unlock()

	current := 5
	for current > 0 {
		err = c.createConnection()
		if err != nil {
			return
		}
		current--
	}

	return
}

func (c *Client) createConnection() (err error) {
	var (
		conn  net.Conn
		msgId Id
		data  []byte
	)
	conn, err = net.Dial("tcp", c.address)
	if err != nil {
		return
	}
	tcpConn := newTcpConn(conn)
	err = tcpConn.writeData(JoinReqId[:], c.chanId)
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
	if c.tcpChannel == nil {
		return nil
	}
	err := c.tcpChannel.Close()
	if err != nil {
		return err
	}
	c.closeFunc()
	c.closeFunc = nil
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

func (c *Client) closedConnectionHandler() (f closeFunc) {
	signalChan := make(chan bool, 1)

	f = func() {
		signalChan <- true
	}

	go func() {
		for c.IsRunning() {
			c.tcpChannelLock.RLock()
			tcpChan := c.tcpChannel
			if tcpChan == nil {
				continue
			}
			c.tcpChannelLock.RUnlock()
			select {
			case <-tcpChan.closedConn:
				err := c.createConnection()
				if err != nil {
					//log.Println(err)
					continue
				}
			case <-signalChan:
				return
			}
		}
	}()

	return
}
