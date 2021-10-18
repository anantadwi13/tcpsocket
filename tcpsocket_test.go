package tcpsocket

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

var lipsum = `
Lorem ipsum dolor sit amet, consectetur adipiscing elit. Etiam blandit malesuada justo, sed maximus nibh suscipit vel. Morbi et porta turpis, non condimentum ligula. Vestibulum ante ipsum primis in faucibus orci luctus et ultrices posuere cubilia curae; Phasellus pharetra sit amet arcu vel consectetur. Praesent fringilla ante sit amet tortor imperdiet, in ornare urna condimentum. Morbi mollis aliquam eleifend. Pellentesque habitant morbi tristique senectus et netus et malesuada fames ac turpis egestas. Sed in molestie orci. Aenean sit amet tristique nulla. Vestibulum bibendum consequat ex ut imperdiet. Integer ut odio id velit laoreet lobortis.

Fusce gravida mollis nisi, quis consequat libero placerat a. Quisque placerat sit amet elit varius imperdiet. Quisque vel semper ipsum. Proin non ex lectus. Etiam sit amet tempor nisi. Morbi elementum nulla neque, eget pharetra nulla faucibus at. Sed ut ligula augue. Sed convallis sed nulla eget accumsan. Vestibulum condimentum in diam in scelerisque. Sed sollicitudin tempus magna, in convallis mi semper eu. Aenean cursus arcu tortor, a pharetra arcu lacinia quis. Nam ullamcorper facilisis ligula, vel dictum purus rhoncus sit amet.

Morbi tempor libero rutrum posuere eleifend. Suspendisse non porta magna. Orci varius natoque penatibus et magnis dis parturient montes, nascetur ridiculus mus. Donec condimentum laoreet urna vitae vestibulum. Curabitur placerat tempus urna eget bibendum. Mauris tristique mollis auctor. Nam at nulla ornare, luctus elit quis, luctus ex. Aliquam ut turpis nec libero imperdiet dictum. Praesent felis nibh, pretium eget lacinia vitae, mattis et eros. Nunc mi nunc, hendrerit in vestibulum et, varius ac velit. Integer et accumsan risus. Ut vitae commodo neque. Duis egestas egestas consectetur.

Vivamus porttitor risus a lacus tincidunt, at hendrerit leo varius. Nullam faucibus, arcu at dictum vestibulum, ligula lorem suscipit metus, fermentum scelerisque est enim et urna. Quisque quis metus urna. Maecenas malesuada sem et vulputate fermentum. Aliquam facilisis, massa ac porttitor dictum, magna nisi interdum sem, quis hendrerit diam ante in arcu. Quisque condimentum dictum dui vel tristique. Nulla posuere pellentesque ornare.

Nulla iaculis at dolor aliquam bibendum. Sed arcu tortor, ultrices sit amet convallis id, rutrum in dui. Aliquam quis lectus sagittis, interdum nulla et, lobortis metus. Nulla id neque aliquet, pellentesque justo vel, gravida dolor. Donec condimentum pretium nulla in mattis. Morbi mattis fringilla nibh, a rutrum quam porttitor sit amet. Duis lectus sem, dignissim eget orci nec, lacinia porttitor est. Nunc condimentum, neque at aliquet elementum, nunc justo fringilla justo, eu faucibus enim nisl vitae massa. Etiam ac velit leo. Donec in ultricies tellus, at vestibulum eros. Duis purus ligula, auctor nec mattis pulvinar, efficitur eget quam. Etiam pharetra vitae odio a vehicula. Aliquam porttitor malesuada justo, aliquam tincidunt velit mollis a. Donec vulputate sapien vel lectus varius, sit amet lacinia dolor pharetra.
`

func TestSocket(t *testing.T) {
	server := NewServer()
	client := NewClient("id-client-1")

	go func() {
		err := server.Listen("127.0.0.1:9191")
		assert.Nil(t, err)
	}()

	for !server.isRunning {
		time.Sleep(time.Microsecond)
	}

	err := client.Dial("127.0.0.1:9191")
	assert.Nil(t, err)

	defer func() {
		server.Shutdown()
		client.Shutdown()
	}()

	chanIds, err := server.ChannelIds()
	assert.Nil(t, err)
	for len(chanIds) <= 0 {
		chanIds, err = server.ChannelIds()
		assert.Nil(t, err)
		time.Sleep(time.Microsecond)
	}

	clientChan, err := client.Channel()
	assert.Nil(t, err)
	assert.NotNil(t, clientChan)

	serverChan, err := server.Channel("id-client-1")
	assert.Nil(t, err)
	assert.NotNil(t, serverChan)

	// Client send

	err = clientChan.Send([]byte("okeoke"))
	assert.Nil(t, err)

	timeout := 10 * time.Second
	receive, err := serverChan.Receive(&timeout)
	assert.Nil(t, err)

	assert.Equal(t, receive, []byte("okeoke"))

	// Server send

	for i := 0; i < 10; i++ {
		lipsum += lipsum
	}

	err = serverChan.Send([]byte(lipsum))
	assert.Nil(t, err)

	bytes, err := clientChan.Receive(nil)
	assert.Nil(t, err)

	assert.Equal(t, bytes, []byte(lipsum))

	// Stop server

	err = server.Shutdown()
	assert.Nil(t, err)

	//time.Sleep(time.Second)
	//
	//err = clientChan.Send([]byte(lipsum))
	//assert.Nil(t, err)
	//
	//err = clientChan.Send([]byte(lipsum))
	//assert.Nil(t, err)
}
