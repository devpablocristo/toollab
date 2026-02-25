package evidence

import (
	"crypto/sha256"
	"encoding/binary"
	"sort"
	"strconv"
)

type sampleScore struct {
	Index int
	Score uint64
	Seq   int64
}

func SelectSampleIndexes(outcomes []OutcomeInput, maxSamples int, runSeed string) []int {
	n := len(outcomes)
	if maxSamples <= 0 || n == 0 {
		return []int{}
	}
	if n <= maxSamples {
		idx := make([]int, n)
		for i := range idx {
			idx[i] = i
		}
		return idx
	}

	scores := make([]sampleScore, 0, n)
	for i, out := range outcomes {
		score := sampleDeterministicScore(runSeed, out.Seq)
		scores = append(scores, sampleScore{Index: i, Score: score, Seq: out.Seq})
	}
	sort.Slice(scores, func(i, j int) bool {
		if scores[i].Score != scores[j].Score {
			return scores[i].Score < scores[j].Score
		}
		return scores[i].Seq < scores[j].Seq
	})

	selected := make([]sampleScore, maxSamples)
	copy(selected, scores[:maxSamples])
	sort.Slice(selected, func(i, j int) bool {
		return selected[i].Seq < selected[j].Seq
	})

	out := make([]int, 0, maxSamples)
	for _, s := range selected {
		out = append(out, s.Index)
	}
	return out
}

func sampleDeterministicScore(runSeed string, seq int64) uint64 {
	src := runSeed + ":sample:" + strconv.FormatInt(seq, 10)
	sum := sha256.Sum256([]byte(src))
	return binary.BigEndian.Uint64(sum[:8])
}
