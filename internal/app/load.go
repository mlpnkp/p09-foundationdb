package app

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"sync"

	"github.com/mlpnkp/p09-foundationdb/internal/domain"
)

var ErrInvalidLoad = errors.New("invalid load")

type TxnRunner interface {
	RunTxn(ctx context.Context, spec domain.TxnSpec, observe func(domain.Attempt)) error
}

type Journal interface {
	Append(a domain.Attempt) error
}

type LoadConfig struct {
	Clients  int
	Seed     uint64
	Workload domain.Workload
	Snapshot bool
}

type firstFailure struct {
	mu     sync.Mutex
	err    error
	cancel context.CancelFunc
}

func (f *firstFailure) record(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err == nil {
		f.err = err
	}
	f.cancel()
}

func (f *firstFailure) result() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.err
}

func RunLoad(ctx context.Context, cfg LoadConfig, runner TxnRunner, journal Journal) error {
	if cfg.Clients < 1 {
		return fmt.Errorf("clients %d: %w", cfg.Clients, ErrInvalidLoad)
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	failure := &firstFailure{cancel: cancel}

	var wg sync.WaitGroup
	for c := range cfg.Clients {
		wg.Go(func() {
			runClient(ctx, cfg, domain.ClientID(c), runner, journal, failure)
		})
	}
	wg.Wait()
	return failure.result()
}

func runClient(ctx context.Context, cfg LoadConfig, client domain.ClientID, runner TxnRunner, journal Journal, failure *firstFailure) {
	rng := rand.New(rand.NewPCG(cfg.Seed, client.Stream())) //nolint:gosec // seeded PRNG keeps every run reproducible
	observe := func(a domain.Attempt) {
		if err := journal.Append(a); err != nil {
			failure.record(fmt.Errorf("journal txn %q attempt %d: %w", a.Txn, a.Number, err))
		}
	}
	for seq := 0; ctx.Err() == nil; seq++ {
		spec := domain.TxnSpec{
			ID:       domain.NewTxnID(client, seq),
			Client:   client,
			Keys:     cfg.Workload.Pick(rng),
			Snapshot: cfg.Snapshot,
		}
		if err := runner.RunTxn(ctx, spec, observe); err != nil && ctx.Err() == nil {
			failure.record(fmt.Errorf("run txn %q: %w", spec.ID, err))
			return
		}
	}
}
