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

func NewGroup(name string, cacheBytes int, getter Getter) *Group {
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
	cacheBytes int // limit for sum of mainCache's and hotCache's size

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
	Accesses      atomic.Int64
	Gets          atomic.Int64
	Hits          atomic.Int64
	Loads         atomic.Int64
	LoadsDeduped  atomic.Int64 // after singleflight
	LocalLoads    atomic.Int64
	LocalLoadErrs atomic.Int64
	PeerLoads     atomic.Int64
	PeerLoadErrs  atomic.Int64
	PeerReqs      atomic.Int64 // requests from peers
	LenBlacklist  atomic.Int64
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
		peers.addGroup(g.name)
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
		g.Stats.LenBlacklist.Store(0)
	}
	g.filter = bloomfilter.New(20*size, 13)
}

func (g *Group) Get(ctx context.Context, key string) (ByteView, error) {
	if g.peers == nil {
		panic("peers must be specified before using a group")
	}
	g.Stats.Accesses.Add(1)
	g.muLimiter.RLock()
	switch g.limitMode {
	case NoLimit:
		g.muLimiter.RUnlock()
	case BlockMode:
		g.limiter.Wait(1)
		g.muLimiter.RUnlock()
	case RejectMode:
		ret := g.limiter.TakeAvailable(1)
		g.muLimiter.RUnlock()
		if ret == 0 {
			return ByteView{}, errors.New("access restricted")
		}
	}
	g.Stats.Gets.Add(1)
	value, hit := g.lookupCache(key)
	if hit {
		g.Stats.Hits.Add(1)
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
	g.Stats.Loads.Add(1)
	val, err := g.loader.Do(key, func() (any, error) {
		// singleflight only works on overlapping concurrent reqs,
		// so just lookup cache again before trying to load data from local or remote.
		if val, hit := g.lookupCache(key); hit {
			g.Stats.Hits.Add(1)
			return val, nil
		}
		g.Stats.LoadsDeduped.Add(1)
		if peer, ok := g.peers.pickPeer(key); ok {
			val, err := g.getFromPeer(ctx, peer, key)
			if err == nil {
				g.Stats.PeerLoads.Add(1)
				return val, nil
			}
			g.Stats.PeerLoadErrs.Add(1)
			if errors.Is(err, ErrNotFound) {
				return nil, err
			}
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
		if g.filter != nil && errors.Is(err, ErrNotFound) {
			g.muFilter.Lock()
			g.filter.Add(key)
			g.muFilter.Unlock()
			g.Stats.LenBlacklist.Add(1)
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

func (g *Group) getFromPeer(ctx context.Context, peer peerGetter, key string) (ByteView, error) {
	req := &pb.Request{
		Group: g.name,
		Key:   key,
	}
	res, err := peer.get(ctx, req)
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
