package raft

type NodeState int

const (
	Follower NodeState = iota
	Candidate
	Leader
)

type LogEntry struct {
	Term    int
	Command interface{}
}
