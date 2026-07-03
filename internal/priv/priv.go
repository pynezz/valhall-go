// Package priv: per-action privilege, unchanged policy from v0.1 —
// stoker never elevates itself; mutating verbs are wrapped with
// non-interactive sudo or refused with an actionable hint.
package priv

import (
	"fmt"
	"os"
	"time"

	"git.pynezz.dev/pynezz/stoker/internal/run"
)

type Priv struct {
	euid   int
	sudoOK bool
	probed bool
}

func New() *Priv { return &Priv{euid: os.Geteuid()} }

func (p *Priv) IsRoot() bool { return p.euid == 0 }

func (p *Priv) sudo() bool {
	if p.IsRoot() {
		return true
	}
	if !p.probed {
		p.sudoOK = run.Do([]string{"sudo", "-n", "true"}, 5*time.Second).OK()
		p.probed = true
	}
	return p.sudoOK
}

// Wrap returns argv ready to run with the needed privilege, or nil
// when the action is not currently possible (surface Hint()).
func (p *Priv) Wrap(argv []string) []string {
	if p.IsRoot() {
		return argv
	}
	if p.sudo() {
		return append([]string{"sudo", "-n"}, argv...)
	}
	return nil
}

func (p *Priv) Hint() string {
	return "root required: run valhall as root, or grant NOPASSWD sudo for the specific commands (visudo)"
}

func (p *Priv) Badge() string {
	if p.IsRoot() {
		return "root"
	}
	return fmt.Sprintf("uid=%d", p.euid)
}
