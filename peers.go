package epcache

import (
	"context"

	pb "github.com/EndlessParadox1/epcache/epcachepb"
)

type ProtoPeer interface {
	// Get loads data from remote using gRPC.
	Get(ctx context.Context, req *pb.Request) (*pb.Response, error)
}

type PeerAgent interface {
	// PickPeer picks peer according to the key.
	PickPeer(key string) (ProtoPeer, bool)
	// SyncAll trys to sync data to all peers.
	SyncAll(data *pb.SyncData)
	SetNode(node *Node)
	// ListPeers lists all peers.
	ListPeers() []string
}

// NoPeer is an implementation of PeerAgent, used for nodes running in standalone mode.
type NoPeer struct{}

func (NoPeer) PickPeer(_ string) (peer ProtoPeer, ok bool) {
	return
}

func (NoPeer) SyncAll(_ *pb.SyncData) {}

func (NoPeer) SetNode(_ *Node) {}

func (NoPeer) ListPeers() (ans []string) {
	return
}
