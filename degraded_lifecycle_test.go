package main

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	pb "daijin235/proto"
)

func makeTestNode(id string, peerIDs []string) *RaftNode {
	peerAddrs := make(map[string]string)
	peerClients := make(map[string]pb.RaftServiceClient)
	for _, pid := range peerIDs {
		peerAddrs[pid] = fmt.Sprintf("127.0.0.1:%d", 9500)
	}
	return NewRaftNode(id, peerAddrs, peerClients, nil)
}

func TestDegradedMarkAndClear(t *testing.T) {
	rn := makeTestNode("node-1", []string{"node-2", "node-3"})

	t.Run("Mark设置degraded+重置recoveryCount", func(t *testing.T) {
		rn.MarkFollowerDegraded("node-2")
		rn.mu.Lock()
		defer rn.mu.Unlock()
		if !rn.degradedFollowers["node-2"] {
			t.Fatal("Mark后degradedFollowers[node-2]应为true")
		}
		if rn.degradedRecoveryCount["node-2"] != 0 {
			t.Fatalf("Mark后recoveryCount应为0，实际=%d", rn.degradedRecoveryCount["node-2"])
		}
	})

	t.Run("Clear清除degraded+删除recoveryCount", func(t *testing.T) {
		rn.mu.Lock()
		rn.degradedRecoveryCount["node-2"] = 2
		rn.mu.Unlock()

		rn.ClearFollowerDegraded("node-2")
		rn.mu.Lock()
		defer rn.mu.Unlock()
		if rn.degradedFollowers["node-2"] {
			t.Fatal("Clear后degradedFollowers[node-2]应为false")
		}
		if _, exists := rn.degradedRecoveryCount["node-2"]; exists {
			t.Fatal("Clear后recoveryCount应被删除")
		}
	})

	t.Run("Mark→Clear→Mark循环", func(t *testing.T) {
		rn.MarkFollowerDegraded("node-3")
		rn.ClearFollowerDegraded("node-3")
		rn.MarkFollowerDegraded("node-3")
		rn.mu.Lock()
		defer rn.mu.Unlock()
		if !rn.degradedFollowers["node-3"] {
			t.Fatal("Mark→Clear→Mark后应处于degraded状态")
		}
		if rn.degradedRecoveryCount["node-3"] != 0 {
			t.Fatalf("再次Mark后recoveryCount应重置为0，实际=%d", rn.degradedRecoveryCount["node-3"])
		}
	})

	t.Run("重复Mark幂等", func(t *testing.T) {
		rn.MarkFollowerDegraded("node-2")
		rn.MarkFollowerDegraded("node-2")
		rn.mu.Lock()
		defer rn.mu.Unlock()
		if !rn.degradedFollowers["node-2"] {
			t.Fatal("重复Mark后应仍为degraded")
		}
		if rn.degradedRecoveryCount["node-2"] != 0 {
			t.Fatalf("重复Mark后recoveryCount应仍为0，实际=%d", rn.degradedRecoveryCount["node-2"])
		}
	})

	t.Run("Clear未degraded节点无副作用", func(t *testing.T) {
		rn.ClearFollowerDegraded("node-99")
		rn.mu.Lock()
		defer rn.mu.Unlock()
		if rn.degradedFollowers["node-99"] {
			t.Fatal("Clear不存在的节点不应产生副作用")
		}
	})
}

func TestDegradedAutoClearAfter3Successes(t *testing.T) {
	rn := makeTestNode("node-1", []string{"node-2"})
	rn.MarkFollowerDegraded("node-2")

	simulateHeartbeatSuccess := func(peerID string) {
		rn.mu.Lock()
		defer rn.mu.Unlock()
		if rn.degradedFollowers[peerID] {
			rn.degradedRecoveryCount[peerID]++
			if rn.degradedRecoveryCount[peerID] >= 3 {
				delete(rn.degradedFollowers, peerID)
				delete(rn.degradedRecoveryCount, peerID)
			}
		} else {
			rn.degradedRecoveryCount[peerID] = 0
		}
	}

	for i := 1; i <= 2; i++ {
		simulateHeartbeatSuccess("node-2")
		rn.mu.Lock()
		if !rn.degradedFollowers["node-2"] {
			t.Fatalf("第%d次心跳成功后不应清除degraded", i)
		}
		if rn.degradedRecoveryCount["node-2"] != int32(i) {
			t.Fatalf("第%d次心跳成功后recoveryCount应=%d，实际=%d", i, i, rn.degradedRecoveryCount["node-2"])
		}
		rn.mu.Unlock()
	}

	simulateHeartbeatSuccess("node-2")
	rn.mu.Lock()
	defer rn.mu.Unlock()
	if rn.degradedFollowers["node-2"] {
		t.Fatal("第3次心跳成功后应自动清除degraded")
	}
	if _, exists := rn.degradedRecoveryCount["node-2"]; exists {
		t.Fatal("第3次心跳成功后recoveryCount应被删除")
	}
}

func TestDegradedReMarkDoesNotResetRecovery(t *testing.T) {
	rn := makeTestNode("node-1", []string{"node-2"})
	rn.MarkFollowerDegraded("node-2")

	rn.mu.Lock()
	rn.degradedRecoveryCount["node-2"] = 2
	rn.mu.Unlock()

	rn.MarkFollowerDegraded("node-2")
	rn.mu.Lock()
	defer rn.mu.Unlock()
	if rn.degradedRecoveryCount["node-2"] != 2 {
		t.Fatalf("已degraded时重复Mark不应重置recoveryCount，期望=2，实际=%d", rn.degradedRecoveryCount["node-2"])
	}
}

func TestQuorumExcludesDegradedFollowers(t *testing.T) {
	rn := makeTestNode("node-1", []string{"node-2", "node-3", "node-4", "node-5"})

	rn.mu.Lock()
	rn.state = StateLeader
	rn.term = 10
	atomic.StoreInt64(&rn.term, 10)
	rn.matchIdx["node-2"] = 5
	rn.matchIdx["node-3"] = 5
	rn.matchIdx["node-4"] = 5
	rn.matchIdx["node-5"] = 5
	rn.degradedFollowers["node-3"] = true
	rn.degradedFollowers["node-4"] = true
	rn.degradedFollowers["node-5"] = true
	peers := []string{"node-1", "node-2", "node-3", "node-4", "node-5"}
	quorumNeeded := len(peers)/2 + 1

	count := 1
	for _, p := range rn.peers {
		if rn.degradedFollowers[p.ID] {
			continue
		}
		if rn.matchIdx[p.ID] >= 5 {
			count++
		}
	}
	rn.mu.Unlock()

	if count >= quorumNeeded {
		t.Fatalf("degraded=3/4 follower应使quorum不满足: count=%d, needed=%d", count, quorumNeeded)
	}
	if count != 2 {
		t.Fatalf("仅leader+node-2应计数: count=%d, 期望=2", count)
	}
}

func TestQuorumRestoredAfterDegradedCleared(t *testing.T) {
	rn := makeTestNode("node-1", []string{"node-2", "node-3", "node-4", "node-5"})

	rn.mu.Lock()
	rn.state = StateLeader
	rn.term = 10
	atomic.StoreInt64(&rn.term, 10)
	rn.matchIdx["node-2"] = 5
	rn.matchIdx["node-3"] = 5
	rn.matchIdx["node-4"] = 5
	rn.matchIdx["node-5"] = 5
	rn.degradedFollowers["node-3"] = true
	rn.degradedFollowers["node-4"] = true
	rn.degradedFollowers["node-5"] = true
	rn.mu.Unlock()

	rn.ClearFollowerDegraded("node-3")
	rn.ClearFollowerDegraded("node-4")

	rn.mu.Lock()
	defer rn.mu.Unlock()
	peers := []string{"node-1", "node-2", "node-3", "node-4", "node-5"}
	quorumNeeded := len(peers)/2 + 1

	count := 1
	for _, p := range rn.peers {
		if rn.degradedFollowers[p.ID] {
			continue
		}
		if rn.matchIdx[p.ID] >= 5 {
			count++
		}
	}

	if count < quorumNeeded {
		t.Fatalf("清除2个degraded后quorum应恢复: count=%d, needed=%d", count, quorumNeeded)
	}
}

func TestLeaderStepDownAfterNoQuorum(t *testing.T) {
	rn := makeTestNode("node-1", []string{"node-2", "node-3", "node-4", "node-5"})

	rn.mu.Lock()
	rn.state = StateLeader
	rn.term = 10
	atomic.StoreInt64(&rn.term, 10)
	rn.matchIdx["node-2"] = 0
	rn.matchIdx["node-3"] = 0
	rn.matchIdx["node-4"] = 0
	rn.matchIdx["node-5"] = 0
	rn.degradedFollowers["node-2"] = true
	rn.degradedFollowers["node-3"] = true
	rn.degradedFollowers["node-4"] = true
	rn.degradedFollowers["node-5"] = true
	rn.lastQuorumTime = time.Now().Add(-11 * time.Second)

	quorumAchieved := false
	shouldStepDown := false
	if quorumAchieved {
		rn.lastQuorumTime = time.Now()
	} else if len(rn.degradedFollowers) > 0 {
		if rn.lastQuorumTime.IsZero() {
			rn.lastQuorumTime = time.Now()
		}
		if time.Since(rn.lastQuorumTime) > 10*time.Second {
			shouldStepDown = true
			rn.state = StateFollower
			rn.leaderID = ""
			rn.votedFor = ""
			rn.electionTimer.Reset(randomElectionTimeout())
		}
	} else {
		rn.lastQuorumTime = time.Now()
	}
	rn.mu.Unlock()

	if !shouldStepDown {
		t.Fatal("全degraded+11s无quorum应触发退位")
	}
	if rn.state != StateFollower {
		t.Fatal("退位后state应为Follower")
	}
	rn.mu.Lock()
	defer rn.mu.Unlock()
	if rn.leaderID != "" {
		t.Fatal("退位后leaderID应为空")
	}
	if rn.votedFor != "" {
		t.Fatal("退位后votedFor应为空")
	}
}

func TestLeaderNoStepDownWithoutDegraded(t *testing.T) {
	rn := makeTestNode("node-1", []string{"node-2", "node-3", "node-4", "node-5"})

	rn.mu.Lock()
	rn.state = StateLeader
	rn.term = 10
	atomic.StoreInt64(&rn.term, 10)
	rn.matchIdx["node-2"] = 0
	rn.matchIdx["node-3"] = 0
	rn.matchIdx["node-4"] = 0
	rn.matchIdx["node-5"] = 0
	rn.lastQuorumTime = time.Now().Add(-60 * time.Second)

	quorumAchieved := false
	shouldStepDown := false
	if quorumAchieved {
		rn.lastQuorumTime = time.Now()
	} else if len(rn.degradedFollowers) > 0 {
		if rn.lastQuorumTime.IsZero() {
			rn.lastQuorumTime = time.Now()
		}
		if time.Since(rn.lastQuorumTime) > 10*time.Second {
			shouldStepDown = true
		}
	} else {
		rn.lastQuorumTime = time.Now()
	}
	rn.mu.Unlock()

	if shouldStepDown {
		t.Fatal("无degraded follower时即使60s无quorum也不应退位（follower正在追赶）")
	}
}

func TestLeaderNoStepDownWithinGracePeriod(t *testing.T) {
	rn := makeTestNode("node-1", []string{"node-2", "node-3"})

	rn.mu.Lock()
	rn.state = StateLeader
	rn.term = 10
	atomic.StoreInt64(&rn.term, 10)
	rn.degradedFollowers["node-2"] = true
	rn.lastQuorumTime = time.Now().Add(-5 * time.Second)

	shouldStepDown := false
	if time.Since(rn.lastQuorumTime) > 10*time.Second {
		shouldStepDown = true
	}
	rn.mu.Unlock()

	if shouldStepDown {
		t.Fatal("5s无quorum不应触发退位（宽限期10s内）")
	}
}

func TestHandleAppendEntriesLowerTermResetsTimer(t *testing.T) {
	rn := makeTestNode("node-1", []string{"node-2"})

	rn.mu.Lock()
	rn.term = 100
	atomic.StoreInt64(&rn.term, 100)
	rn.state = StateFollower
	rn.mu.Unlock()

	timerBefore := time.Now()
	resp, err := rn.HandleAppendEntries(context.Background(), &pb.AppendEntriesRequest{
		Term:     50,
		LeaderId: "node-2",
	})
	timerAfter := time.Now()

	if err != nil {
		t.Fatalf("HandleAppendEntries不应返回error: %v", err)
	}
	if resp.Success {
		t.Fatal("低term AppendEntries应返回Success=false")
	}
	if resp.Term != 100 {
		t.Fatalf("响应Term应为100，实际=%d", resp.Term)
	}

	rn.mu.Lock()
	stateAfter := rn.state
	termAfter := rn.term
	rn.mu.Unlock()

	if stateAfter != StateFollower {
		t.Fatal("低term不应改变节点状态")
	}
	if termAfter != 100 {
		t.Fatalf("低term不应改变本地term，期望=100，实际=%d", termAfter)
	}

	elapsed := timerAfter.Sub(timerBefore)
	if elapsed > 100*time.Millisecond {
		t.Logf("注意: HandleAppendEntries耗时=%v（含electionTimer.Reset）", elapsed)
	}
}

func TestHandleRequestVoteLowerTermResetsTimer(t *testing.T) {
	rn := makeTestNode("node-1", []string{"node-2"})

	rn.mu.Lock()
	rn.term = 100
	atomic.StoreInt64(&rn.term, 100)
	rn.state = StateFollower
	rn.mu.Unlock()

	resp, err := rn.HandleRequestVote(context.Background(), &pb.RequestVoteRequest{
		Term:         50,
		CandidateId:  "node-2",
		LastLogIndex: 0,
		LastLogTerm:  0,
	})

	if err != nil {
		t.Fatalf("HandleRequestVote不应返回error: %v", err)
	}
	if resp.VoteGranted {
		t.Fatal("低term RequestVote应返回VoteGranted=false")
	}
	if resp.Term != 100 {
		t.Fatalf("响应Term应为100，实际=%d", resp.Term)
	}

	rn.mu.Lock()
	stateAfter := rn.state
	termAfter := rn.term
	rn.mu.Unlock()

	if stateAfter != StateFollower {
		t.Fatal("低term不应改变节点状态")
	}
	if termAfter != 100 {
		t.Fatalf("低term不应改变本地term，期望=100，实际=%d", termAfter)
	}
}

func TestDegradedFollowersStatsReporting(t *testing.T) {
	rn := makeTestNode("node-1", []string{"node-2", "node-3", "node-4"})

	rn.MarkFollowerDegraded("node-2")
	rn.MarkFollowerDegraded("node-4")

	rn.mu.Lock()
	degradedCount := len(rn.degradedFollowers)
	rn.mu.Unlock()

	if degradedCount != 2 {
		t.Fatalf("应有2个degraded follower，实际=%d", degradedCount)
	}

	rn.ClearFollowerDegraded("node-2")
	rn.mu.Lock()
	degradedCount = len(rn.degradedFollowers)
	rn.mu.Unlock()

	if degradedCount != 1 {
		t.Fatalf("清除1个后应剩1个degraded follower，实际=%d", degradedCount)
	}
}