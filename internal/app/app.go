// Package app: layout, event loop, key routing. One goroutine owns
// the terminal; workers own subprocesses; they meet only at channels.
package app

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"git.pynezz.dev/pynezz/stoker/internal/config"
	"git.pynezz.dev/pynezz/stoker/internal/mod"
	"git.pynezz.dev/pynezz/stoker/internal/priv"
	"git.pynezz.dev/pynezz/stoker/internal/run"
	"git.pynezz.dev/pynezz/stoker/internal/screen"
	"git.pynezz.dev/pynezz/stoker/internal/term"
	"git.pynezz.dev/pynezz/stoker/internal/tmuxx"
)

const navW = 24

var globalHelp = []string{
	"Tab        switch focus nav <-> content",
	"1-9        jump to module",
	"j/k arrows navigate",
	"Enter      focus content / open item",
	"R / F5     refresh active module",
	"!          shell (tmux split if inside tmux)",
	"Esc        back / clear filter",
	"?          this help",
	"q          quit (from nav)   Q / Ctrl-C quit anywhere",
	"",
	"Each module shows its own keys in the bottom bar.",
}

type App struct {
	scr         *screen.Screen
	keys        <-chan term.Key
	resize      <-chan os.Signal
	runner      *run.Runner
	pv          *priv.Priv
	cfg         config.Config
	modules     []mod.Module
	activeI     int
	focusNav    bool
	inflight    map[string]mod.Module
	quit        bool
	pluginsSkip int
	caps        term.Caps
}

func New(cfg config.Config, pluginsSkipped int) *App {
	return &App{
		runner:      run.NewRunner(3),
		pv:          priv.New(),
		cfg:         cfg,
		modules:     mod.All(),
		focusNav:    true,
		inflight:    map[string]mod.Module{},
		pluginsSkip: pluginsSkipped,
		caps:        term.Detect(),
	}
}

// ---- mod.Host --------------------------------------------------------

func (a *App) Submit(m mod.Module, token string, argv []string, timeout time.Duration) {
	full := m.Name() + "\x00" + token
	a.inflight[full] = m
	a.runner.Submit(full, argv, timeout)
}

func (a *App) Priv() *priv.Priv { return a.pv }
func (a *App) JournalLines() int {
	return a.cfg.JournalLines
}
func (a *App) InTmux() bool { return tmuxx.Inside() }
func (a *App) TmuxSplit(lines int, argv []string) error {
	return tmuxx.Split(true, lines, argv)
}
func (a *App) TmuxWindow(name string, argv []string) error {
	return tmuxx.Window(name, argv)
}

// Confirm blocks on the key channel; worker results buffer meanwhile.
func (a *App) Confirm(question string) bool {
	for {
		a.drawBase()
		a.modal("confirm", []string{question}, "[y] yes   [n/Esc] no")
		a.scr.Flush()
		k := <-a.keys
		switch {
		case k.R == 'y' || k.R == 'Y':
			return true
		case k.R == 'n' || k.R == 'N' || k.R == 'q' || k.Code == term.KEsc:
			return false
		}
	}
}

// Prompt is a single-line editor on the status row. Esc cancels.
func (a *App) Prompt(label, initial string) (string, bool) {
	buf := []rune(initial)
	for {
		a.drawBase()
		row := a.scr.H - 1
		a.scr.HLine(row, 0, a.scr.W, screen.Status)
		a.scr.Put(row, 0, " "+label+": "+string(buf)+"▏", screen.Status, a.scr.W)
		a.scr.Flush()
		k := <-a.keys
		switch {
		case k.Code == term.KEnter:
			return string(buf), true
		case k.Code == term.KEsc || k.Code == term.KCtrlC:
			return "", false
		case k.Code == term.KBackspace:
			if len(buf) > 0 {
				buf = buf[:len(buf)-1]
			}
		case k.Code == term.KRune && k.R >= ' ':
			buf = append(buf, k.R)
		}
	}
}

// ---- main loop -------------------------------------------------------

func (a *App) Run() {
	a.scr = screen.New()
	a.scr.SetTheme(screen.ThemeForName(a.cfg.Theme, a.caps.TrueColor))
	a.keys = term.ReadKeys()
	a.resize = term.WatchResize()
	tick := time.NewTicker(200 * time.Millisecond)
	defer tick.Stop()

	if len(a.modules) > 0 {
		a.modules[a.activeI].Activate(a)
	}
	for !a.quit {
		a.drawBase()
		a.scr.Flush()
		select {
		case k := <-a.keys:
			a.handleKey(k)
		case ev := <-a.runner.Events:
			a.route(ev)
			a.drain()
		case <-tick.C:
			a.active().Tick(a)
		case <-a.resize:
			a.scr.UpdateSize()
		}
	}
}

// drain consumes any burst of already-completed results in one frame.
func (a *App) drain() {
	for {
		select {
		case ev := <-a.runner.Events:
			a.route(ev)
		default:
			return
		}
	}
}

func (a *App) route(ev run.Event) {
	m, ok := a.inflight[ev.Token]
	if !ok {
		return
	}
	delete(a.inflight, ev.Token)
	_, token, _ := strings.Cut(ev.Token, "\x00")
	m.OnResult(a, token, ev.Res)
}

func (a *App) active() mod.Module { return a.modules[a.activeI] }

func (a *App) switchTo(i int) {
	if i >= 0 && i < len(a.modules) && i != a.activeI {
		a.activeI = i
		a.active().Activate(a)
	}
}

// ---- input -----------------------------------------------------------

func (a *App) handleKey(k term.Key) {
	ch, cw := a.contentGeom()
	switch {
	case k.R == 'Q' || k.Code == term.KCtrlC:
		a.quit = true
		return
	case k.R == '?':
		a.helpOverlay()
		return
	case k.R == 'R' || k.Code == term.KF5:
		a.active().Refresh(a)
		return
	case k.R == '!':
		a.shell()
		return
	case k.Code == term.KRune && k.R >= '1' && k.R <= '9':
		a.switchTo(int(k.R - '1'))
		a.focusNav = false
		return
	case k.Code == term.KTab:
		a.focusNav = !a.focusNav
		return
	}

	if a.focusNav {
		switch {
		case k.Code == term.KUp || k.R == 'k':
			a.switchTo(a.activeI - 1)
		case k.Code == term.KDown || k.R == 'j':
			a.switchTo(a.activeI + 1)
		case k.Code == term.KEnter || k.Code == term.KRight || k.R == 'l':
			a.focusNav = false
		case k.R == 'q':
			a.quit = true
		}
		return
	}
	if !a.active().HandleKey(a, k, ch, cw) {
		if k.Code == term.KEsc || k.R == 'q' {
			a.focusNav = true
		}
	}
}

func (a *App) helpOverlay() {
	a.drawBase()
	a.modal("valhall — keys", globalHelp, "any key to close")
	a.scr.Flush()
	<-a.keys
}

// shell: inside tmux, a split beside stoker; otherwise suspend and
// hand the tty to $SHELL. $SHELL is the operator's own login shell —
// same trust as the session itself.
func (a *App) shell() {
	if tmuxx.Inside() {
		_ = tmuxx.Split(true, 0, nil)
		return
	}
	term.Suspend(func() {
		sh := os.Getenv("SHELL")
		if sh == "" {
			sh = "/bin/bash"
		}
		fmt.Println("valhall suspended — exit the shell to return")
		c := exec.Command(sh)
		c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
		_ = c.Run()
	})
	a.scr.UpdateSize()
}

// ---- drawing ---------------------------------------------------------

func (a *App) contentGeom() (h, w int) {
	h = a.scr.H - 3
	if a.caps.Unicode {
		h -= 2 // top + bottom border rows
	}
	w = a.scr.W - navW - 3
	if h < 1 {
		h = 1
	}
	if w < 1 {
		w = 1
	}
	return
}

func (a *App) drawBase() {
	s := a.scr
	s.Clear()
	if s.H < 8 || s.W < 50 {
		s.Put(0, 0, "terminal too small for valhall", screen.Normal, 0)
		return
	}
	// title bar
	s.HLine(0, 0, s.W, screen.Status)
	s.Put(0, 1, " valhall · "+a.active().Title()+" ", screen.StatusBold, 0)
	badges := time.Now().Format("15:04:05") + "  " + a.pv.Badge()
	if tmuxx.Inside() {
		badges += " tmux"
	}
	if os.Getenv("SSH_CONNECTION") != "" {
		badges += " ssh"
	}
	s.Put(0, s.W-len(badges)-2, badges, screen.Status, 0)

	// nav + separator/border
	cy := 1
	if a.caps.Unicode {
		cy = 2
	}
	for i, m := range a.modules {
		row := cy + i
		if row >= s.H-2 {
			break
		}
		st := screen.Normal
		if i == a.activeI {
			st = screen.Select
			if a.focusNav {
				st = screen.SelectFocus
			}
			s.HLine(row, 0, navW, st)
		}
		hot := " "
		if i < 9 {
			hot = string(rune('1' + i))
		}
		s.Put(row, 0, " "+hot+" "+m.Title(), st, navW)
	}
	if a.caps.Unicode {
		a.drawBorders(s)
	} else {
		for row := 1; row < s.H-1; row++ {
			s.Put(row, navW, "│", screen.Dim, 2)
		}
	}

	// content
	ch, cw := a.contentGeom()
	focused := !a.focusNav
	a.active().Render(s, cy, navW+2, ch, cw, focused)

	// plugin warnings: last interior row so they don't stomp the border
	warnRow := s.H - 2
	if a.caps.Unicode {
		warnRow = s.H - 3
	}
	if a.pluginsSkip > 0 {
		s.Put(warnRow, navW+2, fmt.Sprintf("! %d plugin(s) skipped — see valhall --plugins",
			a.pluginsSkip), screen.Warn, 0)
	}

	// status bar
	s.HLine(s.H-1, 0, s.W, screen.Status)
	focus := "content"
	if a.focusNav {
		focus = "nav"
	}
	s.Put(s.H-1, 0, " ["+focus+"] "+a.active().Footer(), screen.Status, 0)
	right := "? help  Q quit "
	if n := len(a.inflight); n > 0 {
		right = fmt.Sprintf("%d job(s)… ", n) + right
	}
	s.Put(s.H-1, s.W-len(right)-1, right, screen.Status, 0)
}

// drawBorders draws a rounded-corner box enclosing the content pane.
// Left wall at column navW; right wall at column W-1;
// top edge at row 1; bottom edge at row H-2.
// Content interior: rows 2..H-3, columns navW+2..W-2 (unchanged from
// the no-border layout — only height changes, not x or width).
func (a *App) drawBorders(s *screen.Screen) {
	bst := screen.Dim
	// top edge
	s.Put(1, navW, "╭", bst, 1)
	for col := navW + 1; col < s.W-1; col++ {
		s.Put(1, col, "─", bst, 1)
	}
	s.Put(1, s.W-1, "╮", bst, 1)
	// side walls
	for row := 2; row < s.H-2; row++ {
		s.Put(row, navW, "│", bst, 1)
		s.Put(row, s.W-1, "│", bst, 1)
	}
	// bottom edge
	s.Put(s.H-2, navW, "╰", bst, 1)
	for col := navW + 1; col < s.W-1; col++ {
		s.Put(s.H-2, col, "─", bst, 1)
	}
	s.Put(s.H-2, s.W-1, "╯", bst, 1)
}

func (a *App) modal(title string, body []string, footer string) {
	s := a.scr
	w := len(title) + 6
	if l := len(footer) + 6; l > w {
		w = l
	}
	for _, b := range body {
		if len(b)+4 > w {
			w = len(b) + 4
		}
	}
	if w < 30 {
		w = 30
	}
	if w > s.W-4 {
		w = s.W - 4
	}
	h := len(body) + 4
	if h > s.H-2 {
		h = s.H - 2
	}
	y0, x0 := (s.H-h)/2, (s.W-w)/2
	for r := 0; r < h; r++ {
		s.HLine(y0+r, x0, w, screen.Status)
	}
	s.Put(y0, x0+2, " "+title+" ", screen.StatusBold, w-4)
	for i, b := range body {
		if i >= h-4 {
			break
		}
		s.Put(y0+2+i, x0+2, b, screen.Status, w-4)
	}
	s.Put(y0+h-1, x0+2, footer, screen.StatusBold, w-4)
}
