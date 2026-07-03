// Package mod: the module framework. Same contract as v0.1 — a module
// is one screen, built-ins and plugins use the identical interface,
// render never blocks, all exec goes through Host.Submit.
package mod

import (
	"sort"
	"time"

	"git.pynezz.dev/pynezz/stoker/internal/priv"
	"git.pynezz.dev/pynezz/stoker/internal/run"
	"git.pynezz.dev/pynezz/stoker/internal/screen"
	"git.pynezz.dev/pynezz/stoker/internal/term"
)

// Host is what the app exposes to modules. Confirm and Prompt are
// synchronous: the app owns the key channel and can block on it while
// worker results buffer.
type Host interface {
	Submit(m Module, token string, argv []string, timeout time.Duration)
	Priv() *priv.Priv
	Confirm(question string) bool
	Prompt(label, initial string) (string, bool)
	JournalLines() int
	InTmux() bool
	TmuxSplit(lines int, argv []string) error
	TmuxWindow(name string, argv []string) error
}

type Module interface {
	Name() string
	Title() string
	Order() int
	Activate(h Host)
	Refresh(h Host)
	Tick(h Host)
	OnResult(h Host, token string, res run.Result)
	HandleKey(h Host, k term.Key, height, width int) bool
	Render(s *screen.Screen, y, x, h, w int, focused bool)
	Footer() string
}

// Base carries identity and the auto-refresh clock. Embedders call
// Stamp() in their Refresh and check Due() in their Tick.
type Base struct {
	Nm       string
	Ttl      string
	Ord      int
	Interval time.Duration
	last     time.Time
}

func (b *Base) Name() string  { return b.Nm }
func (b *Base) Title() string { return b.Ttl }
func (b *Base) Order() int    { return b.Ord }
func (b *Base) Stamp()        { b.last = time.Now() }
func (b *Base) Due() bool {
	return b.Interval > 0 && time.Since(b.last) > b.Interval
}

var registry []Module

func Register(m Module) { registry = append(registry, m) }

func All() []Module {
	out := make([]Module, len(registry))
	copy(out, registry)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Order() != out[j].Order() {
			return out[i].Order() < out[j].Order()
		}
		return out[i].Title() < out[j].Title()
	})
	return out
}
