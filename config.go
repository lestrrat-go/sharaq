package sharaq

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/lestrrat/sharaq/internal/urlcache"
)

func (c *Config) ParseFile(f string) error {
	fh, err := os.Open(f)
	if err != nil {
		return err
	}
	defer fh.Close()

	c.filename = f
	return c.Parse(fh)
}

func (c *Config) Parse(rdr io.Reader) error {
	dec := json.NewDecoder(rdr)
	if err := dec.Decode(c); err != nil {
		return err
	}

	if len(c.Presets) == 0 {
		return fmt.Errorf("error: Presets is empty")
	}

	if c.Listen == "" {
		c.Listen = "0.0.0.0:9090"
	}

	if c.URLCache == nil {
		c.URLCache = &urlcache.Config{}
	}

	if c.URLCache.BackendType == "" {
		c.URLCache.BackendType = "Redis"
	}

	switch c.URLCache.BackendType {
	case "Redis":
		if len(c.URLCache.Redis.Addr) < 1 {
			c.URLCache.Redis.Addr = []string{"127.0.0.1:6379"}
		}
	case "Memcached":
		if len(c.URLCache.Memcached.Addr) < 1 {
			c.URLCache.Memcached.Addr = []string{"127.0.0.1:11211"}
		}
	}

	// Normalize shorthand form to full form
	if l := c.Listen; l[0] == ':' {
		c.Listen = "0.0.0.0" + l
	}

	applyLogDefaults := func(c *LogConfig) {
		if c.RotationTime <= 0 {
			// 1 day
			c.RotationTime = 24 * time.Hour
		}
		if c.MaxAge <= 0 {
			// 30 days
			c.MaxAge = 30 * 24 * time.Hour
		}
	}
	/*
		if c.ErrorLog != nil {
			applyLogDefaults(c.ErrorLog)
		}
	*/
	if c.AccessLog != nil {
		applyLogDefaults(c.AccessLog)
	}

	return nil
}
