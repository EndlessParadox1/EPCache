// Package epcache implements a distributed cache system development framework.
package epcache

import (
	"context"
	"errors"
	"math/rand/v2"
	"sync"
	"sync/atomic"

	"github.com/EndlessParadox1/epcache/bloomfilter"
	pb "github.com/EndlessParadox1/epcache/epcachepb"
	"github.com/EndlessParadox1/epcache/ratelimit"
	"github.com/EndlessParadox1/epcache/singleflight"
)

func NewNode(cacheBytes int64, getter Getter) *Node {
	if getter == nil {
		panic("nil Getter")
	}
	g := &Node{
		loader:     &singleflight.Group{},
		getter:     getter,
		cacheBytes: cacheBytes,
	}
	return g
}

// Node is TODO
type Node struct {
	peers      PeerAgent
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
	limiter   *ratelimit.TokenBucket

	muFilter sync.RWMutex
	filter   *bloomfilter.BloomFilter

	Stats Stats
}

// Stats are statistics for a node.
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
	PeerSyncs     int64 // data-syncs from peers

	LenBlacklist int64
}

// RegisterPeers specifies PeerPicker for a node, e.g. NoPeer, GrpcPool
// or any that implements the PeerPicker.
func (n *Node) RegisterPeers(peers PeerAgent) {
	//if n.peers != nil {
	//	panic("RegisterPeers called more than once")
	//}
	if peers == nil {
		n.peers = NoPeer{}
	} else {
		n.peers = peers
		peers.SetNode(n)
	}
}

type LimitMode int

const (
	NoLimit LimitMode = iota
	BlockMode
	RejectMode
)

// SetLimiter sets a rate limiter working on blocking or rejecting mode.
func (n *Node) SetLimiter(rate int, cap int, mode LimitMode) {
	n.muLimiter.Lock()
	defer n.muLimiter.Unlock()
	n.limitMode = mode
	n.limiter = ratelimit.New(rate, cap)
}

// ResetLimiter disables a rate limiter.
func (n *Node) ResetLimiter() {
	n.muLimiter.Lock()
	defer n.muLimiter.Unlock()
	n.limitMode = NoLimit
	if n.limiter != nil {
		n.limiter.Close()
		n.limiter = nil
	}
}

// SetFilter sets a bloom filter, zero size for none.
// It calculates the required params to build a bloom filter, false positive rate of which will
// be lower than 0.01%, according to user's expected blacklist size.
func (n *Node) SetFilter(size uint32) {
	n.muFilter.Lock()
	defer n.muFilter.Unlock()
	if size == 0 {
		n.filter = nil
	} else {
		n.filter = bloomfilter.New(20*size, 13)
	}
	n.Stats.LenBlacklist = 0
}

func (n *Node) Get(ctx context.Context, key string) (ByteView, error) {
	if n.peers == nil {
		panic("peers must be specified before using a group")
	}
	atomic.AddInt64(&n.Stats.Reqs, 1)
	n.muLimiter.RLock()
	switch n.limitMode {
	case NoLimit:
	case BlockMode:
		if err := n.limiter.Wait(ctx, 1); err != nil {
			n.muLimiter.RUnlock()
			return ByteView{}, err
		}
	case RejectMode:
		if n.limiter.Allow(1) {
			n.muLimiter.RUnlock()
			return ByteView{}, errors.New("access restricted")
		}
	}
	n.muLimiter.RUnlock()
	atomic.AddInt64(&n.Stats.Gets, 1)
	value, hit := n.lookupCache(key)
	if hit {
		atomic.AddInt64(&n.Stats.Hits, 1)
		return value, nil
	}
	if n.filter != nil {
		n.muFilter.RLock()
		ret := n.filter.MightContain(key)
		n.muFilter.RUnlock()
		if ret {
			return ByteView{}, errors.New("forbidden key")
		}
	}
	return n.load(ctx, key)
}

func (n *Node) load(ctx context.Context, key string) (ByteView, error) {
	atomic.AddInt64(&n.Stats.Loads, 1)
	val, err := n.loader.Do(key, func() (any, error) {
		// singleflight only works on overlapping concurrent reqs,
		// so just lookup cache again before trying to load data from local or remote.
		if val, hit := n.lookupCache(key); hit {
			atomic.AddInt64(&n.Stats.Hits, 1)
			return val, nil
		}
		atomic.AddInt64(&n.Stats.LoadsDeduped, 1)
		if peer, ok := n.peers.PickPeer(key); ok {
			val, err := n.getFromPeer(ctx, peer, key)
			if err == nil {
				atomic.AddInt64(&n.Stats.PeerLoads, 1)
				return val, nil
			}
			atomic.AddInt64(&n.Stats.PeerLoadErrs, 1)
			if errors.Is(err, ErrNotFound) {
				return nil, err
			}
		}
		val, err := n.getLocally(ctx, key)
		if err != nil {
			atomic.AddInt64(&n.Stats.LocalLoadErrs, 1)
			return nil, err
		}
		atomic.AddInt64(&n.Stats.LocalLoads, 1)
		n.populateCache(key, val, &n.mainCache)
		return val, nil
	})
	if err != nil {
		if n.filter != nil && errors.Is(err, ErrNotFound) {
			n.muFilter.Lock()
			n.filter.Add(key)
			n.Stats.LenBlacklist++
			n.muFilter.Unlock()
		}
		return ByteView{}, err
	}
	return val.(ByteView), nil
}

func (n *Node) getLocally(ctx context.Context, key string) (ByteView, error) {
	bytes, err := n.getter.Get(ctx, key)
	if err != nil {
		return ByteView{}, err
	}
	return ByteView{cloneBytes(bytes)}, nil
}

func (n *Node) getFromPeer(ctx context.Context, peer ProtoPeer, key string) (ByteView, error) {
	req := &pb.Request{
		Key: key,
	}
	res, err := peer.Get(ctx, req)
	if err != nil {
		return ByteView{}, err
	}
	value := ByteView{res.Value}
	if rand.IntN(10) == 0 {
		n.populateCache(key, value, &n.hotCache)
	}
	return value, nil
}

// OnUpdate updates data in cache and then syncs to all peers using MQ.
// This must be called when data in source is changed to ensure consistency.
func (n *Node) OnUpdate(key string, value []byte) {
	data := &pb.SyncData{
		Method: "U",
		Key:    key,
		Value:  cloneBytes(value),
	}
	n.peers.SyncAll(data)
	n.update(key, value)
}

func (n *Node) update(key string, value []byte) {
	if _, ok := n.mainCache.get(key); ok {
		n.populateCache(key, ByteView{cloneBytes(value)}, &n.mainCache)
		return
	}
	if _, ok := n.hotCache.get(key); ok {
		n.populateCache(key, ByteView{cloneBytes(value)}, &n.hotCache)
	}
}

// OnRemove removes data in cache and then syncs to all peers using MQ.
// This must be called when data in source is purged to ensure consistency.
func (n *Node) OnRemove(key string) {
	data := &pb.SyncData{
		Method: "R",
		Key:    key,
	}
	n.peers.SyncAll(data)
	n.remove(key)
}

func (n *Node) remove(key string) {
	if _, ok := n.mainCache.get(key); ok {
		n.mainCache.remove(key)
		return
	}
	if _, ok := n.hotCache.get(key); ok {
		n.hotCache.remove(key)
	}
}

func (n *Node) lookupCache(key string) (value ByteView, ok bool) {
	value, ok = n.mainCache.get(key)
	if ok {
		return
	}
	value, ok = n.hotCache.get(key)
	return
}

func (n *Node) populateCache(key string, value ByteView, cache *cache) {
	cache.add(key, value)
	for {
		mainBytes := n.mainCache.bytes()
		hotBytes := n.hotCache.bytes()
		if mainBytes+hotBytes <= n.cacheBytes {
			break
		}
		// A magical strategy proven to be effective.
		victim := &n.mainCache
		if hotBytes > mainBytes/8 {
			victim = &n.hotCache
		}
		victim.removeOldest()
	}
}

type CacheType int

const (
	MainCache CacheType = iota + 1
	HotCache
)

func (n *Node) CacheStats(ctype CacheType) CacheStats {
	switch ctype {
	case MainCache:
		return n.mainCache.stats()
	case HotCache:
		return n.hotCache.stats()
	default:
		return CacheStats{}
	}
}
