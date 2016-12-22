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

	if c.Dispatcher.Listen == "" {
		c.Dispatcher.Listen = "0.0.0.0:9090"
	}
	if c.Guardian.Listen == "" {
		c.Guardian.Listen = "0.0.0.0:9191"
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
	if l := c.Dispatcher.Listen; l[0] == ':' {
		c.Dispatcher.Listen = "0.0.0.0" + l
	}

	if l := c.Guardian.Listen; l[0] == ':' {
		c.Guardian.Listen = "0.0.0.0" + l
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
	if c.Dispatcher.AccessLog != nil {
		applyLogDefaults(c.Dispatcher.AccessLog)
	}
	if c.Guardian.AccessLog != nil {
		applyLogDefaults(c.Guardian.AccessLog)
	}

	return nil
}
