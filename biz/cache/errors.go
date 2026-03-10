package cache

import "errors"

var (
	ErrCacheMiss = errors.New("cache miss")
	ErrRedisDown = errors.New("redis unvailable")
	ErrRedisTimeout = errors.New("redis timeout")
)

func IsCacheMiss(err error) bool {
	return errors.Is(err, ErrCacheMiss)
}

func IsRedisDown(err error) bool {
	return errors.Is(err, ErrRedisDown)
}

func IsRedisError(err error) bool {
	return err != nil && !IsCacheMiss(err) && !IsRedisDown(err)
}