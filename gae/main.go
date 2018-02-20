// +build appengine

package sharaq

import (
	"net/http"

	"github.com/lestrrat/go-config/env"
	"github.com/lestrrat-go/sharaq"
)

func init() {
	var c sharaq.Config
	if err := env.NewDecoder(env.System).Prefix("SHARAQ").Decode(&c); err != nil {
		panic(err.Error())
	}

	s, err := sharaq.NewServer(&c)
	if err != nil {
		panic(err.Error())
	}

	if err := s.Initialize(); err != nil {
		panic(err.Error())
	}

	http.Handle("/", s)
}
