package raft

import (
	"context"

	"github.com/siddharth-roy27/distributed-kv-store/internal/rpc"
	raftpb "github.com/siddharth-roy27/distributed-kv-store/proto/raft"
)

// RaftService implements the generated gRPC server and delegates to RaftNode.
type RaftService struct {
	raftpb.UnimplementedRaftServer
	node *RaftNode
}

func NewRaftService(node *RaftNode) *RaftService {
	return &RaftService{node: node}
}

func (s *RaftService) RequestVote(ctx context.Context, req *raftpb.RequestVoteArgs) (*raftpb.RequestVoteReply, error) {
	// Map proto -> internal rpc types or call node method directly
	var reply raftpb.RequestVoteReply
	// Convert args to internal rpc types (simple mapping)
	args := &rpc.RequestVoteArgs{
		Term:         int(req.Term),
		CandidateID:  req.CandidateId,
		LastLogIndex: int(req.LastLogIndex),
		LastLogTerm:  int(req.LastLogTerm),
	}

	// Call internal implementation (which updates s.node state)
	internalReply := &rpc.RequestVoteReply{}
	s.node.RequestVote(args, internalReply)

	reply.Term = int32(internalReply.Term)
	reply.VoteGranted = internalReply.VoteGranted
	return &reply, nil
}

func (s *RaftService) AppendEntries(ctx context.Context, req *raftpb.AppendEntriesArgs) (*raftpb.AppendEntriesReply, error) {
	var reply raftpb.AppendEntriesReply

	// convert proto log entries into internal rpc.LogEntry list
	entries := make([]rpc.LogEntry, 0, len(req.Entries))
	for _, e := range req.Entries {
		entries = append(entries, rpc.LogEntry{Term: int(e.Term), Command: e.Command})
	}

	args := &rpc.AppendEntriesArgs{
		Term:         int(req.Term),
		LeaderID:     req.LeaderId,
		PrevLogIndex: int(req.PrevLogIndex),
		PrevLogTerm:  int(req.PrevLogTerm),
		Entries:      entries,
		LeaderCommit: int(req.LeaderCommit),
	}

	internalReply := &rpc.AppendEntriesReply{}
	s.node.AppendEntries(args, internalReply)

	reply.Term = int32(internalReply.Term)
	reply.Success = internalReply.Success
	return &reply, nil
}
