package app

import "testing"

// Moving down a long list, the view stays put until the selection is scrollMargin rows from its
// bottom, then follows it; moving back up, it stays until scrollMargin rows from the top.
func TestListFirstKeepsRowsAroundTheSelection(t *testing.T) {
	const n, height = 30, 10
	first := 0
	for sel := 0; sel < n; sel++ {
		first = listFirst(first, sel, n, height)
		want := min(max(sel+scrollMargin(height)-height+1, 0), n-height)
		if first != want {
			t.Fatalf("down to %d: first = %d, want %d", sel, first, want)
		}
	}
	for sel := n - 1; sel >= 0; sel-- {
		first = listFirst(first, sel, n, height)
		want := min(max(sel-scrollMargin(height), 0), n-height)
		if first != want {
			t.Fatalf("up to %d: first = %d, want %d", sel, first, want)
		}
	}
}

func TestListFirstEdges(t *testing.T) {
	cases := []struct {
		name                             string
		first, selected, n, height, want int
	}{
		{"fits", 0, 5, 8, 10, 0},
		{"list shrank under the view", 20, 6, 8, 5, 3},
		{"short view keeps the selection inside", 0, 3, 20, 3, 2},
		{"one row", 0, 7, 20, 1, 7},
		{"no room", 4, 2, 20, 0, 0},
	}
	for _, c := range cases {
		if got := listFirst(c.first, c.selected, c.n, c.height); got != c.want {
			t.Errorf("%s: listFirst(%d, %d, %d, %d) = %d, want %d", c.name, c.first, c.selected, c.n, c.height, got, c.want)
		}
	}
}

func TestScrollMarginGrowsWithTheList(t *testing.T) {
	for height, want := range map[int]int{3: 1, 9: 4, 12: 4, 20: 6, 30: 9, 40: 12} {
		if got := scrollMargin(height); got != want {
			t.Errorf("scrollMargin(%d) = %d, want %d", height, got, want)
		}
	}
}
