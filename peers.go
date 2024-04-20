package epcache

import (
	"context"

	pb "github.com/EndlessParadox1/epcache/epcachepb"
)

// PeerGetter loads data from remote using gRPC.
type peerGetter interface {
	get(ctx context.Context, in *pb.Request) (*pb.Response, error)
}

type PeerPicker interface {
	// PickPeer picks peer according to the key.
	pickPeer(key string) (peerGetter, bool)
	addGroup(group string)
}

// NoPeer is an implementation of PeerPicker, used for groups running in standalone mode.
type NoPeer struct{}

func (NoPeer) pickPeer(_ string) (peer peerGetter, ok bool) {
	return
}

func (NoPeer) addGroup(_ string) {}
