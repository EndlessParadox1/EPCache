package epcache

import (
	"context"

	pb "github.com/EndlessParadox1/epcache/epcachepb"
)

// ProtoGetter loads data from remote using gRPC.
type ProtoGetter interface {
	Get(ctx context.Context, in *pb.Request) (*pb.Response, error)
}

type PeerPicker interface {
	// PickPeer picks peer according to the key.
	PickPeer(key string) (ProtoGetter, bool)
}

// NoPeer is an implementation of PeerPicker, used for groups running in standalone mode.
type NoPeer struct{}

func (NoPeer) PickPeer(_ string) (peer ProtoGetter, ok bool) {
	return
}
