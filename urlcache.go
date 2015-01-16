package sharaq

import (
	"crypto/md5"
	"fmt"
	"io"

	"github.com/bradfitz/gomemcache/memcache"
)

type URLCache struct {
	Memcache *memcache.Client
}

func NewURLCache(servers ...string) *URLCache {
	return &URLCache{
		Memcache: memcache.New(servers...),
	}
}

func MakeCacheKey(v ...string) string {
	h := md5.New()

	for _, x := range v {
		io.WriteString(h, x)
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

func (c *URLCache) Lookup(key string) string {
	if it, err := c.Memcache.Get(key); err == nil {
		if len(it.Value) > 0 {
			return string(it.Value)
		}
	}
	return ""
}

func (c *URLCache) Set(key, value string) {
	c.Memcache.Set(&memcache.Item{Key: key, Value: []byte(value)})
}

func (c *URLCache) Delete(key string) {
	c.Memcache.Delete(key)
}
