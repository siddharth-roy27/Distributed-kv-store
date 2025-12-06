package main

import (
    "context"
    "log"
    "time"

    "google.golang.org/grpc"
    pb "github.com/siddharth-roy27/distributed-kv-store/proto"
)

func main() {
    // Connect to the KV server
    conn, err := grpc.Dial("localhost:50051", grpc.WithInsecure())
    if err != nil {
        log.Fatalf("failed to connect: %v", err)
    }
    defer conn.Close()

    client := pb.NewKVClient(conn)

    // Example: Set a key-value pair
    setResp, err := client.Set(context.Background(), &pb.SetRequest{
        Key:   "name",
        Value: "Siddharth",
    })
    if err != nil {
        log.Fatalf("Set RPC failed: %v", err)
    }
    log.Printf("Set Success: %v", setResp.Success)

    // Wait a bit
    time.Sleep(1 * time.Second)

    // Example: Get the value for a key
    getResp, err := client.Get(context.Background(), &pb.GetRequest{
        Key: "name",
    })
    if err != nil {
        log.Fatalf("Get RPC failed: %v", err)
    }

    if getResp.Found {
        log.Printf("Get Response: key=%s, value=%s", "name", getResp.Value)
    } else {
        log.Printf("Key not found")
    }
}
