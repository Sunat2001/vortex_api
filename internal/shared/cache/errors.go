package cache

import "errors"

var (
	// ErrCacheMiss is returned when a key is not found in cache
	ErrCacheMiss = errors.New("cache: key not found")

	// ErrSerialization is returned when serialization/deserialization fails
	ErrSerialization = errors.New("cache: serialization failed")

	// ErrInvalidKey is returned when a key is empty or invalid
	ErrInvalidKey = errors.New("cache: invalid key")

	// ErrInvalidValue is returned when a value is nil or invalid
	ErrInvalidValue = errors.New("cache: invalid value")
)
