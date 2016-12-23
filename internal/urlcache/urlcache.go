package urlcache

import (
	"context"
	"crypto/md5"
	"fmt"
	"io"

	"github.com/lestrrat/sharaq/cache"
	"github.com/pkg/errors"
)

type cacheBackend interface {
	Get(context.Context, string, interface{}) error
	Set(context.Context, string, []byte, int32) error
	Delete(context.Context, string) error
}

type URLCache struct {
	cache   cacheBackend
	expires int32
}

type Config struct {
	BackendType string
	Memcached   cache.MemcacheConfig
	Redis       cache.RedisConfig
	Expires     int32
}

func New(c *Config) (*URLCache, error) {
	switch c.BackendType {
	case "Redis":
		return newRedis(c)
	case "Memcached":
		return newMemcached(c)
	default:
		return nil, errors.Errorf(`urlcache: unknown backend type "%s"`, c.BackendType)
	}
}

func MakeCacheKey(v ...string) string {
	h := md5.New()

	for _, x := range v {
		io.WriteString(h, x)
	}
	return fmt.Sprintf("sharaq:urlcache:%x", h.Sum(nil))
}

func (c *URLCache) Lookup(ctx context.Context, key string) string {
	var s string
	if err := c.cache.Get(ctx, key, &s); err == nil {
		return s
	}
	return ""
}

func (c *URLCache) Set(ctx context.Context, key, value string) error {
	return c.cache.Set(ctx, key, []byte(value), c.expires)
}

func (c *URLCache) Delete(ctx context.Context, key string) error {
	return c.cache.Delete(ctx, key)
}
