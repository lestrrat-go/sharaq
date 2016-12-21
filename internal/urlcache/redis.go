// +build !memcache

package urlcache

import "github.com/lestrrat/sharaq/cache"

type ConfigSource interface {
	RedisAddr() []string
	URLCacheExpires() int32
}

func New(c ConfigSource) *URLCache {
	servers := c.RedisAddr()
	expires := c.URLCacheExpires()
	return &URLCache{
		cache:   cache.NewRedis(servers),
		expires: expires,
	}
}
