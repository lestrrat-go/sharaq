// +build appengine

package util

import (
	"net/http"

	"golang.org/x/net/context"
	"google.golang.org/appengine"
	"google.golang.org/appengine/urlfetch"
)

func RequestCtx(r *http.Request) context.Context {
	return appengine.NewContext(r)
}

func TransportCtx(t http.RoundTripper) context.Context {
	if t2, ok := t.(*urlfetch.Transport); ok {
		return t2.Context
	}
	return context.Background()
}
