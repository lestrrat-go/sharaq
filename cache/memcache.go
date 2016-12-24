// +build !appengine

package cache

import (
	"context"

	"github.com/bradfitz/gomemcache/memcache"
	"github.com/pkg/errors"
)

type Memcache struct {
	client *memcache.Client
}

type MemcacheConfig struct {
	Addr []string
}

func NewMemcache(server ...string) *Memcache {
	return &Memcache{
		client: memcache.New(server...),
	}
}

func (m *Memcache) Get(_ context.Context, key string, value interface{}) error {
	it, err := m.client.Get(key)
	if err != nil {
		return errors.Wrap(err, `failed to fetch from memcached`)
	}

	switch value.(type) {
	case *string:
		s := value.(*string)
		*s = string(it.Value)
	case *[]byte:
		s := value.(*[]byte)
		*s = it.Value
	default:
		return errors.New(`value must be &string or &[]byte`)
	}

	return nil
}

func (m *Memcache) Set(_ context.Context, key string, value []byte, expires int32) error {
	return m.client.Set(&memcache.Item{Key: key, Value: value, Expiration: expires})
}

func (m *Memcache) SetNX(_ context.Context, key string, value []byte, expires int32) error {
	return m.client.Add(&memcache.Item{Key: key, Value: value, Expiration: expires})
}

func (m *Memcache) Delete(_ context.Context, key string) error {
	return m.client.Delete(key)
}
