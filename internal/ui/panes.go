// Package ui: the two workhorse panes. State holders with a Draw
// method — geometry decided by the caller every frame, same contract
// as v0.1.
package ui

import (
	"strings"

	"git.pynezz.dev/pynezz/stoker/internal/screen"
	"git.pynezz.dev/pynezz/stoker/internal/term"
)

type Item struct {
	Label   string
	Payload any
}

type ListPane struct {
	Items  []Item
	Cursor int
	Offset int
	Filter string
}

func (l *ListPane) Visible() []Item {
	if l.Filter == "" {
		return l.Items
	}
	f := strings.ToLower(l.Filter)
	out := make([]Item, 0, len(l.Items))
	for _, it := range l.Items {
		if strings.Contains(strings.ToLower(it.Label), f) {
			out = append(out, it)
		}
	}
	return out
}

func (l *ListPane) Selected() any {
	v := l.Visible()
	if l.Cursor >= 0 && l.Cursor < len(v) {
		return v[l.Cursor].Payload
	}
	return nil
}

func (l *ListPane) clamp(h int) {
	n := len(l.Visible())
	if l.Cursor > n-1 {
		l.Cursor = n - 1
	}
	if l.Cursor < 0 {
		l.Cursor = 0
	}
	if l.Cursor < l.Offset {
		l.Offset = l.Cursor
	}
	if l.Cursor >= l.Offset+h {
		l.Offset = l.Cursor - h + 1
	}
	if l.Offset < 0 {
		l.Offset = 0
	}
}

func (l *ListPane) HandleKey(k term.Key, h int) bool {
	switch {
	case k.Code == term.KUp || k.R == 'k':
		l.Cursor--
	case k.Code == term.KDown || k.R == 'j':
		l.Cursor++
	case k.Code == term.KPgUp:
		l.Cursor -= h
	case k.Code == term.KPgDn:
		l.Cursor += h
	case k.Code == term.KHome || k.R == 'g':
		l.Cursor = 0
	case k.Code == term.KEnd || k.R == 'G':
		l.Cursor = len(l.Visible()) - 1
	default:
		return false
	}
	l.clamp(h)
	return true
}

func (l *ListPane) Draw(s *screen.Screen, y, x, h, w int, focused bool,
	attrFn func(any) screen.Style) {
	l.clamp(h)
	v := l.Visible()
	for row := 0; row < h; row++ {
		i := l.Offset + row
		if i >= len(v) {
			break
		}
		st := screen.Normal
		if attrFn != nil {
			st = attrFn(v[i].Payload)
		}
		if i == l.Cursor {
			st = screen.Select
			if focused {
				st = screen.SelectFocus
			}
			s.HLine(y+row, x, w, st)
		}
		s.Put(y+row, x, v[i].Label, st, w)
	}
	if len(v) == 0 {
		msg := "(empty)"
		if l.Filter != "" {
			msg = "(no matches)"
		}
		s.Put(y, x, msg, screen.Dim, w)
	}
}

type TextPane struct {
	Lines      []string
	Y, X       int
	FollowTail bool
	Search     string
}

func (t *TextPane) SetText(text string) {
	t.Lines = strings.Split(text, "\n")
	if len(t.Lines) == 0 {
		t.Lines = []string{""}
	}
	if t.FollowTail {
		t.Y = 1 << 30
	}
}

func (t *TextPane) clamp(h int) {
	max := len(t.Lines) - h
	if max < 0 {
		max = 0
	}
	if t.Y > max {
		t.Y = max
	}
	if t.Y < 0 {
		t.Y = 0
	}
	if t.X < 0 {
		t.X = 0
	}
}

func (t *TextPane) NextMatch() {
	s := strings.ToLower(t.Search)
	if s == "" {
		return
	}
	for i := t.Y + 1; i < len(t.Lines); i++ {
		if strings.Contains(strings.ToLower(t.Lines[i]), s) {
			t.Y = i
			return
		}
	}
}

func (t *TextPane) HandleKey(k term.Key, h, w int) bool {
	page := h - 1
	if page < 1 {
		page = 1
	}
	switch {
	case k.Code == term.KUp || k.R == 'k':
		t.Y--
	case k.Code == term.KDown || k.R == 'j':
		t.Y++
	case k.Code == term.KPgUp:
		t.Y -= page
	case k.Code == term.KPgDn:
		t.Y += page
	case k.Code == term.KHome || k.R == 'g':
		t.Y = 0
	case k.Code == term.KEnd || k.R == 'G':
		t.Y = 1 << 30
	case k.Code == term.KLeft:
		t.X -= 8
	case k.Code == term.KRight:
		t.X += 8
	case k.R == 'n' && t.Search != "":
		t.NextMatch()
	default:
		return false
	}
	t.clamp(h)
	return true
}

func (t *TextPane) Draw(s *screen.Screen, y, x, h, w int) {
	t.clamp(h)
	needle := strings.ToLower(t.Search)
	for row := 0; row < h; row++ {
		i := t.Y + row
		if i >= len(t.Lines) {
			break
		}
		line := t.Lines[i]
		if t.X < len(line) {
			line = line[t.X:]
		} else {
			line = ""
		}
		st := screen.Normal
		if needle != "" && strings.Contains(strings.ToLower(t.Lines[i]), needle) {
			st = screen.Accent
		}
		s.Put(y+row, x, line, st, w)
	}
	if len(t.Lines) > h { // scroll position hint
		pct := 100 * (t.Y + h) / len(t.Lines)
		if pct > 100 {
			pct = 100
		}
		s.Put(y, x+w-5, itoa3(pct)+"%", screen.Dim, 5)
	}
}

func itoa3(n int) string {
	switch {
	case n >= 100:
		return "100"
	case n >= 10:
		return " " + string(rune('0'+n/10)) + string(rune('0'+n%10))
	default:
		return "  " + string(rune('0'+n))
	}
}
