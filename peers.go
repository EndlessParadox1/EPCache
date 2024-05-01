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
	// PickPeer picks peer with the same group according to the key.
	PickPeer(group, key string) (ProtoPeer, bool)
	// SyncAll trys to sync data to all peers.
	SyncAll(data *pb.SyncData)
	// EnrollGroup enrolls the group.
	EnrollGroup(group string)
	// WithDrawGroup withdraws the group.
	WithDrawGroup(group string)
	// ListPeers lists all peers.
	ListPeers() []string
}

// NoPeer is an implementation of PeerAgent, used for groups running in standalone mode.
type NoPeer struct{}

func (NoPeer) PickPeer(_, _ string) (peer ProtoPeer, ok bool) {
	return
}

func (NoPeer) SyncAll(_ *pb.SyncData) {}

func (NoPeer) EnrollGroup(_ string) {}

func (NoPeer) WithDrawGroup(_ string) {}

func (NoPeer) ListPeers() (ans []string) {
	return
}
