package main

import (
    "context"
    "log"
    "net"

    "google.golang.org/grpc"
    pb "github.com/siddharth-roy27/distributed-kv-store/proto"
)

// In-memory key-value store
var store = make(map[string]string)

// KV service implementation
type server struct {
    pb.UnimplementedKVServer
}

// Get RPC implementation
func (s *server) Get(ctx context.Context, req *pb.GetRequest) (*pb.GetResponse, error) {
    value, ok := store[req.Key]
    return &pb.GetResponse{
        Value: value,
        Found: ok,
    }, nil
}

// Set RPC implementation
func (s *server) Set(ctx context.Context, req *pb.SetRequest) (*pb.SetResponse, error) {
    store[req.Key] = req.Value
    return &pb.SetResponse{
        Success: true,
    }, nil
}

func main() {
    // Listen on port 50051
    lis, err := net.Listen("tcp", ":50051")
    if err != nil {
        log.Fatalf("failed to listen: %v", err)
    }

    // Create gRPC server
    grpcServer := grpc.NewServer()

    // Register KV service
    pb.RegisterKVServer(grpcServer, &server{})

    log.Println("KV gRPC server listening on :50051")

    // Start serving
    if err := grpcServer.Serve(lis); err != nil {
        log.Fatalf("failed to serve: %v", err)
    }
}
