package domain_test

import (
	"errors"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/mlpnkp/p09-foundationdb/internal/domain"
)

func TestNewWorkload_RejectsBadSizes(t *testing.T) {
	cases := []struct {
		name        string
		hot, perTxn int
	}{
		{"zero hot keys", 0, 1},
		{"zero keys per txn", 10, 0},
		{"more keys than hot set", 3, 4},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := domain.NewWorkload(tc.hot, tc.perTxn)
			if !errors.Is(err, domain.ErrInvalidWorkload) {
				t.Fatalf("NewWorkload(%d, %d) error = %v, want ErrInvalidWorkload", tc.hot, tc.perTxn, err)
			}
		})
	}
}

func TestWorkloadPick_DistinctKeysInsideHotSet(t *testing.T) {
	w, err := domain.NewWorkload(5, 3)
	if err != nil {
		t.Fatal(err)
	}
	hot := make(map[domain.Key]bool)
	for i := range 5 {
		hot[w.Key(i)] = true
	}
	rng := rand.New(rand.NewPCG(1, 2))
	for range 1000 {
		keys := w.Pick(rng)
		if len(keys) != 3 {
			t.Fatalf("Pick returned %d keys, want 3", len(keys))
		}
		if !slices.IsSorted(keys) {
			t.Fatalf("Pick returned unsorted keys %v", keys)
		}
		if len(slices.Compact(slices.Clone(keys))) != 3 {
			t.Fatalf("Pick returned duplicate keys %v", keys)
		}
		for _, k := range keys {
			if !hot[k] {
				t.Fatalf("Pick returned key %q outside the hot set", k)
			}
		}
	}
}

func TestWorkloadPick_SameSeedSameKeys(t *testing.T) {
	w, err := domain.NewWorkload(100, 4)
	if err != nil {
		t.Fatal(err)
	}
	a := rand.New(rand.NewPCG(7, 3))
	b := rand.New(rand.NewPCG(7, 3))
	for range 100 {
		if ka, kb := w.Pick(a), w.Pick(b); !slices.Equal(ka, kb) {
			t.Fatalf("same seed gave %v and %v", ka, kb)
		}
	}
}

func TestTxnSpec_ValueUniquePerAttempt(t *testing.T) {
	seen := make(map[domain.Value]bool)
	for client := range 3 {
		for seq := range 3 {
			spec := domain.TxnSpec{ID: domain.NewTxnID(domain.ClientID(client), seq)}
			for attempt := 1; attempt <= 3; attempt++ {
				v := spec.Value(attempt)
				if seen[v] {
					t.Fatalf("value %q repeated", v)
				}
				seen[v] = true
			}
		}
	}
}

func TestOutcome_Committed(t *testing.T) {
	for _, o := range []domain.Outcome{domain.OutcomeConflict, domain.OutcomeAborted, domain.OutcomeUnknown} {
		if o.Committed() {
			t.Errorf("%s.Committed() = true", o)
		}
	}
	if !domain.OutcomeCommitted.Committed() {
		t.Error("committed.Committed() = false")
	}
}
