// +build !appengine

package log

import (
	"log"
	"os"
	"strconv"

	"golang.org/x/net/context"
)

var debug bool

func init() {
	if b, err := strconv.ParseBool(os.Getenv(`SHARAQ_DEBUG`)); err == nil {
		debug = b
	}
}

func Debugf(_ context.Context, f string, args ...interface{}) {
	if !debug {
		return
	}

	log.Printf(f, args...)
}

func Infof(_ context.Context, f string, args ...interface{}) {
	log.Printf(f, args...)
}
