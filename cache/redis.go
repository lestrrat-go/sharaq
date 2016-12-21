package cache

import (
	"sort"
	"strconv"
	"time"

	cache "gopkg.in/go-redis/cache.v5"
	redis "gopkg.in/redis.v5"
	msgpack "gopkg.in/vmihailenco/msgpack.v2"
)

type Redis struct {
	server *redis.Ring
	codec  *cache.Codec
	prefix string
	magic  string
}

type RedisOption interface {
	Configure(*Redis)
}

type RedisOptionFunc func(*Redis)

func (f RedisOptionFunc) Configure(r *Redis) {
	f(r)
}

func WithPrefix(s string) RedisOption {
	return RedisOptionFunc(func(r *Redis) {
		r.prefix = s
	})
}

func WithMagic(s string) RedisOption {
	return RedisOptionFunc(func(r *Redis) {
		r.magic = s
	})
}

func NewRedis(servers []string, options ...RedisOption) *Redis {
	sort.Strings(servers)

	addrs := make(map[string]string)
	for i := 1; i <= len(servers); i++ {
		addrs["server"+strconv.Itoa(i)] = servers[i-1]
	}

	r := redis.NewRing(&redis.RingOptions{
		Addrs: addrs,
	})
	c := &Redis{
		server: r,
		codec: &cache.Codec{
			Redis: r,
			Marshal: func(v interface{}) ([]byte, error) {
				return msgpack.Marshal(v)
			},
			Unmarshal: func(key []byte, v interface{}) error {
				return msgpack.Unmarshal(key, v)
			},
		},
	}
	for _, o := range options {
		o.Configure(c)
	}
	return c
}

func (c *Redis) Get(key string, v interface{}) error {
	return c.codec.Get(key, v)
}

func (c *Redis) Set(key string, value[]byte, expires int32) error {
	it := cache.Item{
		Key:    key,
		Object: value,
		Expiration: time.Duration(expires) * time.Second,
	}
	return c.codec.Set(&it)
}

func (c *Redis) Delete(key string) error {
	return c.codec.Delete(key)
}
