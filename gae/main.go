// +build appengine

package sharaq

import (
	"net/http"

	"github.com/lestrrat/sharaq"
)

func init() {
	s, err := sharaq.NewServer(&sharaq.Config{})
	if err != nil {
		panic(err.Error())
	}
	http.Handle("/", s)
}
