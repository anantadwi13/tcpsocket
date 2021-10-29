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

	channelListener     map[FixedId]ChannelListener
	channelListenerLock sync.RWMutex
}

func NewServer() *Server {
	return &Server{
		tcpChannels:     map[FixedId]*TcpChannel{},
		channelListener: map[FixedId]ChannelListener{},
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
				log.Panicln("error read channel id", err)
			}

			if !mId.Equal(JoinReqId) {
				tcpConn.Close()
				log.Panicln("unable to join, join packet is required, found ", mId)
			}

			chanId, err := parseId(chanIdBytes)
			if err != nil {
				log.Panicln(err)
			}

			var (
				chanIdFixed = chanId.Fixed()
				tcpChan     *TcpChannel
			)

			s.tcpChannelsLock.Lock()
			if tcpChan = s.tcpChannels[chanIdFixed]; tcpChan == nil {
				tcpChan = newTcpChannel(chanIdBytes)
				s.tcpChannels[chanIdFixed] = tcpChan
				s.tcpChannelsLock.Unlock()

				s.closeFuncsLock.Lock()
				s.closeFuncs = append(s.closeFuncs, s.closedConnectionHandler(tcpChan))
				s.closeFuncsLock.Unlock()

				s.channelListenerLock.RLock()
				for _, listener := range s.channelListener {
					listener(tcpChan, nil)
				}
				s.channelListenerLock.RUnlock()
			} else {
				s.tcpChannelsLock.Unlock()
			}

			err = tcpChan.addConn(tcpConn)
			if err != nil {
				tcpConn.Close()
				log.Panicln(err)
			}
			tcpConn.writeData(JoinOkId[:], chanIdBytes)
		}(conn)
	}
}

func (s *Server) AddListener(listener ChannelListener) (listenerId FixedId, err error) {
	listenerId = newId().Fixed()
	s.channelListenerLock.Lock()
	s.channelListener[listenerId] = listener
	s.channelListenerLock.Unlock()
	return
}

func (s *Server) RemoveListener(listenerId FixedId) (err error) {
	s.channelListenerLock.Lock()
	if listener, ok := s.channelListener[listenerId]; listener != nil && ok {
		delete(s.channelListener, listenerId)
	} else {
		err = errors.New("readListener not found")
	}
	s.channelListenerLock.Unlock()
	return
}

func (s *Server) IsRunning() bool {
	s.isRunningLock.RLock()
	defer func() {
		s.isRunningLock.RUnlock()
	}()
	return s.isRunning
}

func (s *Server) Shutdown() error {
	if !s.IsRunning() {
		return nil
	}

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
	s.channelListenerLock.Lock()
	for id := range s.channelListener {
		delete(s.channelListener, id)
	}
	s.channelListenerLock.Unlock()
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
				tcpChan.connPoolLock.RLock()
				if len(tcpChan.connPool) <= 0 {
					tcpChan.connPoolLock.RUnlock()
					s.tcpChannelsLock.Lock()
					delete(s.tcpChannels, tcpChan.chanId.Fixed())
					_ = tcpChan.Close()
					s.tcpChannelsLock.Unlock()
					return
				} else {
					tcpChan.connPoolLock.RUnlock()
				}
			case <-signalChan:
				return
			}
		}
	}()

	return
}
