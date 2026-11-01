package jsonl_test

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mlpnkp/p09-foundationdb/internal/adapter/jsonl"
	"github.com/mlpnkp/p09-foundationdb/internal/domain"
)

func TestJournal_RoundTrip(t *testing.T) {
	want := []domain.Attempt{
		{
			Txn: "c0-0", Number: 1, Client: 0,
			Start: time.Unix(0, 1_000), End: time.Unix(0, 2_000),
			ReadVersion: 10,
			Reads:       []domain.Observation{{Key: "k0000001"}, {Key: "k0000002", Value: "c1-4/2"}},
			Writes:      []domain.Observation{{Key: "k0000001", Value: "c0-0/1"}},
			Outcome:     domain.OutcomeConflict, ErrorCode: 1020,
		},
		{
			Txn: "c0-0", Number: 2, Client: 0,
			Start: time.Unix(0, 3_000), End: time.Unix(0, 4_000),
			ReadVersion: 12, CommitVersion: 15,
			Reads:   []domain.Observation{{Key: "k0000001"}},
			Writes:  []domain.Observation{{Key: "k0000001", Value: "c0-0/2"}},
			Outcome: domain.OutcomeCommitted, Snapshot: true,
		},
	}
	var buf bytes.Buffer
	w := jsonl.NewWriter(&buf)
	for _, a := range want {
		if err := w.Append(a); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}
	if lines := strings.Count(buf.String(), "\n"); lines != len(want) {
		t.Fatalf("journal has %d lines, want %d", lines, len(want))
	}

	got, err := jsonl.ReadAll(&buf)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip:\n got %+v\nwant %+v", got, want)
	}
}

func TestJournal_ReadAllRejectsGarbage(t *testing.T) {
	if _, err := jsonl.ReadAll(strings.NewReader("{\"txn\":\"c0-0\"}\nnot json\n")); err == nil {
		t.Fatal("ReadAll accepted a broken line")
	}
}
