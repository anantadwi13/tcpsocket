package tcpsocket

import (
	"errors"
	"net"
	"sync"
	"time"
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

	defer func() {
		if err != nil {
			c.Shutdown()
		}
	}()

	c.tcpChannelLock.Lock()
	c.tcpChannel = newTcpChannel(c.chanId)
	c.closeFunc = c.closedConnectionHandler(c.tcpChannel)
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
	if !c.IsRunning() {
		return nil
	}

	c.isRunningLock.Lock()
	c.isRunning = false
	c.isRunningLock.Unlock()

	c.tcpChannelLock.RLock()
	if tcpChan := c.tcpChannel; tcpChan != nil {
		c.tcpChannelLock.RUnlock()
		err := tcpChan.Close()
		if err != nil {
			return err
		}
	} else {
		c.tcpChannelLock.RUnlock()
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

func (c *Client) closedConnectionHandler(tcpChan *TcpChannel) (f closeFunc) {
	signalChan := make(chan bool, 1)

	f = func() {
		signalChan <- true
	}

	go func() {
		for c.IsRunning() {
			select {
			case closedConn := <-tcpChan.closedConn:
				err := c.createConnection()
				if err != nil {
					//log.Println(err)
					tcpChan.closedConn <- closedConn
					time.Sleep(100 * time.Millisecond)
					continue
				}
			case <-signalChan:
				return
			}
		}
	}()

	return
}
