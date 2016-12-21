// +build !memcache

package sharaq

import "github.com/lestrrat/sharaq/cache"

func NewURLCache(s *Server) *URLCache {
	servers := s.config.RedisAddr()
	expires := s.config.URLCacheExpires()

	return &URLCache{
		cache:   cache.NewRedis(servers),
		expires: expires,
	}
}
