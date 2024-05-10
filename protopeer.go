package epcache

import (
	"context"
	pb "github.com/EndlessParadox1/epcache/epcachepb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

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
