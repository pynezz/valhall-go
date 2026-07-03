// valhall — terminal control panel for a RHEL/Fedora-family system.
//
// SSH-app mode: `valhall --attach` re-execs itself inside
// `tmux new-session -A -s valhall`, attaching to an existing session or
// creating one. Point a login shell (or an sshd Match/ForceCommand) at
// the stoker-shell wrapper and every SSH login lands in the same
// persistent TUI — surviving disconnects, sshd restarts mid-update,
// and impatient laptops closing lids.
package main

import (
	"flag"
	"fmt"
	"os"
	"runtime/debug"
	"strings"
	"syscall"

	"git.pynezz.dev/pynezz/stoker/internal/app"
	"git.pynezz.dev/pynezz/stoker/internal/config"
	"git.pynezz.dev/pynezz/stoker/internal/mod"
	"git.pynezz.dev/pynezz/stoker/internal/mods"
	"git.pynezz.dev/pynezz/stoker/internal/plugin"
	"git.pynezz.dev/pynezz/stoker/internal/run"
	"git.pynezz.dev/pynezz/stoker/internal/term"
	"git.pynezz.dev/pynezz/stoker/internal/tmuxx"
)

var version = "0.2.0-dev"

func main() {
	os.Exit(realMain())
}

func realMain() int {
	var (
		flagCheck   = flag.Bool("check", false, "load modules+plugins headlessly, exit 0/1 (CI gate)")
		flagPlugins = flag.Bool("plugins", false, "list plugin load results and exit")
		flagAttach  = flag.Bool("attach", false, "attach-or-create the persistent tmux session (SSH-app mode)")
		flagVersion = flag.Bool("version", false, "print version and exit")
	)
	flag.Parse()

	if *flagVersion {
		fmt.Println("valhall", version)
		return 0
	}

	// SSH-app mode: replace ourselves with tmux attach-or-create. If we
	// are already inside tmux, or tmux is missing, fall through to a
	// plain run — degraded, never broken.
	if *flagAttach && !tmuxx.Inside() {
		if tmux := run.Resolve("tmux"); tmux != "" {
			self, err := os.Executable()
			if err == nil {
				argv := []string{tmux, "new-session", "-A", "-s", "valhall", self}
				_ = syscall.Exec(tmux, argv, os.Environ())
				// Exec only returns on failure; fall through.
			}
		}
		fmt.Fprintln(os.Stderr, "valhall: tmux unavailable, running without session persistence")
	}

	cfg := config.Load()
	mods.RegisterBuiltins(cfg.ConfirmDestructive)
	mod.Register(mods.NewPackages())
	rep := plugin.LoadAll(cfg.PluginDirs, os.Geteuid())

	if *flagCheck || *flagPlugins {
		fmt.Println("plugin dirs:\n  " + strings.Join(cfg.PluginDirs, "\n  "))
		for _, l := range rep.Loaded {
			fmt.Println("loaded :", l)
		}
		for _, s := range rep.Skipped {
			fmt.Println("skipped:", s)
		}
		var titles []string
		for _, m := range mod.All() {
			titles = append(titles, m.Title())
		}
		fmt.Println("modules:", strings.Join(titles, ", "))
		if *flagCheck && len(rep.Skipped) > 0 {
			return 1
		}
		return 0
	}

	if err := term.EnterRaw(); err != nil {
		fmt.Fprintln(os.Stderr, "valhall: not a tty (use --check for headless mode):", err)
		return 2
	}
	// Terminal restore must survive every exit path, panics included —
	// a raw-mode tty with no echo is a hostile place to leave an operator.
	code := 0
	func() {
		defer func() {
			if r := recover(); r != nil {
				term.Restore()
				fmt.Fprintf(os.Stderr, "valhall panic: %v\n%s", r, debug.Stack())
				code = 1
				return
			}
			term.Restore()
		}()
		app.New(cfg, len(rep.Skipped)).Run()
	}()
	return code
}
