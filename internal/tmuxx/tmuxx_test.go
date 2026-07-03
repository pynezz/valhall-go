package tmuxx

import "testing"

// The injection gate is the security boundary between untrusted unit
// names and tmux's shell-command argument. Reject-then-quote.
func TestSafeArg(t *testing.T) {
	good := []string{"journalctl", "-f", "-u", "sshd.service",
		"dev-disk-by\\x2duuid.device", "user@1000.service", "-p", "err"}
	for _, g := range good {
		if !safeArg.MatchString(g) {
			t.Errorf("should accept %q", g)
		}
	}
	bad := []string{"a b", "x;rm -rf /", "$(reboot)", "`id`", "a'b",
		"unit\n.service", "", "a\"b", "a|b", "a&b"}
	for _, b := range bad {
		if safeArg.MatchString(b) {
			t.Errorf("must reject %q", b)
		}
	}
}
