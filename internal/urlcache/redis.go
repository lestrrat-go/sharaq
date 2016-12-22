package urlcache

import "github.com/lestrrat/sharaq/cache"

func newRedis(c *Config) (*URLCache, error) {
	redis := c.Redis
	servers := redis.Addr
	expires := c.Expires
	return &URLCache{
		cache:   cache.NewRedis(servers),
		expires: expires,
	}, nil
}
