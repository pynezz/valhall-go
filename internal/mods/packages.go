package mods

import (
	"time"

	"git.pynezz.dev/pynezz/stoker/internal/mod"
	"git.pynezz.dev/pynezz/stoker/internal/term"
)

// Packages embeds the generic command views and adds one verb:
// U = full system update in a *detached* tmux window.
//
// Rationale: a dnf transaction started over SSH must survive the SSH
// session — updates can restart sshd, drop the connection, or take
// longer than the operator's attention span. A detached window is
// owned by the tmux server, not by stoker and not by sshd, so the
// transaction survives all three. Without tmux the verb refuses
// rather than offering a worse, interruptible fallback.
type Packages struct {
	*mod.CmdModule
}

func NewPackages() *Packages {
	return &Packages{mod.NewCmd("packages", "Packages", 50, 0, 120*time.Second,
		[]mod.View{
			{Name: "recent", Argv: []string{"rpm", "-qa", "--last"}},
			{Name: "updates", Argv: []string{"dnf", "-q", "check-update"}},
			{Name: "history", Argv: []string{"dnf", "history", "list"}},
			{Name: "verify", Argv: []string{"rpm", "-Va", "--nofiledigest"}},
		})}
}

func (p *Packages) HandleKey(h mod.Host, k term.Key, height, width int) bool {
	if k.R == 'U' {
		if !h.InTmux() {
			p.Msg = "U needs tmux — the update must survive this session (run stoker --attach)"
			return true
		}
		if !h.Confirm("run 'dnf -y update' in a detached tmux window?") {
			return true
		}
		argv := h.Priv().Wrap([]string{"dnf", "-y", "update"})
		if argv == nil {
			p.Msg = h.Priv().Hint()
			return true
		}
		if err := h.TmuxWindow("dnf-update", argv); err != nil {
			p.Msg = err.Error()
		} else {
			p.Msg = "update running in tmux window 'dnf-update' — switch with prefix+n"
		}
		return true
	}
	return p.CmdModule.HandleKey(h, k, height, width)
}

func (p *Packages) Footer() string {
	return p.CmdModule.Footer() + "  U update (detached)"
}
