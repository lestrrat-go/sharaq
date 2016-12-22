package urlcache

import "github.com/lestrrat/sharaq/cache"

func newMemcached(c *Config) (*URLCache, error) {
	memd := c.Memcached
	servers := memd.Addr
	expires := c.Expires

	return &URLCache{
		cache:   cache.NewMemcache(servers...),
		expires: expires,
	}, nil
}
