package loader

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/go-redis/redis/v8"
)

const (
	DefaultRedisKey = "gost"
)

type redisLoaderOptions struct {
	db       int
	password string
	key      string
}

type RedisLoaderOption func(opts *redisLoaderOptions)

func DBRedisLoaderOption(db int) RedisLoaderOption {
	return func(opts *redisLoaderOptions) {
		opts.db = db
	}
}

func PasswordRedisLoaderOption(password string) RedisLoaderOption {
	return func(opts *redisLoaderOptions) {
		opts.password = password
	}
}

func KeyRedisLoaderOption(key string) RedisLoaderOption {
	return func(opts *redisLoaderOptions) {
		opts.key = key
	}
}

type redisSetLoader struct {
	client *redis.Client
	key    string
}

// RedisSetLoader loads data from redis set.
func RedisSetLoader(addr string, opts ...RedisLoaderOption) Loader {
	var options redisLoaderOptions
	for _, opt := range opts {
		opt(&options)
	}

	key := options.key
	if key == "" {
		key = DefaultRedisKey
	}

	return &redisSetLoader{
		client: redis.NewClient(&redis.Options{
			Addr:     addr,
			Password: options.password,
			DB:       options.db,
		}),
		key: key,
	}
}

func (p *redisSetLoader) Load(ctx context.Context) (io.Reader, error) {
	v, err := p.client.SMembers(ctx, p.key).Result()
	if err != nil {
		return nil, err
	}
	return bytes.NewReader([]byte(strings.Join(v, "\n"))), nil
}

func (p *redisSetLoader) Close() error {
	return p.client.Close()
}

type redisHashLoader struct {
	client *redis.Client
	key    string
}

// RedisHashLoader loads data from redis hash.
func RedisHashLoader(addr string, opts ...RedisLoaderOption) Loader {
	var options redisLoaderOptions
	for _, opt := range opts {
		opt(&options)
	}

	key := options.key
	if key == "" {
		key = DefaultRedisKey
	}

	return &redisHashLoader{
		client: redis.NewClient(&redis.Options{
			Addr:     addr,
			Password: options.password,
			DB:       options.db,
		}),
		key: key,
	}
}

func (p *redisHashLoader) Load(ctx context.Context) (io.Reader, error) {
	m, err := p.client.HGetAll(ctx, p.key).Result()
	if err != nil {
		return nil, err
	}

	var b strings.Builder
	for k, v := range m {
		fmt.Fprintf(&b, "%s %s\n", k, v)
	}
	return bytes.NewBufferString(b.String()), nil
}

func (p *redisHashLoader) Close() error {
	return p.client.Close()
}
