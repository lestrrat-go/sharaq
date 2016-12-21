// +build memcache

package sharaq

func NewURLCache(s *Server) *URLCache {
	servers := s.config.MemcachedAddr()
	expires := s.config.URLCacheExpires()

	return &URLCache{
		cache:   cache.NewMemcache(servers...),
		expires: expires,
	}
}
