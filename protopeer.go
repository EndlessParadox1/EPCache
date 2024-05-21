package epcache

import (
	"context"
	"log"

	pb "github.com/EndlessParadox1/epcache/epcachepb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type protoPeer struct {
	addr  string
	inCh  chan *in
	clsCh chan struct{}
}

type in struct {
	ctx   context.Context
	req   *pb.Request
	outCh chan *out
}

type out struct {
	res *pb.Response
	err error
}

func newProtoPeer(addr string, logger *log.Logger) *protoPeer {
	p := &protoPeer{
		addr:  addr,
		inCh:  make(chan *in),
		clsCh: make(chan struct{}),
	}
	go p.run(logger)
	return p
}

// run starts a long-running gRPC client for every cluster node,
// which avoids performance loss caused by repeatedly establishing TCP connections. TODO
func (p *protoPeer) run(logger *log.Logger) {
	conn, err := grpc.NewClient(p.addr, grpc.WithTransportCredentials(insecure.NewCredentials())) // disable TLS
	if err != nil {
		logger.Fatal("failed to connect to gRPC server")
	}
	defer conn.Close()
	client := pb.NewEPCacheClient(conn)
	for {
		select {
		case in_ := <-p.inCh:
			go func() {
				res, err_ := client.Get(in_.ctx, in_.req)
				in_.outCh <- &out{
					res: res,
					err: err_,
				}
			}()
		case <-p.clsCh:
			return
		}
	}
}

func (p *protoPeer) close() {
	close(p.clsCh)
}

func (p *protoPeer) Get(ctx context.Context, req *pb.Request) (*pb.Response, error) {
	outCh := make(chan *out)
	p.inCh <- &in{
		ctx:   ctx,
		req:   req,
		outCh: outCh,
	}
	out_ := <-outCh
	return out_.res, out_.err
}
