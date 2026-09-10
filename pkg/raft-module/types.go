// =========================================================================
// 岱境235 Module01 — 共享类型定义（纯 Go 标准库）
//
// 从原 daijin235/types.go 剥离 Raft 共识相关类型，去除网关业务字段，
// 形成自洽的共识引擎类型闭包。
// =========================================================================

package raft

import (
	"sync"
)

// NodeState Raft 节点状态枚举（Follower / Candidate / Leader）
type NodeState int32

const (
	StateFollower  NodeState = 0 // 跟随者
	StateCandidate NodeState = 1 // 候选者
	StateLeader    NodeState = 2 // 领导者
)

func (s NodeState) String() string {
	switch s {
	case StateFollower:
		return "Follower"
	case StateCandidate:
		return "Candidate"
	case StateLeader:
		return "Leader"
	default:
		return "Unknown"
	}
}

// PeerInfo 对等节点连接信息
type PeerInfo struct {
	ID      string // 节点 ID
	Address string // RPC 地址 (host:port)
}

// RaftLog Raft 日志条目（纯 Go 实现，替代 gRPC proto 的 LogEntry）
type RaftLog struct {
	Index   int64  // 全局递增索引（从 1 开始）
	Term    int64  // 任期号
	Command []byte // 状态机命令字节
	Hash    []byte // 链式校验哈希（SHA-256，可选）
}

// VoteRecord 投票记录
type VoteRecord struct {
	Term     int64
	VotedFor string
}

// RaftStats 节点运行时统计
type RaftStats struct {
	ID           string `json:"id"`
	State        string `json:"state"`
	Term         int64  `json:"term"`
	LeaderID     string `json:"leader_id"`
	CommitIndex  int64  `json:"commit_index"`
	LastApplied  int64  `json:"last_applied"`
	LogCount     int    `json:"log_count"`
	PeerCount    int    `json:"peer_count"`
	VotedFor     string `json:"voted_for"`
	ElectionTime string `json:"election_time"`
	mu           sync.RWMutex
}

func (s *RaftStats) RLock()   { s.mu.RLock() }
func (s *RaftStats) RUnlock() { s.mu.RUnlock() }
func (s *RaftStats) Lock()    { s.mu.Lock() }
func (s *RaftStats) Unlock()  { s.mu.Unlock() }

// Logger 简明日志接口
type Logger interface {
	Printf(format string, v ...interface{})
}

// =========================================================================
// RPC 请求/响应结构体（本地定义，替代 gRPC proto 生成代码）
// =========================================================================

// RequestVoteRequest 请求投票 RPC 请求
type RequestVoteRequest struct {
	Term         int64  `json:"term"`
	CandidateId  string `json:"candidate_id"`
	LastLogIndex int64  `json:"last_log_index"`
	LastLogTerm  int64  `json:"last_log_term"`
}

// RequestVoteResponse 请求投票 RPC 响应
type RequestVoteResponse struct {
	Term        int64 `json:"term"`
	VoteGranted bool  `json:"vote_granted"`
}

// AppendEntriesRequest 追加日志 RPC 请求（心跳 + 日志复制）
type AppendEntriesRequest struct {
	Term         int64     `json:"term"`
	LeaderId     string    `json:"leader_id"`
	PrevLogIndex int64     `json:"prev_log_index"`
	PrevLogTerm  int64     `json:"prev_log_term"`
	Entries      []RaftLog `json:"entries"`
	LeaderCommit int64     `json:"leader_commit"`
}

// AppendEntriesResponse 追加日志 RPC 响应
type AppendEntriesResponse struct {
	Term    int64 `json:"term"`
	Success bool  `json:"success"`
}

// =========================================================================
// Transport 传输层接口（解耦通信实现，可用 HTTP / channel / 内存桥接）
// =========================================================================

// Transport Raft 节点间通信传输层接口
type Transport interface {
	// RequestVote 请求投票 RPC
	RequestVote(req *RequestVoteRequest) (*RequestVoteResponse, error)
	// AppendEntries 追加日志 RPC（含心跳）
	AppendEntries(req *AppendEntriesRequest) (*AppendEntriesResponse, error)
	// Close 关闭传输连接
	Close() error
}
