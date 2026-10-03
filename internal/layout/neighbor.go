package layout

import (
	"image"
	"slices"
)

// Neighbor finds the closest pane in the given heading from the pane `from`, within area. Among
// panes equally close, it picks the one focused most recently, as listed in recent (the most
// recent last), so going back returns to the pane you came from; then the one best aligned.
func (t *Tree) Neighbor(from PaneID, heading Heading, area image.Rectangle, recent []PaneID) (PaneID, bool) {
	rects := t.Rects(area)
	origin, ok := rects[from]
	if !ok {
		return 0, false
	}
	best, bestScore, found := PaneID(0), [3]int{}, false
	for _, pane := range t.Panes() {
		if pane == from {
			continue
		}
		gap, offset, ok := distanceToward(origin, rects[pane], heading)
		if !ok {
			continue
		}
		score := [3]int{gap, -(slices.Index(recent, pane) + 1), offset}
		if !found || slices.Compare(score[:], bestScore[:]) < 0 {
			best, bestScore, found = pane, score, true
		}
	}
	return best, found
}

// distanceToward measures how far candidate lies from origin in the heading. It fails when the
// candidate is not in that direction or does not overlap origin on the other axis.
func distanceToward(origin, candidate image.Rectangle, heading Heading) (gap, offset int, ok bool) {
	switch heading {
	case Left:
		gap, ok = origin.Min.X-candidate.Max.X, overlaps(origin.Min.Y, origin.Max.Y, candidate.Min.Y, candidate.Max.Y)
		offset = abs(candidate.Min.Y - origin.Min.Y)
	case Right:
		gap, ok = candidate.Min.X-origin.Max.X, overlaps(origin.Min.Y, origin.Max.Y, candidate.Min.Y, candidate.Max.Y)
		offset = abs(candidate.Min.Y - origin.Min.Y)
	case Up:
		gap, ok = origin.Min.Y-candidate.Max.Y, overlaps(origin.Min.X, origin.Max.X, candidate.Min.X, candidate.Max.X)
		offset = abs(candidate.Min.X - origin.Min.X)
	case Down:
		gap, ok = candidate.Min.Y-origin.Max.Y, overlaps(origin.Min.X, origin.Max.X, candidate.Min.X, candidate.Max.X)
		offset = abs(candidate.Min.X - origin.Min.X)
	}
	return gap, offset, ok && gap >= 0
}

func overlaps(start1, end1, start2, end2 int) bool {
	return start1 < end2 && start2 < end1
}

func abs(n int) int {
	return max(n, -n)
}
