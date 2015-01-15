package main

import (
	"encoding/json"
	"net/http"
	"os"

	"github.com/Peatix/sharaq"
)

type Config struct {
	OptTransformerURL string `json:"TransformerURL"`
	OptBucketName     string `json:"BucketName"`
	OptAccessKey      string `json:"AccessKey"`
	OptSecretKey      string `json:"SecretKey"`
}

func (c Config) TransformerURL() string { return c.OptTransformerURL }
func (c Config) BucketName() string     { return c.OptBucketName }
func (c Config) AccessKey() string      { return c.OptAccessKey }
func (c Config) SecretKey() string      { return c.OptSecretKey }

func main() {
	os.Exit(_main())
}

func _main() int {
	cfgfile := "etc/guardian.json"
	config := &Config{}

	f, err := os.Open(cfgfile)
	if err != nil {
		return 1
	}
	dec := json.NewDecoder(f)
	if err = dec.Decode(&config); err != nil {
		return 1
	}

	g, err := sharaq.NewGuardian(config)
	if err != nil {
		return 1
	}

	http.ListenAndServe(":9090", g)

	return 0
}