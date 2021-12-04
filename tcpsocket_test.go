package tcpsocket

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var lipsum = `
Lorem ipsum dolor sit amet, consectetur adipiscing elit. Etiam blandit malesuada justo, sed maximus nibh suscipit vel. Morbi et porta turpis, non condimentum ligula. Vestibulum ante ipsum primis in faucibus orci luctus et ultrices posuere cubilia curae; Phasellus pharetra sit amet arcu vel consectetur. Praesent fringilla ante sit amet tortor imperdiet, in ornare urna condimentum. Morbi mollis aliquam eleifend. Pellentesque habitant morbi tristique senectus et netus et malesuada fames ac turpis egestas. Sed in molestie orci. Aenean sit amet tristique nulla. Vestibulum bibendum consequat ex ut imperdiet. Integer ut odio id velit laoreet lobortis.

Fusce gravida mollis nisi, quis consequat libero placerat a. Quisque placerat sit amet elit varius imperdiet. Quisque vel semper ipsum. Proin non ex lectus. Etiam sit amet tempor nisi. Morbi elementum nulla neque, eget pharetra nulla faucibus at. Sed ut ligula augue. Sed convallis sed nulla eget accumsan. Vestibulum condimentum in diam in scelerisque. Sed sollicitudin tempus magna, in convallis mi semper eu. Aenean cursus arcu tortor, a pharetra arcu lacinia quis. Nam ullamcorper facilisis ligula, vel dictum purus rhoncus sit amet.

Morbi tempor libero rutrum posuere eleifend. Suspendisse non porta magna. Orci varius natoque penatibus et magnis dis parturient montes, nascetur ridiculus mus. Donec condimentum laoreet urna vitae vestibulum. Curabitur placerat tempus urna eget bibendum. Mauris tristique mollis auctor. Nam at nulla ornare, luctus elit quis, luctus ex. Aliquam ut turpis nec libero imperdiet dictum. Praesent felis nibh, pretium eget lacinia vitae, mattis et eros. Nunc mi nunc, hendrerit in vestibulum et, varius ac velit. Integer et accumsan risus. Ut vitae commodo neque. Duis egestas egestas consectetur.

Vivamus porttitor risus a lacus tincidunt, at hendrerit leo varius. Nullam faucibus, arcu at dictum vestibulum, ligula lorem suscipit metus, fermentum scelerisque est enim et urna. Quisque quis metus urna. Maecenas malesuada sem et vulputate fermentum. Aliquam facilisis, massa ac porttitor dictum, magna nisi interdum sem, quis hendrerit diam ante in arcu. Quisque condimentum dictum dui vel tristique. Nulla posuere pellentesque ornare.

Nulla iaculis at dolor aliquam bibendum. Sed arcu tortor, ultrices sit amet convallis id, rutrum in dui. Aliquam quis lectus sagittis, interdum nulla et, lobortis metus. Nulla id neque aliquet, pellentesque justo vel, gravida dolor. Donec condimentum pretium nulla in mattis. Morbi mattis fringilla nibh, a rutrum quam porttitor sit amet. Duis lectus sem, dignissim eget orci nec, lacinia porttitor est. Nunc condimentum, neque at aliquet elementum, nunc justo fringilla justo, eu faucibus enim nisl vitae massa. Etiam ac velit leo. Donec in ultricies tellus, at vestibulum eros. Duis purus ligula, auctor nec mattis pulvinar, efficitur eget quam. Etiam pharetra vitae odio a vehicula. Aliquam porttitor malesuada justo, aliquam tincidunt velit mollis a. Donec vulputate sapien vel lectus varius, sit amet lacinia dolor pharetra.
`

func TestSocket(t *testing.T) {
	sc, cc, server, client, closeFunc, err := initServerClient(t)
	defer closeFunc()
	assert.Nil(t, err)

	serverReceivedData := make(chan []byte, 10)
	clientReceivedData := make(chan []byte, 10)

	_, err = sc.AddReadListener(func(data []byte, err error) {
		serverReceivedData <- data
	})
	assert.Nil(t, err)

	_, err = cc.AddReadListener(func(data []byte, err error) {
		clientReceivedData <- data
	})
	assert.Nil(t, err)

	// Client send

	err = cc.Send([]byte("okeoke"), nil)
	assert.Nil(t, err)

	select {
	case data := <-serverReceivedData:
		assert.Equal(t, data, []byte("okeoke"))
	case <-time.After(10 * time.Second):
		assert.FailNow(t, "timeout")
	}

	// Server send

	for i := 0; i < 10; i++ {
		lipsum += lipsum
	}

	err = sc.Send([]byte(lipsum), nil)
	assert.Nil(t, err)

	select {
	case data := <-clientReceivedData:
		assert.Equal(t, data, []byte(lipsum))
	case <-time.After(10 * time.Second):
		assert.FailNow(t, "timeout")
	}

	// Client send

	err = cc.Send([]byte(lipsum), nil)
	assert.Nil(t, err)

	select {
	case data := <-serverReceivedData:
		assert.Equal(t, data, []byte(lipsum))
	case <-time.After(10 * time.Second):
		assert.FailNow(t, "timeout")
	}

	// Stop server

	_, _ = server, client
	err = server.Shutdown()
	assert.Nil(t, err)

	timeout := 1 * time.Millisecond

	err = cc.Send([]byte(lipsum), &timeout)
	assert.Error(t, err)

	err = sc.Send([]byte(lipsum), &timeout)
	assert.Error(t, err)
}

func TestSocketClosedConnection(t *testing.T) {
	_, cc, server, _, closeFunc, err := initServerClient(t)
	defer closeFunc()
	assert.Nil(t, err)

	serverReceivedData := make(chan []byte, 10)
	clientReceivedData := make(chan []byte, 10)

	_, err = server.AddEventListener(func(event Event) {
		switch e := event.(type) {
		case EventChanEstablished:
			assert.Nil(t, e.Error())

			if e.Channel().ChanId().Equal(cc.ChanId()) {
				_, err = e.Channel().AddReadListener(func(data []byte, err error) {
					serverReceivedData <- data
				})
				assert.Nil(t, err)
			}
		}
	})
	assert.Nil(t, err)

	_, err = cc.AddReadListener(func(data []byte, err error) {
		clientReceivedData <- data
	})
	assert.Nil(t, err)

	// Close connections

	cc.connPoolLock.Lock()
	for conn := range cc.connPool {
		delete(cc.connPool, conn)
		err = conn.Close()
		assert.Nil(t, err)
		cc.closedConn <- conn
	}
	cc.connPoolLock.Unlock()

	// Client send

	timeout := 5 * time.Second

	err = cc.Send([]byte("okeoke"), &timeout)
	assert.Nil(t, err)

	select {
	case data := <-serverReceivedData:
		assert.Equal(t, data, []byte("okeoke"))
	case <-time.After(timeout):
		assert.FailNow(t, "timeout")
	}
}

func BenchmarkSocket1(b *testing.B) {
	sc, _, _, _, closeFunc, err := initServerClient(b)
	defer closeFunc()
	assert.Nil(b, err)

	b.ResetTimer()

	data := []byte(lipsum)

	for i := 0; i < b.N; i++ {
		err = sc.Send(data, nil)
		assert.Nil(b, err)
	}
}

func BenchmarkSocket2(b *testing.B) {
	sc, _, _, _, closeFunc, err := initServerClient(b)
	defer closeFunc()
	assert.Nil(b, err)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		err = sc.Send([]byte{}, nil)
		assert.Nil(b, err)
	}
}

func initServerClient(t assert.TestingT) (
	serverChan, clientChan *TcpChannel, server *Server, client *Client, closeFunc func(), err error,
) {
	server = NewServer()

	go func() {
		err := server.Listen("127.0.0.1:0")
		assert.Nil(t, err, err)
	}()

	tries := 10
	for !server.IsRunning() && tries > 0 {
		tries--
		time.Sleep(10 * time.Millisecond)
	}
	assert.True(t, server.IsRunning())

	serverAddr, err := server.Address()
	assert.Nil(t, err)

	client, err = Dial(serverAddr)
	assert.Nil(t, err)
	assert.NotNil(t, client)

	closeFunc = func() {
		err = client.Shutdown()
		assert.Nil(t, err)
		err = server.Shutdown()
		assert.Nil(t, err)
	}

	chanIds, err := server.ChannelIds()
	tries = 10
	assert.Nil(t, err)
	for len(chanIds) <= 0 && tries > 0 {
		tries--
		chanIds, err = server.ChannelIds()
		assert.Nil(t, err)
		time.Sleep(time.Microsecond)
	}
	assert.NotEmpty(t, chanIds)

	clientChan, err = client.Channel()
	assert.Nil(t, err)
	assert.NotNil(t, clientChan)

	serverChan, err = server.Channel(clientChan.chanId)
	assert.Nil(t, err)
	assert.NotNil(t, serverChan)

	serverChan.connPoolLock.RLock()
	assert.Equal(t, 5, len(serverChan.connPool))
	serverChan.connPoolLock.RUnlock()

	clientChan.connPoolLock.RLock()
	assert.Equal(t, 5, len(clientChan.connPool))
	clientChan.connPoolLock.RUnlock()

	return
}
