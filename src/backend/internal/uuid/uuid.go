package uuid

import (
	"crypto/rand"
	"fmt"
)

const (
	ByteLength = 32
)

// A cryptographically-secure random uuid
type Uuid interface {
	// String returns a string representation of the Uuid
	String() string
}

type uuid struct {
	b []byte
}

func (u *uuid) String() string {
	return fmt.Sprintf("%x", u.b)
}

func NewUuid() (Uuid, error) {
	b := make([]byte, ByteLength)
	_, err := rand.Read(b)
	if err != nil {
		return nil, err
	}
	return &uuid{
		b,
	}, nil
}
