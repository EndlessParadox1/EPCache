package epcache

import "context"

type LoadError string

// ErrNotFound must be returned when Getter can't found the data.
const ErrNotFound LoadError = "key not found in data source"

func (e LoadError) Error() string {
	return string(e)
}

// Getter loads data from source, like a DB.
type Getter interface {
	// Get depends on users' concrete implementation.
	// Context's deadline should be treated properly if existed.
	Get(ctx context.Context, key string) ([]byte, error)
}

// GetterFunc indicates Getter might just be a func.
type GetterFunc func(ctx context.Context, key string) ([]byte, error)

func (f GetterFunc) Get(ctx context.Context, key string) ([]byte, error) {
	return f(ctx, key)
}
