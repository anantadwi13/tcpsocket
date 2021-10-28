package tcpsocket

import (
	"errors"
	"log"
	"net"
	"sync"
)

type Server struct {
	isRunning     bool
	isRunningLock sync.RWMutex
	tcpChannels   map[string]*tcpChannel
	chanLock      sync.RWMutex
	listener      net.Listener
}

func NewServer() *Server {
	return &Server{tcpChannels: make(map[string]*tcpChannel)}
}

func (s *Server) IsRunning() bool {
	s.isRunningLock.RLock()
	defer func() {
		s.isRunningLock.RUnlock()
	}()
	return s.isRunning
}

func (s *Server) Listen(address string) error {
	listen, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}
	s.listener = listen
	s.isRunningLock.Lock()
	s.isRunning = true
	s.isRunningLock.Unlock()

	for {
		if !s.IsRunning() {
			return nil
		}

		conn, err := listen.Accept()
		if err != nil {
			log.Println(err)
		}

		if !s.IsRunning() {
			conn.Close()
			return nil
		}

		go func(conn net.Conn) {
			defer func() {
				if r := recover(); r != nil {
					log.Println("recover from panic", r)
				}
			}()

			tcpConn := newTcpConn(conn)

			mId, chanIdBytes, err := tcpConn.readData()
			if err != nil {
				log.Panicln(err)
			}

			if !mId.equal(JoinReqId) {
				tcpConn.Close()
				log.Panicln("unable to join, join packet is required, found ", mId)
			}

			chanId := string(chanIdBytes)
			s.chanLock.Lock()
			if s.tcpChannels[chanId] == nil {
				s.tcpChannels[chanId] = newTcpChannel(chanId)
			}
			s.chanLock.Unlock()
			err = s.tcpChannels[chanId].addConn(tcpConn)
			if err != nil {
				tcpConn.Close()
				log.Panicln(err)
			}
			tcpConn.writeData(JoinOkId[:], chanIdBytes)
		}(conn)
	}
}

func (s *Server) Shutdown() error {
	s.isRunningLock.Lock()
	s.isRunning = false
	s.isRunningLock.Unlock()
	s.chanLock.Lock()
	for chanId, channel := range s.tcpChannels {
		channel.Close()
		delete(s.tcpChannels, chanId)
	}
	s.chanLock.Unlock()
	return nil
}

func (s *Server) Address() (string, error) {
	if !s.IsRunning() {
		return "", errors.New("server is not running")
	}
	if s.listener == nil {
		return "", errors.New("server is not listening to port")
	}
	return s.listener.Addr().String(), nil
}

func (s *Server) ChannelIds() ([]string, error) {
	if !s.IsRunning() {
		return nil, errors.New("server is not running")
	}

	s.chanLock.RLock()
	cIds := make([]string, len(s.tcpChannels))
	x := 0
	for chanId := range s.tcpChannels {
		cIds[x] = chanId
		x++
	}
	s.chanLock.RUnlock()
	return cIds, nil
}

func (s *Server) Channel(channelId string) (*tcpChannel, error) {
	if !s.IsRunning() {
		return nil, errors.New("server is not running")
	}

	s.chanLock.RLock()
	defer func() {
		s.chanLock.RUnlock()
	}()
	return s.tcpChannels[channelId], nil
}
