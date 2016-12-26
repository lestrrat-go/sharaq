// +build !appengine

package util

import (
	"net/http"

	"github.com/lestrrat/sharaq/internal/context"
)

func RequestCtx(r *http.Request) context.Context {
	return r.Context()
}
