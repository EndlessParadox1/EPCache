// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.2.0
// - protoc             v5.26.1
// source: epcachepb.proto

package epcachepb

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

// EPCacheClient is the client API for EPCache service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type EPCacheClient interface {
	Get(ctx context.Context, opts ...grpc.CallOption) (EPCache_GetClient, error)
}

type ePCacheClient struct {
	cc grpc.ClientConnInterface
}

func NewEPCacheClient(cc grpc.ClientConnInterface) EPCacheClient {
	return &ePCacheClient{cc}
}

func (c *ePCacheClient) Get(ctx context.Context, opts ...grpc.CallOption) (EPCache_GetClient, error) {
	stream, err := c.cc.NewStream(ctx, &EPCache_ServiceDesc.Streams[0], "/epcachepb.EPCache/Get", opts...)
	if err != nil {
		return nil, err
	}
	x := &ePCacheGetClient{stream}
	return x, nil
}

type EPCache_GetClient interface {
	Send(*Request) error
	Recv() (*Response, error)
	grpc.ClientStream
}

type ePCacheGetClient struct {
	grpc.ClientStream
}

func (x *ePCacheGetClient) Send(m *Request) error {
	return x.ClientStream.SendMsg(m)
}

func (x *ePCacheGetClient) Recv() (*Response, error) {
	m := new(Response)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// EPCacheServer is the server API for EPCache service.
// All implementations must embed UnimplementedEPCacheServer
// for forward compatibility
type EPCacheServer interface {
	Get(EPCache_GetServer) error
	mustEmbedUnimplementedEPCacheServer()
}

// UnimplementedEPCacheServer must be embedded to have forward compatible implementations.
type UnimplementedEPCacheServer struct {
}

func (UnimplementedEPCacheServer) Get(EPCache_GetServer) error {
	return status.Errorf(codes.Unimplemented, "method Get not implemented")
}
func (UnimplementedEPCacheServer) mustEmbedUnimplementedEPCacheServer() {}

// UnsafeEPCacheServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to EPCacheServer will
// result in compilation errors.
type UnsafeEPCacheServer interface {
	mustEmbedUnimplementedEPCacheServer()
}

func RegisterEPCacheServer(s grpc.ServiceRegistrar, srv EPCacheServer) {
	s.RegisterService(&EPCache_ServiceDesc, srv)
}

func _EPCache_Get_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(EPCacheServer).Get(&ePCacheGetServer{stream})
}

type EPCache_GetServer interface {
	Send(*Response) error
	Recv() (*Request, error)
	grpc.ServerStream
}

type ePCacheGetServer struct {
	grpc.ServerStream
}

func (x *ePCacheGetServer) Send(m *Response) error {
	return x.ServerStream.SendMsg(m)
}

func (x *ePCacheGetServer) Recv() (*Request, error) {
	m := new(Request)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// EPCache_ServiceDesc is the grpc.ServiceDesc for EPCache service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var EPCache_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "epcachepb.EPCache",
	HandlerType: (*EPCacheServer)(nil),
	Methods:     []grpc.MethodDesc{},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "Get",
			Handler:       _EPCache_Get_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
	},
	Metadata: "epcachepb.proto",
}
