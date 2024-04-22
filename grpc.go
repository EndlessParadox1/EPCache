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
	"syscall"
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
	mu           sync.RWMutex // guards peers and protoGetters
	peers        *consistenthash.Map
	protoGetters map[string]*protoGetter // keys like "localhost:8080"

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

func (gp *GrpcPool) PickPeer(key string) (ProtoGetter, bool) {
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

// Run TODO
func (gp *GrpcPool) Run() {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   gp.registry,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		log.Fatalf("connecting to etcd failed: %v", err)
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

func (gp *GrpcPool) startServer(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	lis, err := net.Listen("tcp", gp.self)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	server := grpc.NewServer()
	pb.RegisterEPCacheServer(server, gp)
	go func() {
		<-ctx.Done()
		log.Println("Shutting down gRPC server...")
		server.GracefulStop()
	}()
	log.Printf("GrpcPool listening at %v\n", lis.Addr())
	if err_ := server.Serve(lis); err_ != nil {
		log.Fatalf("failed to serve: %v", err_)
	}
}

// register exposes self to the registry using a mechanism similar to heartbeat. TODO
func (gp *GrpcPool) register(ctx context.Context, wg *sync.WaitGroup, cli *clientv3.Client, ch chan<- struct{}) {
	defer wg.Done()
	lease, err := cli.Grant(context.Background(), 60)
	if err != nil {
		log.Fatalf("failed to grant lease: %v", err)
	}
	key, value := "epcache/"+gp.self, ""
	_, err = cli.Put(context.Background(), key, value, clientv3.WithLease(lease.ID))
	if err != nil {
		log.Fatalf("failed to put key: %v", err)
	}
	ch <- struct{}{}
	leaseResCh, err_ := cli.KeepAlive(context.Background(), lease.ID)
	if err_ != nil {
		log.Fatalf("failed to keepalive: %v", err)
	}
	for {
		select {
		case _, ok := <-leaseResCh:
			if !ok {
				log.Fatalf("failed to mantain leease")
			}
		case <-ctx.Done():
			_, _err := cli.Revoke(context.Background(), lease.ID)
			if _err != nil {
				log.Fatalf("failed to revoke lease: %v", err)
			}
			log.Println("Lease maintenance stopped")
			return
		}
	}
}

// discover finds out all other peers every 20 minutes. TODO
func (gp *GrpcPool) discover(ctx context.Context, wg *sync.WaitGroup, cli *clientv3.Client, ch <-chan struct{}) {
	defer wg.Done()
	ticker := time.Tick(20 * time.Minute)
	<-ch
	for {
		select {
		case <-ticker:
			res, err := cli.Get(context.Background(), "epcache/", clientv3.WithPrefix())
			if err != nil {
				log.Fatalf("Error retrieving service list: %v", err)
			}
			var peers []string
			for _, kv := range res.Kvs {
				peers = append(peers, string(kv.Key))
			}
			gp.set(peers...)
		case <-ctx.Done():
			log.Println("Service discovery stopped")
			return
		}
	}
}

func (gp *GrpcPool) set(peers ...string) {
	gp.mu.Lock()
	defer gp.mu.Unlock()
	gp.peers = consistenthash.New(gp.opts.Replicas, gp.opts.HashFn)
	gp.peers.Add(peers...)
	gp.protoGetters = make(map[string]*protoGetter)
	for _, peer := range peers {
		gp.protoGetters[peer] = &protoGetter{addr: peer}
	}
}
