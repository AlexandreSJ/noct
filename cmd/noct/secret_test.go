package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestASecretAnswerIsMaskedAsItIsTyped(t *testing.T) {
	for _, c := range []struct {
		typed, answer, shown string
		interrupted          bool
	}{
		{"s3cr\x7fet\r", "s3cet", "••••\b \b••", false},
		{"pá\r", "pá", "••", false},
		{"junk\x15ok\n", "ok", "••••\b \b\b \b\b \b\b \b••", false},
		{"a\x1b[Db\x1bOA\r", "ab", "••", false},
		{"ab\x03", "", "••", true},
		{"ab", "ab", "••", false},
	} {
		var out bytes.Buffer
		answer, interrupted := readMasked(strings.NewReader(c.typed), &out)
		if answer != c.answer || interrupted != c.interrupted || out.String() != c.shown {
			t.Errorf("%q: answer %q, interrupted %v, shown %q", c.typed, answer, interrupted, out.String())
		}
	}
}
