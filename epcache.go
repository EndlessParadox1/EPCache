// Package epcache implements a distributed cache system.
package epcache

import (
	"context"
	"errors"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/EndlessParadox1/epcache/bloomfilter"
	pb "github.com/EndlessParadox1/epcache/epcachepb"
	"github.com/EndlessParadox1/epcache/singleflight"
	"github.com/juju/ratelimit"
)

func init() {
	rand.NewSource(time.Now().UnixNano())
}

var (
	mu        sync.RWMutex
	groups    = make(map[string]*Group)
	groupHook func(*Group)
)

func NewGroup(name string, cacheBytes int64, getter Getter) *Group {
	if getter == nil {
		panic("nil Getter")
	}
	mu.Lock()
	defer mu.Unlock()
	if _, dup := groups[name]; dup {
		panic("duplicate registration of group: " + name)
	}
	g := &Group{
		name:       name,
		loader:     &singleflight.Group{},
		getter:     getter,
		cacheBytes: cacheBytes,
	}
	if groupHook != nil {
		groupHook(g)
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

// RegisterGroupHook sets func that will be executed each time a group is created.
func RegisterGroupHook(fn func(*Group)) {
	if groupHook != nil {
		panic("RegisterGroupHook called more than once")
	}
	groupHook = fn
}

// Group is a set of associated data spreading over one or more processes.
type Group struct {
	name       string
	peers      PeerPicker
	loader     *singleflight.Group
	getter     Getter
	cacheBytes int64 // limit for sum of mainCache's and hotCache's size

	// mainCache contains data for which this process is authoritative.
	mainCache cache
	// hotCache contains data for which other processes are authoritative,
	// aiming to reduce the network IO overhead.
	hotCache cache

	muLimiter sync.RWMutex
	limitMode LimitMode
	limiter   *ratelimit.Bucket

	muFilter sync.RWMutex
	filter   *bloomfilter.BloomFilter

	Stats Stats
}

// Stats are statistics for group.
type Stats struct {
	Reqs          int64
	Gets          int64
	Hits          int64
	Loads         int64
	LoadsDeduped  int64 // after singleflight
	LocalLoads    int64
	LocalLoadErrs int64
	PeerLoads     int64
	PeerLoadErrs  int64
	PeerReqs      int64 // requests from peers
	LenBlacklist  int64
}

func (g *Group) Name() string {
	return g.name
}

// RegisterPeers specifies PeerPicker for a group, e.g. NoPeer, GrpcPool
// or any that implements the PeerPicker.
func (g *Group) RegisterPeers(peers PeerPicker) {
	if g.peers != nil {
		panic("RegisterPeers called more than once")
	}
	if peers == nil {
		g.peers = NoPeer{}
	} else {
		g.peers = peers
	}
}

type LimitMode int

const (
	NoLimit = iota
	BlockMode
	RejectMode
)

// SetLimiter sets a rate limiter working on blocking or rejecting mode.
func (g *Group) SetLimiter(rate float64, cap int64, mode LimitMode) {
	g.muLimiter.Lock()
	defer g.muLimiter.Unlock()
	g.limitMode = mode
	g.limiter = ratelimit.NewBucketWithRate(rate, cap)
}

// ResetLimiter disables a rate limiter.
func (g *Group) ResetLimiter() {
	g.muLimiter.Lock()
	defer g.muLimiter.Unlock()
	g.limitMode = NoLimit
	g.limiter = nil
}

// SetFilter sets a bloom filter, zero size for none.
// It calculates the required params to build a bloom filter, false positive rate of which will
// be lower than 0.01%, according to user's expected blacklist size.
func (g *Group) SetFilter(size uint32) {
	g.muFilter.Lock()
	defer g.muFilter.Unlock()
	if size == 0 {
		g.filter = nil
		atomic.StoreInt64(&g.Stats.LenBlacklist, 0)
	} else {
		g.filter = bloomfilter.New(20*size, 13)
	}
}

func (g *Group) Get(ctx context.Context, key string) (ByteView, error) {
	if g.peers == nil {
		panic("peers must be specified before using a group")
	}
	atomic.AddInt64(&g.Stats.Reqs, 1)
	g.muLimiter.RLock()
	switch g.limitMode {
	case NoLimit:
		g.muLimiter.RUnlock()
	case BlockMode:
		if t, ok := ctx.Deadline(); ok {
			if !g.limiter.WaitMaxDuration(1, time.Since(t)) {
				g.muLimiter.RUnlock()
				return ByteView{}, errors.New("request timeout")
			}
		} else {
			g.limiter.Wait(1)
		}
		g.muLimiter.RUnlock()
	case RejectMode:
		ret := g.limiter.TakeAvailable(1)
		g.muLimiter.RUnlock()
		if ret == 0 {
			return ByteView{}, errors.New("access restricted")
		}
	}
	atomic.AddInt64(&g.Stats.Gets, 1)
	value, hit := g.lookupCache(key)
	if hit {
		atomic.AddInt64(&g.Stats.Hits, 1)
		return value, nil
	}
	if g.filter != nil {
		g.muFilter.RLock()
		ret := g.filter.MightContain(key)
		g.muFilter.RUnlock()
		if ret {
			return ByteView{}, errors.New("forbidden key")
		}
	}
	return g.load(ctx, key)
}

func (g *Group) load(ctx context.Context, key string) (ByteView, error) {
	atomic.AddInt64(&g.Stats.Loads, 1)
	val, err := g.loader.Do(key, func() (any, error) {
		// singleflight only works on overlapping concurrent reqs,
		// so just lookup cache again before trying to load data from local or remote.
		if val, hit := g.lookupCache(key); hit {
			atomic.AddInt64(&g.Stats.Hits, 1)
			return val, nil
		}
		atomic.AddInt64(&g.Stats.LoadsDeduped, 1)
		if peer, ok := g.peers.PickPeer(key); ok {
			val, err := g.getFromPeer(ctx, peer, key)
			if err == nil {
				atomic.AddInt64(&g.Stats.PeerLoads, 1)
				return val, nil
			}
			atomic.AddInt64(&g.Stats.PeerLoadErrs, 1)
			if errors.Is(err, ErrNotFound) {
				return nil, err
			}
		}
		val, err := g.getLocally(ctx, key)
		if err != nil {
			atomic.AddInt64(&g.Stats.LocalLoadErrs, 1)
			return nil, err
		}
		atomic.AddInt64(&g.Stats.LocalLoads, 1)
		g.populateCache(key, val, &g.mainCache)
		return val, nil
	})
	if err != nil {
		if g.filter != nil && errors.Is(err, ErrNotFound) {
			g.muFilter.Lock()
			g.filter.Add(key)
			g.muFilter.Unlock()
			atomic.AddInt64(&g.Stats.LenBlacklist, 1)
		}
		return ByteView{}, err
	}
	return val.(ByteView), nil
}

func (g *Group) getLocally(ctx context.Context, key string) (ByteView, error) {
	bytes, err := g.getter.Get(ctx, key)
	if err != nil {
		return ByteView{}, err
	}
	return ByteView{cloneBytes(bytes)}, nil
}

func (g *Group) getFromPeer(ctx context.Context, peer ProtoGetter, key string) (ByteView, error) {
	req := &pb.Request{
		Group: g.name,
		Key:   key,
	}
	res, err := peer.Get(ctx, req)
	if err != nil {
		return ByteView{}, err
	}
	value := ByteView{res.GetValue()}
	if rand.Intn(10) == 0 {
		g.populateCache(key, value, &g.hotCache)
	}
	return value, nil
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
		// A magical strategy proven to be effective
		victim := &g.mainCache
		if hotBytes > mainBytes/8 {
			victim = &g.hotCache
		}
		victim.removeOldest()
	}
}

type CacheType int

const (
	MainCache CacheType = iota + 1
	HotCache
)

func (g *Group) CacheStats(ctype CacheType) CacheStats {
	switch ctype {
	case MainCache:
		return g.mainCache.stats()
	case HotCache:
		return g.hotCache.stats()
	default:
		return CacheStats{}
	}
}
