// Package epcache implements a distributed cache system.
package epcache

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/EndlessParadox1/epcache/singleflight"
)

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

var (
	mu     sync.RWMutex
	groups = make(map[string]*Group)
)

func NewGroup(name string, cacheBytes int, getter Getter) *Group {
	if getter == nil {
		panic("nil Getter")
	}
	mu.Lock()
	defer mu.Unlock()
	g := &Group{
		name:       name,
		loader:     &singleflight.Group{},
		getter:     getter,
		cacheBytes: cacheBytes,
	}
	groups[name] = g
	return g
}

func GetGroup(name string) *Group {
	mu.RLock()
	defer mu.RUnlock()
	g := groups[name]
	return g
}

// Group is a set of associated data spreading over one or more processes.
type Group struct {
	name       string
	peersOnce  sync.Once
	peers      PeerPiker
	loader     *singleflight.Group
	getter     Getter
	cacheBytes int // limit for sum of mainCache's and hotCache's size

	// mainCache contains data for which this process is authoritative.
	mainCache cache
	// hotCache contains data for which other processes are authoritative,
	// aiming to reduce the network IO overhead.
	hotCache cache

	Stats Stats
}

// Stats are statistics for group.
type Stats struct {
	Gets          atomic.Int64
	Hits          atomic.Int64
	Loads         atomic.Int64
	LoadsDeduped  atomic.Int64 // after singleflight
	LocalLoads    atomic.Int64
	LocalLoadErrs atomic.Int64
	PeerLoads     atomic.Int64
	PeerLoadErrs  atomic.Int64
	PeerReqs      atomic.Int64 // requests from peers TODO
}

func (g *Group) Get(ctx context.Context, key string) (ByteView, error) {
	g.Stats.Gets.Add(1)
	value, hit := g.lookupCache(key)
	if hit {
		g.Stats.Hits.Add(1)
		return value, nil
	}
	return g.load(ctx, key)
}

func (g *Group) load(ctx context.Context, key string) (ByteView, error) {
	g.Stats.Loads.Add(1)
	val, err := g.loader.Do(key, func() (any, error) {
		// singleflight only works on overlapping concurrent reqs,
		// so just lookup cache again before trying to load data from local or remote.
		if val, hit := g.lookupCache(key); hit {
			g.Stats.Hits.Add(1)
			return val, nil
		}
		g.Stats.LoadsDeduped.Add(1)
		if peer, ok := g.peers.PickPeer(key); ok {
			val, err := g.getFromPeer(ctx, peer, key)
			if err == nil {
				g.Stats.PeerLoads.Add(1)
				return val, nil
			}
			g.Stats.PeerLoadErrs.Add(1)
		}
		val, err := g.getLocally(ctx, key)
		if err != nil {
			g.Stats.LocalLoadErrs.Add(1)
			return nil, err
		}
		g.Stats.LocalLoads.Add(1)
		g.populateCache(key, val, &g.mainCache)
		return val, nil
	})
	if err != nil {
		return ByteView{}, err
	}
	return val.(ByteView), nil
}

func (g *Group) getLocally(ctx context.Context, key string) (ByteView, error) {
	val, err := g.getter.Get(ctx, key)
	if err != nil {
		return ByteView{}, err
	}
	return ByteView{val}, nil
}

func (g *Group) getFromPeer(ctx context.Context, peer PeerGetter, key string) (ByteView, error) {
	// TODO
	return ByteView{}, nil
}

func (g *Group) lookupCache(key string) (value ByteView, ok bool) {
	value, ok = g.mainCache.get(key)
	if ok {
		return
	}
	value, ok = g.hotCache.get(key)
	return
}

func (g *Group) populateCache(key string, value ByteView, cache *cache) {
	cache.add(key, value)
	for {
		mainBytes := g.mainCache.bytes()
		hotBytes := g.hotCache.bytes()
		if mainBytes+hotBytes <= g.cacheBytes {
			break
		}
		victim := &g.mainCache
		if hotBytes > mainBytes/8 {
			victim = &g.hotCache
		}
		victim.removeOldest()
	}
}
