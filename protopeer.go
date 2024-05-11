package epcache

import (
	"context"
	pb "github.com/EndlessParadox1/epcache/epcachepb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type protoPeer struct {
	addr string
	ch   chan *pb.Request
	ch2  chan *pb.Response
}

func newProtoPeer(addr string) *protoPeer {
	p := &protoPeer{addr: addr, ch: make(chan *pb.Request)}
	go p.run()
	return p
}

func (p *protoPeer) run() {
	conn, err := grpc.NewClient(p.addr, grpc.WithTransportCredentials(insecure.NewCredentials())) // disable tls
	if err != nil {
		return
	}
	defer conn.Close()
	client := pb.NewEPCacheClient(conn)
	stream, _ := client.Get(context.Background())
	for {
		in := <-p.ch
		stream.Send(in)
		ret, _ := stream.Recv()
		p.ch2 <- ret
	}
}

func (p *protoPeer) Get(_ context.Context, in *pb.Request) (*pb.Response, error) {
	p.ch <- in
	return <-p.ch2, nil
}
