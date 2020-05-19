package censor

import (
	"crypto/hmac"
	"crypto/sha256"
	"fmt"
	"hash"
)

const (
	// Minimum key size in bytes
	MinKeySize = 32
)

type censor struct {
	initialized bool
	hmacHash    hash.Hash
}

var (
	defaultCensor = &censor{
		initialized: false,
	}
)

func (c *censor) censor(s string) []byte {
	if !c.initialized {
		panic("call to censor.Censor() before censor.Init()")
	}

	c.hmacHash.Write([]byte(s))
	b := c.hmacHash.Sum(nil)
	c.hmacHash.Reset()
	return b
}

func Init(secret []byte) error {
	if defaultCensor.initialized {
		return fmt.Errorf("already initialized")
	}
	if len(secret) < MinKeySize {
		return fmt.Errorf("secret is %d bytes, but minimum length is %d", len(secret), MinKeySize)
	}
	defaultCensor.hmacHash = hmac.New(sha256.New, secret)
	defaultCensor.initialized = true
	return nil
}

func Censor(s string) []byte {
	return defaultCensor.censor(s)
}
