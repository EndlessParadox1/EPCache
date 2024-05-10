package epcache

import (
	"context"
	"google.golang.org/grpc/resolver"
	"log"
	"time"

	"google.golang.org/grpc"

	"go.etcd.io/etcd/client/v3"
)

func main() {
	// 注册 etcd 解析器构建器
	resolver.Register(&etcdResolverBuilder{})

	// 创建 gRPC 连接
	conn, err := grpc.Dial("etcd:///your_service_name", grpc.WithInsecure())
	if err != nil {
		log.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	// 创建 gRPC 客户端
	client := pb.NewYourServiceClient(conn)

	// 开始向每个服务器发送消息
	startStreams(client)
}

// startStreams 函数用于开始流式处理消息
func startStreams(client pb.YourServiceClient) {
	// 获取所有服务器的地址
	addresses := getServerAddresses()

	// 建立流连接并发送消息
	for _, address := range addresses {
		go func(addr string) {
			// 建立流连接
			stream, err := client.SendMessages(context.Background())
			if err != nil {
				log.Fatalf("failed to open stream: %v", err)
			}
			defer stream.CloseSend()

			// 发送消息
			for {
				// 构造消息
				message := &pb.Message{
					Data: "Your message content",
				}

				// 发送消息
				if err := stream.Send(message); err != nil {
					log.Printf("failed to send message to %s: %v", addr, err)
					return
				}

				// 模拟发送的间隔时间
				time.Sleep(time.Second)
			}
		}(address)
	}
}

// getServerAddresses 函数用于从 etcd 中获取所有服务器的地址
func getServerAddresses() []string {
	// 实现获取服务器地址的逻辑，这里假设从 etcd 中获取
	return []string{"server1_address", "server2_address", "server3_address"}
}

// etcdResolverBuilder 是 etcd 解析器构建器
type etcdResolverBuilder struct{}

// Build 构建解析器
func (*etcdResolverBuilder) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOption) (resolver.Resolver, error) {
	r := &etcdResolver{
		target: target,
		cc:     cc,
	}
	r.start()
	return r, nil
}

// Scheme 返回解析器的协议方案
func (*etcdResolverBuilder) Scheme() string {
	return "etcd"
}

// etcdResolver 是 etcd 解析器
type etcdResolver struct {
	target resolver.Target
	cc     resolver.ClientConn
}

// start 启动解析器
func (r *etcdResolver) start() {
	// 连接 etcd
	cli, err := clientv3.New(clientv3.Config{
		Endpoints: []string{"your_etcd_endpoints"},
	})
	if err != nil {
		log.Fatalf("failed to connect to etcd: %v", err)
	}
	defer cli.Close()

	// 监听 etcd 中服务的变化
	rch := cli.Watch(context.Background(), "your_service_name", clientv3.WithPrefix())
	for wresp := range rch {
		for _, ev := range wresp.Events {
			log.Printf("etcd event received: %s %q : %q\n", ev.Type, ev.Kv.Key, ev.Kv.Value)
			// 解析 etcd 事件，并更新连接状态
			// 这里可以获取到服务的节点信息，然后更新连接状态
			// 参考 etcd 的 API 文档：https://pkg.go.dev/go.etcd.io/etcd/clientv3#Event
			// 更新连接状态的方式为：r.cc.NewAddress()、r.cc.NewServiceConfig()
			// 例如：r.cc.NewAddress([]resolver.Address{{Addr: "127.0.0.1:50051"}})
		}
	}
}

// ResolveNow 实现 ResolveNow 方法
func (*etcdResolver) ResolveNow(resolver.ResolveNowOptions) {}

// Close 实现 Close 方法
func (*etcdResolver) Close() {}
