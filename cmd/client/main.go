package main

import (
    "context"
    "flag"
    "log"

    pb "github.com/siddharth-roy27/distributed-kv-store/proto/kv"
    "google.golang.org/grpc"
)

func main() {
    target := flag.String("target", "localhost:50051", "KV gRPC server address")
    flag.Parse()

    conn, err := grpc.Dial(*target, grpc.WithInsecure())
    if err != nil {
        log.Fatalf("failed to connect: %v", err)
    }
    defer conn.Close()

    client := pb.NewKVClient(conn)

    // Test Set
    setResp, err := client.Set(context.Background(), &pb.SetRequest{
        Key:   "name",
        Value: "Siddharth",
    })
    if err != nil {
        log.Fatalf("Set RPC failed: %v", err)
    }
    log.Printf("Set Success: %v", setResp.Success)

    // Test Get
    getResp, err := client.Get(context.Background(), &pb.GetRequest{
        Key: "name",
    })
    if err != nil {
        log.Fatalf("Get RPC failed: %v", err)
    }
    log.Printf("Get Response: key=%s, value=%s, found=%v", "name", getResp.Value, getResp.Found)
}
