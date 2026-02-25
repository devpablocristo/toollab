package determinism

import (
	"sort"
	"strconv"
	"strings"
	"sync"

	"toolab-core/pkg/utils"
)

type TapeEntry struct {
	SeedLabel    string   `json:"seed_label"`
	Stream       string   `json:"stream"`
	RequestSeq   int64    `json:"request_seq"`
	DecisionType string   `json:"decision_type"`
	DecisionIdx  int      `json:"decision_idx"`
	KeyExtra     []string `json:"key_extra"`
	ValueUint64  string   `json:"value_uint64"`
}

type TapeRecorder struct {
	mu      sync.Mutex
	entries []TapeEntry
}

func NewTapeRecorder() *TapeRecorder {
	return &TapeRecorder{}
}

func (r *TapeRecorder) Add(entry TapeEntry) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries = append(r.entries, entry)
}

func (r *TapeRecorder) EntriesSorted() []TapeEntry {
	r.mu.Lock()
	defer r.mu.Unlock()
	copyEntries := append([]TapeEntry(nil), r.entries...)
	sort.Slice(copyEntries, func(i, j int) bool {
		a := copyEntries[i]
		b := copyEntries[j]
		if a.RequestSeq != b.RequestSeq {
			return a.RequestSeq < b.RequestSeq
		}
		if a.SeedLabel != b.SeedLabel {
			return a.SeedLabel < b.SeedLabel
		}
		if a.Stream != b.Stream {
			return a.Stream < b.Stream
		}
		if a.DecisionType != b.DecisionType {
			return a.DecisionType < b.DecisionType
		}
		if a.DecisionIdx != b.DecisionIdx {
			return a.DecisionIdx < b.DecisionIdx
		}
		ak := strings.Join(a.KeyExtra, "\x1f")
		bk := strings.Join(b.KeyExtra, "\x1f")
		if ak != bk {
			return ak < bk
		}
		return a.ValueUint64 < b.ValueUint64
	})
	return copyEntries
}

func (r *TapeRecorder) Hash() (string, error) {
	canonical, err := utils.CanonicalJSON(r.EntriesSorted())
	if err != nil {
		return "", err
	}
	return utils.SHA256Hex(canonical), nil
}

func (r *TapeRecorder) JSONLines() (string, error) {
	entries := r.EntriesSorted()
	lines := make([]string, 0, len(entries))
	for _, entry := range entries {
		canonical, err := utils.CanonicalJSON(entry)
		if err != nil {
			return "", err
		}
		lines = append(lines, string(canonical))
	}
	return strings.Join(lines, "\n") + trailingNewline(lines), nil
}

func trailingNewline(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	return "\n"
}

func uint64ToString(v uint64) string {
	return strconv.FormatUint(v, 10)
}
