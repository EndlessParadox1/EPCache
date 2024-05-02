package epcache

import (
	"context"
	"errors"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/EndlessParadox1/epcache/consistenthash"
	pb "github.com/EndlessParadox1/epcache/epcachepb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

const defaultReplicas = 50

type GrpcPool struct {
	self     string
	prefix   string
	registry []string
	opts     GrpcPoolOptions
	logger   *log.Logger
	pb.UnimplementedEPCacheServer

	muGroups sync.RWMutex
	groups   map[string]bool

	muPeers    sync.RWMutex
	peers      map[string]*consistenthash.Map // maps groups to different hash rings
	protoPeers map[string]*protoPeer          // keys like "localhost:8080"
}

type GrpcPoolOptions struct {
	Replicas int
	HashFn   consistenthash.Hash
}

var grpcPoolExist bool

// NewGrpcPool returns a GrpcPool instance.
//
//	prefix: The working directory of the EPCache cluster.
//	registry: The listening addresses of the etcd cluster.
func NewGrpcPool(self, prefix string, registry []string, opts *GrpcPoolOptions) *GrpcPool {
	if grpcPoolExist {
		panic("NewGrpcPool called more than once")
	}
	grpcPoolExist = true
	gp := &GrpcPool{
		self:     self,
		groups:   make(map[string]bool),
		logger:   log.New(os.Stdin, "[EPCache] ", log.LstdFlags),
		prefix:   prefix,
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

func (gp *GrpcPool) PickPeer(group, key string) (ProtoPeer, bool) {
	gp.muPeers.RLock()
	defer gp.muPeers.RUnlock()
	if gp.peers == nil {
		return nil, false
	}
	if peer := gp.peers[group].Get(key); peer != gp.self {
		return gp.protoPeers[peer], true
	}
	return nil, false
}

// SyncAll trys to sync data to all peers in an async way, and logs error if any.
func (gp *GrpcPool) SyncAll(data *pb.SyncData) {
	gp.muPeers.RLock()
	defer gp.muPeers.RUnlock()
	if gp.peers == nil {
		return
	}
	ch := make(chan error)
	count := 0
	for _, peer := range gp.protoPeers {
		go peer.SyncOne(data, ch)
		count++
	}
	var failSyncPeers []string
	for err := range ch {
		if err != nil {
			failSyncPeers = append(failSyncPeers, err.Error())
		}
		count--
		if count == 0 {
			break
		}
	}
	if len(failSyncPeers) > 0 {
		gp.logger.Println("failed to sync to these peers:", failSyncPeers)
	}
}

func (gp *GrpcPool) EnrollGroup(group string) {
	gp.muGroups.Lock()
	gp.groups[group] = true
	gp.muGroups.Unlock()
}

func (gp *GrpcPool) WithDrawGroup(group string) {
	gp.muGroups.Lock()
	delete(gp.groups, group)
	gp.muGroups.Unlock()
}

func (gp *GrpcPool) listGroups() string {
	gp.muGroups.RLock()
	defer gp.muGroups.RUnlock()
	var ans []string
	for group := range groups {
		ans = append(ans, group)
	}
	return strings.Join(ans, " ")
}

func (gp *GrpcPool) hasGroup(group string) bool {
	gp.muGroups.RLock()
	defer gp.muGroups.RUnlock()
	return gp.groups[group]
}

func (gp *GrpcPool) ListPeers() (ans []string) {
	gp.muPeers.RLock()
	defer gp.muPeers.RUnlock()
	if gp.peers == nil {
		return
	}
	for addr := range gp.protoPeers {
		ans = append(ans, addr)
	}
	return
}

// Implementing GrpcPool as the EPCacheServer.

func (gp *GrpcPool) Get(ctx context.Context, in *pb.Request) (*pb.Response, error) {
	groupName := in.GetGroup()
	group := GetGroup(groupName)
	if group == nil {
		return nil, errors.New("no such group: " + groupName)
	}
	atomic.AddInt64(&group.Stats.PeerReqs, 1)
	val, err := group.Get(ctx, in.GetKey())
	if err != nil {
		return nil, err
	}
	out := &pb.Response{Value: val.ByteSlice()}
	return out, nil
}

func (gp *GrpcPool) Sync(_ context.Context, data *pb.SyncData) (out *emptypb.Empty, err error) {
	groupName := data.GetGroup()
	group := GetGroup(groupName)
	if group == nil {
		return
	}
	switch data.GetMethod() {
	case "U":
		go group.update(data.GetKey(), data.GetValue())
	case "R":
		go group.remove(data.GetKey())
	}
	return
}

// Run starts a node of the EPCache cluster.
func (gp *GrpcPool) Run() {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   gp.registry,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		gp.logger.Fatal("connecting to etcd failed:", err)
	}
	defer cli.Close()
	ctx, cancel := context.WithCancel(context.Background())
	// This ensures that the first service registration from self
	// can be caught by service discovery's watch.
	ch := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go gp.register(ctx, &wg, cli, ch)
	wg.Add(1)
	go gp.discover(ctx, &wg, cli, ch)
	wg.Add(1)
	go gp.startServer(ctx, &wg)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
	gp.logger.Println("Shutting down gracefully...")
	cancel()  // notifying all goroutines to stop
	wg.Wait() // waiting for all cleaning works to be completed
}

// startServer starts a gRPC server.
func (gp *GrpcPool) startServer(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	lis, err := net.Listen("tcp", gp.self)
	if err != nil {
		gp.logger.Fatal("failed to listen:", err)
	}
	server := grpc.NewServer()
	pb.RegisterEPCacheServer(server, gp)
	go func() {
		<-ctx.Done()
		gp.logger.Println("Shutting down gRPC server...")
		server.GracefulStop()
	}()
	gp.logger.Println("GrpcPool listening at", lis.Addr())
	if err_ := server.Serve(lis); err_ != nil {
		gp.logger.Fatal("failed to serve:", err_)
	}
}

// register will update groups owned to the etcd every 30 minutes.
func (gp *GrpcPool) register(ctx context.Context, wg *sync.WaitGroup, cli *clientv3.Client, ch chan struct{}) {
	defer wg.Done()
	lease, err := cli.Grant(context.Background(), 60)
	if err != nil {
		gp.logger.Fatal("failed to obtain lease:", err)
	}
	leaseResCh, err_ := cli.KeepAlive(context.Background(), lease.ID)
	if err_ != nil {
		gp.logger.Fatal("failed to keep alive:", err)
	}
	<-ch
	key := gp.prefix + gp.self
	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case _, ok := <-leaseResCh:
			if !ok {
				gp.logger.Fatal("failed to maintain lease")
			}
		case <-ticker.C:
			value := gp.listGroups()
			_, err = cli.Put(context.Background(), key, value, clientv3.WithLease(lease.ID))
			if err != nil {
				gp.logger.Fatal("failed to put key:", err)
			}
		case <-ctx.Done():
			_, err = cli.Revoke(context.Background(), lease.ID)
			if err != nil {
				gp.logger.Println("failed to proactively revoke lease:", err)
			}
			gp.logger.Println("Service register stopped")
			return
		}
	}
}

// discover will find out all peers and the groups they owned from etcd when changes happen.
func (gp *GrpcPool) discover(ctx context.Context, wg *sync.WaitGroup, cli *clientv3.Client, ch chan struct{}) {
	defer wg.Done()
	watchChan := cli.Watch(context.Background(), gp.prefix, clientv3.WithPrefix())
	close(ch)
	for {
		select {
		case <-watchChan:
			res, err := cli.Get(context.Background(), gp.prefix, clientv3.WithPrefix())
			if err != nil {
				gp.logger.Fatal("failed to retrieve service list:", err)
			}
			m := make(map[string][]string) // maps group to addrs
			gp.muPeers.Lock()
			gp.protoPeers = make(map[string]*protoPeer)
			for _, kv := range res.Kvs {
				addr := string(kv.Key)
				groups_ := strings.Split(string(kv.Value), " ")
				if addr != gp.self {
					gp.protoPeers[addr] = &protoPeer{addr}
				}
				for _, group := range groups_ {
					m[group] = append(m[group], addr)
				}
			}
			gp.setPeers(m)
			gp.muPeers.Unlock()
		case <-ctx.Done():
			gp.logger.Println("Service discovery stopped")
			return
		}
	}
}

func (gp *GrpcPool) setPeers(m map[string][]string) {
	gp.peers = make(map[string]*consistenthash.Map)
	for group, addrs := range m {
		if gp.hasGroup(group) {
			gp.peers[group] = consistenthash.New(gp.opts.Replicas, gp.opts.HashFn)
			gp.peers[group].Add(addrs...)
		}
	}
}
