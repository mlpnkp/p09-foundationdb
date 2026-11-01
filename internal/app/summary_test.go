package app_test

import (
	"math"
	"testing"
	"time"

	"github.com/mlpnkp/p09-foundationdb/internal/app"
	"github.com/mlpnkp/p09-foundationdb/internal/domain"
)

func at(ms int) time.Time {
	return time.Unix(1_800_000_000, 0).Add(time.Duration(ms) * time.Millisecond)
}

func TestSummarize_KnownHistory(t *testing.T) {
	history := []domain.Attempt{
		{Txn: "a", Number: 1, Start: at(0), End: at(10), Outcome: domain.OutcomeConflict},
		{Txn: "a", Number: 2, Start: at(10), End: at(30), Outcome: domain.OutcomeCommitted},
		{Txn: "b", Number: 1, Start: at(0), End: at(10), Outcome: domain.OutcomeCommitted},
		{Txn: "c", Number: 1, Start: at(0), End: at(50), Outcome: domain.OutcomeUnknown},
	}

	got := app.Summarize(history)

	want := app.Summary{
		Txns: 3, Committed: 2, Attempts: 4, Conflicts: 1, Unknown: 1,
		P50: 10 * time.Millisecond, P95: 30 * time.Millisecond, P99: 30 * time.Millisecond,
	}
	if got.Txns != want.Txns || got.Committed != want.Committed || got.Attempts != want.Attempts ||
		got.Conflicts != want.Conflicts || got.Unknown != want.Unknown || got.Aborted != 0 {
		t.Fatalf("counts = %+v, want %+v", got, want)
	}
	if got.P50 != want.P50 || got.P95 != want.P95 || got.P99 != want.P99 {
		t.Fatalf("percentiles = %v %v %v, want %v %v %v", got.P50, got.P95, got.P99, want.P50, want.P95, want.P99)
	}
	near := func(name string, got, want float64) {
		if math.Abs(got-want) > 1e-9 {
			t.Errorf("%s = %v, want %v", name, got, want)
		}
	}
	near("ConflictRate", got.ConflictRate, 0.25)
	near("RetriesPerTxn", got.RetriesPerTxn, 1.0/3)
	near("Throughput", got.Throughput, 40)
}

func TestSummarize_EmptyHistory(t *testing.T) {
	if got := app.Summarize(nil); got != (app.Summary{}) {
		t.Fatalf("Summarize(nil) = %+v, want zero", got)
	}
}
