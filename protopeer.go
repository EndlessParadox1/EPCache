package epcache

import (
	"context"
	"errors"
	"log"

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

func (p *protoPeer) SyncOne(data *pb.SyncData, ch chan<- error) {
	conn, err := grpc.NewClient(p.addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		ch <- errors.New(p.addr)
		return
	}
	defer conn.Close()
	client := pb.NewEPCacheClient(conn)
	stream, err_ := client.Sync(context.Background())
	if err_ != nil {
		log.Fatal(err) // TODO
	}
	for _, point := range points {
		if err := stream.Send(point); err != nil {
			log.Fatalf("client.RecordRoute: stream.Send(%v) failed: %v", point, err)
		}
	}
	if err_ != nil {
		ch <- errors.New(p.addr)
		return
	}
	ch <- nil
}
