// +build appengine

package sharaq

import (
	"os"
	"testing"

	"github.com/kelseyhightower/envconfig"
	envload "github.com/lestrrat/go-envload"
	"github.com/stretchr/testify/assert"
)

func TestConfig(t *testing.T) {
	l := envload.New()
	defer l.Restore()

	var c Config
	if !assert.NoError(t, envconfig.Process("SHARAQ", &c), "envconfig.Process should succeed") {
		return
	}

	os.Setenv("SHARAQ_BACKEND_TYPE", "gcp")
	if !assert.Equal(t, "gcp", c.Backend.Type, "backend should be gcp") {
		return
	}
}
