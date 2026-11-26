package sections

import "testing"

func TestEachViewShowsItsPart(t *testing.T) {
	out := "login: ana\n" + Full + "\n▶ run $ echo hi\n" + Both + "hi\n" + Clean + "\nexit 0\n" + Full + "\n✓ done\n" + Both
	for view, want := range map[View]string{
		CleanView: "login: ana\nhi\n\nexit 0\n",
		FullView:  "login: ana\n\n▶ run $ echo hi\nhi\n\n✓ done\n",
	} {
		f := Filter{View: view}
		var got []byte
		for i := 0; i < len(out); i += 3 { // marks cut between writes
			got = append(got, f.Write([]byte(out[i:min(i+3, len(out))]))...)
		}
		if string(got) != want {
			t.Errorf("view %d: %q, want %q", view, got, want)
		}
	}
}
