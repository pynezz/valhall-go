package mods

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"git.pynezz.dev/pynezz/stoker/internal/mod"
	"git.pynezz.dev/pynezz/stoker/internal/run"
	"git.pynezz.dev/pynezz/stoker/internal/screen"
	"git.pynezz.dev/pynezz/stoker/internal/term"
	"git.pynezz.dev/pynezz/stoker/internal/ui"
)

var unitScopes = []string{"all", "failed", "service", "timer", "socket"}

type unitAction struct {
	verb    string
	confirm bool
}

var unitActions = map[rune]unitAction{
	's': {"start", false},
	't': {"stop", true},
	'r': {"restart", true},
	'e': {"reload", false},
}

type unitInfo struct {
	unit, active, sub string
}

// Units: the screen an engineer lives in. Failed-first sort, scope
// cycling, filter; mutating verbs go through Priv.Wrap and (for
// destructive ones) a confirmation.
type Units struct {
	mod.Base
	list    ui.ListPane
	detail  ui.TextPane
	mode    string // list | detail
	scopeI  int
	msg     string
	confirm bool // from config
}

func NewUnits(confirmDestructive bool) *Units {
	return &Units{
		Base: mod.Base{Nm: "units", Ttl: "Units", Ord: 10,
			Interval: 10 * time.Second},
		mode:    "list",
		confirm: confirmDestructive,
	}
}

func (u *Units) Activate(h mod.Host) { u.Refresh(h) }

func (u *Units) Refresh(h mod.Host) {
	u.Stamp()
	scope := unitScopes[u.scopeI]
	argv := []string{"systemctl", "list-units", "--all", "--plain",
		"--no-legend", "--no-pager", "--full"}
	switch scope {
	case "all":
	case "failed":
		argv = append(argv, "--failed")
	default:
		argv = append(argv, "--type", scope)
	}
	h.Submit(u, "list", argv, 10*time.Second)
}

func (u *Units) Tick(h mod.Host) {
	if u.Due() {
		u.Refresh(h)
	}
}

func (u *Units) OnResult(h mod.Host, token string, res run.Result) {
	switch {
	case token == "list":
		if res.Err != "" {
			u.list.Items = []ui.Item{{Label: "(" + res.Err + ")"}}
			return
		}
		var items []ui.Item
		for _, line := range strings.Split(res.Stdout, "\n") {
			f := strings.Fields(line)
			if len(f) < 4 {
				continue
			}
			info := unitInfo{unit: f[0], active: f[2], sub: f[3]}
			desc := strings.Join(f[4:], " ")
			items = append(items, ui.Item{
				Label: fmt.Sprintf("%-44.44s %-8s %-10s %s",
					info.unit, info.active, info.sub, desc),
				Payload: info,
			})
		}
		// failed first, then alphabetical — problems float to the top
		sort.SliceStable(items, func(i, j int) bool {
			a := items[i].Payload.(unitInfo)
			b := items[j].Payload.(unitInfo)
			af, bf := a.active == "failed", b.active == "failed"
			if af != bf {
				return af
			}
			return a.unit < b.unit
		})
		u.list.Items = items
	case token == "status":
		u.detail.SetText(res.Text())
	case strings.HasPrefix(token, "act:"):
		verb := strings.TrimPrefix(token, "act:")
		if res.OK() {
			u.msg = verb + ": ok"
		} else {
			e := strings.TrimSpace(res.Stderr)
			if e == "" {
				e = res.Err
			}
			if len(e) > 120 {
				e = e[:120]
			}
			u.msg = verb + " failed: " + e
		}
		u.Refresh(h)
	}
}

func (u *Units) selected() (unitInfo, bool) {
	if p, ok := u.list.Selected().(unitInfo); ok {
		return p, true
	}
	return unitInfo{}, false
}

func (u *Units) act(h mod.Host, a unitAction) {
	sel, ok := u.selected()
	if !ok {
		return
	}
	if a.confirm && u.confirm && !h.Confirm(a.verb+" "+sel.unit+"?") {
		return
	}
	argv := h.Priv().Wrap([]string{"systemctl", a.verb, sel.unit})
	if argv == nil {
		u.msg = h.Priv().Hint()
		return
	}
	u.msg = a.verb + " " + sel.unit + "…"
	h.Submit(u, "act:"+a.verb, argv, 30*time.Second)
}

func (u *Units) HandleKey(h mod.Host, k term.Key, height, width int) bool {
	if u.mode == "detail" {
		if k.Code == term.KEsc || k.Code == term.KLeft || k.R == 'q' || k.R == 'h' {
			u.mode = "list"
			return true
		}
		return u.detail.HandleKey(k, height-2, width)
	}
	switch {
	case k.Code == term.KEnter || k.Code == term.KRight || k.R == 'l':
		sel, ok := u.selected()
		if !ok {
			return true
		}
		u.mode = "detail"
		u.detail.SetText("loading…")
		u.detail.Y = 0
		h.Submit(u, "status",
			[]string{"systemctl", "status", "--no-pager", "--full",
				"-n", "30", sel.unit}, 10*time.Second)
		return true
	case k.R == 'f':
		u.scopeI = (u.scopeI + 1) % len(unitScopes)
		u.Refresh(h)
		return true
	case k.R == '/':
		if s, ok := h.Prompt("filter", u.list.Filter); ok {
			u.list.Filter = s
			u.list.Cursor = 0
		}
		return true
	case k.Code == term.KEsc && u.list.Filter != "":
		u.list.Filter = ""
		return true
	}
	if a, ok := unitActions[k.R]; ok {
		u.act(h, a)
		return true
	}
	return u.list.HandleKey(k, height-2)
}

func unitAttr(p any) screen.Style {
	info, ok := p.(unitInfo)
	if !ok {
		return screen.Dim
	}
	switch info.active {
	case "failed":
		return screen.Err
	case "active":
		return screen.OK
	}
	return screen.Dim
}

func (u *Units) Render(s *screen.Screen, y, x, h, w int, focused bool) {
	head := "scope: " + unitScopes[u.scopeI]
	if u.list.Filter != "" {
		head += "  /" + u.list.Filter
	}
	head += fmt.Sprintf("   (%d units)", len(u.list.Visible()))
	st := screen.Dim
	if focused {
		st = screen.Focus
	}
	s.Put(y, x, head, st, w)
	if u.mode == "detail" {
		u.detail.Draw(s, y+1, x, h-2, w)
	} else {
		u.list.Draw(s, y+1, x, h-2, w, focused, unitAttr)
	}
	if u.msg != "" {
		s.Put(y+h-1, x, u.msg, screen.Warn, w)
	}
}

func (u *Units) Footer() string {
	if u.mode == "detail" {
		return "Esc back  j/k scroll"
	}
	return "Enter status  s start  t stop  r restart  e reload  f scope  / filter"
}
