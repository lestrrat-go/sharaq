// +build appengine

package sharaq

import (
	"os"
	"testing"

	"github.com/lestrrat/go-config/env"
	envload "github.com/lestrrat/go-envload"
	"github.com/lestrrat/sharaq/gcp"
	"github.com/lestrrat/sharaq/internal/urlcache"
	"github.com/stretchr/testify/assert"
)

func TestConfig(t *testing.T) {
	l := envload.New()
	defer l.Restore()

	os.Setenv("SHARAQ_BACKEND_TYPE", "gcp")
	os.Setenv("SHARAQ_BACKEND_GCP_BUCKET_NAME", "media")
	os.Setenv("SHARAQ_PRESETS", "small-square=200x200,medium-square=400x400,large-square=600x600")
	os.Setenv("SHARAQ_TOKENS", "token1,token2,token3")
	os.Setenv("SHARAQ_URLCACHE_TYPE", "Redis")

	var c Config
	if !assert.NoError(t, env.NewDecoder(env.System).Prefix("SHARAQ").Decode(&c), "Decode should succeed") {
		return
	}

	var expected = Config{
		Tokens: []string{"token1", "token2", "token3"},
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
		URLCache: &urlcache.Config{
			Type: "Redis",
		},
	}

	if !assert.Equal(t, &expected, &c, "config matches") {
		return
	}
}
