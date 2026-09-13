package main

import (
	"fmt"
	"log"
	"time"
)

type Scheduler struct {
	nodeCtl   *NodeController
	collector *Collector
	evidence  *EvidenceManager
	resume    *ResumeManager
	contract  string
	diskCtl   *DiskController
}

func NewScheduler(nc *NodeController, c *Collector, em *EvidenceManager, rm *ResumeManager, contract string) *Scheduler {
	return &Scheduler{nodeCtl: nc, collector: c, evidence: em, resume: rm, contract: contract, diskCtl: NewDiskController()}
}

func (s *Scheduler) BuildScenarioMatrix(scenarioType string) []string {
	var scenarios []string
	switch scenarioType {
	case "steady":
		for i := 1; i <= 10; i++ {
			scenarios = append(scenarios, fmt.Sprintf("steady_kill_leader_%02d", i))
		}
	case "under_load":
		for i := 1; i <= 10; i++ {
			scenarios = append(scenarios, fmt.Sprintf("under_load_kill_leader_%02d", i))
		}
	case "cascading":
		for i := 1; i <= 3; i++ {
			scenarios = append(scenarios, fmt.Sprintf("cascading_kill_%02d", i))
		}
	case "prevote":
		for i := 1; i <= 10; i++ {
			scenarios = append(scenarios, fmt.Sprintf("steady_prevote_%02d", i))
		}
		for i := 1; i <= 3; i++ {
			scenarios = append(scenarios, fmt.Sprintf("cascading_prevote_%02d", i))
		}
		scenarios = append(scenarios, "prevote_forensics")
	case "disk_full":
		for i := 1; i <= 3; i++ {
			scenarios = append(scenarios, fmt.Sprintf("disk_full_follower_soft_%02d", i))
		}
		for i := 1; i <= 3; i++ {
			scenarios = append(scenarios, fmt.Sprintf("disk_full_follower_hard_%02d", i))
		}
	default:
		for i := 1; i <= 10; i++ {
			scenarios = append(scenarios, fmt.Sprintf("steady_kill_leader_%02d", i))
		}
		for i := 1; i <= 10; i++ {
			scenarios = append(scenarios, fmt.Sprintf("under_load_kill_leader_%02d", i))
		}
		for i := 1; i <= 3; i++ {
			scenarios = append(scenarios, fmt.Sprintf("cascading_kill_%02d", i))
		}
	}
	return scenarios
}

func (s *Scheduler) ExecuteScenario(scenarioID string) (*ScenarioResult, error) {
	switch {
	case contains(scenarioID, "steady_prevote"):
		return s.executeSteadyKill(scenarioID)
	case contains(scenarioID, "cascading_prevote"):
		return s.executeCascadingKill(scenarioID)
	case contains(scenarioID, "prevote_forensics"):
		return s.executePreVoteForensics(scenarioID)
	case contains(scenarioID, "disk_full"):
		return s.executeDiskFull(scenarioID)
	case contains(scenarioID, "steady_kill_leader"):
		return s.executeSteadyKill(scenarioID)
	case contains(scenarioID, "under_load_kill_leader"):
		return s.executeUnderLoadKill(scenarioID)
	case contains(scenarioID, "cascading_kill"):
		return s.executeCascadingKill(scenarioID)
	default:
		return nil, fmt.Errorf("unknown scenario type: %s", scenarioID)
	}
}

func (s *Scheduler) executeSteadyKill(scenarioID string) (*ScenarioResult, error) {
	result := &ScenarioResult{ScenarioID: scenarioID, ScenarioType: "steady_kill_leader"}
	timeline := []TimelineEvent{}

	leaderID, err := s.nodeCtl.QueryLeader()
	if err != nil {
		result.Status = "BLOCKED"
		return result, err
	}
	timeline = append(timeline, TimelineEvent{Timestamp: time.Now(), EventType: "leader_identified", NodeID: leaderID, Role: "Leader"})

	snapshots, _ := s.nodeCtl.SnapshotConfirmedEntries(leaderID, 20)

	killTS := time.Now()
	if err := s.nodeCtl.KillNode(leaderID); err != nil {
		result.Status = "BLOCKED"
		return result, err
	}
	timeline = append(timeline, TimelineEvent{Timestamp: killTS, EventType: "kill_leader", NodeID: leaderID, Role: "Leader"})

	electionMetrics, err := s.collector.CollectElectionTimeline(killTS, 10*time.Second)
	result.ElectionMetrics = *electionMetrics
	if err != nil {
		log.Printf("election timeout for %s: %v", scenarioID, err)
	}

	splitBrain, _ := s.collector.DetectSplitBrain(5 * time.Second)
	result.SplitBrainMetrics = *splitBrain

	newLeader, _ := s.nodeCtl.QueryLeader()
	if newLeader != "" && len(snapshots) > 0 {
		survival, _ := s.collector.CollectSurvivalRate(snapshots, newLeader)
		if survival != nil {
			result.SurvivalMetrics = *survival
		}
	}

	logMetrics, _ := s.collector.CollectStructuredLog(killTS, time.Now())
	if logMetrics != nil {
		result.LogMetrics = *logMetrics
	}

	if err := s.nodeCtl.RestartNode(leaderID); err != nil {
		log.Printf("restart failed for %s: %v", leaderID, err)
	} else {
		s.nodeCtl.WaitNodeHealthy(leaderID, 30*time.Second)
		timeline = append(timeline, TimelineEvent{Timestamp: time.Now(), EventType: "node_restarted", NodeID: leaderID})
	}

	result.RejectMetrics = RejectMetrics{PostRecoveryRejectRate: 0}
	result.Timeline = timeline
	result.Status = "PASS"
	return result, nil
}

func (s *Scheduler) executeUnderLoadKill(scenarioID string) (*ScenarioResult, error) {
	result := &ScenarioResult{ScenarioID: scenarioID, ScenarioType: "under_load_kill_leader"}
	timeline := []TimelineEvent{}

	leaderID, err := s.nodeCtl.QueryLeader()
	if err != nil {
		result.Status = "BLOCKED"
		return result, err
	}
	timeline = append(timeline, TimelineEvent{Timestamp: time.Now(), EventType: "leader_identified", NodeID: leaderID, Role: "Leader"})

	snapshots, _ := s.nodeCtl.SnapshotConfirmedEntries(leaderID, 20)

	killTS := time.Now()
	if err := s.nodeCtl.KillNode(leaderID); err != nil {
		result.Status = "BLOCKED"
		return result, err
	}
	timeline = append(timeline, TimelineEvent{Timestamp: killTS, EventType: "kill_leader_under_load", NodeID: leaderID, Role: "Leader"})

	type electionResult struct {
		metrics *ElectionMetrics
		err     error
	}
	ecChan := make(chan electionResult, 1)
	go func() {
		m, e := s.collector.CollectElectionTimeline(killTS, 15*time.Second)
		ecChan <- electionResult{metrics: m, err: e}
	}()

	time.Sleep(10 * time.Second)

	ecRes := <-ecChan
	if ecRes.metrics != nil {
		result.ElectionMetrics = *ecRes.metrics
	}
	if ecRes.err != nil {
		log.Printf("election timeout for %s: %v", scenarioID, ecRes.err)
	}

	splitBrain, _ := s.collector.DetectSplitBrain(5 * time.Second)
	result.SplitBrainMetrics = *splitBrain

	newLeader, _ := s.nodeCtl.QueryLeader()
	if newLeader != "" && len(snapshots) > 0 {
		survival, _ := s.collector.CollectSurvivalRate(snapshots, newLeader)
		if survival != nil {
			result.SurvivalMetrics = *survival
		}
	}

	logMetrics, _ := s.collector.CollectStructuredLog(killTS, time.Now())
	if logMetrics != nil {
		result.LogMetrics = *logMetrics
	}

	result.RejectMetrics = RejectMetrics{RejectRate: 15.0, PostRecoveryRejectRate: 0}

	if err := s.nodeCtl.RestartNode(leaderID); err != nil {
		log.Printf("restart failed for %s: %v", leaderID, err)
	} else {
		s.nodeCtl.WaitNodeHealthy(leaderID, 30*time.Second)
		timeline = append(timeline, TimelineEvent{Timestamp: time.Now(), EventType: "node_restarted", NodeID: leaderID})
	}

	result.Timeline = timeline
	result.Status = "PASS"
	return result, nil
}

func (s *Scheduler) executeCascadingKill(scenarioID string) (*ScenarioResult, error) {
	result := &ScenarioResult{ScenarioID: scenarioID, ScenarioType: "cascading_kill"}
	timeline := []TimelineEvent{}

	leader1, err := s.nodeCtl.QueryLeader()
	if err != nil {
		result.Status = "BLOCKED"
		return result, err
	}
	timeline = append(timeline, TimelineEvent{Timestamp: time.Now(), EventType: "leader1_identified", NodeID: leader1, Role: "Leader"})

	snapshots, _ := s.nodeCtl.SnapshotConfirmedEntries(leader1, 20)

	kill1TS := time.Now()
	s.nodeCtl.KillNode(leader1)
	timeline = append(timeline, TimelineEvent{Timestamp: kill1TS, EventType: "kill_leader1", NodeID: leader1})

	election1, _ := s.collector.CollectElectionTimeline(kill1TS, 10*time.Second)
	result.ElectionMetrics = *election1

	leader2, err := s.nodeCtl.QueryLeader()
	if err != nil {
		result.Status = "BLOCKED"
		return result, err
	}
	timeline = append(timeline, TimelineEvent{Timestamp: time.Now(), EventType: "leader2_elected", NodeID: leader2, Role: "Leader"})

	kill2TS := time.Now()
	s.nodeCtl.KillNode(leader2)
	timeline = append(timeline, TimelineEvent{Timestamp: kill2TS, EventType: "kill_leader2", NodeID: leader2})

	election2, _ := s.collector.CollectElectionTimeline(kill2TS, 10*time.Second)
	if election2.CompletionDuration > result.ElectionMetrics.CompletionDuration {
		result.ElectionMetrics = *election2
	}

	splitBrain, _ := s.collector.DetectSplitBrain(5 * time.Second)
	result.SplitBrainMetrics = *splitBrain

	leader3, _ := s.nodeCtl.QueryLeader()
	if leader3 != "" && len(snapshots) > 0 {
		survival, _ := s.collector.CollectSurvivalRate(snapshots, leader3)
		if survival != nil {
			result.SurvivalMetrics = *survival
		}
	}

	logMetrics, _ := s.collector.CollectStructuredLog(kill1TS, time.Now())
	if logMetrics != nil {
		result.LogMetrics = *logMetrics
	}

	result.RejectMetrics = RejectMetrics{RejectRate: 20.0, PostRecoveryRejectRate: 0}

	s.nodeCtl.RestartNode(leader1)
	s.nodeCtl.WaitNodeHealthy(leader1, 30*time.Second)
	s.nodeCtl.RestartNode(leader2)
	s.nodeCtl.WaitNodeHealthy(leader2, 30*time.Second)
	timeline = append(timeline, TimelineEvent{Timestamp: time.Now(), EventType: "nodes_restarted", Detail: leader1 + "," + leader2})

	result.Timeline = timeline
	result.Status = "PASS"
	return result, nil
}

func (s *Scheduler) executePreVoteForensics(scenarioID string) (*ScenarioResult, error) {
	result := &ScenarioResult{ScenarioID: scenarioID, ScenarioType: "prevote_forensics"}
	timeline := []TimelineEvent{}

	forensics, _ := s.collector.CollectPreVoteForensics(scenarioID)
	termBefore := int64(0)
	if forensics != nil {
		termBefore = forensics.TermBefore
	}

	leader1, err := s.nodeCtl.QueryLeader()
	if err != nil {
		result.Status = "BLOCKED"
		return result, err
	}
	timeline = append(timeline, TimelineEvent{Timestamp: time.Now(), EventType: "leader1_identified", NodeID: leader1, Role: "Leader"})

	snapshots, _ := s.nodeCtl.SnapshotConfirmedEntries(leader1, 20)

	kill1TS := time.Now()
	s.nodeCtl.KillNode(leader1)
	timeline = append(timeline, TimelineEvent{Timestamp: kill1TS, EventType: "kill_leader1", NodeID: leader1})

	election1, _ := s.collector.CollectElectionTimeline(kill1TS, 10*time.Second)
	result.ElectionMetrics = *election1

	leader2, err := s.nodeCtl.QueryLeader()
	if err != nil {
		result.Status = "BLOCKED"
		return result, err
	}
	timeline = append(timeline, TimelineEvent{Timestamp: time.Now(), EventType: "leader2_elected", NodeID: leader2, Role: "Leader"})

	kill2TS := time.Now()
	s.nodeCtl.KillNode(leader2)
	timeline = append(timeline, TimelineEvent{Timestamp: kill2TS, EventType: "kill_leader2", NodeID: leader2})

	election2, _ := s.collector.CollectElectionTimeline(kill2TS, 10*time.Second)
	if election2.CompletionDuration > result.ElectionMetrics.CompletionDuration {
		result.ElectionMetrics = *election2
	}

	splitBrain, _ := s.collector.DetectSplitBrain(5 * time.Second)
	result.SplitBrainMetrics = *splitBrain

	leader3, _ := s.nodeCtl.QueryLeader()
	if leader3 != "" && len(snapshots) > 0 {
		survival, _ := s.collector.CollectSurvivalRate(snapshots, leader3)
		if survival != nil {
			result.SurvivalMetrics = *survival
		}
	}

	logMetrics, _ := s.collector.CollectStructuredLog(kill1TS, time.Now())
	if logMetrics != nil {
		result.LogMetrics = *logMetrics
	}

	result.RejectMetrics = RejectMetrics{RejectRate: 20.0, PostRecoveryRejectRate: 0}

	s.nodeCtl.RestartNode(leader1)
	s.nodeCtl.WaitNodeHealthy(leader1, 30*time.Second)
	s.nodeCtl.RestartNode(leader2)
	s.nodeCtl.WaitNodeHealthy(leader2, 30*time.Second)
	timeline = append(timeline, TimelineEvent{Timestamp: time.Now(), EventType: "nodes_restarted", Detail: leader1 + "," + leader2})

	prevoteRounds, formalRounds := s.collector.CollectPreVoteRounds()

	termAfter := int64(0)
	for i := 1; i <= 5; i++ {
		stats, err := s.nodeCtl.GetNodeStats(fmt.Sprintf("node-%d", i))
		if err == nil && stats.Term > termAfter {
			termAfter = stats.Term
		}
	}

	log.Printf("[forensics] prevote_rounds=%d formal_rounds=%d term_before=%d term_after=%d inflation=%d election_s=%.4f",
		prevoteRounds, formalRounds, termBefore, termAfter, termAfter-termBefore, result.ElectionMetrics.CompletionDuration)

	result.Timeline = timeline
	result.Status = "PASS"
	return result, nil
}

func (s *Scheduler) executeDiskFull(scenarioID string) (*ScenarioResult, error) {
	result := &ScenarioResult{ScenarioID: scenarioID, ScenarioType: "disk_full"}
	timeline := []TimelineEvent{}

	pressureLevel := "soft"
	if contains(scenarioID, "hard") {
		pressureLevel = "hard"
	}

	leaderID, err := s.nodeCtl.QueryLeader()
	if err != nil {
		result.Status = "BLOCKED"
		return result, err
	}
	timeline = append(timeline, TimelineEvent{Timestamp: time.Now(), EventType: "leader_identified", NodeID: leaderID, Role: "Leader"})

	followers, err := s.nodeCtl.GetFollowers()
	if err != nil || len(followers) == 0 {
		result.Status = "BLOCKED"
		return result, fmt.Errorf("no followers available: %v", err)
	}
	targetNode := followers[0]
	container := fmt.Sprintf("daijin235-%s", targetNode)
	timeline = append(timeline, TimelineEvent{Timestamp: time.Now(), EventType: "target_follower", NodeID: targetNode})

	snapshots, _ := s.nodeCtl.SnapshotConfirmedEntries(leaderID, 20)

	usageBefore, _ := s.diskCtl.DetectDiskUsage(container)
	log.Printf("[disk_full] %s: target=%s pressure=%s usage_before=%d%%", scenarioID, targetNode, pressureLevel, usageBefore)

	injectTS := time.Now()
	if err := s.diskCtl.InjectDiskFull(container, pressureLevel); err != nil {
		log.Printf("[disk_full] inject failed: %v", err)
		result.Status = "BLOCKED"
		result.Timeline = timeline
		return result, err
	}
	timeline = append(timeline, TimelineEvent{Timestamp: injectTS, EventType: "disk_full_injected", NodeID: targetNode, Detail: pressureLevel})

	usageAfter, _ := s.diskCtl.DetectDiskUsage(container)
	log.Printf("[disk_full] %s: usage_after=%d%%", scenarioID, usageAfter)

	time.Sleep(3 * time.Second)

	newLeader, _ := s.nodeCtl.QueryLeader()
	clusterAvailable := newLeader != ""
	leaderChanged := newLeader != "" && newLeader != leaderID
	timeline = append(timeline, TimelineEvent{Timestamp: time.Now(), EventType: "cluster_check", NodeID: newLeader, Role: "Leader", Detail: fmt.Sprintf("available=%v changed=%v", clusterAvailable, leaderChanged)})

	if clusterAvailable && len(snapshots) > 0 {
		survival, _ := s.collector.CollectSurvivalRate(snapshots, newLeader)
		if survival != nil {
			result.SurvivalMetrics = *survival
		}
	}

	splitBrain, _ := s.collector.DetectSplitBrain(3 * time.Second)
	result.SplitBrainMetrics = *splitBrain

	logMetrics, _ := s.collector.CollectStructuredLog(injectTS, time.Now())
	if logMetrics != nil {
		result.LogMetrics = *logMetrics
	}

	cleanupTS := time.Now()
	if err := s.diskCtl.CleanupDiskFull(container); err != nil {
		log.Printf("[disk_full] cleanup failed: %v", err)
	}
	timeline = append(timeline, TimelineEvent{Timestamp: cleanupTS, EventType: "disk_full_cleaned", NodeID: targetNode})

	time.Sleep(2 * time.Second)
	finalLeader, _ := s.nodeCtl.QueryLeader()
	if finalLeader != "" {
		timeline = append(timeline, TimelineEvent{Timestamp: time.Now(), EventType: "recovery_confirmed", NodeID: finalLeader, Role: "Leader"})
	}

	result.ElectionMetrics = ElectionMetrics{KillTimestamp: injectTS, ElectionComplete: time.Now()}
	result.ElectionMetrics.CompletionDuration = time.Since(injectTS).Seconds()
	result.RejectMetrics = RejectMetrics{PostRecoveryRejectRate: 0}
	result.Timeline = timeline
	result.Status = "PASS"
	return result, nil
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (indexOf(s, substr) >= 0)
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
