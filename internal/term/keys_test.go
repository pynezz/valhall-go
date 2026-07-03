package term

import "testing"

func TestParseSequences(t *testing.T) {
	cases := []struct {
		in   string
		code Code
		r    rune
	}{
		{"\x1b[A", KUp, 0}, {"\x1b[B", KDown, 0},
		{"\x1b[5~", KPgUp, 0}, {"\x1b[6~", KPgDn, 0},
		{"\x1b[15~", KF5, 0}, {"\x1bOH", KHome, 0},
		{"\r", KEnter, 0}, {"\t", KTab, 0}, {"\x7f", KBackspace, 0},
		{"q", KRune, 'q'}, {"æ", KRune, 'æ'}, {"\x03", KCtrlC, 0},
	}
	for _, c := range cases {
		k, adv, more := parse([]byte(c.in))
		if more || k == nil || k.Code != c.code || (c.code == KRune && k.R != c.r) {
			t.Errorf("parse(%q) = %+v adv=%d more=%v", c.in, k, adv, more)
		}
		if adv != len(c.in) {
			t.Errorf("parse(%q) consumed %d, want %d", c.in, adv, len(c.in))
		}
	}
	// incomplete sequences must ask for more, not misparse
	for _, s := range []string{"\x1b", "\x1b[", "\x1b[1"} {
		if _, _, more := parse([]byte(s)); !more {
			t.Errorf("parse(%q) should need more input", s)
		}
	}
}
