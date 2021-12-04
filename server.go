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

	eventListener     map[FixedId]EventListener
	eventListenerLock sync.RWMutex
}

func NewServer() *Server {
	return &Server{
		tcpChannels:   map[FixedId]*TcpChannel{},
		eventListener: map[FixedId]EventListener{},
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
			_ = conn.Close()
			return nil
		}

		go func(s *Server, conn net.Conn) {
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
				_ = tcpConn.Close()
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

				s.eventListenerLock.RLock()
				for _, listener := range s.eventListener {
					listener(&eventChanEstablished{
						id:      newId(),
						error:   nil,
						channel: tcpChan,
					})
				}
				s.eventListenerLock.RUnlock()
			} else {
				s.tcpChannelsLock.Unlock()
			}

			err = tcpChan.addConn(tcpConn)
			if err != nil {
				_ = tcpConn.Close()
				log.Panicln(err)
			}
			_ = tcpConn.writeData(JoinOkId[:], chanIdBytes)
		}(s, conn)
	}
}

func (s *Server) AddEventListener(listener EventListener) (listenerId FixedId, err error) {
	listenerId = newId().Fixed()
	s.eventListenerLock.Lock()
	s.eventListener[listenerId] = listener
	s.eventListenerLock.Unlock()
	return
}

func (s *Server) RemoveEventListener(listenerId FixedId) (err error) {
	s.eventListenerLock.Lock()
	if listener, ok := s.eventListener[listenerId]; listener != nil && ok {
		delete(s.eventListener, listenerId)
	} else {
		err = errors.New("readListener not found")
	}
	s.eventListenerLock.Unlock()
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
		_ = channel.Close()
		delete(s.tcpChannels, chanId)
	}
	s.tcpChannelsLock.Unlock()
	s.closeFuncsLock.Lock()
	for _, f := range s.closeFuncs {
		f()
	}
	s.closeFuncs = []closeFunc{}
	s.closeFuncsLock.Unlock()
	s.eventListenerLock.Lock()
	for id := range s.eventListener {
		delete(s.eventListener, id)
	}
	s.eventListenerLock.Unlock()
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
	closeSignal := make(chan bool, 1)

	f = func() {
		closeSignal <- true
	}

	go func(s *Server) {
		defer func() {
			s.eventListenerLock.RLock()
			for _, listener := range s.eventListener {
				listener(&eventChanClosed{
					id:      newId(),
					error:   nil,
					channel: tcpChan,
				})
			}
			s.eventListenerLock.RUnlock()
		}()

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
			case <-closeSignal:
				return
			}
		}
	}(s)

	return
}
