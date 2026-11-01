package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mlpnkp/p09-foundationdb/internal/adapter/fdbstore"
	"github.com/mlpnkp/p09-foundationdb/internal/adapter/jsonl"
	"github.com/mlpnkp/p09-foundationdb/internal/app"
	"github.com/mlpnkp/p09-foundationdb/internal/domain"
)

type options struct {
	clusterFile string
	out         string
	prefix      string
	clients     int
	hotKeys     int
	keysPerTxn  int
	duration    time.Duration
	timeout     time.Duration
	seed        uint64
	snapshot    bool
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "loadgen:", err)
		os.Exit(1)
	}
}

func parse() options {
	var o options
	flag.StringVar(&o.clusterFile, "cluster-file", "/etc/foundationdb/fdb.cluster", "FoundationDB cluster file")
	flag.StringVar(&o.out, "out", "journal.jsonl", "journal of attempts, JSONL")
	flag.StringVar(&o.prefix, "prefix", "p09/", "key prefix, cleared before the run")
	flag.IntVar(&o.clients, "clients", 8, "concurrent clients")
	flag.IntVar(&o.hotKeys, "hot", 1000, "hot set size: fewer keys, more conflicts")
	flag.IntVar(&o.keysPerTxn, "keys", 4, "keys read and written by every transaction")
	flag.DurationVar(&o.duration, "duration", 30*time.Second, "run length")
	flag.DurationVar(&o.timeout, "timeout", 5*time.Second, "timeout of one attempt")
	flag.Uint64Var(&o.seed, "seed", 1, "seed of the key choice")
	flag.BoolVar(&o.snapshot, "snapshot", false, "read through snapshot: no read conflicts, breaks serializability on purpose")
	flag.Parse()
	return o
}

func run() error {
	o := parse()
	workload, err := domain.NewWorkload(o.hotKeys, o.keysPerTxn)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	store, err := fdbstore.Open(o.clusterFile, o.prefix, o.timeout)
	if err != nil {
		return err
	}
	defer store.Close()

	clearCtx, cancelClear := context.WithTimeout(ctx, 30*time.Second)
	defer cancelClear()
	if err := store.Clear(clearCtx); err != nil {
		return err
	}

	file, err := os.Create(o.out)
	if err != nil {
		return fmt.Errorf("create journal %q: %w", o.out, err)
	}
	journal := jsonl.NewWriter(file)

	runCtx, cancelRun := context.WithTimeout(ctx, o.duration)
	defer cancelRun()
	cfg := app.LoadConfig{Clients: o.clients, Seed: o.seed, Workload: workload, Snapshot: o.snapshot}
	runErr := app.RunLoad(runCtx, cfg, store, journal)

	return errors.Join(runErr, journal.Flush(), file.Close())
}
