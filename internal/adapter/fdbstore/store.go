package fdbstore

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/apple/foundationdb/bindings/go/src/fdb"

	"github.com/mlpnkp/p09-foundationdb/internal/domain"
)

const (
	apiVersion       = 800
	codeTooOld       = 1007
	codeNotCommitted = 1020
	codeTimedOut     = 1031
)

type Store struct {
	db     fdb.Database
	prefix string
}

func Open(clusterFile, prefix string, attemptTimeout time.Duration) (*Store, error) {
	if err := fdb.APIVersion(apiVersion); err != nil {
		return nil, fmt.Errorf("select api version %d: %w", apiVersion, err)
	}
	db, err := fdb.OpenDatabase(clusterFile)
	if err != nil {
		return nil, fmt.Errorf("open database %q: %w", clusterFile, err)
	}
	if err := db.Options().SetTransactionTimeout(attemptTimeout.Milliseconds()); err != nil {
		db.Close()
		return nil, fmt.Errorf("set transaction timeout %s: %w", attemptTimeout, err)
	}
	return &Store{db: db, prefix: prefix}, nil
}

func (s *Store) Close() {
	s.db.Close()
}

func (s *Store) Clear(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	begin := fdb.Key(s.prefix)
	end := fdb.Key(s.prefix + "\xff")
	_, err := s.db.Transact(func(t fdb.Transaction) (any, error) {
		t.ClearRange(fdb.KeyRange{Begin: begin, End: end})
		return nil, nil
	})
	if err != nil {
		return fmt.Errorf("clear prefix %q: %w", s.prefix, err)
	}
	return nil
}

func (s *Store) RunTxn(ctx context.Context, spec domain.TxnSpec, observe func(domain.Attempt)) error {
	tr, err := s.newTxn()
	if err != nil {
		return err
	}
	for n := 1; ; n++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		a, attemptErr := s.attempt(tr, spec, n)
		observe(a)
		if attemptErr == nil {
			return nil
		}
		if tr, err = s.retry(tr, attemptErr); err != nil {
			return fmt.Errorf("txn %q attempt %d: %w", spec.ID, n, err)
		}
	}
}

func (s *Store) newTxn() (fdb.Transaction, error) {
	tr, err := s.db.CreateTransaction()
	if err != nil {
		return fdb.Transaction{}, fmt.Errorf("create transaction: %w", err)
	}
	return tr, nil
}

func (s *Store) retry(tr fdb.Transaction, cause error) (fdb.Transaction, error) {
	fe, ok := errors.AsType[fdb.Error](cause)
	if !ok {
		return tr, cause
	}
	err := tr.OnError(fe).Get()
	if err == nil {
		return tr, nil
	}
	if timedOut(err) {
		tr.Cancel()
		return s.newTxn()
	}
	return tr, fmt.Errorf("not retryable: %w", err)
}

func (s *Store) attempt(tr fdb.Transaction, spec domain.TxnSpec, n int) (domain.Attempt, error) {
	a := domain.Attempt{Txn: spec.ID, Number: n, Client: spec.Client, Snapshot: spec.Snapshot, Start: time.Now()}

	rv, err := tr.GetReadVersion().Get()
	if err != nil {
		return finish(a, domain.OutcomeAborted, err), err
	}
	a.ReadVersion = rv

	if a.Reads, err = s.read(tr, spec); err != nil {
		return finish(a, domain.OutcomeAborted, err), err
	}

	value := spec.Value(n)
	a.Writes = make([]domain.Observation, 0, len(spec.Keys))
	for _, k := range spec.Keys {
		tr.Set(s.key(k), []byte(value))
		a.Writes = append(a.Writes, domain.Observation{Key: k, Value: value})
	}

	if err := tr.Commit().Get(); err != nil {
		return finish(a, commitOutcome(err), err), err
	}
	if a.CommitVersion, err = tr.GetCommittedVersion(); err != nil {
		a.CommitVersion = -1
	}
	return finish(a, domain.OutcomeCommitted, nil), nil
}

func (s *Store) read(tr fdb.Transaction, spec domain.TxnSpec) ([]domain.Observation, error) {
	var rt fdb.ReadTransaction = tr
	if spec.Snapshot {
		rt = tr.Snapshot()
	}
	futures := make([]fdb.FutureByteSlice, 0, len(spec.Keys))
	for _, k := range spec.Keys {
		futures = append(futures, rt.Get(s.key(k)))
	}
	reads := make([]domain.Observation, 0, len(spec.Keys))
	for i, f := range futures {
		v, err := f.Get()
		if err != nil {
			return reads, err
		}
		reads = append(reads, domain.Observation{Key: spec.Keys[i], Value: domain.Value(v)})
	}
	return reads, nil
}

func (s *Store) key(k domain.Key) fdb.Key {
	return fdb.Key(s.prefix + string(k))
}

func commitOutcome(err error) domain.Outcome {
	fe, ok := errors.AsType[fdb.Error](err)
	if !ok {
		return domain.OutcomeUnknown
	}
	switch fe.Code {
	case codeNotCommitted:
		return domain.OutcomeConflict
	case codeTooOld:
		return domain.OutcomeAborted
	default:
		return domain.OutcomeUnknown
	}
}

func timedOut(err error) bool {
	fe, ok := errors.AsType[fdb.Error](err)
	return ok && fe.Code == codeTimedOut
}

func finish(a domain.Attempt, outcome domain.Outcome, err error) domain.Attempt {
	a.End = time.Now()
	a.Outcome = outcome
	if fe, ok := errors.AsType[fdb.Error](err); ok {
		a.ErrorCode = fe.Code
	}
	return a
}
