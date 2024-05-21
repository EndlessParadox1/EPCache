package epcache

import (
	"context"
	"log"
	"sync"

	pb "github.com/EndlessParadox1/epcache/epcachepb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type protoPeer struct {
	addr  string
	inCh  chan *In
	clsCh chan struct{}
}

type In struct {
	ctx   context.Context
	req   *pb.Request
	outCh chan *Out
}

type Out struct {
	res *pb.Response
	err error
}

func newProtoPeer(addr string, logger *log.Logger) *protoPeer {
	p := &protoPeer{
		addr:  addr,
		inCh:  make(chan *In),
		clsCh: make(chan struct{}),
	}
	go p.run(logger)
	return p
}

func (p *protoPeer) run(logger *log.Logger) {
	var wg sync.WaitGroup
	for range 8 {
		conn, err := grpc.NewClient(p.addr, grpc.WithTransportCredentials(insecure.NewCredentials())) // disable tls
		if err != nil {
			logger.Fatal("failed to connect to gRPC server")
		}
		client := pb.NewEPCacheClient(conn)
		wg.Add(1)
		go func() {
			defer conn.Close()
			for {
				select {
				case in := <-p.inCh:
					go func() {
						res, err_ := client.Get(in.ctx, in.req)
						in.outCh <- &Out{
							res: res,
							err: err_,
						}
					}()
				case <-p.clsCh:
					wg.Done()
					return
				}
			}
		}()
	}
	wg.Wait()
} // TODO

func (p *protoPeer) Close() {
	close(p.clsCh)
}

func (p *protoPeer) Get(ctx context.Context, req *pb.Request) (*pb.Response, error) {
	outCh := make(chan *Out)
	p.inCh <- &In{ctx: ctx, req: req, outCh: outCh}
	out := <-outCh
	return out.res, out.err
}
