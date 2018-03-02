package httputil

import (
	"net/http"

	"github.com/lestrrat-go/sharaq/internal/util"
	"google.golang.org/appengine/log"
)

type redirectContent string

func (s redirectContent) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Debugf(util.RequestCtx(r), "Object %s exists. Redirecting to proper location", string(s))
	w.Header().Add("Location", string(s))
	w.WriteHeader(http.StatusFound)
}

func RedirectContent(u string) http.Handler {
	return redirectContent(u)
}
