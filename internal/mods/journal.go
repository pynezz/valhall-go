package mods

import (
	"strconv"
	"strings"
	"time"

	"git.pynezz.dev/pynezz/stoker/internal/mod"
	"git.pynezz.dev/pynezz/stoker/internal/run"
	"git.pynezz.dev/pynezz/stoker/internal/screen"
	"git.pynezz.dev/pynezz/stoker/internal/term"
	"git.pynezz.dev/pynezz/stoker/internal/ui"
)

var priorities = []string{"debug", "info", "notice", "warning", "err"}

// Journal: tail with priority/boot/unit/grep filters. In-pane follow
// stays the 2 s re-poll from v0.1; `F` inside tmux splits a real
// `journalctl -f` below stoker — the honest way to stream, with pipe
// lifecycle owned by tmux instead of us.
type Journal struct {
	mod.Base
	pane   ui.TextPane
	prioI  int
	boot   string
	unit   string
	grep   string
	follow bool
	msg    string
}

func NewJournal() *Journal {
	j := &Journal{
		Base:  mod.Base{Nm: "journal", Ttl: "Journal", Ord: 20},
		prioI: 1, boot: "0",
	}
	j.pane.SetText("loading…")
	return j
}

func (j *Journal) argv(h mod.Host) []string {
	a := []string{"journalctl", "--no-pager", "-o", "short-iso",
		"-b", j.boot, "-p", priorities[j.prioI],
		"-n", strconv.Itoa(h.JournalLines())}
	if j.unit != "" {
		a = append(a, "-u", j.unit)
	}
	if j.grep != "" {
		a = append(a, "-g", j.grep)
	}
	return a
}

func (j *Journal) Activate(h mod.Host) { j.Refresh(h) }

func (j *Journal) Refresh(h mod.Host) {
	j.Stamp()
	h.Submit(j, "tail", j.argv(h), 12*time.Second)
}

func (j *Journal) Tick(h mod.Host) {
	if j.follow && j.Due() {
		j.Refresh(h)
	}
}

func (j *Journal) OnResult(h mod.Host, token string, res run.Result) {
	if token != "tail" {
		return
	}
	j.pane.FollowTail = j.follow
	j.pane.SetText(res.Text())
}

func (j *Journal) HandleKey(h mod.Host, k term.Key, height, width int) bool {
	j.msg = ""
	switch k.R {
	case 'p':
		j.prioI = (j.prioI + 1) % len(priorities)
	case 'b':
		if j.boot == "0" {
			j.boot = "-1"
		} else {
			j.boot = "0"
		}
	case 'f':
		j.follow = !j.follow
		if j.follow {
			j.Interval = 2 * time.Second
		} else {
			j.Interval = 0
		}
	case 'F': // live follow in a tmux split
		if !h.InTmux() {
			j.msg = "F needs tmux — run valhall inside a tmux session"
			return true
		}
		args := []string{"journalctl", "-f", "-o", "short-iso",
			"-p", priorities[j.prioI]}
		if j.unit != "" {
			args = append(args, "-u", j.unit)
		}
		if err := h.TmuxSplit(12, args); err != nil {
			j.msg = err.Error()
		}
		return true
	case 'u':
		s, ok := h.Prompt("unit (empty = all)", j.unit)
		if !ok {
			return true
		}
		j.unit = strings.TrimSpace(s)
	case 'G':
		s, ok := h.Prompt("grep (journalctl -g)", j.grep)
		if !ok {
			return true
		}
		j.grep = strings.TrimSpace(s)
	case '/':
		if s, ok := h.Prompt("search on screen", j.pane.Search); ok {
			j.pane.Search = s
			j.pane.NextMatch()
		}
		return true
	default:
		return j.pane.HandleKey(k, height-1, width)
	}
	j.Refresh(h)
	return true
}

func (j *Journal) Render(s *screen.Screen, y, x, h, w int, focused bool) {
	bits := []string{"prio≥" + priorities[j.prioI]}
	if j.boot == "0" {
		bits = append(bits, "boot:current")
	} else {
		bits = append(bits, "boot:previous")
	}
	if j.unit != "" {
		bits = append(bits, "unit:"+j.unit)
	}
	if j.grep != "" {
		bits = append(bits, "grep:"+j.grep)
	}
	if j.follow {
		bits = append(bits, "FOLLOW")
	}
	st := screen.Dim
	if focused {
		st = screen.Focus
	}
	s.Put(y, x, strings.Join(bits, "  "), st, w)
	j.pane.Draw(s, y+1, x, h-1, w)
	if j.msg != "" {
		s.Put(y+h-1, x, j.msg, screen.Warn, w)
	}
}

func (j *Journal) Footer() string {
	return "p prio  b boot  u unit  G grep  f poll-follow  F tmux-follow  / search"
}

// RegisterBuiltins wires the whole built-in inventory. Declarative
// views adjusted for a RHEL 8 target: nmcli instead of resolvectl
// (systemd-resolved isn't the resolver there), vmstat alongside PSI
// (the 4.18 kernel usually lacks /proc/pressure).
func RegisterBuiltins(confirmDestructive bool) {
	mod.Register(NewOverview())
	mod.Register(NewUnits(confirmDestructive))
	mod.Register(NewDeps())
	mod.Register(NewJournal())

	mod.Register(mod.NewCmd("storage", "Storage", 30, 0, 15*time.Second,
		[]mod.View{
			{Name: "block", Argv: []string{"lsblk", "-o",
				"NAME,SIZE,TYPE,FSTYPE,LABEL,MOUNTPOINTS", "--tree"}},
			{Name: "mounts", Argv: []string{"findmnt", "--df", "--real"}},
			{Name: "btrfs", Argv: []string{"btrfs", "filesystem", "usage", "/"}},
			{Name: "luks", Argv: []string{"lsblk", "-o", "NAME,TYPE,FSTYPE,MOUNTPOINTS", "--tree"}},
			{Name: "diskstats", Argv: []string{"cat", "/proc/diskstats"}},
		}))

	mod.Register(mod.NewCmd("network", "Network", 40, 10*time.Second, 15*time.Second,
		[]mod.View{
			{Name: "addr", Argv: []string{"ip", "-br", "addr"}},
			{Name: "routes", Argv: []string{"ip", "route"}},
			{Name: "sockets", Argv: []string{"ss", "-tulpn"}},
			{Name: "nm", Argv: []string{"nmcli", "device", "status"}},
			{Name: "dns", Argv: []string{"cat", "/etc/resolv.conf"}},
			{Name: "links", Argv: []string{"ip", "-s", "link"}},
		}))

	mod.Register(mod.NewCmd("procs", "Processes", 45, 5*time.Second, 15*time.Second,
		[]mod.View{
			{Name: "top-cpu", Argv: []string{"ps", "-eo",
				"pid,ppid,user,pcpu,pmem,stat,etime,comm", "--sort=-pcpu"}},
			{Name: "top-mem", Argv: []string{"ps", "-eo",
				"pid,ppid,user,pcpu,pmem,stat,etime,comm", "--sort=-pmem"}},
			{Name: "cgroups", Argv: []string{"systemd-cgls", "--no-pager", "-l"}},
			{Name: "vmstat", Argv: []string{"vmstat", "1", "2"}},
			{Name: "pressure", Argv: []string{"cat", "/proc/pressure/cpu",
				"/proc/pressure/memory", "/proc/pressure/io"}},
		}))

	mod.Register(mod.NewCmd("selinux", "SELinux", 60, 0, 30*time.Second,
		[]mod.View{
			{Name: "status", Argv: []string{"sestatus", "-v"}},
			{Name: "denials", Argv: []string{"ausearch", "-m", "AVC,USER_AVC", "-ts", "recent"}},
			{Name: "booleans", Argv: []string{"getsebool", "-a"}},
			{Name: "modules", Argv: []string{"semodule", "-l"}},
		}))
}
