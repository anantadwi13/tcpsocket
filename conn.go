package tcpsocket

import (
	"bufio"
	"encoding/binary"
	"errors"
	"io"
	"net"
)

type tcpConn struct {
	net.Conn
	reader *bufio.Reader
}

func newTcpConn(conn net.Conn) *tcpConn {
	return &tcpConn{Conn: conn, reader: bufio.NewReader(conn)}
}

// Datagram
// 8 byte length (int64) | 16 byte message id | data

//readData returns msgId, data, error
func (t *tcpConn) readData() (Id, []byte, error) {
	lenDataByte := make([]byte, 8)
	n, err := io.ReadFull(t.reader, lenDataByte)
	if err != nil {
		return nil, nil, err
	}
	if n != 8 {
		return nil, nil, errors.New("invalid message length")
	}

	msgId := make([]byte, 16)
	n, err = io.ReadFull(t.reader, msgId)
	if err != nil {
		return nil, nil, err
	}
	if n != 16 {
		return nil, nil, errors.New("invalid length of message id")
	}

	lenData := int64(binary.LittleEndian.Uint64(lenDataByte))
	data := make([]byte, lenData)
	n, err = io.ReadFull(t.reader, data)
	if err != nil {
		return nil, nil, err
	}
	if int64(n) != lenData {
		return nil, nil, errors.New("invalid length of message data")
	}
	return msgId, data, nil
}

func (t *tcpConn) writeData(mId Id, data []byte) error {
	if len(mId) != 16 {
		return errors.New("invalid length of message id")
	}

	lenAndMsgId := make([]byte, 24)
	binary.LittleEndian.PutUint64(lenAndMsgId, uint64(len(data)))
	copy(lenAndMsgId[8:24], mId)
	_, err := t.Write(lenAndMsgId)
	if err != nil {
		return err
	}
	_, err = t.Write(data)
	if err != nil {
		return err
	}
	return nil
}
