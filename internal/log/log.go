// +build !appengine

package log

import (
	"log"

	"golang.org/x/net/context"
)

func Debugf(_ context.Context, f string, args ...interface{}) {
	log.Printf(f, args...)
}
