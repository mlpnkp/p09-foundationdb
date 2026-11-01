package app

import (
	"slices"
	"time"

	"github.com/mlpnkp/p09-foundationdb/internal/domain"
)

type Summary struct {
	Txns          int
	Committed     int
	Attempts      int
	Conflicts     int
	Unknown       int
	Aborted       int
	ConflictRate  float64
	RetriesPerTxn float64
	Throughput    float64
	P50, P95, P99 time.Duration
}

type txnSpan struct {
	start     time.Time
	committed time.Time
}

func Summarize(history []domain.Attempt) Summary {
	if len(history) == 0 {
		return Summary{}
	}
	var s Summary
	spans := make(map[domain.TxnID]*txnSpan)
	first, last := history[0].Start, history[0].End
	for i := range history {
		a := &history[i]
		s.count(a.Outcome)
		first = earliest(first, a.Start)
		last = latest(last, a.End)
		span, ok := spans[a.Txn]
		if !ok {
			span = &txnSpan{start: a.Start}
			spans[a.Txn] = span
		}
		span.start = earliest(span.start, a.Start)
		if a.Outcome.Committed() {
			span.committed = a.End
		}
	}

	latencies := make([]time.Duration, 0, len(spans))
	for _, span := range spans {
		if !span.committed.IsZero() {
			latencies = append(latencies, span.committed.Sub(span.start))
		}
	}
	slices.Sort(latencies)

	s.Txns = len(spans)
	s.Committed = len(latencies)
	s.ConflictRate = float64(s.Conflicts) / float64(s.Attempts)
	s.RetriesPerTxn = float64(s.Attempts-s.Txns) / float64(s.Txns)
	if window := last.Sub(first).Seconds(); window > 0 {
		s.Throughput = float64(s.Committed) / window
	}
	s.P50 = percentile(latencies, 50)
	s.P95 = percentile(latencies, 95)
	s.P99 = percentile(latencies, 99)
	return s
}

func (s *Summary) count(o domain.Outcome) {
	s.Attempts++
	switch o {
	case domain.OutcomeConflict:
		s.Conflicts++
	case domain.OutcomeUnknown:
		s.Unknown++
	case domain.OutcomeAborted:
		s.Aborted++
	case domain.OutcomeCommitted:
	}
}

func percentile(sorted []time.Duration, p int) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	rank := (p*len(sorted) + 99) / 100
	return sorted[max(rank, 1)-1]
}

func earliest(a, b time.Time) time.Time {
	if b.Before(a) {
		return b
	}
	return a
}

func latest(a, b time.Time) time.Time {
	if b.After(a) {
		return b
	}
	return a
}
