package naming

import "testing"

func TestIsValid(t *testing.T) {
	for name, want := range map[string]bool{
		"deploy": true, "a.b-c_d": true, "9lives": true,
		"": false, ".hidden": false, "-x": false, "a/b": false, "a b": false, "ação": false,
	} {
		if got := IsValid(name); got != want {
			t.Errorf("IsValid(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestIsIdentifier(t *testing.T) {
	for name, want := range map[string]bool{
		"pattern": true, "_x": true, "a1_b": true,
		"": false, "1a": false, "a-b": false, "a.b": false,
	} {
		if got := IsIdentifier(name); got != want {
			t.Errorf("IsIdentifier(%q) = %v, want %v", name, got, want)
		}
	}
}
