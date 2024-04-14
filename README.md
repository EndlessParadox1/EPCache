# EPCache

> A distributed cache implemented by go.

## Structure
```
epcache
    |--consistenthash
        |--consistenthash.go // 一致性哈希节点选择策略
    |--epcachepb
        |--epcachepb.pb.go  // protobuf 具体实现
        |--epcachepb.proto  // protobuf 数据结构的定义
    |--lru
        |--lru.go  // lru 缓存淘汰策略
    |--byteview.go // 缓存值的抽象与封装
    |--cache.go    // 并发控制
    |--epcache.go // 负责与外部交互，控制缓存存储和获取的主流程
    |--http.go     // 提供节点间通讯的能力(基于http)
```

## Procedure
```

```

## Usage 

Only work in a 64-bits machine.
