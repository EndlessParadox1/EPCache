package epcache

import (
	"context"

	pb "github.com/EndlessParadox1/epcache/epcachepb"
)

type ProtoPeer interface {
	// Get loads data from remote using gRPC.
	Get(ctx context.Context, in *pb.Request) (*pb.Response, error)
	// SyncOne trys to sync data to remote using gRPC.
	SyncOne(data *pb.SyncData, ch chan<- error)
}

type PeerAgent interface {
	// PickPeer picks peer according to the key.
	PickPeer(key string) (ProtoPeer, bool)
	// SyncAll trys to sync data to all peers.
	SyncAll(data *pb.SyncData)
	// ListPeers lists all peers.
	ListPeers() []string
}

// NoPeer is an implementation of PeerAgent, used for groups running in standalone mode.
type NoPeer struct{}

func (NoPeer) PickPeer(_ string) (peer ProtoPeer, ok bool) {
	return
}

func (NoPeer) SyncAll(_ *pb.SyncData) {}

func (NoPeer) ListPeers() (ans []string) {
	return
}
