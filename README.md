# EPCache

> EPCache is Endless Paradox's cache, Experiment-Purpose cache and Enhanced-Performance cache. 
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
│                  which provides benefit of decoupling from data source and preventing users from 
│                  accidentally modifying the EPCache cluster's data.
├── cache.go: wraps a lru cache and its operators, using Mutex to provide concurrent safety 
│               and recording relevant statistical data. 
├── epcache.go: group, the orgainizational form of data, provides APIs to an EPCache cluster node, 
│                 like Get, OnUpdate and OnDelete etc. The ratelimiter and bloomfilter here can be enable and
│                 disabled at any time.
├── getter.go: provides the interface Getter, which must be implemented by users to access to the data source.
├── grpc.go: implements GrpcPool as a PeerAgent, which communicates with other nodes using gRPC, 
│              it will automatically deal with the service registration and discovery work based on etcd,
│              and of course satrt a gRPC server, all of which support graceful shutdown.
├── peers.go: provides the interface standards of ProtoPeer and PeerAgent, which are responsible for
│               the interation work among the EPCache cluster nodes; also implements NoPeer as a PeerAgent.
└── protopeer.go: implements protoPeer as a ProtoPeer with a gRPC client, which is used by GrpcPool.
```

## Procedure

```
                         y
Get -----> cache hit? -----> retrun
            | n                                  
            |----> consistent hashing 
                    |                 y                                 y
                    |----> remote? -----> load from peer -----> suc? -----> popuate hot cache in 1/10 chance -----> return
                            | n                                  | n                            y             
                            |                                    |----> due to non-existent? -----> return error
                            |                                            | n                                  
                            |                                            |                            y
                            |-----------------------------------------> load locally -----> exist? -----> popuate main cache -----> return
                                                                                             | n                                  
                                                                                             |----> return error
OnUpdate/OnDelete -----> opt locally if exist
                          | go                                  
                          |----> sync to peers 
                                  | go all
                                  |                                      y             
                                  |----> sync to one peer -----> suc? -----> opt remotely if exist
                                                                  | n                                  
                                                                  |----> collet 
                                                                          | then
                                                                          |
                                                                          |----> log all
```

## Highlights

1. Using gRPC/ProtoBuf to achieve efficient communication between nodes: request forwarding and data synchronization.
2. Implementing cache elimination strategy based on LRU, and implement load balancing based on consistent hashing.
3. Using mutexes and semaphores to prevent cache penetration, and providing bloom filters to prevent cache penetration.
4. Providing the token bucket algorithm to limit the request flow of the cache system.
5. Implementing service registration and service discovery based on etcd to achieve synchronization 
   when nodes and their groups are dynamically adjusted.

## Guide

1. You can build up a cache system as you like by importing this module.
2. Getter is implemented by you, a normal one might be using DB as data source.
3. ProtoPeer and PeerAgent can also be implemented by you, and using protoPeer and GrpcPool is recommended
   when attempting to build up a cluster, as well as NoPeer when you just need standalone one.
4. Set up ratelimiter and bloomfilter when you need them.
5. The GrpcPool will log something important when up.
6. An API server is the best practise to be built in front of an EPCache cluster node.

## Contributing

Issues and Pull Requests are accepted. Feel free to contribute to this project.

# Benchmark

* On standalone mode: 56k qps for non-existent keys, and 143k qps for existing keys.


## License

[MIT © EndlessParadox1](./LICENSE)
