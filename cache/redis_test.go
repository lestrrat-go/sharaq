package cache_test

import (
	"testing"

	"github.com/lestrrat-go/sharaq/cache"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
	redis "gopkg.in/redis.v5"
)

var redisAddr = "127.0.0.1:6379"

func redisAvailable() bool {
	client := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})
	if _, err := client.Ping().Result(); err != nil {
		return false
	}
	return true
}

func TestRedis(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var v, x []byte
	v = []byte("Hello")

	c := cache.NewRedis([]string{redisAddr})

	key := "foo"
	c.Delete(ctx, key)
	if !assert.Error(t, c.Get(ctx, key, &x), "Get should fail") {
		return
	}

	if !assert.NoError(t, c.Set(ctx, key, v, 10), "Set should succeed") {
		return
	}

	if !assert.NoError(t, c.Get(ctx, key, &x), "Get should succeed") {
		return
	}

	if !assert.Equal(t, x, v, "items should be equal") {
		return
	}

	if !assert.NoError(t, c.Delete(ctx, key), "Delete should succeed") {
		return
	}

	if !assert.Error(t, c.Get(ctx, key, &x), "Get should fail") {
		return
	}
}
