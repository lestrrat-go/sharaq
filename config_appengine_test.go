// +build appengine

package sharaq

import (
	"os"
	"testing"

	"github.com/lestrrat/go-config/env"
	envload "github.com/lestrrat/go-envload"
	"github.com/lestrrat/sharaq/gcp"
	"github.com/stretchr/testify/assert"
)

func TestConfig(t *testing.T) {
	l := envload.New()
	defer l.Restore()

	os.Setenv("SHARAQ_BACKEND_TYPE", "gcp")
	os.Setenv("SHARAQ_BACKEND_GCP_BUCKET_NAME", "media")
	os.Setenv("SHARAQ_PRESETS", "small-square=200x200,medium-square=400x400,large-square=600x600")

	var c Config
	if !assert.NoError(t, env.NewDecoder(env.System).Prefix("SHARAQ").Decode(&c), "Decode should succeed") {
		return
	}

	var expected = Config{
		Presets: map[string]string{
			"small-square":  "200x200",
			"medium-square": "400x400",
			"large-square":  "600x600",
		},
		Backend: BackendConfig{
			Type: "gcp",
			Google: gcp.Config{
				BucketName: "media",
			},
		},
	}

	if !assert.Equal(t, &expected, &c, "config matches") {
		return
	}
}
