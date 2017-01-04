// +build !appengine

package transformer

import (
	"net/http"

	"golang.org/x/net/context"
)

func newClient(ctx context.Context) *http.Client {
	return &http.Client{
		Transport: &TransformingTransport{
			transport: &http.Transport{},
		},
	}
}
