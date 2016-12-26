// +build appengine

package util

import (
	"net/http"

	"github.com/lestrrat/sharaq/internal/context"
	"google.golang.org/appengine"
)

func RequestCtx(r *http.Request) context.Context {
	return appengine.NewContext(r)
}
