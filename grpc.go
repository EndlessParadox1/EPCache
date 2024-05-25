package epcache

import (
	"context"
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
	"github.com/EndlessParadox1/epcache/msgctl"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
)

const defaultReplicas = 50

const defaultPrefix = "epcache/"

const defaultExchange = "epcache"

type GrpcPool struct {
	self     string
	registry []string
	mqBroker string
	opts     GrpcPoolOptions

	logger    *log.Logger
	syncCh    chan *pb.SyncData
	dscMsgCon *msgctl.MsgController
	node      *Node
	pb.UnimplementedEPCacheServer

	muPeers    sync.RWMutex
	peers      *consistenthash.Map
	protoPeers map[string]*protoPeer // keys like "localhost:8080"
}

// GrpcPoolOptions are options to build a GrpcPool instance.
//
//	Prefix: The etcd namespace to which an EPCache cluster instance belongs.
//	Exchange: The MQ exchange used by an EPCache cluster instance.
type GrpcPoolOptions struct {
	Prefix   string
	Exchange string
	Replicas int
	HashFn   consistenthash.Hash
}

var grpcPoolExist bool

// NewGrpcPool returns a GrpcPool instance.
//
//		registry: The listening addresses of the etcd cluster.
//	 mqBroker: The listening address of the MQ broker.
func NewGrpcPool(self string, registry []string, mqBroker string, opts *GrpcPoolOptions) *GrpcPool {
	if grpcPoolExist {
		panic("NewGrpcPool called more than once")
	}
	grpcPoolExist = true
	gp := &GrpcPool{
		self:      self,
		registry:  registry,
		mqBroker:  mqBroker,
		logger:    log.New(os.Stdin, "[EPCache] ", log.LstdFlags),
		syncCh:    make(chan *pb.SyncData),
		dscMsgCon: msgctl.New(time.Second),
	}
	if opts != nil {
		gp.opts = *opts
	}
	if gp.opts.Replicas == 0 {
		gp.opts.Replicas = defaultReplicas
	}
	if gp.opts.Prefix == "" {
		gp.opts.Prefix = defaultPrefix
	}
	if gp.opts.Exchange == "" {
		gp.opts.Exchange = defaultExchange
	}
	go gp.run()
	return gp
}

func (gp *GrpcPool) PickPeer(key string) (ProtoPeer, bool) {
	gp.muPeers.RLock()
	defer gp.muPeers.RUnlock()
	if gp.peers == nil {
		return nil, false
	}
	if peer := gp.peers.Get(key); peer != gp.self {
		return gp.protoPeers[peer], true
	}
	return nil, false
}

// SyncAll just publishes a message to the MQ exchange working in fanout pattern.
func (gp *GrpcPool) SyncAll(data *pb.SyncData) {
	gp.syncCh <- data
}

func (gp *GrpcPool) SetNode(node *Node) {
	gp.node = node
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

func (gp *GrpcPool) Get(ctx context.Context, req *pb.Request) (*pb.Response, error) {
	atomic.AddInt64(&gp.node.Stats.PeerReqs, 1)
	val, err := gp.node.Get(ctx, req.Key)
	if err != nil {
		return nil, err
	}
	out_ := &pb.Response{Value: val.ByteSlice()}
	return out_, nil
}

// run starts the service registration and discovery, the data sync sending and receiving, as well as the gRPC server.
// all of which support graceful shutdown.
func (gp *GrpcPool) run() {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   gp.registry,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		gp.logger.Fatal("connecting to etcd failed:", err)
	}
	defer cli.Close()
	ctx, cancel := context.WithCancel(context.Background())
	// This ensures that the service registration from self can be caught by the service discovery's watch.
	ch := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go gp.register(ctx, &wg, cli, ch)
	wg.Add(1)
	go gp.discover(ctx, &wg, cli, ch)
	wg.Add(1)
	go gp.startServer(ctx, &wg)
	wg.Add(1)
	go gp.produce(ctx, &wg)
	wg.Add(1)
	go gp.consume(ctx, &wg)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		count := 0
		for {
			<-sigChan
			count++
			if count == 1 {
				gp.logger.Println("Shutting down gracefully...SIG again to force")
				cancel() // notifying some goroutines to clean up
			} else {
				os.Exit(1)
			}
		}
	}()
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
		server.GracefulStop()
		gp.logger.Println("gRPC server stopped")
	}()
	gp.logger.Println("gRPC server listening at", lis.Addr())
	if err_ := server.Serve(lis); err_ != nil {
		gp.logger.Fatal("failed to serve:", err_)
	}
}

// register keeps alive the registration in etcd and revokes it when shutdown gracefully.
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
	key := gp.opts.Prefix + gp.self
	_, err = cli.Put(context.Background(), key, "", clientv3.WithLease(lease.ID))
	if err != nil {
		gp.logger.Fatal("failed to put key:", err)
	}
	for {
		select {
		case _, ok := <-leaseResCh:
			if !ok {
				gp.logger.Fatal("failed to maintain lease")
			}
		case <-ctx.Done():
			cli.Revoke(context.Background(), lease.ID)
			gp.logger.Println("Unregistered immediately from the registry (might failed)")
			return
		}
	}
}

// discover finds out all peers from etcd if changes happen, then rebuilds the hash ring ans protoPeers with them.
// All running gRPC clients will be stopped when shutdown gracefully.
func (gp *GrpcPool) discover(ctx context.Context, wg *sync.WaitGroup, cli *clientv3.Client, ch chan struct{}) {
	defer wg.Done()
	watchChan := cli.Watch(context.Background(), gp.opts.Prefix, clientv3.WithPrefix())
	close(ch)
	defer gp.dscMsgCon.Close()
	for {
		select {
		case <-watchChan:
			gp.dscMsgCon.Send()
		case <-gp.dscMsgCon.Recv():
			res, err := cli.Get(context.Background(), gp.opts.Prefix, clientv3.WithPrefix())
			if err != nil {
				gp.logger.Fatal("failed to retrieve service list:", err)
			}
			var addrs []string
			for _, kv := range res.Kvs {
				addrs = append(addrs, strings.TrimPrefix(string(kv.Key), gp.opts.Prefix))
			}
			gp.setPeers(addrs)
		case <-ctx.Done():
			closeAll(gp.protoPeers)
			gp.logger.Println("All gRPC clients stopped")
			return
		}
	}
}

func (gp *GrpcPool) setPeers(addrs []string) {
	gp.muPeers.Lock()
	defer gp.muPeers.Unlock()
	old := gp.protoPeers
	closeAll(old)
	gp.protoPeers = make(map[string]*protoPeer)
	for _, addr := range addrs {
		if addr != gp.self {
			gp.protoPeers[addr] = newProtoPeer(addr, gp.logger)
		}
	}
	gp.peers = consistenthash.New(gp.opts.Replicas, gp.opts.HashFn)
	gp.peers.Add(addrs...)
}

func closeAll(ps map[string]*protoPeer) {
	for _, p := range ps {
		p.close()
	}
}
