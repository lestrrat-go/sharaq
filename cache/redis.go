package cache

import (
	"sort"
	"strconv"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/net/context"

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

type RedisConfig struct {
	Addr []string
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

func (c *Redis) Get(_ context.Context, key string, v interface{}) error {
	return c.codec.Get(key, v)
}

func (c *Redis) Set(_ context.Context, key string, value []byte, expires int32) error {
	it := cache.Item{
		Key:        key,
		Object:     value,
		Expiration: time.Duration(expires) * time.Second,
	}
	return c.codec.Set(&it)
}

func (c *Redis) SetNX(_ context.Context, key string, value []byte, expires int32) error {
	ok, err := c.server.SetNX(key, value, time.Second*time.Duration(expires)).Result()
	if err != nil {
		return err
	}
	if !ok {
		return errors.New(`redis: setNX failed`)
	}
	return nil
}

func (c *Redis) Delete(_ context.Context, key string) error {
	return c.codec.Delete(key)
}
