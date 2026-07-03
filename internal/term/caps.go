package term

import (
	"os"
	"strings"
)

// Caps reports what the attached terminal can safely render.
// Detected once at startup from environment variables — no runtime probing.
type Caps struct {
	TrueColor bool // 24-bit RGB via \x1b[38;2;r;g;bm
	Unicode   bool // box-drawing, rounded corners, and non-ASCII glyphs
}

// Detect reads $TERM, $COLORTERM, $TERM_PROGRAM, and a handful of
// well-known vendor variables to determine terminal capabilities.
func Detect() Caps {
	t := os.Getenv("TERM")
	// Dumb/legacy/framebuffer consoles: no colour, no Unicode.
	switch t {
	case "", "dumb", "vt100", "vt220", "linux", "cons25":
		return Caps{}
	}
	c := Caps{Unicode: true}
	if os.Getenv("NO_COLOR") != "" {
		return c // unicode yes, colour no
	}
	ct := strings.ToLower(os.Getenv("COLORTERM"))
	if ct == "truecolor" || ct == "24bit" {
		c.TrueColor = true
	}
	switch os.Getenv("TERM_PROGRAM") {
	case "iTerm.app", "WezTerm", "kitty", "Hyper", "vscode":
		c.TrueColor = true
	}
	if os.Getenv("KITTY_WINDOW_ID") != "" {
		c.TrueColor = true
	}
	if os.Getenv("VTE_VERSION") != "" { // GNOME Terminal, Tilix, Terminator, …
		c.TrueColor = true
	}
	if strings.Contains(t, "256color") {
		// 256-color implies at least xterm-compatible; most also do truecolor
		// but we stay conservative — only set TrueColor from explicit signals.
	}
	return c
}
