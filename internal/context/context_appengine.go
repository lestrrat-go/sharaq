// +build appengine

package context

import (
	"golang.org/x/net/context"
)

type Context interface {
	context.Context
}

func Background() Context {
	return context.Background()
}

func TODO() Context {
	return context.TODO()
}

func WithCancel(ctx Context) (Context, func()) {
	return context.WithCancel(ctx)
}
