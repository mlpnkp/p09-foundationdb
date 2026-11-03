package jsonl

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/mlpnkp/p09-foundationdb/internal/domain"
)

type observation struct {
	Key   string `json:"k"`
	Value string `json:"v,omitempty"`
}

type record struct {
	Txn           string        `json:"txn"`
	Attempt       int           `json:"attempt"`
	Client        int           `json:"client"`
	StartNs       int64         `json:"start_ns"`
	EndNs         int64         `json:"end_ns"`
	ReadVersion   int64         `json:"read_version"`
	CommitVersion int64         `json:"commit_version,omitzero"`
	Reads         []observation `json:"reads"`
	Writes        []observation `json:"writes"`
	Outcome       string        `json:"outcome"`
	Error         int           `json:"error,omitzero"`
	Snapshot      bool          `json:"snapshot,omitzero"`
}

type Writer struct {
	mu  sync.Mutex
	buf *bufio.Writer
	enc *json.Encoder
}

func NewWriter(w io.Writer) *Writer {
	buf := bufio.NewWriter(w)
	return &Writer{buf: buf, enc: json.NewEncoder(buf)}
}

func (w *Writer) Append(a domain.Attempt) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.enc.Encode(toRecord(&a)); err != nil {
		return fmt.Errorf("encode txn %q attempt %d: %w", a.Txn, a.Number, err)
	}
	return nil
}

func (w *Writer) Flush() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.buf.Flush(); err != nil {
		return fmt.Errorf("flush journal: %w", err)
	}
	return nil
}

func ReadAll(r io.Reader) ([]domain.Attempt, error) {
	var attempts []domain.Attempt
	dec := json.NewDecoder(r)
	for line := 1; dec.More(); line++ {
		var rec record
		if err := dec.Decode(&rec); err != nil {
			return nil, fmt.Errorf("decode record %d: %w", line, err)
		}
		attempts = append(attempts, rec.toAttempt())
	}
	return attempts, nil
}

func toRecord(a *domain.Attempt) record {
	return record{
		Txn:           string(a.Txn),
		Attempt:       a.Number,
		Client:        int(a.Client),
		StartNs:       a.Start.UnixNano(),
		EndNs:         a.End.UnixNano(),
		ReadVersion:   a.ReadVersion,
		CommitVersion: a.CommitVersion,
		Reads:         toObservations(a.Reads),
		Writes:        toObservations(a.Writes),
		Outcome:       string(a.Outcome),
		Error:         a.ErrorCode,
		Snapshot:      a.Snapshot,
	}
}

func (r *record) toAttempt() domain.Attempt {
	return domain.Attempt{
		Txn:           domain.TxnID(r.Txn),
		Number:        r.Attempt,
		Client:        domain.ClientID(r.Client),
		Start:         time.Unix(0, r.StartNs),
		End:           time.Unix(0, r.EndNs),
		ReadVersion:   r.ReadVersion,
		CommitVersion: r.CommitVersion,
		Reads:         fromObservations(r.Reads),
		Writes:        fromObservations(r.Writes),
		Outcome:       domain.Outcome(r.Outcome),
		ErrorCode:     r.Error,
		Snapshot:      r.Snapshot,
	}
}

func toObservations(obs []domain.Observation) []observation {
	out := make([]observation, 0, len(obs))
	for _, o := range obs {
		out = append(out, observation{Key: string(o.Key), Value: string(o.Value)})
	}
	return out
}

func fromObservations(obs []observation) []domain.Observation {
	if len(obs) == 0 {
		return nil
	}
	out := make([]domain.Observation, 0, len(obs))
	for _, o := range obs {
		out = append(out, domain.Observation{Key: domain.Key(o.Key), Value: domain.Value(o.Value)})
	}
	return out
}
