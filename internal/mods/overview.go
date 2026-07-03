package mods

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"git.pynezz.dev/pynezz/stoker/internal/mod"
	"git.pynezz.dev/pynezz/stoker/internal/run"
	"git.pynezz.dev/pynezz/stoker/internal/screen"
	"git.pynezz.dev/pynezz/stoker/internal/term"
)

// Overview: "is this box healthy?" in one look. Local facts straight
// from /proc — zero subprocesses, so it renders instantly even on a
// struggling machine. systemd state arrives asynchronously.
type Overview struct {
	mod.Base
	failed   []string
	sysstate string
}

func NewOverview() *Overview {
	return &Overview{
		Base:     mod.Base{Nm: "overview", Ttl: "Overview", Ord: 0, Interval: 5 * time.Second},
		sysstate: "…",
	}
}

func (o *Overview) Activate(h mod.Host) { o.Refresh(h) }

func (o *Overview) Refresh(h mod.Host) {
	o.Stamp()
	h.Submit(o, "failed",
		[]string{"systemctl", "--failed", "--no-legend", "--plain"}, 8*time.Second)
	h.Submit(o, "state", []string{"systemctl", "is-system-running"}, 8*time.Second)
}

func (o *Overview) Tick(h mod.Host) {
	if o.Due() {
		o.Refresh(h)
	}
}

func (o *Overview) OnResult(h mod.Host, token string, res run.Result) {
	switch token {
	case "failed":
		o.failed = o.failed[:0]
		if res.Err != "" {
			o.failed = append(o.failed, "("+res.Err+")")
			return
		}
		for _, l := range strings.Split(res.Stdout, "\n") {
			if f := strings.Fields(l); len(f) > 0 {
				o.failed = append(o.failed, f[0])
			}
		}
	case "state":
		o.sysstate = strings.TrimSpace(res.Stdout)
		if o.sysstate == "" {
			o.sysstate = res.Err
			if o.sysstate == "" {
				o.sysstate = "unknown"
			}
		}
	}
}

func (o *Overview) HandleKey(h mod.Host, k term.Key, height, width int) bool {
	return false
}

// valhallLogo is the VALHALL word-mark in box-drawing characters.
// Each line is exactly 28 runes wide: V(4) + 6×(sep+letter(3)).
var valhallLogo = [3]string{
	`╦  ╦ ╔═╗ ╦   ╦ ╦ ╔═╗ ╦   ╦  `,
	`╚╗╔╝ ╠═╣ ║   ╠═╣ ╠═╣ ║   ║  `,
	` ╚╝  ╩ ╩ ╩═╝ ╩ ╩ ╩ ╩ ╩═╝ ╩═╝`,
}

// logoShine is the cycling style palette for the shine wave.
// A warm peak (Warn=yellow) surrounded by cyan wings fading to dim.
var logoShine = []screen.Style{
	screen.Dim, screen.Dim,
	screen.Normal,
	screen.Accent,
	screen.Focus,
	screen.Warn,
	screen.Focus,
	screen.Accent,
	screen.Normal,
	screen.Dim, screen.Dim,
}

func (o *Overview) renderLogo(s *screen.Screen, y, x, w int) {
	const logoW = 28
	cx := x + (w-logoW)/2
	if cx < x {
		cx = x
	}
	frame := int(time.Now().UnixMilli() / 200)
	n := len(logoShine)
	for row, line := range valhallLogo {
		col := 0
		for _, r := range line {
			si := ((frame - col) % n + n) % n
			s.Put(y+row, cx+col, string(r), logoShine[si], 1)
			col++
		}
	}
	tag := "system control panel"
	tagX := cx + (logoW-len([]rune(tag)))/2
	s.Put(y+3, tagX, tag, screen.Dim, w-(tagX-x))
}

func firstField(path string, prefix string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return "?"
	}
	for _, l := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(l, prefix) {
			return l
		}
	}
	return "?"
}

func meminfo() (totalKB, availKB int64) {
	data, _ := os.ReadFile("/proc/meminfo")
	for _, l := range strings.Split(string(data), "\n") {
		f := strings.Fields(l)
		if len(f) < 2 {
			continue
		}
		v, _ := strconv.ParseInt(f[1], 10, 64)
		switch f[0] {
		case "MemTotal:":
			totalKB = v
		case "MemAvailable:":
			availKB = v
		}
	}
	return
}

func uptimeStr() string {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return "?"
	}
	f := strings.Fields(string(data))
	secs, _ := strconv.ParseFloat(f[0], 64)
	d := int(secs) / 86400
	hh := int(secs) % 86400 / 3600
	mm := int(secs) % 3600 / 60
	if d > 0 {
		return fmt.Sprintf("%dd %dh %dm", d, hh, mm)
	}
	return fmt.Sprintf("%dh %dm", hh, mm)
}

func osRelease() string {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return "unknown"
	}
	for _, l := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(l, "PRETTY_NAME=") {
			return strings.Trim(strings.TrimPrefix(l, "PRETTY_NAME="), `"`)
		}
	}
	return "unknown"
}

func loadavg() (l1, l5, l15 float64) {
	data, _ := os.ReadFile("/proc/loadavg")
	f := strings.Fields(string(data))
	if len(f) >= 3 {
		l1, _ = strconv.ParseFloat(f[0], 64)
		l5, _ = strconv.ParseFloat(f[1], 64)
		l15, _ = strconv.ParseFloat(f[2], 64)
	}
	return
}

func mountpoints() map[string]bool {
	out := map[string]bool{}
	data, _ := os.ReadFile("/proc/self/mounts")
	for _, l := range strings.Split(string(data), "\n") {
		if f := strings.Fields(l); len(f) >= 2 {
			out[f[1]] = true
		}
	}
	return out
}

type row struct {
	label, value string
	st           screen.Style
}

func (o *Overview) Render(s *screen.Screen, y, x, h, w int, focused bool) {
	var uts syscall.Utsname
	_ = syscall.Uname(&uts)
	host, _ := os.Hostname()
	total, avail := meminfo()
	usedPct := int64(0)
	if total > 0 {
		usedPct = 100 * (total - avail) / total
	}
	l1, l5, l15 := loadavg()
	ncpu := numCPU()

	stateSt := screen.Err
	if o.sysstate == "running" {
		stateSt = screen.OK
	}
	loadSt := screen.Normal
	if l1 > float64(ncpu) {
		loadSt = screen.Warn
	}
	memSt := screen.Normal
	if usedPct > 85 {
		memSt = screen.Warn
	}

	rows := []row{
		{"host", host + "   " + osRelease(), screen.Normal},
		{"kernel", utsStr(uts.Sysname) + " " + utsStr(uts.Release) +
			" (" + utsStr(uts.Machine) + ")", screen.Normal},
		{"uptime", uptimeStr(), screen.Normal},
		{"state", o.sysstate, stateSt},
		{"load", fmt.Sprintf("%.2f %.2f %.2f  (%d cpu)", l1, l5, l15, ncpu), loadSt},
		{"memory", fmt.Sprintf("%d / %d MiB (%d%%)",
			(total-avail)/1024, total/1024, usedPct), memSt},
	}
	mp := mountpoints()
	for _, m := range []string{"/", "/var", "/home"} {
		if !mp[m] {
			continue
		}
		var st syscall.Statfs_t
		if syscall.Statfs(m, &st) != nil || st.Blocks == 0 {
			continue
		}
		tot := st.Blocks * uint64(st.Bsize)
		free := st.Bavail * uint64(st.Bsize)
		pct := 100 * (tot - free) / tot
		dst := screen.Normal
		if pct > 85 {
			dst = screen.Warn
		}
		rows = append(rows, row{"disk " + m,
			fmt.Sprintf("%d / %d GiB (%d%%)",
				(tot-free)>>30, tot>>30, pct), dst})
	}
	fst := screen.OK
	fval := "0"
	realFail := len(o.failed) > 0 && !strings.HasPrefix(o.failed[0], "(")
	if realFail {
		fst = screen.Err
		fval = strconv.Itoa(len(o.failed))
	} else if len(o.failed) > 0 {
		fval = o.failed[0]
		fst = screen.Dim
	}
	rows = append(rows, row{"failed units", fval, fst})

	logoH := 0
	if h > 7 {
		o.renderLogo(s, y, x, w)
		logoH = 5
	}
	line := y + logoH
	for _, r := range rows {
		s.Put(line, x, fmt.Sprintf("%14s", r.label), screen.Dim, 15)
		s.Put(line, x+16, r.value, r.st, w-16)
		line++
	}
	if realFail {
		line++
		s.Put(line, x, "failed:", screen.Err, w)
		for _, u := range o.failed {
			line++
			if line >= y+h {
				break
			}
			s.Put(line, x+2, u, screen.Err, w-2)
		}
	}
}

func (o *Overview) Footer() string { return "auto-refresh 5s" }

func numCPU() int {
	data, _ := os.ReadFile("/proc/cpuinfo")
	n := strings.Count(string(data), "\nprocessor")
	if strings.HasPrefix(string(data), "processor") {
		n++
	}
	if n == 0 {
		n = 1
	}
	return n
}

func utsStr(f [65]int8) string {
	b := make([]byte, 0, 65)
	for _, c := range f {
		if c == 0 {
			break
		}
		b = append(b, byte(c))
	}
	return string(b)
}
