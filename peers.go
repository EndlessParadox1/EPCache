package epcache

import (
	"context"

	pb "github.com/EndlessParadox1/epcache/epcachepb"
)

// ProtoPeer loads data from remote using gRPC.
type ProtoPeer interface {
	Get(ctx context.Context, in *pb.Request) (*pb.Response, error)
	SyncOne(data *pb.SyncData, ch chan<- error)
}

type PeerAgent interface {
	// PickPeer picks peer according to the key.
	PickPeer(key string) (ProtoPeer, bool)
	SyncAll(data *pb.SyncData) error
}

// NoPeer is an implementation of PeerPicker, used for groups running in standalone mode.
type NoPeer struct{}

func (NoPeer) PickPeer(_ string) (peer ProtoPeer, ok bool) {
	return
}

func (NoPeer) SyncAll(_ *pb.SyncData) (err error) {
	return
}

// TODO
