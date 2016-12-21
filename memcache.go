// +build memcache

package sharaq

import "github.com/lestrrat/sharaq/cache"

func NewURLCache(s *Server) *URLCache {
	servers := s.config.MemcachedAddr()
	expires := s.config.URLCacheExpires()

	return &URLCache{
		cache:   cache.NewMemcache(servers...),
		expires: expires,
	}
}
