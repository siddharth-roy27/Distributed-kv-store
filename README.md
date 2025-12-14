# Distributed Key-Value Store with Raft Consensus

[![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat&logo=go)](https://golang.org/)
[![gRPC](https://img.shields.io/badge/gRPC-1.77+-4285F4?style=flat&logo=grpc)](https://grpc.io/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

A production-ready, fault-tolerant distributed key-value store implementing the Raft consensus algorithm in Go. This project demonstrates deep understanding of distributed systems, consensus protocols, and building reliable distributed applications.

## 🎯 Project Overview

This is a fully functional distributed key-value store that uses the Raft consensus algorithm to ensure consistency across multiple nodes. The system can tolerate node failures, automatically elect leaders, and maintain data consistency even in the presence of network partitions.

### Why This Project?

I built this project to:
- **Deep dive into consensus algorithms**: Understanding Raft at the implementation level, not just theory
- **Build production-grade distributed systems**: Experience with real-world challenges like leader election, log replication, and fault tolerance
- **Master Go concurrency**: Leveraging goroutines, channels, and mutexes for concurrent distributed operations
- **Learn gRPC**: Building efficient inter-node communication with protocol buffers
- **Interview preparation**: Demonstrating systems design skills that are highly valued at top tech companies

This project showcases skills in:
- **Distributed Systems**: Consensus, replication, fault tolerance
- **Systems Programming**: Low-level concurrency, network programming
- **Software Engineering**: Clean architecture, error handling, testing

## ✨ Features

-  **Raft Consensus Algorithm**: Full implementation of leader election, log replication, and commit logic
-  **Fault Tolerance**: Handles node failures and network partitions gracefully
-  **Leader Election**: Automatic leader election with randomized timeouts
-  **Log Replication**: Majority-based replication with conflict resolution
-  **gRPC API**: High-performance RPC for both client operations and inter-node communication
-  **Client Forwarding**: Followers automatically forward write requests to the leader
-  **Strong Consistency**: Linearizable reads and writes guaranteed by Raft
-  **In-Memory Store**: Fast key-value operations with thread-safe access

##  Architecture

```
┌─────────────┐         ┌─────────────┐         ┌─────────────┐
│   Node 1    │◄───────►│   Node 2    │◄───────►│   Node 3    │
│  (Leader)   │  Raft   │  (Follower) │  Raft   │  (Follower) │
│             │  gRPC   │             │  gRPC   │             │
│  ┌───────┐  │         │  ┌───────┐  │         │  ┌───────┐  │
│  │ Raft  │  │         │  │ Raft  │  │         │  │ Raft  │  │
│  │ Node  │  │         │  │ Node  │  │         │  │ Node  │  │
│  └───┬───┘  │         │  └───┬───┘  │         │  └───┬───┘  │
│      │      │         │      │      │         │      │      │
│  ┌───▼───┐  │         │  ┌───▼───┐  │         │  ┌───▼───┐  │
│  │ Store │  │         │  │ Store │  │         │  │ Store │  │
│  └───────┘  │         │  └───────┘  │         │  └───────┘  │
└──────┬──────┘         └──────┬──────┘         └──────┬──────┘
       │                       │                       │
       │         KV gRPC (50051, 50052, 50053)        │
       └───────────────────────┼───────────────────────┘
                               │
                        ┌──────▼──────┐
                        │   Client    │
                        └─────────────┘
```

### Components

- **Raft Node**: Implements leader election, log replication, and commit logic
- **State Machine**: In-memory key-value store that applies committed log entries
- **gRPC Services**: 
  - KV Service: Client-facing API for Get/Set operations
  - Raft Service: Inter-node communication for consensus
- **Client**: Simple CLI tool to interact with the cluster

## 🛠️ Tech Stack

- **Go 1.25+**: Systems programming and concurrency
- **gRPC**: High-performance RPC framework
- **Protocol Buffers**: Efficient serialization
- **Raft Algorithm**: Consensus protocol for distributed systems

## 📦 Installation

### Prerequisites

- Go 1.25 or higher
- Git

### Clone the Repository

```bash
git clone https://github.com/siddharth-roy27/distributed-kv-store.git
cd distributed-kv-store
```

### Install Dependencies

```bash
go mod download
```

## 🚀 Quick Start

### 1. Start the Cluster

Open three terminal windows and run:

**Terminal 1 - Node 1 (Leader candidate):**
```bash
go run cmd/node.go --id node1 --port 8001 --grpc-port 50051 --peers localhost:8002,localhost:8003
```

**Terminal 2 - Node 2:**
```bash
go run cmd/node.go --id node2 --port 8002 --grpc-port 50052 --peers localhost:8001,localhost:8003
```

**Terminal 3 - Node 3:**
```bash
go run cmd/node.go --id node3 --port 8003 --grpc-port 50053 --peers localhost:8001,localhost:8002
```

### 2. Run the Client

In a fourth terminal:
```bash
go run cmd/client/main.go -target localhost:50052
```

You should see:
```
Set Success: true
Get Response: key=name, value=Siddharth, found=true
```

## 📖 Usage

### Command Line Options

#### Node Server

```bash
go run cmd/node.go [flags]
```

Flags:
- `--id`: Unique node identifier (e.g., `node1`, `node2`, `node3`)
- `--port`: Raft gRPC server port for inter-node communication (e.g., `8001`)
- `--grpc-port`: KV gRPC server port for client connections (e.g., `50051`)
- `--peers`: Comma-separated list of peer addresses (e.g., `localhost:8002,localhost:8003`)

#### Client

```bash
go run cmd/client/main.go -target <grpc-address>
```

Options:
- `-target`: Address of any node in the cluster (e.g., `localhost:50052`)

### Example: 3-Node Cluster Setup

```bash
# Node 1
go run cmd/node.go --id node1 --port 8001 --grpc-port 50051 --peers localhost:8002,localhost:8003

# Node 2  
go run cmd/node.go --id node2 --port 8002 --grpc-port 50052 --peers localhost:8001,localhost:8003

# Node 3
go run cmd/node.go --id node3 --port 8003 --grpc-port 50053 --peers localhost:8001,localhost:8002

# Client (can connect to any node)
go run cmd/client/main.go -target localhost:50052
```

### Testing Fault Tolerance

1. Start all three nodes
2. Run client operations to verify writes/reads
3. Kill the leader node (Ctrl+C)
4. Observe automatic leader election
5. Continue operations - the system remains available!

##  Project Structure

```
distributed-kv-store/
├── cmd/
│   ├── node.go              # Main node server entry point
│   └── client/
│       └── main.go           # Client CLI tool
├── internal/
│   ├── raft/
│   │   ├── raft.go           # Core Raft implementation
│   │   ├── service.go        # Raft gRPC service
│   │   ├── client.go         # Peer client for RPC calls
│   │   └── types.go          # Raft type definitions
│   ├── state_machine/
│   │   └── store.go          # In-memory KV store
│   └── rpc/
│       └── raft_rpc.go       # RPC type definitions
├── proto/
│   ├── kv/                   # KV service protobuf definitions
│   └── raft/                 # Raft service protobuf definitions
├── go.mod
├── go.sum
└── README.md
```

##  How It Works

### Raft Consensus

1. **Leader Election**: Nodes start as followers. If no leader heartbeat is received, a node becomes a candidate and requests votes. A node with majority votes becomes leader.

2. **Log Replication**: 
   - Client sends write request to leader
   - Leader appends entry to its log
   - Leader replicates entry to all followers via AppendEntries RPC
   - Entry is committed when majority of nodes have replicated it
   - Committed entries are applied to the state machine

3. **Commit Logic**: Leader only commits entries from its current term after majority replication. This ensures safety guarantees.

4. **Fault Tolerance**: System can tolerate (N-1)/2 node failures in an N-node cluster.

### Client Operations

- **Set Operation**: 
  - If follower receives request, it forwards to leader
  - Leader replicates via Raft, commits, and applies to store
  - Returns success only after commit

- **Get Operation**: 
  - Can be served by any node (reads from local store)
  - For strong consistency, should read from leader (future enhancement)

## Learning Outcomes

This project demonstrates:

- **Consensus Algorithms**: Deep understanding of Raft's leader election, log replication, and safety properties
- **Concurrent Programming**: Proper use of goroutines, channels, mutexes, and synchronization primitives
- **Network Programming**: gRPC, protocol buffers, and efficient RPC design
- **Systems Design**: Building fault-tolerant, distributed systems from scratch
- **Software Engineering**: Clean code, proper error handling, and maintainable architecture

## 🔮 Future Enhancements

- [ ] Persistent storage (WAL, snapshots)
- [ ] Dynamic cluster membership changes
- [ ] Read-only replicas for better read scalability
- [ ] Metrics and observability (Prometheus, Grafana)
- [ ] Client libraries for multiple languages
- [ ] Transaction support
- [ ] TTL (time-to-live) for keys
- [ ] Cluster health monitoring

## 🤝 Contributing

Contributions are welcome! This is a learning project, so feel free to:
- Report bugs
- Suggest improvements
- Submit pull requests
- Share your experience using it

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 👤 Author

**Siddharth Roy**

- GitHub: [@siddharth-roy27](https://github.com/siddharth-roy27)
- Project Link: [https://github.com/siddharth-roy27/distributed-kv-store](https://github.com/siddharth-roy27/distributed-kv-store)

##  Acknowledgments

- Raft paper: ["In Search of an Understandable Consensus Algorithm"](https://raft.github.io/raft.pdf) by Diego Ongaro and John Ousterhout
- Go gRPC documentation and examples
- The distributed systems community for excellent resources

---

⭐ If you find this project interesting or useful, please consider giving it a star!
