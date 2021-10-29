package tcpsocket

import (
	"errors"
	"log"
	"net"
	"sync"
)

type Server struct {
	isRunning       bool
	isRunningLock   sync.RWMutex
	tcpChannels     map[FixedId]*TcpChannel
	tcpChannelsLock sync.RWMutex
	listener        net.Listener
	closeFuncs      []closeFunc
	closeFuncsLock  sync.Mutex
}

func NewServer() *Server {
	return &Server{
		tcpChannels: make(map[FixedId]*TcpChannel),
	}
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

			if !mId.Equal(JoinReqId) {
				tcpConn.Close()
				log.Panicln("unable to join, join packet is required, found ", mId)
			}

			chanId, err := parseId(chanIdBytes)
			if err != nil {
				log.Panicln(err)
			}
			chanIdFixed := chanId.Fixed()
			s.tcpChannelsLock.Lock()
			if s.tcpChannels[chanIdFixed] == nil {
				s.tcpChannels[chanIdFixed] = newTcpChannel(chanIdBytes)
				s.closeFuncsLock.Lock()
				s.closeFuncs = append(s.closeFuncs, s.closedConnectionHandler(s.tcpChannels[chanIdFixed]))
				s.closeFuncsLock.Unlock()
			}
			s.tcpChannelsLock.Unlock()
			s.tcpChannelsLock.RLock()
			err = s.tcpChannels[chanIdFixed].addConn(tcpConn)
			s.tcpChannelsLock.RUnlock()
			if err != nil {
				tcpConn.Close()
				log.Panicln(err)
			}
			tcpConn.writeData(JoinOkId[:], chanIdBytes)
		}(conn)
	}
}

func (s *Server) IsRunning() bool {
	s.isRunningLock.RLock()
	defer func() {
		s.isRunningLock.RUnlock()
	}()
	return s.isRunning
}

func (s *Server) Shutdown() error {
	s.isRunningLock.Lock()
	s.isRunning = false
	s.isRunningLock.Unlock()
	s.tcpChannelsLock.Lock()
	for chanId, channel := range s.tcpChannels {
		channel.Close()
		delete(s.tcpChannels, chanId)
	}
	s.tcpChannelsLock.Unlock()
	s.closeFuncsLock.Lock()
	for _, f := range s.closeFuncs {
		f()
	}
	s.closeFuncs = []closeFunc{}
	s.closeFuncsLock.Unlock()
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

func (s *Server) ChannelIds() ([]Id, error) {
	if !s.IsRunning() {
		return nil, errors.New("server is not running")
	}

	s.tcpChannelsLock.RLock()
	cIds := make([]Id, len(s.tcpChannels))
	x := 0
	for _, tcpChan := range s.tcpChannels {
		cIds[x] = tcpChan.chanId
		x++
	}
	s.tcpChannelsLock.RUnlock()
	return cIds, nil
}

func (s *Server) Channel(channelId Id) (*TcpChannel, error) {
	if !s.IsRunning() {
		return nil, errors.New("server is not running")
	}

	s.tcpChannelsLock.RLock()
	defer func() {
		s.tcpChannelsLock.RUnlock()
	}()
	return s.tcpChannels[channelId.Fixed()], nil
}

func (s *Server) closedConnectionHandler(tcpChan *TcpChannel) (f closeFunc) {
	signalChan := make(chan bool, 1)

	f = func() {
		signalChan <- true
	}

	go func() {
		for s.IsRunning() {
			select {
			case <-tcpChan.closedConn:
			case <-signalChan:
				return
			}
		}
	}()

	return
}
