package epcache

import (
	"context"
	"errors"
	"log"
	"net"
	"sync"

	"github.com/EndlessParadox1/epcache/consistenthash"
	pb "github.com/EndlessParadox1/epcache/epcachepb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const defaultReplicas = 50

// protoGetter implements PeerGetter with gRPC
type protoGetter struct {
	addr string
}

func (pg *protoGetter) Get(ctx context.Context, in *pb.Request) (*pb.Response, error) {
	conn, err := grpc.NewClient(pg.addr, grpc.WithTransportCredentials(insecure.NewCredentials())) // disable tls
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	client := pb.NewEPCacheClient(conn)
	return client.Get(ctx, in)
}

type GrpcPool struct {
	self         string
	opts         GrpcPoolOptions
	mu           sync.RWMutex
	peers        *consistenthash.Map
	protoGetters map[string]*protoGetter // key like 'localhost:8080'

	pb.UnimplementedEPCacheServer
}

type GrpcPoolOptions struct {
	Replicas int
	HashFn   consistenthash.Hash
}

func NewGrpcPool(self string, opts *GrpcPoolOptions) *GrpcPool {
	gp := &GrpcPool{
		self: self,
	}
	if opts != nil {
		gp.opts = *opts
	}
	if gp.opts.Replicas == 0 {
		gp.opts.Replicas = defaultReplicas
	}
	return gp
}

// Set reset the pool's list of peers, including self
func (gp *GrpcPool) Set(peers ...string) {
	gp.mu.Lock()
	defer gp.mu.Unlock()
	gp.peers = consistenthash.New(gp.opts.Replicas, gp.opts.HashFn)
	gp.peers.Add(peers...)
	gp.protoGetters = make(map[string]*protoGetter)
	for _, peer := range peers {
		gp.protoGetters[peer] = &protoGetter{addr: peer}
	}
}

// PickPeer picks a peer according to the key.
func (gp *GrpcPool) PickPeer(key string) (PeerGetter, bool) {
	gp.mu.RLock()
	defer gp.mu.RUnlock()
	if gp.peers.IsEmpty() {
		return nil, false
	}
	if peer := gp.peers.Get(key); peer != "" && peer != gp.self {
		return gp.protoGetters[peer], true
	}
	return nil, false
}

func (gp *GrpcPool) Get(ctx context.Context, in *pb.Request) (*pb.Response, error) {
	groupName := in.GetGroup()
	key := in.GetKey()
	group := GetGroup(groupName)
	if group == nil {
		return nil, errors.New("no such group: " + groupName)
	}
	group.Stats.PeerReqs.Add(1)
	val, err := group.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	out := &pb.Response{Value: val.ByteSlice()}
	return out, nil
}

func (gp *GrpcPool) Run() {
	lis, err := net.Listen("tcp", gp.self)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	GrpcPool := grpc.NewServer()
	pb.RegisterEPCacheServer(GrpcPool, gp)
	log.Printf("GrpcPool listening at %v", lis.Addr())
	if err_ := GrpcPool.Serve(lis); err_ != nil {
		log.Fatalf("failed to serve: %v", err_)
	}
}
