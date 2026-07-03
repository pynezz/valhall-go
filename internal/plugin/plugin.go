// Package plugin: script plugins, header format frozen since v0.1 —
// plugins written against the Python build load unchanged:
//
//	#!/usr/bin/env bash
//	# stoker-plugin: AVC denials (24h)
//	# stoker-order: 81
//	# stoker-timeout: 30
//	# stoker-root: yes
//
// The Go port drops in-process Python plugins by design: the script
// tier is THE plugin tier now. Anything needing real logic can be a
// compiled helper that a script invokes — same trust model, and the
// TUI process stays free of foreign code.
//
// Trust model unchanged: a plugin is arbitrary code, so the only
// meaningful control is who can write the directory. Dirs and files
// must be owned by root or the invoking user and not group/world
// writable; failures are skipped and reported, never loaded.
package plugin

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"git.pynezz.dev/pynezz/stoker/internal/mod"
	"git.pynezz.dev/pynezz/stoker/internal/run"
	"git.pynezz.dev/pynezz/stoker/internal/screen"
	"git.pynezz.dev/pynezz/stoker/internal/term"
	"git.pynezz.dev/pynezz/stoker/internal/ui"
)

type Report struct {
	Loaded  []string
	Skipped []string // "path (reason)"
}

func trusted(path string, euid int) (bool, string) {
	fi, err := os.Stat(path)
	if err != nil {
		return false, err.Error()
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return false, "no stat info"
	}
	if st.Uid != 0 && int(st.Uid) != euid {
		return false, fmt.Sprintf("owner uid %d is neither root nor uid %d", st.Uid, euid)
	}
	if fi.Mode().Perm()&0o022 != 0 {
		return false, "group/world writable"
	}
	return true, ""
}

type meta struct {
	title   string
	order   int
	timeout time.Duration
	root    bool
}

func parseHeader(path string) *meta {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	m := meta{order: 80, timeout: 30 * time.Second}
	sc := bufio.NewScanner(f)
	for i := 0; i < 20 && sc.Scan(); i++ {
		line := strings.TrimSpace(sc.Text())
		switch {
		case strings.HasPrefix(line, "# stoker-plugin:"):
			m.title = strings.TrimSpace(strings.TrimPrefix(line, "# stoker-plugin:"))
		case strings.HasPrefix(line, "# stoker-order:"):
			if n, err := strconv.Atoi(strings.TrimSpace(
				strings.TrimPrefix(line, "# stoker-order:"))); err == nil {
				m.order = n
			}
		case strings.HasPrefix(line, "# stoker-timeout:"):
			if n, err := strconv.Atoi(strings.TrimSpace(
				strings.TrimPrefix(line, "# stoker-timeout:"))); err == nil && n > 0 {
				m.timeout = time.Duration(n) * time.Second
			}
		case strings.HasPrefix(line, "# stoker-root:"):
			v := strings.ToLower(strings.TrimSpace(
				strings.TrimPrefix(line, "# stoker-root:")))
			m.root = v == "yes" || v == "true" || v == "1"
		}
	}
	if m.title == "" {
		return nil
	}
	return &m
}

// Script wraps an executable as a read-only screen. Runs on first
// entry, then on explicit 'r' — scripts may be expensive.
type Script struct {
	mod.Base
	path    string
	timeout time.Duration
	root    bool
	pane    ui.TextPane
	ran     bool
}

func (p *Script) Activate(h mod.Host) {
	if !p.ran {
		p.Refresh(h)
	}
}

func (p *Script) Refresh(h mod.Host) {
	p.Stamp()
	p.ran = true
	argv := []string{p.path}
	if p.root {
		argv = h.Priv().Wrap(argv)
		if argv == nil {
			p.pane.SetText(h.Priv().Hint())
			return
		}
	}
	p.pane.SetText("running…")
	h.Submit(p, "run", argv, p.timeout)
}

func (p *Script) Tick(mod.Host) {}

func (p *Script) OnResult(h mod.Host, token string, res run.Result) {
	p.pane.SetText(res.Text())
}

func (p *Script) HandleKey(h mod.Host, k term.Key, height, width int) bool {
	switch k.R {
	case 'r':
		p.Refresh(h)
		return true
	case '/':
		if s, ok := h.Prompt("search", p.pane.Search); ok {
			p.pane.Search = s
			p.pane.NextMatch()
		}
		return true
	}
	return p.pane.HandleKey(k, height-1, width)
}

func (p *Script) Render(s *screen.Screen, y, x, h, w int, focused bool) {
	st := screen.Dim
	if focused {
		st = screen.Focus
	}
	s.Put(y, x, "plugin: "+p.path, st, w)
	p.pane.Draw(s, y+1, x, h-1, w)
}

func (p *Script) Footer() string { return "r rerun  / search" }

// LoadAll walks the plugin dirs and registers every trusted script
// carrying a valid header.
func LoadAll(dirs []string, euid int) Report {
	var rep Report
	for _, d := range dirs {
		fi, err := os.Stat(d)
		if err != nil || !fi.IsDir() {
			continue
		}
		if ok, why := trusted(d, euid); !ok {
			rep.Skipped = append(rep.Skipped, d+" ("+why+")")
			continue
		}
		entries, err := os.ReadDir(d)
		if err != nil {
			rep.Skipped = append(rep.Skipped, d+" ("+err.Error()+")")
			continue
		}
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			if !e.IsDir() {
				names = append(names, e.Name())
			}
		}
		sort.Strings(names)
		for _, name := range names {
			path := filepath.Join(d, name)
			if ok, why := trusted(path, euid); !ok {
				rep.Skipped = append(rep.Skipped, path+" ("+why+")")
				continue
			}
			fi, err := os.Stat(path)
			if err != nil || fi.Mode()&0o111 == 0 {
				continue // not executable: silently ignore (README, etc.)
			}
			m := parseHeader(path)
			if m == nil {
				rep.Skipped = append(rep.Skipped, path+" (no stoker-plugin header)")
				continue
			}
			mod.Register(&Script{
				Base: mod.Base{Nm: "sp-" + name, Ttl: m.title, Ord: m.order},
				path: path, timeout: m.timeout, root: m.root,
			})
			rep.Loaded = append(rep.Loaded, "sh:"+m.title)
		}
	}
	return rep
}
