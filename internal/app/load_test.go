package app_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/mlpnkp/p09-foundationdb/internal/app"
	"github.com/mlpnkp/p09-foundationdb/internal/domain"
)

type fakeRunner struct {
	attemptsPerTxn int
	failAfter      int
	mu             sync.Mutex
	calls          int
}

var errRunner = errors.New("runner broke")

func (r *fakeRunner) RunTxn(ctx context.Context, spec domain.TxnSpec, observe func(domain.Attempt)) error {
	r.mu.Lock()
	r.calls++
	calls := r.calls
	r.mu.Unlock()
	if r.failAfter > 0 && calls > r.failAfter {
		return errRunner
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	for n := 1; n <= r.attemptsPerTxn; n++ {
		outcome := domain.OutcomeConflict
		if n == r.attemptsPerTxn {
			outcome = domain.OutcomeCommitted
		}
		observe(domain.Attempt{Txn: spec.ID, Number: n, Client: spec.Client, Outcome: outcome})
	}
	time.Sleep(time.Millisecond)
	return nil
}

type memJournal struct {
	mu       sync.Mutex
	attempts []domain.Attempt
	failAt   int
}

var errJournal = errors.New("disk full")

func (j *memJournal) Append(a domain.Attempt) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.failAt > 0 && len(j.attempts) >= j.failAt {
		return errJournal
	}
	j.attempts = append(j.attempts, a)
	return nil
}

func config(t *testing.T, clients int) app.LoadConfig {
	t.Helper()
	w, err := domain.NewWorkload(10, 2)
	if err != nil {
		t.Fatal(err)
	}
	return app.LoadConfig{Clients: clients, Seed: 1, Workload: w}
}

func TestRunLoad_JournalsEveryAttempt(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	runner := &fakeRunner{attemptsPerTxn: 3}
	journal := &memJournal{}

	if err := app.RunLoad(ctx, config(t, 2), runner, journal); err != nil {
		t.Fatalf("RunLoad: %v", err)
	}

	perTxn := make(map[domain.TxnID]int)
	for _, a := range journal.attempts {
		perTxn[a.Txn]++
	}
	if len(perTxn) == 0 {
		t.Fatal("journal is empty")
	}
	for id, n := range perTxn {
		if n != 3 {
			t.Errorf("txn %s journaled %d attempts, want 3", id, n)
		}
	}
}

func TestRunLoad_TxnIDsUnique(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	journal := &memJournal{}

	if err := app.RunLoad(ctx, config(t, 4), &fakeRunner{attemptsPerTxn: 1}, journal); err != nil {
		t.Fatalf("RunLoad: %v", err)
	}

	seen := make(map[domain.TxnID]domain.ClientID)
	for _, a := range journal.attempts {
		if c, ok := seen[a.Txn]; ok && c != a.Client {
			t.Fatalf("txn %s used by clients %d and %d", a.Txn, c, a.Client)
		}
		seen[a.Txn] = a.Client
	}
	if len(seen) != len(journal.attempts) {
		t.Fatalf("%d attempts but %d distinct txn ids", len(journal.attempts), len(seen))
	}
}

func TestRunLoad_StopsWhenContextEnds(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()
	done := make(chan error, 1)

	go func() { done <- app.RunLoad(ctx, config(t, 3), &fakeRunner{attemptsPerTxn: 1}, &memJournal{}) }()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("RunLoad after deadline: %v, want nil", err)
		}
	case <-time.After(time.Second):
		t.Fatal("RunLoad did not stop after the context ended")
	}
}

func TestRunLoad_FailsOnJournalError(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()

	err := app.RunLoad(ctx, config(t, 2), &fakeRunner{attemptsPerTxn: 1}, &memJournal{failAt: 5})

	if !errors.Is(err, errJournal) {
		t.Fatalf("RunLoad error = %v, want journal error", err)
	}
}

func TestRunLoad_FailsOnRunnerError(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()

	err := app.RunLoad(ctx, config(t, 2), &fakeRunner{attemptsPerTxn: 1, failAfter: 3}, &memJournal{})

	if !errors.Is(err, errRunner) {
		t.Fatalf("RunLoad error = %v, want runner error", err)
	}
}

func TestRunLoad_RejectsNoClients(t *testing.T) {
	err := app.RunLoad(t.Context(), config(t, 0), &fakeRunner{attemptsPerTxn: 1}, &memJournal{})

	if !errors.Is(err, app.ErrInvalidLoad) {
		t.Fatalf("RunLoad error = %v, want ErrInvalidLoad", err)
	}
}
