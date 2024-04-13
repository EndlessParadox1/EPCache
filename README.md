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
                          是
接收 key --> 检查是否被缓存 -----> 返回缓存值
                 |  否                        
                 |-----> 使用一致性哈希选择节点             是                                   是
                                 |-----> 是否是远程节点 -----> HTTP 客户端访问远程节点 --> 是否成功-----> 服务端返回混存值
                                              |  否                                     ↓  否
                                              |---------------------------------> 回退到本地节点处理
                                                                                         |-----> 调用`回调函数`，获取值并添加到缓存 --> 返回缓存值 
```

## Usage 

Only work in a 64-bits machine.
