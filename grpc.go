package epcache

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"

	"time"

	"github.com/EndlessParadox1/epcache/consistenthash"
	pb "github.com/EndlessParadox1/epcache/epcachepb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
)

const defaultReplicas = 50

type protoPeer struct {
	addr string
}

func (p *protoPeer) Get(ctx context.Context, in *pb.Request) (*pb.Response, error) {
	conn, err := grpc.NewClient(p.addr, grpc.WithTransportCredentials(insecure.NewCredentials())) // disable tls
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	client := pb.NewEPCacheClient(conn)
	return client.Get(ctx, in)
}

func (p *protoPeer) SyncOne(data *pb.SyncData, ch chan<- error) {
	conn, err := grpc.NewClient(p.addr, grpc.WithTransportCredentials(insecure.NewCredentials())) // disable tls
	if err != nil {
		ch <- errors.New(p.addr)
		return
	}
	defer conn.Close()
	client := pb.NewEPCacheClient(conn)
	_, err = client.Sync(context.Background(), data)
	if err != nil {
		ch <- errors.New(p.addr)
		return
	}
	ch <- nil
}

type GrpcPool struct {
	self       string
	opts       GrpcPoolOptions
	mu         sync.RWMutex // guards peers and protoGetters
	peers      *consistenthash.Map
	protoPeers map[string]*protoPeer // keys like "localhost:8080"

	logger   *log.Logger
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
		logger:   log.New(os.Stdin, "[EPCache] ", log.LstdFlags),
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

func (gp *GrpcPool) PickPeer(key string) (ProtoPeer, bool) {
	gp.mu.RLock()
	defer gp.mu.RUnlock()
	if gp.peers == nil {
		return nil, false
	}
	if peer := gp.peers.Get(key); peer != gp.self {
		return gp.protoPeers[peer], true
	}
	return nil, false
}

// SyncAll TODO
func (gp *GrpcPool) SyncAll(data *pb.SyncData) {
	gp.mu.RLock()
	defer gp.mu.RUnlock()
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
		gp.logger.Printf("failed to sync to these peers: %v\n", failSyncPeers)
	}
}

func (gp *GrpcPool) ListPeers() (ans []string) {
	gp.mu.RLock()
	defer gp.mu.RUnlock()
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
	key := in.GetKey()
	val, err := group.Get(ctx, key) // TODO
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
} // TODO

// Run TODO
func (gp *GrpcPool) Run() {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   gp.registry,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		gp.logger.Fatalf("connecting to etcd failed: %v", err)
	}
	defer cli.Close()
	ctx, cancel := context.WithCancel(context.Background())
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
	fmt.Println("Received shutdown signal. Shutting down gracefully...")
	cancel()
	wg.Wait()
}

// startServer TODO
func (gp *GrpcPool) startServer(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	lis, err := net.Listen("tcp", gp.self)
	if err != nil {
		gp.logger.Fatalf("failed to listen: %v", err)
	}
	server := grpc.NewServer()
	pb.RegisterEPCacheServer(server, gp)
	go func() {
		<-ctx.Done()
		gp.logger.Println("Shutting down gRPC server...")
		server.GracefulStop()
	}()
	gp.logger.Printf("GrpcPool listening at %v\n", lis.Addr())
	if err_ := server.Serve(lis); err_ != nil {
		gp.logger.Fatalf("failed to serve: %v", err_)
	}
}

// register TODO
func (gp *GrpcPool) register(ctx context.Context, wg *sync.WaitGroup, cli *clientv3.Client, ch chan<- struct{}) {
	defer wg.Done()
	lease, err := cli.Grant(context.Background(), 60)
	if err != nil {
		gp.logger.Fatalf("failed to grant lease: %v", err)
	}
	key, value := "epcache/"+gp.self, ""
	_, err = cli.Put(context.Background(), key, value, clientv3.WithLease(lease.ID))
	if err != nil {
		gp.logger.Fatalf("failed to put key: %v", err)
	}
	close(ch)
	leaseResCh, err_ := cli.KeepAlive(context.Background(), lease.ID)
	if err_ != nil {
		gp.logger.Fatalf("failed to keepalive: %v", err)
	}
	for {
		select {
		case _, ok := <-leaseResCh:
			if !ok {
				gp.logger.Fatalf("failed to mantain leease")
			}
		case <-ctx.Done():
			_, _err := cli.Revoke(context.Background(), lease.ID)
			if _err != nil {
				gp.logger.Fatalf("failed to revoke lease: %v", err)
			}
			gp.logger.Println("Lease maintenance stopped")
			return
		}
	}
}

// discover TODO
func (gp *GrpcPool) discover(ctx context.Context, wg *sync.WaitGroup, cli *clientv3.Client, ch <-chan struct{}) {
	defer wg.Done()
	ticker := time.Tick(20 * time.Minute)
	<-ch
	for {
		select {
		case <-ticker:
			res, err := cli.Get(context.Background(), "epcache/", clientv3.WithPrefix())
			if err != nil {
				gp.logger.Fatalf("Error retrieving service list: %v", err)
			}
			var peers []string
			for _, kv := range res.Kvs {
				peers = append(peers, string(kv.Key))
			}
			gp.set(peers...)
		case <-ctx.Done():
			gp.logger.Println("Service discovery stopped")
			return
		}
	}
}

func (gp *GrpcPool) set(peers ...string) {
	gp.mu.Lock()
	defer gp.mu.Unlock()
	gp.peers = consistenthash.New(gp.opts.Replicas, gp.opts.HashFn)
	gp.peers.Add(peers...)
	gp.protoPeers = make(map[string]*protoPeer)
	for _, peer := range peers {
		if peer != gp.self {
			gp.protoPeers[peer] = &protoPeer{addr: peer}
		}
	}
}
