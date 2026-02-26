package determinism

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math"
	"strconv"
	"strings"
)

type Engine struct {
	seedValue uint64
	seedText  string
	seedLabel string
	tape      *TapeRecorder
}

func NewEngine(seedText, seedLabel string, tape *TapeRecorder) (*Engine, error) {
	if seedText == "" {
		return nil, fmt.Errorf("seed cannot be empty")
	}
	seedValue, err := strconv.ParseUint(seedText, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("parse seed: %w", err)
	}
	if seedLabel == "" {
		seedLabel = "seed"
	}
	return &Engine{seedValue: seedValue, seedText: seedText, seedLabel: seedLabel, tape: tape}, nil
}

func (e *Engine) Uint64(stream string, requestSeq int64, decisionType string, decisionIdx int, keyExtra ...string) uint64 {
	key := joinDecisionKey(e.seedText, stream, requestSeq, decisionType, decisionIdx, keyExtra)
	sum := sha256.Sum256([]byte(key))
	salt := binary.BigEndian.Uint64(sum[:8])

	sm := &splitmix64{state: e.seedValue ^ salt}
	state := [4]uint64{sm.Next(), sm.Next(), sm.Next(), sm.Next()}
	if state[0] == 0 && state[1] == 0 && state[2] == 0 && state[3] == 0 {
		state[0] = 1
	}

	xo := &xoshiro256ss{s: state}
	value := xo.Next()

	if e.tape != nil {
		e.tape.Add(TapeEntry{
			SeedLabel:    e.seedLabel,
			Stream:       stream,
			RequestSeq:   requestSeq,
			DecisionType: decisionType,
			DecisionIdx:  decisionIdx,
			KeyExtra:     append([]string(nil), keyExtra...),
			ValueUint64:  uint64ToString(value),
		})
	}

	return value
}

func (e *Engine) Float64(stream string, requestSeq int64, decisionType string, decisionIdx int, keyExtra ...string) float64 {
	u := e.Uint64(stream, requestSeq, decisionType, decisionIdx, keyExtra...)
	return float64(u) / float64(math.MaxUint64)
}

func (e *Engine) IntN(n int, stream string, requestSeq int64, decisionType string, decisionIdx int, keyExtra ...string) int {
	if n <= 1 {
		return 0
	}
	u := e.Uint64(stream, requestSeq, decisionType, decisionIdx, keyExtra...)
	return int(u % uint64(n))
}

func joinDecisionKey(seedText, stream string, requestSeq int64, decisionType string, decisionIdx int, keyExtra []string) string {
	parts := []string{seedText, stream, strconv.FormatInt(requestSeq, 10), decisionType, strconv.Itoa(decisionIdx)}
	parts = append(parts, keyExtra...)
	return strings.Join(parts, "\x1f")
}
