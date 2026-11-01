package domain

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"slices"
	"time"
)

var ErrInvalidWorkload = errors.New("invalid workload")

type ClientID int

func (c ClientID) Stream() uint64 {
	if c < 0 {
		return 0
	}
	return uint64(c)
}

type TxnID string

func NewTxnID(client ClientID, seq int) TxnID {
	return TxnID(fmt.Sprintf("c%d-%d", client, seq))
}

type Key string

type Value string

type Workload struct {
	hotKeys int
	perTxn  int
}

func NewWorkload(hotKeys, perTxn int) (Workload, error) {
	if hotKeys < 1 || perTxn < 1 || perTxn > hotKeys {
		return Workload{}, fmt.Errorf("hot keys %d, keys per txn %d: %w", hotKeys, perTxn, ErrInvalidWorkload)
	}
	return Workload{hotKeys: hotKeys, perTxn: perTxn}, nil
}

func (w Workload) HotKeys() int { return w.hotKeys }

func (w Workload) KeysPerTxn() int { return w.perTxn }

func (w Workload) Key(i int) Key {
	return Key(fmt.Sprintf("k%07d", i))
}

func (w Workload) Pick(rng *rand.Rand) []Key {
	picked := make(map[int]bool, w.perTxn)
	keys := make([]Key, 0, w.perTxn)
	for len(keys) < w.perTxn {
		i := rng.IntN(w.hotKeys)
		if picked[i] {
			continue
		}
		picked[i] = true
		keys = append(keys, w.Key(i))
	}
	slices.Sort(keys)
	return keys
}

type TxnSpec struct {
	ID       TxnID
	Client   ClientID
	Keys     []Key
	Snapshot bool
}

func (s TxnSpec) Value(attempt int) Value {
	return Value(fmt.Sprintf("%s/%d", s.ID, attempt))
}

type Outcome string

const (
	OutcomeCommitted Outcome = "committed"
	OutcomeConflict  Outcome = "conflict"
	OutcomeAborted   Outcome = "aborted"
	OutcomeUnknown   Outcome = "unknown"
)

func (o Outcome) Committed() bool { return o == OutcomeCommitted }

type Observation struct {
	Key   Key
	Value Value
}

type Attempt struct {
	Txn           TxnID
	Number        int
	Client        ClientID
	Start, End    time.Time
	ReadVersion   int64
	CommitVersion int64
	Reads         []Observation
	Writes        []Observation
	Outcome       Outcome
	ErrorCode     int
	Snapshot      bool
}
