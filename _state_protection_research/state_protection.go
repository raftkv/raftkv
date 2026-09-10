package stateprotection

import (
	"fmt"
	"os"
	"sync"
	"time"
)

type StateProtector struct {
	config      StateProtectorConfig
	persistence *StatePersistence
	hmac        *HMACCalculator
	keyDeriver  *HMACKeyDeriver
	resync      *StateResyncManager
	metrics     *IntegrityMetrics
	logger      Logger

	hmacKey      []byte
	snapshotMu   sync.Mutex
	lastSnapshot VoteRecord
	stopCh       chan struct{}
	stopped      bool
}

func NewStateProtector(cfg StateProtectorConfig, logger Logger) (*StateProtector, error) {
	if cfg.StateBinPath == "" {
		return nil, fmt.Errorf("StateBinPath 不能为空")
	}
	if cfg.StateHmacPath == "" {
		return nil, fmt.Errorf("StateHmacPath 不能为空")
	}
	if cfg.SnapshotIntervalMs <= 0 {
		cfg.SnapshotIntervalMs = 100
	}

	metrics := NewIntegrityMetrics()

	return &StateProtector{
		config:      cfg,
		persistence: NewStatePersistence(cfg.StateBinPath),
		hmac:        NewHMACCalculator(cfg.StateHmacPath),
		keyDeriver:  NewHMACKeyDeriver(cfg.RsaPubKeyPath, cfg.HmacKeyPath),
		resync:      NewStateResyncManager(cfg.ResyncConfig, metrics, logger),
		metrics:     metrics,
		logger:      logger,
		stopCh:      make(chan struct{}),
	}, nil
}

func (p *StateProtector) LoadAndVerify() (LoadResult, error) {
	p.metrics.IncCheckTotal()

	key, err := p.keyDeriver.Derive()
	if err != nil {
		return LoadResult{Status: INTEGRITY_TAMPERED, Error: err}, err
	}
	p.hmacKey = key

	if !p.persistence.Exists() {
		if p.logger != nil {
			p.logger.Printf("[STATE-PROTECTION] INFO new node, no state.bin found")
		}
		return LoadResult{Status: INTEGRITY_NEW_NODE}, nil
	}

	vr, err := p.persistence.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[CRITICAL] state.bin integrity check failed\n")
		if p.logger != nil {
			p.logger.Printf("[CRITICAL] state.bin integrity check failed: %v", err)
		}
		p.persistence.Discard()
		p.hmac.DiscardHmac()
		p.metrics.IncCheckFailed()
		return LoadResult{Status: INTEGRITY_TAMPERED}, nil
	}

	if !p.hmac.ExistsHmac() {
		if p.logger != nil {
			p.logger.Printf("[STATE-PROTECTION] INFO legacy init, generating HMAC for existing state.bin")
		}
		data := p.persistence.Serialize(vr)
		hmac := p.hmac.Compute(data, key)
		if err := p.hmac.SaveHmac(hmac); err != nil {
			return LoadResult{Status: INTEGRITY_TAMPERED, Error: err}, err
		}
		return LoadResult{Status: INTEGRITY_LEGACY_NO_HMAC, VoteRecord: &vr}, nil
	}

	storedHmac, err := p.hmac.LoadHmac()
	if err != nil {
		return LoadResult{Status: INTEGRITY_TAMPERED, Error: err}, err
	}

	data := p.persistence.Serialize(vr)
	if !p.hmac.Verify(data, key, storedHmac) {
		fmt.Fprintf(os.Stderr, "[CRITICAL] state.bin integrity check failed\n")
		if p.logger != nil {
			p.logger.Printf("[CRITICAL] state.bin integrity check failed")
		}

		p.persistence.Discard()
		p.hmac.DiscardHmac()
		p.metrics.IncCheckFailed()

		return LoadResult{Status: INTEGRITY_TAMPERED}, nil
	}

	return LoadResult{Status: INTEGRITY_OK, VoteRecord: &vr}, nil
}

func (p *StateProtector) PersistVoteRecord(vr VoteRecord) error {
	data := p.persistence.Serialize(vr)

	if err := p.persistence.Save(vr); err != nil {
		return err
	}

	hmac := p.hmac.Compute(data, p.hmacKey)
	if err := p.hmac.SaveHmac(hmac); err != nil {
		return err
	}

	return nil
}

func (p *StateProtector) StartSnapshotLoop(node RaftNodeAdapter) func() {
	p.stopCh = make(chan struct{})
	p.stopped = false

	go func() {
		ticker := time.NewTicker(time.Duration(p.config.SnapshotIntervalMs) * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-p.stopCh:
				return
			case <-ticker.C:
				p.snapshotMu.Lock()
				currentTerm := node.Term()
				currentVotedFor := node.GetVotedFor()

				if currentTerm != p.lastSnapshot.Term || currentVotedFor != p.lastSnapshot.VotedFor {
					vr := VoteRecord{Term: currentTerm, VotedFor: currentVotedFor}
					if err := p.PersistVoteRecord(vr); err != nil {
						if p.logger != nil {
							p.logger.Printf("[STATE-PROTECTION] WARN snapshot persist failed: %v", err)
						}
					} else {
						p.lastSnapshot = vr
						if p.logger != nil {
							p.logger.Printf("[STATE-PROTECTION] DEBUG snapshot saved (term=%d, votedFor=%s)", currentTerm, currentVotedFor)
						}
					}
				}
				p.snapshotMu.Unlock()
			}
		}
	}()

	return func() {
		p.snapshotMu.Lock()
		defer p.snapshotMu.Unlock()
		if !p.stopped {
			close(p.stopCh)
			p.stopped = true
		}
	}
}

func (p *StateProtector) Shutdown() error {
	p.snapshotMu.Lock()
	defer p.snapshotMu.Unlock()
	if !p.stopped {
		close(p.stopCh)
		p.stopped = true
	}
	return nil
}

func (p *StateProtector) Metrics() *IntegrityMetrics {
	return p.metrics
}

func (p *StateProtector) HandleTampered(node RaftNodeAdapter) bool {
	result := p.resync.TriggerResync(node)
	if result.Success {
		err := p.resync.RebuildState(node, p.persistence, p.hmac, p.hmacKey)
		if err != nil {
			if p.logger != nil {
				p.logger.Printf("[STATE-PROTECTION] ERROR state rebuild failed: %v", err)
			}
			return false
		}
		return true
	}
	return false
}
