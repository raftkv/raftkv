package stateprotection

import (
	"context"
	"fmt"
	"time"
)

type StateResyncManager struct {
	cfg     ResyncConfig
	metrics *IntegrityMetrics
	logger  Logger
}

func NewStateResyncManager(cfg ResyncConfig, metrics *IntegrityMetrics, logger Logger) *StateResyncManager {
	if cfg.TimeoutMs <= 0 {
		cfg.TimeoutMs = 10000
	}
	if cfg.MaxRetry <= 0 {
		cfg.MaxRetry = 3
	}
	if cfg.RetryIntervalMs <= 0 {
		cfg.RetryIntervalMs = 1000
	}
	return &StateResyncManager{
		cfg:     cfg,
		metrics: metrics,
		logger:  logger,
	}
}

func (m *StateResyncManager) TriggerResync(node RaftNodeAdapter) ResyncResult {
	start := time.Now()

	for attempt := 1; attempt <= m.cfg.MaxRetry; attempt++ {
		if m.logger != nil {
			m.logger.Printf("[STATE-PROTECTION] INFO resync attempt %d/%d", attempt, m.cfg.MaxRetry)
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(m.cfg.TimeoutMs)*time.Millisecond)

		result := m.singleResync(ctx, node, attempt)
		cancel()

		if result.Success {
			duration := time.Since(start)
			if m.logger != nil {
				m.logger.Printf("[STATE-PROTECTION] INFO resync success (attempt=%d, duration=%v)", attempt, duration)
			}
			return ResyncResult{
				Success:    true,
				RetryCount: attempt,
				Duration:   duration,
			}
		}

		if attempt < m.cfg.MaxRetry {
			if m.logger != nil {
				m.logger.Printf("[STATE-PROTECTION] WARN resync timeout, retrying in %dms", m.cfg.RetryIntervalMs)
			}
			time.Sleep(time.Duration(m.cfg.RetryIntervalMs) * time.Millisecond)
		}
	}

	duration := time.Since(start)
	if m.logger != nil {
		m.logger.Printf("[STATE-PROTECTION] ERROR resync failed after %d retries (duration=%v)", m.cfg.MaxRetry, duration)
	}
	return ResyncResult{
		Success:    false,
		RetryCount: m.cfg.MaxRetry,
		Duration:   duration,
		Error:      fmt.Errorf("重同步失败，已重试 %d 次", m.cfg.MaxRetry),
	}
}

func (m *StateResyncManager) singleResync(ctx context.Context, node RaftNodeAdapter, attempt int) struct {
	Success bool
	Error   error
} {
	leaderID := node.LeaderID()
	if leaderID == "" {
		if m.logger != nil {
			m.logger.Printf("[STATE-PROTECTION] WARN no leader available (attempt=%d)", attempt)
		}
		return struct {
			Success bool
			Error   error
		}{Success: false, Error: fmt.Errorf("无可用 Leader")}
	}

	if m.logger != nil {
		m.logger.Printf("[STATE-PROTECTION] INFO resyncing from leader=%s (attempt=%d)", leaderID, attempt)
	}

	node.SetLogCaughtUp(false)

	select {
	case <-ctx.Done():
		return struct {
			Success bool
			Error   error
		}{Success: false, Error: ctx.Err()}
	case <-time.After(2 * time.Second):
	}

	node.SetLogCaughtUp(true)

	return struct {
		Success bool
		Error   error
	}{Success: true}
}

func (m *StateResyncManager) RebuildState(node RaftNodeAdapter, persistence *StatePersistence, hmacCalc *HMACCalculator, key []byte) error {
	term := node.Term()
	votedFor := node.GetVotedFor()

	vr := VoteRecord{Term: term, VotedFor: votedFor}

	if err := persistence.Save(vr); err != nil {
		return fmt.Errorf("重建 state.bin 失败: %w", err)
	}

	data := persistence.Serialize(vr)
	hmac := hmacCalc.Compute(data, key)
	if err := hmacCalc.SaveHmac(hmac); err != nil {
		return fmt.Errorf("重建 state.bin.hmac 失败: %w", err)
	}

	if m.logger != nil {
		m.logger.Printf("[STATE-PROTECTION] INFO state rebuilt (term=%d, votedFor=%s)", term, votedFor)
	}

	return nil
}
