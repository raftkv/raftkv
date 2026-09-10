package main

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	pb "daijin235/proto"
)

type BatchSyncState int32

const (
	BatchSyncIdle BatchSyncState = iota
	BatchSyncInProgress
	BatchSyncCompleted
	BatchSyncFailed
	BatchSyncRetrying
)

type BatchSyncTask struct {
	TargetFollowerID string
	StartIdx         int64
	EndIdx           int64
	BatchSize        int64
	State            int32
	RetryCount       int32
}

type BatchSyncConfig struct {
	Enable       bool
	LagThreshold int64
	MaxBatchSize int64
	MaxRetries   int32
	SyncInterval time.Duration
}

func DefaultBatchSyncConfig() BatchSyncConfig {
	return BatchSyncConfig{
		Enable:       true,
		LagThreshold: 100,
		MaxBatchSize: 4096,
		MaxRetries:   5,
		SyncInterval: 200 * time.Millisecond,
	}
}

func BatchSyncConfigFromEnv() BatchSyncConfig {
	cfg := DefaultBatchSyncConfig()
	if os.Getenv("BATCH_SYNC_ENABLE") == "false" || os.Getenv("BATCH_SYNC_ENABLE") == "0" {
		cfg.Enable = false
	}
	if v := osGetenvInt("BATCH_SYNC_LAG_THRESHOLD"); v > 0 {
		cfg.LagThreshold = int64(v)
	}
	if v := osGetenvInt("BATCH_SYNC_MAX_BATCH"); v > 0 {
		cfg.MaxBatchSize = int64(v)
	}
	return cfg
}

type BatchSyncManager struct {
	mu     sync.Mutex
	tasks  map[string]*BatchSyncTask
	config BatchSyncConfig
	node   *RaftNode
	stopCh chan struct{}
	wg     sync.WaitGroup
}

func NewBatchSyncManager(node *RaftNode, config BatchSyncConfig) *BatchSyncManager {
	return &BatchSyncManager{
		tasks:  make(map[string]*BatchSyncTask),
		config: config,
		node:   node,
		stopCh: make(chan struct{}),
	}
}

func (m *BatchSyncManager) Start() {
	m.wg.Add(1)
	go m.SyncLoop()
}

func (m *BatchSyncManager) Stop() {
	close(m.stopCh)
	m.wg.Wait()
}

func (m *BatchSyncManager) SyncLoop() {
	defer m.wg.Done()
	ticker := time.NewTicker(m.config.SyncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			if !m.config.Enable {
				continue
			}
			m.node.CheckGapAlerts() // R-04修复C: gap>阈值持续10s告警
			lagging := m.node.IdentifyLaggingFollowers()
			if len(lagging) == 0 {
				continue
			}

			var wg sync.WaitGroup
			for _, f := range lagging {
				wg.Add(1)
				go func(lf LaggingFollower) {
					defer wg.Done()
					m.SyncFollower(lf)
				}(f)
			}
			wg.Wait()
		}
	}
}

func (m *BatchSyncManager) SyncFollower(f LaggingFollower) {
	client, ok := m.node.GetPeerClient(f.PeerID)
	if !ok {
		return
	}

	batchSize := f.BatchSize
	if batchSize < 128 {
		batchSize = 128
	}

	startIdx := f.StartIdx
	retryCount := int32(0)

	for startIdx <= f.EndIdx {
		endIdx := startIdx + batchSize - 1
		if endIdx > f.EndIdx {
			endIdx = f.EndIdx
		}

		entries := m.node.GetLogEntries(startIdx, endIdx)
		if len(entries) == 0 {
			break
		}

		// 探针1(M1): batch sync 发送侧观测
		m.node.logf("[SYNC] leader=%s follower=%s startIdx=%d endIdx=%d entries=%d",
			m.node.id, f.PeerID, startIdx, endIdx, len(entries))
		// 探针2(M2): 序列化缓冲大小观测
		bufBytes := 0
		for _, e := range entries {
			bufBytes += 16 + len(e.Command) + len(e.Sm3Hash)
		}
		m.node.logf("[SYNCBUF] leader=%s follower=%s entries=%d bufferBytes=%d",
			m.node.id, f.PeerID, len(entries), bufBytes)

		prevLogIdx := startIdx - 1
		prevLogTerm := int64(0)
		if prevLogIdx > 0 {
			prevLogTerm = m.node.GetLogTerm(prevLogIdx)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		req := &pb.AppendEntriesRequest{
			Term:         m.node.Term(),
			LeaderId:     m.node.ID(),
			PrevLogIndex: prevLogIdx,
			PrevLogTerm:  prevLogTerm,
			Entries:      entries,
			LeaderCommit: m.node.getCommitIdx(),
		}

		resp, err := client.AppendEntries(ctx, req)
		cancel()

		if err != nil {
			retryCount++
			if retryCount > m.config.MaxRetries {
				m.node.logf("[raft/%s] 批量同步失败: follower=%s, startIdx=%d, 重试%d次后放弃",
					m.node.id, f.PeerID, startIdx, retryCount)
				m.node.MarkFollowerDegraded(f.PeerID) // R-04修复B: 标记降级
				return
			}
			batchSize = batchSize / 2
			if batchSize < 128 {
				batchSize = 128
			}
			time.Sleep(100 * time.Millisecond)
			continue
		}

		if resp.Success {
			m.node.UpdateFollowerProgress(f.PeerID, endIdx)
			m.node.ClearFollowerDegraded(f.PeerID) // R-04修复B: 同步成功清除降级
			startIdx = endIdx + 1
		} else {
			retryCount++
			if retryCount > m.config.MaxRetries {
				m.node.MarkFollowerDegraded(f.PeerID) // R-04修复B: 标记降级
				return
			}
			batchSize = batchSize / 2
			if batchSize < 128 {
				batchSize = 128
			}
			time.Sleep(100 * time.Millisecond)
		}
	}

	atomic.StoreInt32(&m.node.consecutiveSuccess, 0)
}

func (m *BatchSyncManager) String() string {
	return fmt.Sprintf("BatchSyncManager(enabled=%v, lagThreshold=%d, maxBatch=%d, tasks=%d)",
		m.config.Enable, m.config.LagThreshold, m.config.MaxBatchSize, len(m.tasks))
}
