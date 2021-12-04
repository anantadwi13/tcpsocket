package tcpsocket

type Event interface {
	Id() Id
	Error() error
}

type EventChanEstablished interface {
	eventChanEstablished()
	Event
	Channel() *TcpChannel
}

type EventChanClosed interface {
	eventChanClosed()
	Event
	Channel() *TcpChannel
}
