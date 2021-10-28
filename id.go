package tcpsocket

import (
	"errors"

	"github.com/google/uuid"
)

type Id []byte
type FixedId [16]byte

func newId() Id {
	id := uuid.New()
	return id[:]
}

func parseId(b []byte) (Id, error) {
	if len(b) != 16 {
		return nil, errors.New("id length is not 16")
	}

	return b, nil
}

func (m Id) Equal(other interface{}) bool {
	o, ok := other.(Id)
	if !ok {
		arrBytes, ok := other.([16]byte)
		if !ok {
			sliceBytes, ok := other.([]byte)
			if !ok {
				return false
			} else {
				o = sliceBytes
			}
		} else {
			o = arrBytes[:]
		}
	}

	if len(o) != len(m) {
		return false
	}

	for i, v := range m {
		if v != o[i] {
			return false
		}
	}

	return true
}

func (m Id) Fixed() (b FixedId) {
	copy(b[:], m)
	return
}
