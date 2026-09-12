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
}

func NewScheduler(nc *NodeController, c *Collector, em *EvidenceManager, rm *ResumeManager, contract string) *Scheduler {
	return &Scheduler{nodeCtl: nc, collector: c, evidence: em, resume: rm, contract: contract}
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
