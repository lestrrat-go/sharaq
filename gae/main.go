// +build appengine

package sharaq

import (
	"net/http"

	"github.com/kelseyhightower/envconfig"
	"github.com/lestrrat/sharaq"
)

func init() {
	var c sharaq.Config
	if err := envconfig.Process("SHARAQ", &c); err != nil {
		panic(err.Error())
	}

	s, err := sharaq.NewServer(&c)
	if err != nil {
		panic(err.Error())
	}
	http.Handle("/", s)
}
