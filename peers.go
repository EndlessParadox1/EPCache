package epcache

import pb "github.com/EndlessParadox1/epcache/epcachepb"

type PeerGetter interface {
	Get(in *pb.Request, out *pb.Response) error
}

type PeerPiker interface {
	PickPeer(key string) (PeerGetter, bool)
}
