package mods

import (
	"strings"
	"time"
	"unicode"

	"git.pynezz.dev/pynezz/stoker/internal/mod"
	"git.pynezz.dev/pynezz/stoker/internal/run"
	"git.pynezz.dev/pynezz/stoker/internal/screen"
	"git.pynezz.dev/pynezz/stoker/internal/term"
	"git.pynezz.dev/pynezz/stoker/internal/ui"
)

// Deps: systemd service mapper and dependency graph explorer.
//
// Pick mode: list of all loaded units. Enter opens the dependency tree.
// Tree mode: systemctl list-dependencies output with a cursor row for
// drill-down. 'r' toggles forward/reverse direction. Esc goes back.
type Deps struct {
	mod.Base
	list      ui.ListPane
	treeLines []string
	treeRow   int // cursor row within treeLines
	treeOff   int // scroll offset
	mode      string
	unit      string // currently explored unit
	reverse   bool
	msg       string
}

func NewDeps() *Deps {
	return &Deps{
		Base: mod.Base{Nm: "deps", Ttl: "Deps", Ord: 15, Interval: 30 * time.Second},
		mode: "pick",
	}
}

func (d *Deps) Activate(h mod.Host) { d.Refresh(h) }

func (d *Deps) Refresh(h mod.Host) {
	d.Stamp()
	h.Submit(d, "units",
		[]string{"systemctl", "list-units", "--all", "--plain",
			"--no-legend", "--no-pager", "--full"},
		15*time.Second)
}

func (d *Deps) Tick(h mod.Host) {
	if d.mode == "pick" && d.Due() {
		d.Refresh(h)
	}
}

func (d *Deps) fetchTree(h mod.Host) {
	argv := []string{"systemctl", "list-dependencies", "--no-pager"}
	if d.reverse {
		argv = append(argv, "--reverse")
	}
	argv = append(argv, d.unit)
	h.Submit(d, "tree", argv, 15*time.Second)
	d.treeLines = []string{"loading…"}
	d.treeRow = 0
	d.treeOff = 0
}

func (d *Deps) OnResult(h mod.Host, token string, res run.Result) {
	switch token {
	case "units":
		var items []ui.Item
		for _, line := range strings.Split(res.Stdout, "\n") {
			f := strings.Fields(line)
			if len(f) < 1 {
				continue
			}
			items = append(items, ui.Item{Label: f[0], Payload: f[0]})
		}
		d.list.Items = items

	case "tree":
		if res.Err != "" {
			d.treeLines = []string{"error: " + res.Err}
		} else {
			lines := strings.Split(strings.TrimRight(res.Text(), "\n"), "\n")
			if len(lines) == 0 {
				lines = []string{"(no dependencies)"}
			}
			d.treeLines = lines
		}
		d.treeRow = 0
		d.treeOff = 0
	}
}

// unitFromTreeLine extracts a unit name from a list-dependencies tree line.
// Strips leading box-drawing chars (└─ ├─ │ ● ○) and whitespace.
func unitFromTreeLine(line string) string {
	s := strings.TrimLeftFunc(line, func(r rune) bool {
		return unicode.IsSpace(r) || r == '●' || r == '○' ||
			r == '└' || r == '├' || r == '│' || r == '─'
	})
	f := strings.Fields(s)
	if len(f) == 0 {
		return ""
	}
	return f[0]
}

func (d *Deps) clampTree(h int) {
	n := len(d.treeLines)
	if d.treeRow >= n {
		d.treeRow = n - 1
	}
	if d.treeRow < 0 {
		d.treeRow = 0
	}
	if d.treeRow < d.treeOff {
		d.treeOff = d.treeRow
	}
	if d.treeRow >= d.treeOff+h {
		d.treeOff = d.treeRow - h + 1
	}
	if d.treeOff < 0 {
		d.treeOff = 0
	}
}

func (d *Deps) HandleKey(h mod.Host, k term.Key, height, width int) bool {
	treeH := height - 2

	if d.mode == "tree" {
		switch {
		case k.Code == term.KEsc || k.Code == term.KLeft || k.R == 'q' || k.R == 'h':
			d.mode = "pick"
			d.msg = ""
			return true

		case k.R == 'r':
			d.reverse = !d.reverse
			d.fetchTree(h)
			return true

		case k.Code == term.KEnter || k.Code == term.KRight || k.R == 'l':
			if d.treeRow >= 0 && d.treeRow < len(d.treeLines) {
				u := unitFromTreeLine(d.treeLines[d.treeRow])
				if u != "" && u != d.unit {
					d.unit = u
					d.fetchTree(h)
				}
			}
			return true

		case k.Code == term.KUp || k.R == 'k':
			d.treeRow--
		case k.Code == term.KDown || k.R == 'j':
			d.treeRow++
		case k.Code == term.KPgUp:
			d.treeRow -= treeH
		case k.Code == term.KPgDn:
			d.treeRow += treeH
		case k.Code == term.KHome || k.R == 'g':
			d.treeRow = 0
		case k.Code == term.KEnd || k.R == 'G':
			d.treeRow = len(d.treeLines) - 1
		default:
			return false
		}
		d.clampTree(treeH)
		return true
	}

	// pick mode
	switch {
	case k.Code == term.KEnter || k.Code == term.KRight || k.R == 'l':
		if u, ok := d.list.Selected().(string); ok && u != "" {
			d.unit = u
			d.mode = "tree"
			d.fetchTree(h)
		}
		return true
	case k.R == 'r':
		d.reverse = !d.reverse
		d.msg = ""
		if d.reverse {
			d.msg = "reverse: showing what depends on the unit"
		}
		return true
	case k.R == '/':
		if s, ok := h.Prompt("filter", d.list.Filter); ok {
			d.list.Filter = s
			d.list.Cursor = 0
		}
		return true
	case k.Code == term.KEsc && d.list.Filter != "":
		d.list.Filter = ""
		return true
	}
	return d.list.HandleKey(k, height-2)
}

func (d *Deps) Render(s *screen.Screen, y, x, h, w int, focused bool) {
	st := screen.Dim
	if focused {
		st = screen.Focus
	}

	if d.mode == "tree" {
		dir := "deps"
		if d.reverse {
			dir = "rdeps"
		}
		head := dir + ": " + d.unit
		pct := ""
		n := len(d.treeLines)
		treeH := h - 2
		if n > treeH {
			p := 100 * (d.treeOff + treeH)
			if p > 100*n {
				p = 100 * n
			}
			pct = " " + depsPct(p/n) + "%"
		}
		s.Put(y, x, head+pct, st, w)

		d.clampTree(treeH)
		for row := 0; row < treeH; row++ {
			i := d.treeOff + row
			if i >= n {
				break
			}
			lst := screen.Normal
			if i == d.treeRow {
				lst = screen.SelectFocus
				if !focused {
					lst = screen.Select
				}
				s.HLine(y+1+row, x, w, lst)
			}
			s.Put(y+1+row, x, d.treeLines[i], lst, w)
		}
		return
	}

	// pick mode
	head := "deps explorer"
	if d.reverse {
		head += "  [reverse]"
	}
	if d.list.Filter != "" {
		head += "  /" + d.list.Filter
	}
	s.Put(y, x, head, st, w)
	d.list.Draw(s, y+1, x, h-2, w, focused, nil)
	if d.msg != "" {
		s.Put(y+h-1, x, d.msg, screen.Warn, w)
	}
}

func depsPct(n int) string {
	if n >= 100 {
		return "100"
	}
	if n >= 10 {
		return " " + string(rune('0'+n/10)) + string(rune('0'+n%10))
	}
	return "  " + string(rune('0'+n))
}

func (d *Deps) Footer() string {
	if d.mode == "tree" {
		return "Esc back  j/k select  Enter drill-in  r reverse"
	}
	return "Enter explore  r toggle-reverse  / filter"
}
