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

type GrpcPool struct {
	self     string
	prefix   string
	registry []string
	opts     GrpcPoolOptions
	logger   *log.Logger
	pb.UnimplementedEPCacheServer

	ch        chan *pb.SyncData
	msgBroker string

	node      *Node
	dscMsgCon *msgctl.MsgController

	muPeers    sync.RWMutex
	peers      *consistenthash.Map
	protoPeers map[string]*protoPeer // keys like "localhost:8080"
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
func NewGrpcPool(self, prefix string, registry []string, msgBroker string, opts *GrpcPoolOptions) *GrpcPool {
	if grpcPoolExist {
		panic("NewGrpcPool called more than once")
	}
	grpcPoolExist = true
	gp := &GrpcPool{
		self:       self,
		prefix:     prefix,
		registry:   registry,
		msgBroker:  msgBroker,
		logger:     log.New(os.Stdin, "[EPCache] ", log.LstdFlags),
		dscMsgCon:  msgctl.New(time.Second), // TODO
		protoPeers: make(map[string]*protoPeer),
	}
	if opts != nil {
		gp.opts = *opts
	}
	if gp.opts.Replicas == 0 {
		gp.opts.Replicas = defaultReplicas
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

// SyncAll trys to sync data to all peers in an async way, and logs error if any.
func (gp *GrpcPool) SyncAll(data *pb.SyncData) {
	gp.ch <- data
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

var bufferPool = sync.Pool{
	New: func() interface{} {
		return new(pb.Response)
	},
}

// Implementing GrpcPool as the EPCacheServer.

func (gp *GrpcPool) Get(ctx context.Context, req *pb.Request) (*pb.Response, error) {
	atomic.AddInt64(&gp.node.Stats.PeerReqs, 1)
	val, err := gp.node.Get(ctx, req.Key)
	if err != nil {
		return nil, err
	}
	b := bufferPool.Get().(*pb.Response)
	b.Value = val.ByteSlice()
	return b, nil
}

// run starts a node of the EPCache cluster.
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
	// This ensures that the first service registration from self
	// can be caught by service discovery's watch.
	ch := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go gp.register(ctx, &wg, cli, ch)
	go gp.discover(cli, ch)
	wg.Add(1)
	go gp.startServer(ctx, &wg)
	go gp.producer() // TODO
	go gp.consumer()
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
	_, err = cli.Put(context.Background(), key, "", clientv3.WithLease(lease.ID))
	gp.logger.Println("put self")
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

// discover will find out all peers and the groups they owned from etcd when changes happen.
func (gp *GrpcPool) discover(cli *clientv3.Client, ch chan struct{}) {
	watchChan := cli.Watch(context.Background(), gp.prefix, clientv3.WithPrefix())
	close(ch)
	go func() {
		for range watchChan {
			gp.dscMsgCon.Send()
		}
	}()
	for {
		<-gp.dscMsgCon.Recv()
		gp.logger.Println("find one")
		res, err := cli.Get(context.Background(), gp.prefix, clientv3.WithPrefix())
		if err != nil {
			gp.logger.Fatal("failed to retrieve service list:", err)
		}
		var addrs []string
		for _, kv := range res.Kvs {
			addrs = append(addrs, strings.TrimPrefix(string(kv.Key), gp.prefix))
		}
		gp.setPeers(addrs)
	}
}

func (gp *GrpcPool) setPeers(addrs []string) {
	gp.muPeers.Lock()
	defer gp.muPeers.Unlock()
	old := gp.protoPeers
	go closeAll(old)
	gp.protoPeers = make(map[string]*protoPeer)
	for _, addr := range addrs {
		if addr != gp.self {
			gp.protoPeers[addr] = newProtoPeer(addr)
		}
	}
	gp.peers = consistenthash.New(gp.opts.Replicas, gp.opts.HashFn)
	gp.peers.Add(addrs...)
}

func closeAll(ps map[string]*protoPeer) {
	for _, p := range ps {
		p.Close()
	}
}
