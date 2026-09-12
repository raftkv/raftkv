package main

import "time"

type ScenarioResult struct {
	ScenarioID        string            `json:"scenario_id"`
	ScenarioType      string            `json:"scenario_type"`
	Timeline          []TimelineEvent   `json:"timeline"`
	ElectionMetrics   ElectionMetrics   `json:"election_metrics"`
	RejectMetrics     RejectMetrics     `json:"reject_metrics"`
	SurvivalMetrics   SurvivalMetrics   `json:"survival_metrics"`
	SplitBrainMetrics SplitBrainMetrics `json:"split_brain_metrics"`
	LogMetrics        LogMetrics        `json:"log_metrics"`
	Status            string            `json:"status"`
	EvidencePath      string            `json:"evidence_path"`
}

type TimelineEvent struct {
	Timestamp time.Time `json:"timestamp"`
	EventType string    `json:"event_type"`
	NodeID    string    `json:"node_id"`
	Role      string    `json:"role"`
	Detail    string    `json:"detail"`
}

type ElectionMetrics struct {
	KillTimestamp      time.Time `json:"kill_timestamp"`
	ElectionStart      time.Time `json:"election_start"`
	ElectionComplete   time.Time `json:"election_complete"`
	CompletionDuration float64   `json:"completion_duration"`
}

type RejectMetrics struct {
	TotalRequests          int64     `json:"total_requests"`
	RejectedRequests       int64     `json:"rejected_requests"`
	RejectRate             float64   `json:"reject_rate"`
	RecoveryTimestamp      time.Time `json:"recovery_timestamp"`
	PostRecoveryRejectRate float64   `json:"post_recovery_reject_rate"`
}

type SurvivalMetrics struct {
	SampledEntriesCount  int             `json:"sampled_entries_count"`
	SurvivedEntriesCount int             `json:"survived_entries_count"`
	SurvivalRate         float64         `json:"survival_rate"`
	MismatchedEntries    []EntryMismatch `json:"mismatched_entries"`
}

type SplitBrainMetrics struct {
	MaxConcurrentLeaders int         `json:"max_concurrent_leaders"`
	SplitBrainDetected   bool        `json:"split_brain_detected"`
	DetectionTimestamps  []time.Time `json:"detection_timestamps"`
}

type LogMetrics struct {
	LogEntriesCount  int64 `json:"log_entries_count"`
	TimelineComplete bool  `json:"timeline_complete"`
	Replayable       bool  `json:"replayable"`
}

type EntryMismatch struct {
	EntryIndex    int64  `json:"entry_index"`
	ExpectedTerm  int64  `json:"expected_term"`
	ActualTerm    int64  `json:"actual_term"`
	ExpectedValue string `json:"expected_value"`
	ActualValue   string `json:"actual_value"`
}

type EntrySnapshot struct {
	EntryIndex int64  `json:"entry_index"`
	EntryTerm  int64  `json:"entry_term"`
	EntryValue string `json:"entry_value"`
}

type ResumeState struct {
	SessionID          string    `json:"session_id"`
	TotalScenarios     int       `json:"total_scenarios"`
	LastUpdated        time.Time `json:"last_updated"`
	CompletedScenarios []string  `json:"completed_scenarios"`
}

type RaftStats struct {
	ID      string `json:"id"`
	State   string `json:"state"`
	Term    int64  `json:"term"`
	Leader  string `json:"leader"`
	Commit  int64  `json:"commit"`
	Applied int64  `json:"applied"`
}

type SurvivalResult struct {
	SurvivalRate      float64         `json:"survival_rate"`
	SurvivedCount     int             `json:"survived_count"`
	TotalCount        int             `json:"total_count"`
	MismatchedEntries []EntryMismatch `json:"mismatched_entries"`
}
