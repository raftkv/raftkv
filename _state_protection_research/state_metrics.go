package stateprotection

import (
	"fmt"
	"net/http"
	"sync/atomic"
)

type IntegrityMetrics struct {
	checkTotal       uint64
	checkFailedTotal uint64
}

func NewIntegrityMetrics() *IntegrityMetrics {
	return &IntegrityMetrics{}
}

func (m *IntegrityMetrics) IncCheckTotal() {
	atomic.AddUint64(&m.checkTotal, 1)
}

func (m *IntegrityMetrics) IncCheckFailed() {
	atomic.AddUint64(&m.checkFailedTotal, 1)
}

func (m *IntegrityMetrics) GetCheckTotal() uint64 {
	return atomic.LoadUint64(&m.checkTotal)
}

func (m *IntegrityMetrics) GetCheckFailed() uint64 {
	return atomic.LoadUint64(&m.checkFailedTotal)
}

func (m *IntegrityMetrics) PrometheusFormat() string {
	return fmt.Sprintf(
		"state_integrity_check_total %d\nstate_integrity_check_failed_total %d\n",
		m.GetCheckTotal(),
		m.GetCheckFailed(),
	)
}

func (m *IntegrityMetrics) RegisterHTTPHandler(mux *http.ServeMux) {
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		fmt.Fprint(w, m.PrometheusFormat())
	})
}
