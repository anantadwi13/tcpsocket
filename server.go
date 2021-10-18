package tcpsocket

import (
	"errors"
	"log"
	"net"
	"sync"
)

type Server struct {
	isRunning   bool
	runningLock sync.RWMutex
	tcpChannels map[string]*TcpChannel
	chanLock    sync.RWMutex
}

func NewServer() *Server {
	return &Server{tcpChannels: make(map[string]*TcpChannel)}
}

func (s *Server) Listen(address string) error {
	listen, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}
	s.runningLock.Lock()
	s.isRunning = true
	s.runningLock.Unlock()

	for {
		s.runningLock.RLock()
		if !s.isRunning {
			s.runningLock.RUnlock()
			return nil
		}
		s.runningLock.RUnlock()

		conn, err := listen.Accept()
		if err != nil {
			log.Println(err)
		}

		s.runningLock.RLock()
		if !s.isRunning {
			s.runningLock.RUnlock()
			conn.Close()
			return nil
		}
		s.runningLock.RUnlock()

		go func(conn net.Conn) {
			defer func() {
				if r := recover(); r != nil {
					log.Println("recover from panic", r)
				}
			}()

			tcpConn := NewTcpConn(conn)

			chanIdBytes, err := tcpConn.ReadData()
			if err != nil {
				log.Panicln(err)
			}

			chanId := string(chanIdBytes)
			s.chanLock.Lock()
			if s.tcpChannels[chanId] == nil {
				s.tcpChannels[chanId] = NewTcpChannel(chanId)
			}
			s.chanLock.Unlock()
			err = s.tcpChannels[chanId].AddConn(tcpConn)
			if err != nil {
				log.Panicln(err)
			}
		}(conn)
	}
}

func (s *Server) Shutdown() error {
	s.runningLock.Lock()
	s.isRunning = false
	s.runningLock.Unlock()
	s.chanLock.Lock()
	for chanId, channel := range s.tcpChannels {
		channel.Close()
		delete(s.tcpChannels, chanId)
	}
	s.chanLock.Unlock()
	return nil
}

func (s *Server) ChannelIds() ([]string, error) {
	s.runningLock.RLock()
	if !s.isRunning {
		s.runningLock.RUnlock()
		return nil, errors.New("server is not running")
	}
	s.runningLock.RUnlock()

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

func (s *Server) Channel(channelId string) (*TcpChannel, error) {
	s.runningLock.RLock()
	if !s.isRunning {
		s.runningLock.RUnlock()
		return nil, errors.New("server is not running")
	}
	s.runningLock.RUnlock()

	s.chanLock.RLock()
	defer func() {
		s.chanLock.RUnlock()
	}()
	return s.tcpChannels[channelId], nil
}
