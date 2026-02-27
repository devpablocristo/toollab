package determinism

const EngineVersion = "splitmix64+xoshiro256ss-v1"

type splitmix64 struct {
	state uint64
}

func (s *splitmix64) Next() uint64 {
	s.state += 0x9e3779b97f4a7c15
	z := s.state
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	return z ^ (z >> 31)
}

type xoshiro256ss struct {
	s [4]uint64
}

func (x *xoshiro256ss) Next() uint64 {
	result := rotl(x.s[1]*5, 7) * 9
	t := x.s[1] << 17

	x.s[2] ^= x.s[0]
	x.s[3] ^= x.s[1]
	x.s[1] ^= x.s[2]
	x.s[0] ^= x.s[3]

	x.s[2] ^= t
	x.s[3] = rotl(x.s[3], 45)

	return result
}

func rotl(x uint64, k int) uint64 {
	return (x << k) | (x >> (64 - k))
}
