# EPCache

> A lightweight and highly customizable distributed cache system construction package implemented by golang. 
> Data synchronization between nodes is asynchronous, with high concurrent access capabilities and ease of use.

## Structure
```
.
├── bloomfilter
│   └── bloomfilter.go: implements a bloomfilter using bitmap and murmur3, working as a blacklist 
│                         to avoid cache penetration, which might cause a false potive problem.
├── consistenthash
│   └── consistenthash.go: implements a hash ring mapping reqs to a specific node having the same group,
│                            which establishes a basic load balance of the EPCache cluster.
├── epcachepb
│   └── epcachepb.proto: defines the protobuf messages and service used by ProtoPeer and PeerAgent.
├── etcd
│   └── startup.sh: provides an example to start an etcd cluster.
├── lru
│   └── lru.go: implements a lru cache.
├── singleflight
│   └── singleflight.go: provides a duplicate func call suppression mechanism using Mutex and WaitGroup,
│                          to avoid cache breakdown.
├── byteview.go: implements an immutable view of bytes, used inside the EPCache cluster and presented to users,
│                  which provides benefit of decoupling from source data and preventing users from 
│                  accidentally modifying the EPCache cluster's data.
├── cache.go: wraps a lru cache and its operators, using Mutex to provide concurrent safety 
│               and recording relevant statistical data. 
├── epcache.go: 
├── getter.go: 
├── grpc.go: 
├── peers.go: provides the interface standards of ProtoPeer and PeerAgent, which are responsible for
│               the interation work among the EPCache cluster nodes; also implements a PeerAgent: NoPeer.
└── protopeer.go: 
```

## Procedure

## Highlights

## Guide

## Contributing

Issues and Pull Requests are accepted. Feel free to contribute to this project.

## License

[MIT © EndlessParadox1](./LICENSE)
