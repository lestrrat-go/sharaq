// +build appengine

package util

import (
	"net/http"

	"golang.org/x/net/context"
	"google.golang.org/appengine"
)

func RequestCtx(r *http.Request) context.Context {
	return appengine.NewContext(r)
}
