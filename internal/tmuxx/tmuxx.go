// Package tmuxx: optional tmux integration, gated on $TMUX. tmux is a
// force multiplier a single-process TUI can't match for live streams:
// `F` in the journal splits a real `journalctl -f` below stoker
// instead of stoker owning pipe lifecycle. Target is tmux 2.7
// (RHEL 8) — no display-popup (3.2+), so splits only.
package tmuxx

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"git.pynezz.dev/pynezz/stoker/internal/run"
)

func Inside() bool {
	// run.Do children get a scrubbed env; this reads our own.
	return os.Getenv("TMUX") != ""
}

// tmux 2.7's split-window takes a single shell-command string, so the
// command we hand it is a shell injection surface. Two layers: every
// argument must match safeArg, and every argument is single-quoted
// anyway. Reject-then-quote, not quote-and-hope.
var safeArg = regexp.MustCompile(`^[A-Za-z0-9@%:,._/=+\\-]+$`)

func Split(vertical bool, lines int, argv []string) error {
	var parts []string
	for _, a := range argv {
		if !safeArg.MatchString(a) {
			return fmt.Errorf("refusing to pass unsafe argument to tmux: %q", a)
		}
		parts = append(parts, "'"+a+"'") // safeArg admits no single quotes
	}
	t := []string{"tmux", "split-window"}
	if vertical {
		t = append(t, "-v")
	} else {
		t = append(t, "-h")
	}
	if lines > 0 {
		t = append(t, "-l", fmt.Sprint(lines))
	}
	if len(parts) > 0 {
		t = append(t, strings.Join(parts, " "))
	}
	res := run.Do(t, 5*time.Second)
	if !res.OK() {
		msg := res.Stderr
		if msg == "" {
			msg = res.Err
		}
		if i := strings.IndexByte(msg, '\n'); i >= 0 {
			msg = msg[:i]
		}
		return fmt.Errorf("tmux: %s", msg)
	}
	return nil
}

// Window opens a *detached* tmux window running argv. This is the
// disconnect-safe path for long mutations (dnf update): the
// transaction outlives the SSH session, the tmux client, and stoker
// itself. Same reject-then-quote rule as Split.
func Window(name string, argv []string) error {
	var parts []string
	for _, a := range argv {
		if !safeArg.MatchString(a) {
			return fmt.Errorf("refusing to pass unsafe argument to tmux: %q", a)
		}
		parts = append(parts, "'"+a+"'")
	}
	if !safeArg.MatchString(name) {
		name = "stoker-job"
	}
	t := []string{"tmux", "new-window", "-d", "-n", name, strings.Join(parts, " ")}
	res := run.Do(t, 5*time.Second)
	if !res.OK() {
		msg := res.Stderr
		if msg == "" {
			msg = res.Err
		}
		if i := strings.IndexByte(msg, '\n'); i >= 0 {
			msg = msg[:i]
		}
		return fmt.Errorf("tmux: %s", msg)
	}
	return nil
}
