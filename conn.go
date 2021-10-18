package tcpsocket

import (
	"bufio"
	"encoding/binary"
	"errors"
	"io"
	"net"
)

type TcpConn struct {
	net.Conn
	reader *bufio.Reader
}

func NewTcpConn(conn net.Conn) *TcpConn {
	return &TcpConn{Conn: conn, reader: bufio.NewReader(conn)}
}

func (t *TcpConn) ReadData() ([]byte, error) {
	lenDataByte := make([]byte, 8)
	n, err := io.ReadFull(t.reader, lenDataByte)
	if err != nil {
		return nil, err
	}
	if n != 8 {
		return nil, errors.New("invalid packet")
	}
	lenData := int64(binary.LittleEndian.Uint64(lenDataByte))
	data := make([]byte, lenData)
	n, err = io.ReadFull(t.reader, data)
	if err != nil {
		return nil, err
	}
	if int64(n) != lenData {
		return nil, errors.New("invalid length of data")
	}
	return data, nil
}

func (t *TcpConn) WriteData(data []byte) error {
	dLen := make([]byte, 8)
	binary.LittleEndian.PutUint64(dLen, uint64(len(data)))
	_, err := t.Write(dLen)
	if err != nil {
		return err
	}
	_, err = t.Write(data)
	if err != nil {
		return err
	}
	return nil
}
