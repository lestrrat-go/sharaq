// +build appengine

package cache

import (
	"time"

	"github.com/pkg/errors"
	"golang.org/x/net/context"
	"google.golang.org/appengine/memcache"
)

type Memcache struct{} // dummy, as appengine just uses a default client
type MemcacheConfig struct {
	Addr []string // exists for compatibility, but is ignored
}

var instance Memcache

func NewMemcache(_ ...string) *Memcache {
	return &instance
}

func (m *Memcache) Get(ctx context.Context, key string, value interface{}) error {
	it, err := memcache.Get(ctx, key)
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

func (m *Memcache) Set(ctx context.Context, key string, value []byte, expires int32) error {
	return memcache.Set(ctx, &memcache.Item{Key: key, Value: value, Expiration: time.Duration(expires) * time.Second})
}

func (m *Memcache) SetNX(ctx context.Context, key string, value []byte, expires int32) error {
	return memcache.Add(ctx, &memcache.Item{Key: key, Value: value, Expiration: time.Duration(expires) * time.Second})
}

func (m *Memcache) Delete(ctx context.Context, key string) error {
	return memcache.Delete(ctx, key)
}
