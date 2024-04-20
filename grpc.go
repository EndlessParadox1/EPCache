package epcache

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/EndlessParadox1/epcache/consistenthash"
	pb "github.com/EndlessParadox1/epcache/epcachepb"
	"go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const defaultReplicas = 50

// protoGetter implements PeerGetter with gRPC
type protoGetter struct {
	addr string
}

func (pg *protoGetter) get(ctx context.Context, in *pb.Request) (*pb.Response, error) {
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
	mu           sync.RWMutex // guards peers and protoGetters
	peers        *consistenthash.Map
	protoGetters map[string]*protoGetter // keys like "localhost:8080"

	groups   []string
	registry []string
	pb.UnimplementedEPCacheServer
}

type GrpcPoolOptions struct {
	Replicas int
	HashFn   consistenthash.Hash
}

var grpcPoolExist bool

func NewGrpcPool(self string, registry []string, opts *GrpcPoolOptions) *GrpcPool {
	if grpcPoolExist {
		panic("NewGrpcPool called more than once")
	}
	grpcPoolExist = true
	gp := &GrpcPool{
		self:     self,
		registry: registry,
	}
	if opts != nil {
		gp.opts = *opts
	}
	if gp.opts.Replicas == 0 {
		gp.opts.Replicas = defaultReplicas
	}
	return gp
}

func (gp *GrpcPool) addGroup(group string) {
	if gp.groups == nil {
		gp.groups = []string{}
	}
	gp.groups = append(gp.groups, group)
}

func (gp *GrpcPool) pickPeer(key string) (peerGetter, bool) {
	gp.mu.RLock()
	defer gp.mu.RUnlock()
	if gp.peers == nil {
		return nil, false
	}
	if peer := gp.peers.Get(key); peer != gp.self {
		return gp.protoGetters[peer], true
	}
	return nil, false
}

// Implementing GrpcPool as the EPCacheServer.

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

// Run starts the GrpcPool as the EPCacheServer.
func (gp *GrpcPool) Run() {
	lis, err := net.Listen("tcp", gp.self)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	server := grpc.NewServer()
	pb.RegisterEPCacheServer(server, gp)
	go func() {
		log.Printf("GrpcPool listening at %v\n", lis.Addr())
		if err_ := server.Serve(lis); err_ != nil {
			log.Fatalf("failed to serve: %v", err_)
		}
	}()
	etcdClient, err := clientv3.New(clientv3.Config{
		Endpoints:   gp.registry, // etcd server address
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		log.Fatalf("connecting to etcd failed: %v", err)
	}
	defer etcdClient.Close()
	leaseRes, err := etcdClient.Grant(context.Background(), 10)
	if err != nil {
		log.Fatalf("Error granting lease: %v", err)
	}
	key, value := "epcache/"+gp.self, strings.Join(gp.groups, ",")
	_, err = etcdClient.Put(context.Background(), key, value, clientv3.WithLease(leaseRes.ID))
	if err != nil {
		log.Fatalf("Error putting key: %v", err)
	}
	ch, err := etcdClient.KeepAlive(context.TODO(), leaseRes.ID)
	if err != nil {
		log.Fatal(err)
	}
	go func() {
		for {
			ka := <-ch
			fmt.Println("ttl:", ka.TTL)
		}
	}()
}

func (gp *GrpcPool) set(peers ...string) {
	gp.mu.Lock()
	defer gp.mu.Unlock()
	gp.peers = consistenthash.New(gp.opts.Replicas, gp.opts.HashFn)
	gp.peers.Add(gp.self)
	gp.peers.Add(peers...)
	gp.protoGetters = make(map[string]*protoGetter)
	for _, peer := range peers {
		gp.protoGetters[peer] = &protoGetter{addr: peer}
	}
}
