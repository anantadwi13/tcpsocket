package tcpsocket

type eventChanEstablished struct {
	id      Id
	error   error
	channel *TcpChannel
}

func (e *eventChanEstablished) eventChanEstablished() {

}

func (e *eventChanEstablished) Id() Id {
	return e.id
}

func (e *eventChanEstablished) Error() error {
	return e.error
}

func (e *eventChanEstablished) Channel() *TcpChannel {
	return e.channel
}

type eventChanClosed struct {
	id      Id
	error   error
	channel *TcpChannel
}

func (e *eventChanClosed) eventChanClosed() {

}

func (e *eventChanClosed) Id() Id {
	return e.id
}

func (e *eventChanClosed) Error() error {
	return e.error
}

func (e *eventChanClosed) Channel() *TcpChannel {
	return e.channel
}
