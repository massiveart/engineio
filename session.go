package engineio

import (
	"crypto/rand"
	"crypto/sha1"
	"fmt"
)

func newSessionId() string {
	buf := make([]byte, 20)
	rand.Read(buf)
	hash := sha1.New()
	hash.Write(buf)
	return string(fmt.Sprintf("%x", hash.Sum(nil)))
}
