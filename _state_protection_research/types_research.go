package stateprotection

import (
	"fmt"
	"time"
)

type IntegrityStatus int32

const (
	INTEGRITY_OK             IntegrityStatus = 0
	INTEGRITY_TAMPERED       IntegrityStatus = 1
	INTEGRITY_LEGACY_NO_HMAC IntegrityStatus = 2
	INTEGRITY_NEW_NODE       IntegrityStatus = 3
)

func (s IntegrityStatus) String() string {
	switch s {
	case INTEGRITY_OK:
		return "OK"
	case INTEGRITY_TAMPERED:
		return "TAMPERED"
	case INTEGRITY_LEGACY_NO_HMAC:
		return "LEGACY_NO_HMAC"
	case INTEGRITY_NEW_NODE:
		return "NEW_NODE"
	default:
		return "UNKNOWN"
	}
}

type KeySource int32

const (
	RSA_PUBLIC_KEY_HASH KeySource = 0
	RANDOM_FALLBACK     KeySource = 1
)

func (k KeySource) String() string {
	switch k {
	case RSA_PUBLIC_KEY_HASH:
		return "RSA_PUBLIC_KEY_HASH"
	case RANDOM_FALLBACK:
		return "RANDOM_FALLBACK"
	default:
		return "UNKNOWN"
	}
}

type VoteRecord struct {
	Term     int64
	VotedFor string
}

type LoadResult struct {
	Status     IntegrityStatus
	VoteRecord *VoteRecord
	Error      error
}

type StateProtectorConfig struct {
	StateBinPath       string
	StateHmacPath      string
	HmacKeyPath        string
	RsaPubKeyPath      string
	SnapshotIntervalMs int
	ResyncConfig       ResyncConfig
}

type ResyncConfig struct {
	TimeoutMs       int
	MaxRetry        int
	RetryIntervalMs int
}

type ResyncResult struct {
	Success    bool
	RetryCount int
	Duration   time.Duration
	Error      error
}

type Logger interface {
	Printf(format string, args ...any)
}

type StateError struct {
	Code    string
	Message string
	Cause   error
}

func (e *StateError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *StateError) Unwrap() error {
	return e.Cause
}

var (
	ErrStateBinNotFound = &StateError{Code: "STATE_BIN_NOT_FOUND", Message: "state.bin 文件不存在"}
	ErrHmacFileNotFound = &StateError{Code: "HMAC_FILE_NOT_FOUND", Message: "state.bin.hmac 文件不存在"}
	ErrHmacMismatch     = &StateError{Code: "HMAC_MISMATCH", Message: "HMAC 校验失败，state.bin 可能被篡改"}
	ErrKeyDeriveFailed  = &StateError{Code: "KEY_DERIVE_FAILED", Message: "HMAC 密钥派生失败"}
	ErrPersistFailed    = &StateError{Code: "PERSIST_FAILED", Message: "状态持久化失败"}
)

type RaftNodeAdapter interface {
	Term() int64
	LeaderID() string
	SetLogCaughtUp(caughtUp bool)
	GetVotedFor() string
}

type StdLogger struct {
	Prefix string
}

func (l *StdLogger) Printf(format string, args ...any) {
	fmt.Printf("%s "+format+"\n", append([]any{l.Prefix}, args...)...)
}
