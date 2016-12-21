// +build memcache

package urlcache

import "github.com/lestrrat/sharaq/cache"

type ConfigSource interface {
	MemcachedAddr() []string
	URLCacheExpires() int32
}

func New(c ConfigSource) *URLCache {
	servers := c.MemcachedAddr()
	expires := c.URLCacheExpires()

	return &URLCache{
		cache:   cache.NewMemcache(servers...),
		expires: expires,
	}
}
