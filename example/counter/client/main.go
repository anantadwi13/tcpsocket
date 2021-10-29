package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/anantadwi13/tcpsocket"
)

var (
	res     = make(chan int64, 256)
	timeout = 5 * time.Second
)

func main() {
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	client, err := tcpsocket.Dial("127.0.0.1:10234")
	if err != nil {
		log.Panicln(err)
	}

	channel, err := client.Channel()
	if err != nil {
		log.Panicln(err)
	}

	defer func() {
		_ = client.Shutdown()
	}()

	_, err = channel.AddReadListener(readListener)
	if err != nil {
		log.Panicln(err)
	}

	go func() {
		for {
			var i int64
			fmt.Print("Put any integer: ")
			_, err = fmt.Scanf("%d", &i)
			if err != nil {
				log.Println(err)
				continue
			}
			req := make([]byte, 8)
			binary.LittleEndian.PutUint64(req, uint64(i))
			err = channel.Send(req, &timeout)
			if err != nil {
				log.Println(err)
				continue
			}
			fmt.Println("current: ", <-res)
		}
	}()

	select {
	case <-sigc:
		return
	}
}

func readListener(data []byte, err error) {
	res <- int64(binary.LittleEndian.Uint64(data))
}
