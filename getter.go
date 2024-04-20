package epcache

import "context"

type LoadError string

// ErrNotFound must be returned when Getter can't found the data.
const ErrNotFound LoadError = "key not found in the data source"

func (le LoadError) Error() string {
	return string(le)
}

// Getter loads data from source, like a DB.
type Getter interface {
	// Get depends on users' concrete implementation.
	Get(ctx context.Context, key string) ([]byte, error)
}

// GetterFunc indicates Getter might just be a func.
type GetterFunc func(ctx context.Context, key string) ([]byte, error)

func (f GetterFunc) Get(ctx context.Context, key string) ([]byte, error) {
	return f(ctx, key)
}
