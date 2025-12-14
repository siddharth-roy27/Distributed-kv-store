package raft

import (
	"context"
	"fmt"
	"time"

	raftpb "github.com/siddharth-roy27/distributed-kv-store/proto/raft"
	"google.golang.org/grpc"
)

// PeerClient wraps a gRPC client to a peer
type PeerClient struct {
	id     string
	addr   string
	client raftpb.RaftClient
	conn   *grpc.ClientConn
}

func NewPeerClient(addr string) (*PeerClient, error) {
	conn, err := grpc.Dial(addr, grpc.WithInsecure(), grpc.WithBlock(), grpc.WithTimeout(2*time.Second))
	if err != nil {
		return nil, err
	}
	c := raftpb.NewRaftClient(conn)
	return &PeerClient{addr: addr, client: c, conn: conn}, nil
}

func (pc *PeerClient) Close() error {
	if pc.conn != nil {
		return pc.conn.Close()
	}
	return nil
}

func (pc *PeerClient) RequestVote(term int, candidateID string, lastLogIndex int, lastLogTerm int) (*raftpb.RequestVoteReply, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req := &raftpb.RequestVoteArgs{
		Term:         int32(term),
		CandidateId:  candidateID,
		LastLogIndex: int32(lastLogIndex),
		LastLogTerm:  int32(lastLogTerm),
	}
	return pc.client.RequestVote(ctx, req)
}

func (pc *PeerClient) AppendEntries(term int, leaderID string, prevIndex, prevTerm int, entries []LogEntry, leaderCommit int) (*raftpb.AppendEntriesReply, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	protoEntries := make([]*raftpb.LogEntry, 0, len(entries))
	for _, e := range entries {
		protoEntries = append(protoEntries, &raftpb.LogEntry{Term: int32(e.Term), Command: fmt.Sprintf("%v", e.Command)})
	}

	req := &raftpb.AppendEntriesArgs{
		Term:         int32(term),
		LeaderId:     leaderID,
		PrevLogIndex: int32(prevIndex),
		PrevLogTerm:  int32(prevTerm),
		Entries:      protoEntries,
		LeaderCommit: int32(leaderCommit),
	}
	return pc.client.AppendEntries(ctx, req)
}
