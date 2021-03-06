// +build !appengine

package util

import (
	"net/http"

	"golang.org/x/net/context"
)

func RequestCtx(r *http.Request) context.Context {
	return r.Context()
}

func TransportCtx(t http.RoundTripper) context.Context {
	return context.Background()
}
