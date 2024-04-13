// Package epcache implements a distributed cache system.
package epcache

import (
	"fmt"
	"log"
	"sync"

	pb "github.com/EndlessParadox1/epcache/epcachepb"
	"github.com/EndlessParadox1/epcache/singleflight"
)

// Getter loads data for a key
type Getter interface {
	// Get depends on users' concrete implementation
	Get(key string) ([]byte, error)
}

// GetterFunc indicates Getter might just be a func
type GetterFunc func(key string) ([]byte, error)

func (f GetterFunc) Get(key string) ([]byte, error) {
	return f(key)
}

// Group is a set of associated data spreading over one or more processes.
type Group struct {
	name       string
	getter     Getter
	peersOnce  sync.Once
	peers      PeerPiker
	loader     *singleflight.Group
	cacheBytes int // limit for sum of mainCache's and hotCache's size

	// mainCache contains data for which this process is authoritative.
	mainCache cache
	// hotCache contains data for which other processes are authoritative,
	// aiming to reduce the network IO overhead.
	hotCache cache

	//Stats    Stats
}

var (
	mu     sync.RWMutex
	groups = make(map[string]*Group)
)

func NewGroup(name string, cacheBytes int64, getter Getter) *Group {
	if getter == nil {
		panic("nil getter")
	}
	mu.Lock()
	defer mu.Unlock()
	g := &Group{
		name:      name,
		getter:    getter,
		mainCache: cache{},
		loader:    &singleflight.Group{},
	}
	groups[name] = g
	return g
}

func GetGroup(name string) *Group {
	mu.RLock()
	g := groups[name]
	mu.RUnlock()
	return g
}

func (g *Group) RegisterPeers(peers PeerPiker) {
	if g.peers != nil {
		panic("register peers once again")
	}
	g.peers = peers
}

func (g *Group) Get(key string) (ByteView, error) {
	if key == "" {
		return ByteView{}, fmt.Errorf("key is required")
	}
	if v, ok := g.mainCache.get(key); ok {
		log.Println("[GenCache] hit")
		return v, nil
	}
	return g.load(key)
}

func (g *Group) load(key string) (ByteView, error) {
	view, err := g.loader.Do(key, func() (any, error) {
		if g.peers != nil {
			if peer, ok := g.peers.PickPeer(key); ok {
				if view_, err_ := g.getFromPeer(peer, key); err_ == nil {
					return view_, nil
				} else {
					log.Println("[GenCache] failed to get from peer:", err_)
				}
			}
		}
		return g.getLocally(key)
	})
	if err == nil {
		return view.(ByteView), nil
	}
	return ByteView{}, err
}

func (g *Group) getLocally(key string) (ByteView, error) {
	bytes, err := g.getter.Get(key)
	if err != nil {
		return ByteView{}, err
	}
	value := ByteView{cloneBytes(bytes)}
	g.populateCache(key, value)
	return value, nil
}

func (g *Group) getFromPeer(peer PeerGetter, key string) (ByteView, error) {
	req := &pb.Request{
		Group: g.name,
		Key:   key,
	}
	res := &pb.Response{}
	err := peer.Get(req, res)
	if err != nil {
		return ByteView{}, err
	}
	return ByteView{res.GetValue()}, nil
}

func (g *Group) populateCache(key string, value ByteView) {
	g.mainCache.add(key, value)
}
