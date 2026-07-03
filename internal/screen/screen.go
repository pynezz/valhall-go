// Package screen is the renderer: a cell buffer flushed as one ANSI
// frame. Immediate mode carried over from v0.1 — every frame redraws
// everything from module state, so resize handling stays free.
//
// Serial-console consideration: Flush trims trailing blanks per row
// and clears with EL, so a typical frame is a few hundred bytes rather
// than W*H. At 115200 baud that is the difference between usable and
// slideshow.
package screen

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"unicode"

	"git.pynezz.dev/pynezz/stoker/internal/term"
)

type Style uint8

const (
	Normal Style = iota
	Dim
	Title
	Status
	StatusBold
	Select
	SelectFocus
	Focus
	OK
	Warn
	Err
	Accent
)

var sgrColor = map[Style]string{
	Normal:      "",
	Dim:         "\x1b[2m",
	Title:       "\x1b[1;36m",
	Status:      "\x1b[30;46m",
	StatusBold:  "\x1b[1;30;46m",
	Select:      "\x1b[7m",
	SelectFocus: "\x1b[1;7m",
	Focus:       "\x1b[1;36m",
	OK:          "\x1b[32m",
	Warn:        "\x1b[33m",
	Err:         "\x1b[1;31m",
	Accent:      "\x1b[35m",
}

// Monochrome fallback: attributes only, for serial consoles and dumb terms.
var sgrMono = map[Style]string{
	Normal:      "",
	Dim:         "\x1b[2m",
	Title:       "\x1b[1m",
	Status:      "\x1b[7m",
	StatusBold:  "\x1b[1;7m",
	Select:      "\x1b[7m",
	SelectFocus: "\x1b[1;7m",
	Focus:       "\x1b[1m",
	OK:          "\x1b[1m",
	Warn:        "\x1b[4m",
	Err:         "\x1b[1;4m",
	Accent:      "\x1b[1m",
}

type cell struct {
	r  rune
	st Style
}

type Screen struct {
	W, H  int
	cells []cell
	sgr   map[Style]string
}

func New() *Screen {
	s := &Screen{sgr: sgrColor}
	t := os.Getenv("TERM")
	if os.Getenv("NO_COLOR") != "" || t == "dumb" || t == "vt100" || t == "vt220" {
		s.sgr = sgrMono
	}
	s.UpdateSize()
	return s
}

func (s *Screen) UpdateSize() {
	s.W, s.H = term.Size()
	s.cells = make([]cell, s.W*s.H)
	s.Clear()
}

func (s *Screen) Clear() {
	for i := range s.cells {
		s.cells[i] = cell{' ', Normal}
	}
}

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]|\x1b.`)

// Sanitize strips escape sequences and control characters from
// untrusted text (tool output, plugin output, journal lines) before it
// reaches the terminal. Displayed text must never be able to inject
// sequences into the operator's tty.
func Sanitize(in string) string {
	in = ansiRe.ReplaceAllString(in, "")
	in = strings.ReplaceAll(in, "\t", "    ")
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, in)
}

// Put writes one sanitized line at (y,x), clipped to maxw (<=0 means
// to the right edge).
func (s *Screen) Put(y, x int, text string, st Style, maxw int) {
	if y < 0 || y >= s.H || x >= s.W {
		return
	}
	limit := s.W - x
	if maxw > 0 && maxw < limit {
		limit = maxw
	}
	col := x
	for _, r := range Sanitize(text) {
		if col < 0 {
			col++
			continue
		}
		if col >= x+limit || col >= s.W {
			break
		}
		s.cells[y*s.W+col] = cell{r, st}
		col++
	}
}

func (s *Screen) HLine(y, x, w int, st Style) {
	if y < 0 || y >= s.H {
		return
	}
	for c := x; c < x+w && c < s.W; c++ {
		if c >= 0 {
			s.cells[y*s.W+c] = cell{' ', st}
		}
	}
}

// Flush emits the frame. Rows are trimmed after their last non-default
// cell and terminated with SGR-reset + erase-to-EOL.
func (s *Screen) Flush() {
	var b strings.Builder
	b.Grow(s.W * s.H / 4)
	b.WriteString("\x1b[H")
	for row := 0; row < s.H; row++ {
		end := -1
		base := row * s.W
		for col := s.W - 1; col >= 0; col-- {
			c := s.cells[base+col]
			if c.r != ' ' || c.st != Normal {
				end = col
				break
			}
		}
		fmt.Fprintf(&b, "\x1b[%d;1H", row+1)
		cur := Style(255)
		for col := 0; col <= end; col++ {
			c := s.cells[base+col]
			if c.st != cur {
				b.WriteString("\x1b[0m")
				b.WriteString(s.sgr[c.st])
				cur = c.st
			}
			b.WriteRune(c.r)
		}
		b.WriteString("\x1b[0m\x1b[K")
	}
	os.Stdout.WriteString(b.String())
}
