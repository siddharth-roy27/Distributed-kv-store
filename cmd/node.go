package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	"github.com/siddharth-roy27/distributed-kv-store/internal/raft"
	"github.com/siddharth-roy27/distributed-kv-store/internal/state_machine"

	"google.golang.org/grpc"

	pb "github.com/siddharth-roy27/distributed-kv-store/proto/kv"
	raftpb "github.com/siddharth-roy27/distributed-kv-store/proto/raft"
)

// KV gRPC adapter backed by Raft
type server struct {
	pb.UnimplementedKVServer
	raftNode      *raft.RaftNode
	nodeIDToGRPC  map[string]string // Map node ID to gRPC address
	selfGRPCAddr  string            // This node's gRPC address
}

func (s *server) Get(ctx context.Context, req *pb.GetRequest) (*pb.GetResponse, error) {
	if s.raftNode == nil || s.raftNode.Store == nil {
		return &pb.GetResponse{Found: false}, nil
	}
	v, ok := s.raftNode.Store.Get(req.Key)
	return &pb.GetResponse{Value: v, Found: ok}, nil
}

func (s *server) Set(ctx context.Context, req *pb.SetRequest) (*pb.SetResponse, error) {
	if s.raftNode == nil {
		return &pb.SetResponse{Success: false}, nil
	}
	// If not leader, forward to known leader
	if !s.raftNode.IsLeader() {
		leaderID := s.raftNode.LeaderAddr()
		if leaderID == "" {
			return &pb.SetResponse{Success: false}, nil
		}
		// Get gRPC address for leader
		leaderGRPC, ok := s.nodeIDToGRPC[leaderID]
		if !ok {
			// Leader not in our map, can't forward
			return &pb.SetResponse{Success: false}, nil
		}
		// Forward using gRPC KV client
		conn, err := grpc.Dial(leaderGRPC, grpc.WithInsecure())
		if err != nil {
			return &pb.SetResponse{Success: false}, nil
		}
		defer conn.Close()
		kc := pb.NewKVClient(conn)
		ctx2, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		resp, err := kc.Set(ctx2, &pb.SetRequest{Key: req.Key, Value: req.Value})
		if err != nil {
			return &pb.SetResponse{Success: false}, nil
		}
		return &pb.SetResponse{Success: resp.Success}, nil
	}

	// If leader, propose the set to Raft
	ok := s.raftNode.ProposeSet(req.Key, req.Value)
	return &pb.SetResponse{Success: ok}, nil
}

func normalizePeers(peersFlag string) []string {
	peers := []string{}
	if peersFlag == "" {
		return peers
	}
	for _, p := range strings.Split(peersFlag, ",") {
		t := strings.TrimSpace(p)
		if t == "" {
			continue
		}
		if !strings.Contains(t, ":") {
			t = "localhost:" + t
		}
		peers = append(peers, t)
	}
	return peers
}

// buildNodeIDToGRPC creates a mapping from node IDs to gRPC addresses
// This assumes a convention: node1->50051, node2->50052, etc.
// For a production system, this should be passed as configuration
func buildNodeIDToGRPC(nodeID string, kvPort string, peers []string) map[string]string {
	m := make(map[string]string)
	// Add self
	selfGRPC := fmt.Sprintf("localhost:%s", kvPort)
	m[nodeID] = selfGRPC

	// Try to infer peer gRPC addresses from their Raft ports
	// Convention: if Raft port is 8001, gRPC port is 50051 (offset 42050)
	// Or: if Raft port is 8002, gRPC port is 50052, etc.
	// For now, we use hardcoded mapping for common 3-node setup

	// For a 3-node cluster with standard setup:
	// node1: Raft 8001, gRPC 50051
	// node2: Raft 8002, gRPC 50052
	// node3: Raft 8003, gRPC 50053
	// We'll populate based on common patterns
	if nodeID == "node1" {
		m["node2"] = "localhost:50052"
		m["node3"] = "localhost:50053"
	} else if nodeID == "node2" {
		m["node1"] = "localhost:50051"
		m["node3"] = "localhost:50053"
	} else if nodeID == "node3" {
		m["node1"] = "localhost:50051"
		m["node2"] = "localhost:50052"
	} else {
		// Try to infer from port numbers
		// If kvPort is 50052, we might be node2, so node1 is 50051, node3 is 50053
		// This is a heuristic and may not work for all setups
	}

	return m
}

func main() {
	nodeID := flag.String("id", "node1", "Raft node ID")
	raftPort := flag.String("port", "8001", "Raft peer port")
	kvPort := flag.String("grpc-port", "50051", "KV gRPC server port")
	peersFlag := flag.String("peers", "", "Comma-separated list of peer host:ports")
	flag.Parse()

	peers := normalizePeers(*peersFlag)

	// Create Raft node
	rn := &raft.RaftNode{
		ID:    *nodeID,
		Peers: peers,
		Store: state_machine.NewStore(),
	}
	rn.StartRaft()

	log.Printf("Raft node started with ID: %s, listening on :%s (peers=%v)", *nodeID, *raftPort, peers)

	// Start Raft gRPC server (for inter-node communication)
	raftLis, err := net.Listen("tcp", fmt.Sprintf(":%s", *raftPort))
	if err != nil {
		log.Fatalf("failed to listen on Raft port %s: %v", *raftPort, err)
	}
	raftService := raft.NewRaftService(rn)
	raftGrpcServer := grpc.NewServer()
	raftpb.RegisterRaftServer(raftGrpcServer, raftService)
	go func() {
		log.Printf("Raft gRPC server listening on :%s", *raftPort)
		if err := raftGrpcServer.Serve(raftLis); err != nil {
			log.Fatalf("failed to serve Raft gRPC: %v", err)
		}
	}()

	// Build node ID to gRPC address mapping
	nodeIDToGRPC := buildNodeIDToGRPC(*nodeID, *kvPort, peers)
	selfGRPCAddr := fmt.Sprintf("localhost:%s", *kvPort)

	// Start KV gRPC server
	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", *kvPort))
	if err != nil {
		log.Fatalf("failed to listen on KV gRPC port %s: %v", *kvPort, err)
	}
	grpcServer := grpc.NewServer()
	pb.RegisterKVServer(grpcServer, &server{
		raftNode:     rn,
		nodeIDToGRPC: nodeIDToGRPC,
		selfGRPCAddr: selfGRPCAddr,
	})

	log.Printf("KV gRPC server listening on :%s", *kvPort)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve KV gRPC: %v", err)
	}
}
