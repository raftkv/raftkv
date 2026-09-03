package stateprotection

import (
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

type SimpleRaftNode struct {
	id          string
	term        int64
	votedFor    string
	leaderID    string
	logCaughtUp int32
	mu          sync.RWMutex
	peers       []string
	isLeader    int32
}

func NewSimpleRaftNode(id string, peers []string) *SimpleRaftNode {
	return &SimpleRaftNode{
		id:    id,
		peers: peers,
	}
}

func (n *SimpleRaftNode) Term() int64 {
	return atomic.LoadInt64(&n.term)
}

func (n *SimpleRaftNode) LeaderID() string {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.leaderID
}

func (n *SimpleRaftNode) GetVotedFor() string {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.votedFor
}

func (n *SimpleRaftNode) SetLogCaughtUp(caughtUp bool) {
	if caughtUp {
		atomic.StoreInt32(&n.logCaughtUp, 1)
	} else {
		atomic.StoreInt32(&n.logCaughtUp, 0)
	}
}

func (n *SimpleRaftNode) IsLogCaughtUp() bool {
	return atomic.LoadInt32(&n.logCaughtUp) == 1
}

func (n *SimpleRaftNode) ID() string {
	return n.id
}

func (n *SimpleRaftNode) SetTerm(term int64) {
	atomic.StoreInt64(&n.term, term)
}

func (n *SimpleRaftNode) SetVotedFor(votedFor string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.votedFor = votedFor
}

func (n *SimpleRaftNode) SetLeaderID(leaderID string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.leaderID = leaderID
}

func (n *SimpleRaftNode) BecomeLeader() {
	atomic.StoreInt32(&n.isLeader, 1)
	n.mu.Lock()
	n.leaderID = n.id
	n.votedFor = n.id
	n.mu.Unlock()
	atomic.AddInt64(&n.term, 1)
}

func (n *SimpleRaftNode) IsLeader() bool {
	return atomic.LoadInt32(&n.isLeader) == 1
}

func (n *SimpleRaftNode) StatusJSON() map[string]any {
	n.mu.RLock()
	defer n.mu.RUnlock()
	state := "Follower"
	if n.IsLeader() {
		state = "Leader"
	}
	return map[string]any{
		"id":            n.id,
		"state":         state,
		"term":          atomic.LoadInt64(&n.term),
		"leader_id":     n.leaderID,
		"voted_for":     n.votedFor,
		"log_caught_up": n.IsLogCaughtUp(),
		"peers":         n.peers,
	}
}

func (n *SimpleRaftNode) StartElectionLoop(leaderID string) {
	if leaderID != "" {
		n.SetLeaderID(leaderID)
		n.SetLogCaughtUp(true)
	}

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			if n.IsLeader() {
				n.SetTerm(n.Term() + 1)
			}
		}
	}()
}

func (n *SimpleRaftNode) RegisterHTTPHandlers(mux *http.ServeMux) {

	mux.HandleFunc("/raft/stats", func(w http.ResponseWriter, r *http.Request) {
		s := n.StatusJSON()
		fmt.Fprintf(w, "id=%s state=%s term=%d leader=%s voted=%s caught_up=%t",
			s["id"], s["state"], s["term"], s["leader_id"], s["voted_for"], s["log_caught_up"])
	})

	mux.HandleFunc("/health/live", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	mux.HandleFunc("/health/ready", func(w http.ResponseWriter, r *http.Request) {
		if n.IsLogCaughtUp() {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("READY"))
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("NOT READY"))
		}
	})
}
