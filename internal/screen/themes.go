package screen

import "strings"

// ThemeDark is the default 4-bit colour theme (unchanged from v0.1).
var ThemeDark = map[Style]string{
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

// ThemeLight is a 4-bit theme tuned for light-background terminals.
var ThemeLight = map[Style]string{
	Normal:      "",
	Dim:         "\x1b[2m",
	Title:       "\x1b[1;34m",
	Status:      "\x1b[7m",
	StatusBold:  "\x1b[1;7m",
	Select:      "\x1b[7m",
	SelectFocus: "\x1b[1;7m",
	Focus:       "\x1b[1;34m",
	OK:          "\x1b[32m",
	Warn:        "\x1b[33m",
	Err:         "\x1b[1;31m",
	Accent:      "\x1b[35m",
}

// ThemeMono is the attribute-only fallback for serial consoles and dumb terms.
var ThemeMono = map[Style]string{
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

// ThemeNord is a 24-bit TrueColor theme based on the Nord palette.
// https://www.nordtheme.com/
var ThemeNord = map[Style]string{
	// nord4 #D8DEE9 — primary text
	Normal: "\x1b[38;2;216;222;233m",
	// nord3 #4C566A — dim/comments
	Dim: "\x1b[38;2;76;86;106m",
	// nord8 #88C0D0 — teal highlight
	Title: "\x1b[1;38;2;136;192;208m",
	// nord10 bg #5E81AC + nord6 fg #ECEFF4
	Status:     "\x1b[38;2;236;239;244;48;2;94;129;172m",
	StatusBold: "\x1b[1;38;2;236;239;244;48;2;94;129;172m",
	// nord1 #3B4252 bg
	Select: "\x1b[48;2;59;66;82m",
	// nord2 #434C5E bg, bold
	SelectFocus: "\x1b[1;48;2;67;76;94m",
	// nord8 #88C0D0
	Focus: "\x1b[38;2;136;192;208m",
	// nord14 #A3BE8C — green
	OK: "\x1b[38;2;163;190;140m",
	// nord13 #EBCB8B — yellow
	Warn: "\x1b[38;2;235;203;139m",
	// nord11 #BF616A — red, bold
	Err: "\x1b[1;38;2;191;97;106m",
	// nord15 #B48EAD — purple
	Accent: "\x1b[38;2;180;142;173m",
}

// ThemeGruvbox is a 24-bit TrueColor theme based on Gruvbox Dark.
// https://github.com/morhetz/gruvbox
var ThemeGruvbox = map[Style]string{
	// fg #EBDBB2
	Normal: "\x1b[38;2;235;219;178m",
	// fg3 #A89984 — muted
	Dim: "\x1b[38;2;168;153;132m",
	// bright_blue #83A598
	Title: "\x1b[1;38;2;131;165;152m",
	// bg2 #504945 bg + fg #EBDBB2
	Status:     "\x1b[38;2;235;219;178;48;2;80;73;69m",
	StatusBold: "\x1b[1;38;2;235;219;178;48;2;80;73;69m",
	// bg2 #504945 bg
	Select: "\x1b[48;2;80;73;69m",
	// bg3 #665C54 bg, bold
	SelectFocus: "\x1b[1;48;2;102;92;84m",
	// bright_blue #83A598
	Focus: "\x1b[38;2;131;165;152m",
	// bright_green #B8BB26
	OK: "\x1b[38;2;184;187;38m",
	// bright_yellow #FABD2F
	Warn: "\x1b[38;2;250;189;47m",
	// bright_red #FB4934, bold
	Err: "\x1b[1;38;2;251;73;52m",
	// bright_purple #D3869B
	Accent: "\x1b[38;2;211;134;155m",
}

// ThemeForName resolves a theme name to its SGR map.
// truecolor must be true for Nord and Gruvbox; otherwise they fall back to Dark.
func ThemeForName(name string, truecolor bool) map[Style]string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "light":
		return ThemeLight
	case "nord":
		if truecolor {
			return ThemeNord
		}
		return ThemeDark
	case "gruvbox":
		if truecolor {
			return ThemeGruvbox
		}
		return ThemeDark
	case "mono":
		return ThemeMono
	default: // "dark" or ""
		return ThemeDark
	}
}
