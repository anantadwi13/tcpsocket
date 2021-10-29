package main

import (
	"encoding/binary"
	"log"
	"sync"

	"github.com/anantadwi13/tcpsocket"
)

var (
	counter = int64(0)
	mu      sync.Mutex
	server  *tcpsocket.Server
)

func main() {
	server = tcpsocket.NewServer()

	_, err := server.AddListener(channelListener)
	if err != nil {
		log.Panicln(err)
	}

	defer func() {
		_ = server.Shutdown()
	}()

	err = server.Listen("127.0.0.1:10234")
	if err != nil {
		log.Panicln(err)
	}
}

func channelListener(channel *tcpsocket.TcpChannel, err error) {
	addr, err := server.ChannelIds()
	if err != nil {
		log.Panicln(err)
	}
	log.Println("channel established", channel.ChanId(), addr)
	_, err = channel.AddReadListener(func(data []byte, err error) {
		i := int64(binary.LittleEndian.Uint64(data))
		mu.Lock()
		counter += i
		res := make([]byte, 8)
		binary.LittleEndian.PutUint64(res, uint64(counter))
		log.Println("current: ", counter)
		mu.Unlock()

		err = channel.Send(res, nil)
		if err != nil {
			log.Println(err)
			return
		}
	})
	if err != nil {
		log.Println(err)
		return
	}
}
