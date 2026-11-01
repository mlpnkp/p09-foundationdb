package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/mlpnkp/p09-foundationdb/internal/adapter/jsonl"
	"github.com/mlpnkp/p09-foundationdb/internal/app"
	"github.com/mlpnkp/p09-foundationdb/internal/domain"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "summary:", err)
		os.Exit(1)
	}
}

func run() error {
	flag.Parse()
	for _, path := range flag.Args() {
		history, err := load(path)
		if err != nil {
			return err
		}
		s := app.Summarize(history)
		fmt.Printf("%s\n  txns %d, committed %d, attempts %d\n  conflicts %d (%.1f%% of attempts), unknown %d, aborted %d\n  retries per txn %.3f, throughput %.1f txn/s\n  latency p50 %v, p95 %v, p99 %v\n",
			path, s.Txns, s.Committed, s.Attempts,
			s.Conflicts, 100*s.ConflictRate, s.Unknown, s.Aborted,
			s.RetriesPerTxn, s.Throughput,
			s.P50, s.P95, s.P99)
	}
	return nil
}

func load(path string) ([]domain.Attempt, error) {
	file, err := os.Open(path) //nolint:gosec // the journal path is the command's argument
	if err != nil {
		return nil, fmt.Errorf("open journal %q: %w", path, err)
	}
	history, readErr := jsonl.ReadAll(file)
	if err := errors.Join(readErr, file.Close()); err != nil {
		return nil, fmt.Errorf("read journal %q: %w", path, err)
	}
	return history, nil
}
