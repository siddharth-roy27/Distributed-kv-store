package raft

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/siddharth-roy27/distributed-kv-store/internal/rpc"
	"github.com/siddharth-roy27/distributed-kv-store/internal/state_machine"
)

type RaftNode struct {
	ID            string
	State         NodeState
	CurrentTerm   int
	VotedFor      string
	Log           []LogEntry
	CommitIndex   int
	LastApplied   int
	ElectionTimer *time.Timer

	Peers []string

	mu     sync.Mutex
	leader string

	// Leader-only: track replication progress per peer
	nextIndex  map[string]int // next log entry to send to each peer
	matchIndex map[string]int // highest log entry known to be replicated on each peer

	Store *state_machine.Store

	// Channel to signal when new entries are committed
	commitCh chan struct{}
	stopCh   chan struct{}
}

// StartRaft initializes timers and starts election loop and apply loop
func (rn *RaftNode) StartRaft() {
	rn.mu.Lock()
	if rn.Store == nil {
		rn.Store = state_machine.NewStore()
	}
	if rn.commitCh == nil {
		rn.commitCh = make(chan struct{}, 10)
	}
	if rn.stopCh == nil {
		rn.stopCh = make(chan struct{})
	}
	if rn.nextIndex == nil {
		rn.nextIndex = make(map[string]int)
	}
	if rn.matchIndex == nil {
		rn.matchIndex = make(map[string]int)
	}
	// Initialize log with a dummy entry at index 0 (Raft convention)
	if len(rn.Log) == 0 {
		rn.Log = []LogEntry{{Term: 0, Command: nil}}
	}
	rn.State = Follower
	rn.resetElectionTimer()
	rn.mu.Unlock()

	go rn.electionLoop()
	go rn.applyLoop()
}

// Reset the election timer with a random timeout
func (rn *RaftNode) resetElectionTimer() {
	if rn.ElectionTimer != nil {
		rn.ElectionTimer.Stop()
	}
	rn.ElectionTimer = time.NewTimer(randomElectionTimeout())
}

// Generate a random election timeout between 150–300ms
func randomElectionTimeout() time.Duration {
	return time.Duration(150+rand.Intn(150)) * time.Millisecond
}

func (rn *RaftNode) electionLoop() {
	for {
		<-rn.ElectionTimer.C
		rn.startElection()
		rn.resetElectionTimer()
	}
}

// startElection initiates a simple majority-based election
func (rn *RaftNode) startElection() {
	rn.mu.Lock()
	if rn.State == Leader {
		rn.mu.Unlock()
		return
	}
	rn.State = Candidate
	rn.CurrentTerm++
	rn.VotedFor = rn.ID
	lastLogIndex := len(rn.Log) - 1
	lastLogTerm := 0
	if lastLogIndex > 0 {
		lastLogTerm = rn.Log[lastLogIndex].Term
	}
	currentTerm := rn.CurrentTerm
	rn.mu.Unlock()

	votes := 1 // vote for self
	var wg sync.WaitGroup
	voteCh := make(chan bool, len(rn.Peers))

	for _, p := range rn.Peers {
		wg.Add(1)
		go func(peerAddr string) {
			defer wg.Done()
			pc, err := NewPeerClient(peerAddr)
			if err != nil {
				voteCh <- false
				return
			}
			defer pc.Close()
			resp, err := pc.RequestVote(currentTerm, rn.ID, lastLogIndex, lastLogTerm)
			if err != nil {
				voteCh <- false
				return
			}
			// Check if term was updated (we're behind)
			rn.mu.Lock()
			if int(resp.Term) > rn.CurrentTerm {
				rn.CurrentTerm = int(resp.Term)
				rn.State = Follower
				rn.VotedFor = ""
			}
			rn.mu.Unlock()
			voteCh <- resp.VoteGranted
		}(p)
	}

	// collect votes with timeout
	go func() {
		wg.Wait()
		close(voteCh)
	}()

	for granted := range voteCh {
		if granted {
			votes++
		}
	}

	rn.mu.Lock()
	// Check if we're still a candidate and have majority
	if rn.State == Candidate && votes > (len(rn.Peers)+1)/2 {
		// become leader
		rn.State = Leader
		rn.leader = rn.ID
		// Initialize leader state
		for _, peer := range rn.Peers {
			rn.nextIndex[peer] = len(rn.Log)
			rn.matchIndex[peer] = 0
		}
		rn.mu.Unlock()
		go rn.leaderLoop()
		return
	}
	// If we didn't win, become follower
	if rn.State == Candidate {
		rn.State = Follower
		rn.VotedFor = ""
	}
	rn.mu.Unlock()
}

func (rn *RaftNode) leaderLoop() {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		rn.mu.Lock()
		if rn.State != Leader {
			rn.mu.Unlock()
			return
		}
		rn.mu.Unlock()

		// send AppendEntries to all peers (heartbeat + replication)
		for _, p := range rn.Peers {
			go func(peerAddr string) {
				rn.mu.Lock()
				if rn.State != Leader {
					rn.mu.Unlock()
					return
				}
				nextIdx := rn.nextIndex[peerAddr]
				if nextIdx == 0 {
					nextIdx = len(rn.Log)
					rn.nextIndex[peerAddr] = nextIdx
				}
				prevIndex := nextIdx - 1
				prevTerm := 0
				if prevIndex >= 0 && prevIndex < len(rn.Log) {
					prevTerm = rn.Log[prevIndex].Term
				}
				entries := []LogEntry{}
				if nextIdx < len(rn.Log) {
					entries = rn.Log[nextIdx:]
				}
				leaderCommit := rn.CommitIndex
				term := rn.CurrentTerm
				rn.mu.Unlock()

				pc, err := NewPeerClient(peerAddr)
				if err != nil {
					return
				}
				defer pc.Close()
				resp, err := pc.AppendEntries(term, rn.ID, prevIndex, prevTerm, entries, leaderCommit)
				if err != nil {
					return
				}

				rn.mu.Lock()
				defer rn.mu.Unlock()
				if rn.State != Leader || rn.CurrentTerm != term {
					return
				}
				if int(resp.Term) > rn.CurrentTerm {
					// Step down
					rn.CurrentTerm = int(resp.Term)
					rn.State = Follower
					rn.VotedFor = ""
					rn.leader = ""
					return
				}
				if resp.Success {
					// Update nextIndex and matchIndex
					if nextIdx+len(entries) > rn.nextIndex[peerAddr] {
						rn.nextIndex[peerAddr] = nextIdx + len(entries)
					}
					if prevIndex+len(entries) > rn.matchIndex[peerAddr] {
						rn.matchIndex[peerAddr] = prevIndex + len(entries)
					}
					// Try to advance commitIndex
					rn.advanceCommitIndex()
				} else {
					// Decrement nextIndex and retry
					if rn.nextIndex[peerAddr] > 1 {
						rn.nextIndex[peerAddr]--
					}
				}
			}(p)
		}

		<-ticker.C
	}
}

// advanceCommitIndex advances commitIndex if majority of peers have replicated an entry
func (rn *RaftNode) advanceCommitIndex() {
	if rn.State != Leader {
		return
	}
	// Find the highest index that is replicated on majority
	for n := rn.CommitIndex + 1; n < len(rn.Log); n++ {
		if rn.Log[n].Term != rn.CurrentTerm {
			continue // Only commit entries from current term
		}
		count := 1 // count self
		for _, peer := range rn.Peers {
			if rn.matchIndex[peer] >= n {
				count++
			}
		}
		if count > (len(rn.Peers)+1)/2 {
			rn.CommitIndex = n
			// Signal that new entries are committed
			select {
			case rn.commitCh <- struct{}{}:
			default:
			}
		}
	}
}

// ProposeSet appends a Set command to the log and tries to replicate it to peers.
// Returns true if the entry was committed (majority replicated).
func (rn *RaftNode) ProposeSet(key, value string) bool {
	rn.mu.Lock()
	if rn.State != Leader {
		rn.mu.Unlock()
		return false
	}
	entry := LogEntry{Term: rn.CurrentTerm, Command: fmt.Sprintf("set:%s:%s", key, value)}
	rn.Log = append(rn.Log, entry)
	index := len(rn.Log) - 1
	rn.mu.Unlock()

	// The leaderLoop will replicate this entry and advance commitIndex
	// Wait for it to be committed (with timeout)
	timeout := time.After(5 * time.Second)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return false
		case <-ticker.C:
			rn.mu.Lock()
			committed := rn.CommitIndex >= index
			stillLeader := rn.State == Leader
			rn.mu.Unlock()
			if committed && stillLeader {
				return true
			}
			if !stillLeader {
				return false
			}
		}
	}
}

// applyLoop continuously applies committed log entries to the state machine
func (rn *RaftNode) applyLoop() {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-rn.stopCh:
			return
		case <-rn.commitCh:
			// New entries committed, apply them
			rn.applyCommittedEntries()
		case <-ticker.C:
			// Periodically check for committed entries (in case we missed a signal)
			rn.applyCommittedEntries()
		}
	}
}

// applyCommittedEntries applies all committed entries that haven't been applied yet
func (rn *RaftNode) applyCommittedEntries() {
	rn.mu.Lock()
	for rn.LastApplied < rn.CommitIndex {
		rn.LastApplied++
		// Skip dummy entry at index 0
		if rn.LastApplied == 0 {
			continue
		}
		entry := rn.Log[rn.LastApplied]
		store := rn.Store
		rn.mu.Unlock()
		// Apply to state machine
		if store != nil {
			if cmd, ok := entry.Command.(string); ok && cmd != "" {
				if err := store.Apply(cmd); err != nil {
					// Log error but continue applying
					fmt.Printf("Error applying command %s: %v\n", cmd, err)
				}
			}
		}
		rn.mu.Lock()
	}
	rn.mu.Unlock()
}

// IsLeader returns true if this node believes it is the leader.
func (rn *RaftNode) IsLeader() bool {
	rn.mu.Lock()
	defer rn.mu.Unlock()
	return rn.State == Leader
}

// LeaderAddr returns the leader ID/address known to this node (may be empty).
func (rn *RaftNode) LeaderAddr() string {
	rn.mu.Lock()
	defer rn.mu.Unlock()
	return rn.leader
}

// ---------------- Raft RPCs ---------------- //

// Handle RequestVote RPC
func (rn *RaftNode) RequestVote(args *rpc.RequestVoteArgs, reply *rpc.RequestVoteReply) {
	reply.VoteGranted = false
	rn.mu.Lock()
	defer rn.mu.Unlock()

	if args.Term > rn.CurrentTerm {
		// Update term and become follower
		rn.CurrentTerm = args.Term
		rn.State = Follower
		rn.VotedFor = ""
		rn.leader = ""
		rn.resetElectionTimer()
	}

	reply.Term = rn.CurrentTerm

	if args.Term < rn.CurrentTerm {
		return
	}

	// Check if candidate's log is at least as up-to-date as ours
	lastLogIndex := len(rn.Log) - 1
	lastLogTerm := 0
	if lastLogIndex > 0 {
		lastLogTerm = rn.Log[lastLogIndex].Term
	}
	logOk := args.LastLogTerm > lastLogTerm ||
		(args.LastLogTerm == lastLogTerm && args.LastLogIndex >= lastLogIndex)

	// Grant vote if haven't voted (or voted for this candidate) and log is up-to-date
	if (rn.VotedFor == "" || rn.VotedFor == args.CandidateID) && logOk {
		rn.VotedFor = args.CandidateID
		reply.VoteGranted = true
		rn.resetElectionTimer()
	}
}

// Handle AppendEntries RPC (heartbeat + log replication)
func (rn *RaftNode) AppendEntries(args *rpc.AppendEntriesArgs, reply *rpc.AppendEntriesReply) {
	reply.Success = false
	rn.mu.Lock()

	if args.Term < rn.CurrentTerm {
		reply.Term = rn.CurrentTerm
		rn.mu.Unlock()
		return
	}

	// If we see a higher term, become follower
	if args.Term > rn.CurrentTerm {
		rn.CurrentTerm = args.Term
		rn.State = Follower
		rn.VotedFor = ""
	}

	// Reset election timer on valid AppendEntries
	rn.resetElectionTimer()
	rn.leader = args.LeaderID
	reply.Term = rn.CurrentTerm

	// Check if previous log entry matches
	if args.PrevLogIndex >= 0 && args.PrevLogIndex < len(rn.Log) {
		if rn.Log[args.PrevLogIndex].Term != args.PrevLogTerm {
			// Conflict: truncate log from this point
			rn.Log = rn.Log[:args.PrevLogIndex]
			rn.mu.Unlock()
			return
		}
	} else if args.PrevLogIndex >= len(rn.Log) {
		// We don't have the previous entry
		rn.mu.Unlock()
		return
	}

	// Append new entries (skip if already present)
	insertIndex := args.PrevLogIndex + 1
	for i, e := range args.Entries {
		entryIndex := insertIndex + i
		if entryIndex < len(rn.Log) {
			// Check for conflict
			if rn.Log[entryIndex].Term != int(e.Term) {
				// Conflict: truncate and append
				rn.Log = rn.Log[:entryIndex]
				rn.Log = append(rn.Log, LogEntry{Term: int(e.Term), Command: e.Command})
			}
			// Otherwise, entry already exists, skip
		} else {
			// Append new entry
			rn.Log = append(rn.Log, LogEntry{Term: int(e.Term), Command: e.Command})
		}
	}

	reply.Success = true

	// Update commit index
	if args.LeaderCommit > rn.CommitIndex {
		oldCommitIndex := rn.CommitIndex
		if args.LeaderCommit < len(rn.Log)-1 {
			rn.CommitIndex = args.LeaderCommit
		} else {
			rn.CommitIndex = len(rn.Log) - 1
		}
		// Signal apply loop if commit index advanced
		if rn.CommitIndex > oldCommitIndex {
			select {
			case rn.commitCh <- struct{}{}:
			default:
			}
		}
	}

	rn.mu.Unlock()
}
