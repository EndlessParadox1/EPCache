# EPCache

> EPCache is Endless Paradox's Cache, Experiment-Purpose Cache and Enhanced-Performance Cache. 
  A lightweight and customizable distributed cache system developing framework implemented by golang. 
  Suitable for scenarios with more reads and less writes, has high concurrent access capabilities, 
  and ensures eventual consistency.

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
├── msgctl
│   └── msgctl.go: provides a tool for merging messages within a specified interval.
├── ratelimit
│   └── ratelimit.go: implements a token bucket for rate limiting.
├── singleflight
│   └── singleflight.go: provides a duplicate func call suppression mechanism using Mutex and WaitGroup,
│                          to avoid cache breakdown.
├── amqp
│   └── amqp.go: handles data synchronization across nodes based on the fanout pattern of an MQ instance.
├── byteview.go: implements an immutable view of bytes, used inside the EPCache cluster and presented to users,
│                  which provides benefit of decoupling from data source and preventing users from 
│                  accidentally modifying the EPCache cluster's data.
├── cache.go: wraps a lru cache and its operators, using Mutex to provide concurrent safety 
│               and recording relevant statistical data. 
├── epcache.go: provides APIs to an EPCache cluster node, like Get, OnUpdate and OnDelete etc. 
│                 The ratelimiter and bloomfilter here can be enable and disabled at any time.
├── getter.go: provides the interface Getter, which must be implemented by users to access to the data source.
├── grpc.go: implements GrpcPool as a PeerAgent, which communicates with other nodes using gRPC, 
│              it will deal with the service registration and discovery based on etcd,
│              the data synchronization sending and receiving based on MQ,
│              and of course start a gRPC server, all of which support graceful shutdown. 
├── peers.go: provides the interface standards of ProtoPeer and PeerAgent, which are responsible for
│               the interation work among the EPCache cluster nodes; also implements NoPeer as a PeerAgent.
└── protopeer.go: implements protoPeer as a ProtoPeer with a long-running gRPC client, which will be used by GrpcPool.
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
```

## Highlights

1. Using gRPC/ProtoBuf to achieve efficient communication between nodes: request forwarding and data synchronization.
2. Implementing cache elimination strategy based on LRU, and implement load balancing based on consistent hashing.
3. Use mutex and semaphore to prevent cache breakdown, Bloom filters to prevent cache penetration, 
   and token bucket algorithm to implement request rate limiting.
4. Asynchronous data synchronization across nodes based on AMQP-style message queue
   has the advantages of order guarantee and decoupling.
5. Implementing service registration and discovery based on etcd cluster to achieve dynamic adjustment of nodes.

## Guide

1. You can build up a cache system as you like by importing this module.
2. Getter is implemented by you, a normal one might be using DB as data source.
3. ProtoPeer and PeerAgent can also be implemented by you, and using protoPeer and GrpcPool are recommended
   if you attempt to build up a cluster, while NoPeer if you just need standalone one.
4. GrpcPool requires both a running etcd cluster instance and a running AMQP-style MQ instance. 
5. Set up ratelimiter and bloomfilter when you need them.
6. The GrpcPool will log something important when up.
7. An API server is the best practise to be built in front of an EPCache cluster node.

## Contributing

Issues and Pull Requests are accepted. Feel free to contribute to this project.

# Benchmark

* For 100k data entries, up to 3030k qps if cache hit locally and 33k qps if cache hit remotely 
  when used concurrently.
* For 100k data entries, as short as 633 ms to complete the writing regardless of the time spent on 
  retrieving them from source.
* For 100k data entries, as short as 595 ms to update all and publish messages related when used concurrently.
* For 100k data entries, as short as 1.72 s to consume messages related and update all.

## License

[MIT © EndlessParadox1](./LICENSE)
