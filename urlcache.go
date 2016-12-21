package sharaq

import (
	"crypto/md5"
	"fmt"
	"io"
)

type cacheBackend interface {
	Get(string, interface{}) error
	Set(string, []byte, int32) error
	Delete(string) error
}

type URLCache struct {
	cache   cacheBackend
	expires int32
}

func MakeCacheKey(v ...string) string {
	h := md5.New()

	for _, x := range v {
		io.WriteString(h, x)
	}
	return fmt.Sprintf("sharaq:urlcache:%x", h.Sum(nil))
}

func (c *URLCache) Lookup(key string) string {
	var s string
	if err := c.cache.Get(key, &s); err == nil {
		return s
	}
	return ""
}

func (c *URLCache) Set(key, value string) error {
	return c.cache.Set(key, []byte(value), c.expires)
}

func (c *URLCache) Delete(key string) error {
	return c.cache.Delete(key)
}
