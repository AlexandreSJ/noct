package app

import "math"

// The animation effects as functions of time: t goes from 0 at the start to 1 at the end.

// jitterRunes are what a cell shows before it settles on its own character.
const jitterRunes = "░▒▓█#%&@$*+="

func clamp01(t float64) float64 { return min(max(t, 0), 1) }

// easeOut starts fast and slows down to the end.
func easeOut(t float64) float64 {
	t = clamp01(t)
	return 1 - math.Pow(1-t, 3)
}

// noise is a number in [0, 1) that stays the same for the same cell and seed.
func noise(x, y int, seed int64) float64 {
	h := uint64(x)*0x9e3779b97f4a7c15 ^ uint64(y)*0xc2b2ae3d27d4eb4f ^ uint64(seed)*0x165667b19e3779f9
	h ^= h >> 29
	h *= 0xbf58476d1ce4e5b9
	h ^= h >> 32
	return float64(h%1_000_000) / 1_000_000
}

// binaryAt is r once t reaches revealAt, and a 0 or a 1 before.
func binaryAt(r rune, t, revealAt float64, seed int64) rune {
	if t >= revealAt || r == ' ' {
		return r
	}
	return '0' + rune(seed&1)
}

// jitterAt is r once t reaches settleAt, and one of the jitter characters before.
func jitterAt(r rune, t, settleAt float64, seed int64) rune {
	if t >= settleAt || r == ' ' {
		return r
	}
	runes := []rune(jitterRunes)
	return runes[uint64(seed)%uint64(len(runes))]
}
