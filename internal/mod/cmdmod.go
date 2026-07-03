package mod

import (
	"time"

	"git.pynezz.dev/pynezz/stoker/internal/run"
	"git.pynezz.dev/pynezz/stoker/internal/screen"
	"git.pynezz.dev/pynezz/stoker/internal/term"
	"git.pynezz.dev/pynezz/stoker/internal/ui"
)

type View struct {
	Name string
	Argv []string
}

// CmdModule: N named read-only command views cycled with [ and ].
// Declaring one of these is the five-line cost of a new inspection
// screen; if adding a screen ever needs more than this, the design
// has failed (carried over verbatim from PLAN.md).
type CmdModule struct {
	Base
	Views   []View
	Timeout time.Duration
	Msg     string
	idx     int
	pane    ui.TextPane
	loading bool
}

func NewCmd(name, title string, order int, interval time.Duration,
	timeout time.Duration, views []View) *CmdModule {
	c := &CmdModule{
		Base:    Base{Nm: name, Ttl: title, Ord: order, Interval: interval},
		Views:   views,
		Timeout: timeout,
	}
	c.pane.SetText("loading…")
	return c
}

func (c *CmdModule) Activate(h Host) { c.Refresh(h) }

func (c *CmdModule) Refresh(h Host) {
	c.Stamp()
	c.loading = true
	h.Submit(c, c.Views[c.idx].Name, c.Views[c.idx].Argv, c.Timeout)
}

func (c *CmdModule) Tick(h Host) {
	if c.Due() {
		c.Refresh(h)
	}
}

func (c *CmdModule) OnResult(h Host, token string, res run.Result) {
	if token != c.Views[c.idx].Name {
		return // stale result from a view we already left
	}
	c.loading = false
	c.pane.SetText(res.Text())
}

func (c *CmdModule) HandleKey(h Host, k term.Key, height, width int) bool {
	switch k.R {
	case ']':
		c.idx = (c.idx + 1) % len(c.Views)
	case '[':
		c.idx = (c.idx - 1 + len(c.Views)) % len(c.Views)
	case '/':
		if s, ok := h.Prompt("search", c.pane.Search); ok {
			c.pane.Search = s
			c.pane.NextMatch()
		}
		return true
	default:
		return c.pane.HandleKey(k, height-1, width)
	}
	c.pane.Y = 0
	c.Refresh(h)
	return true
}

func (c *CmdModule) Render(s *screen.Screen, y, x, h, w int, focused bool) {
	tabs := ""
	for i, v := range c.Views {
		if i == c.idx {
			tabs += "[" + v.Name + "]  "
		} else {
			tabs += " " + v.Name + "   "
		}
	}
	if c.loading {
		tabs += "…"
	}
	st := screen.Dim
	if focused {
		st = screen.Focus
	}
	s.Put(y, x, tabs, st, w)
	c.pane.Draw(s, y+1, x, h-1, w)
	if c.Msg != "" {
		s.Put(y+h-1, x, c.Msg, screen.Warn, w)
	}
}

func (c *CmdModule) Footer() string { return "[/] views  / search  n next" }
