package crc64

import (
	"fmt"
	"hash/crc64"
	"io"
)

var crc64Table *crc64.Table

func init() {
	crc64Table = crc64.MakeTable(crc64.ISO)
}

func Sum(s ...string) uint64 {
	h := crc64.New(crc64Table)
	for _, v := range s {
		io.WriteString(h, v)
	}
	return h.Sum64()
}

func EncodeString(s ...string) string {
	return fmt.Sprintf("%x", Sum(s...))
}
