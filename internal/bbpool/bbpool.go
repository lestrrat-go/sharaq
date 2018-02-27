package bbpool

import (
	"bytes"

	bufferpool "github.com/lestrrat-go/bufferpool"
)

var pool = bufferpool.New()

func Get() *bytes.Buffer {
	return pool.Get()
}

func Release(buf *bytes.Buffer) {
	pool.Release(buf)
}
