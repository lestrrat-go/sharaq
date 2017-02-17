// +build appengine

package log

import (
	"golang.org/x/net/context"
	"google.golang.org/appengine/log"
)

func Debugf(ctx context.Context, format string, args ...interface{}) {
	log.Debugf(ctx, format, args...)
}

func Infof(ctx context.Context, format string, args ...interface{}) {
	log.Infof(ctx, format, args...)
}
