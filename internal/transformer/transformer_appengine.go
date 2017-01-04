// +build appengine

package transformer

import (
	"net/http"

	"golang.org/x/net/context"
	"google.golang.org/appengine/urlfetch"
)

func newClient(ctx context.Context) *http.Client {
	return &http.Client{
		Transport: &TransformingTransport{
			transport: &urlfetch.Transport{Context: ctx},
		},
	}
}
