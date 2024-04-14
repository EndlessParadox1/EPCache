package epcache

import (
	"context"

	pb "github.com/EndlessParadox1/epcache/epcachepb"
)

// PeerGetter loads data from remote using gRPC.
type PeerGetter interface {
	Get(ctx context.Context, in *pb.Request, out *pb.Response) error
}

type PeerPiker interface {
	PickPeer(key string) (PeerGetter, bool)
}

// NoPeer is an implementation of PeerPicker, used for processes in standalone mode.
type NoPeer struct{}

func (NoPeer) PickPeer(_ string) (peer PeerGetter, ok bool) {
	return
}
