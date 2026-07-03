// Package run: the only place a process gets spawned. Same rules as
// v0.1's execsafe, now with process-group SIGKILL on timeout so a
// timed-out script cannot leave grandchildren behind.
package run

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

var systemDirs = []string{"/usr/bin", "/usr/sbin", "/bin", "/sbin"}

// Env for children: constructed from scratch, never inherited.
var childEnv = []string{
	"PATH=/usr/bin:/usr/sbin:/bin:/sbin",
	"LANG=C.UTF-8",
	"LC_ALL=C.UTF-8",
	"TERM=dumb",
	"NO_COLOR=1",
	"SYSTEMD_COLORS=0",
	"SYSTEMD_PAGER=",
	"PAGER=cat",
	"HOME=" + os.Getenv("HOME"), // some tools (dnf) want it; value is ours anyway
}

const DefaultTimeout = 15 * time.Second

// Resolve maps a bare binary name to an absolute path inside
// systemDirs. Empty string means not installed — callers degrade.
func Resolve(bin string) string {
	if strings.HasPrefix(bin, "/") {
		if isExec(bin) {
			return bin
		}
		return ""
	}
	if strings.Contains(bin, "/") { // relative paths never accepted
		return ""
	}
	for _, d := range systemDirs {
		p := filepath.Join(d, bin)
		if isExec(p) {
			return p
		}
	}
	return ""
}

func isExec(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir() && fi.Mode()&0o111 != 0
}

type Result struct {
	Argv   []string
	Code   int
	Stdout string
	Stderr string
	Err    string // spawn/timeout-level failure, not exit status
}

func (r Result) OK() bool { return r.Err == "" && r.Code == 0 }

// Text renders a best-effort body for a text pane.
func (r Result) Text() string {
	if r.Err != "" {
		return "[stoker] " + r.Err + "\n"
	}
	body := r.Stdout
	if r.Code != 0 {
		body += fmt.Sprintf("\n[exit %d] %s\n%s",
			r.Code, strings.Join(r.Argv, " "), r.Stderr)
	}
	return body
}

// Do runs argv synchronously. Never returns an error for tool failure;
// everything lands in Result.
func Do(argv []string, timeout time.Duration) Result {
	if len(argv) == 0 {
		return Result{Argv: argv, Code: -1, Err: "empty command"}
	}
	path := Resolve(argv[0])
	if path == "" {
		return Result{Argv: argv, Code: -1,
			Err: fmt.Sprintf("'%s' not found in %s", argv[0], strings.Join(systemDirs, ":"))}
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, path, argv[1:]...)
	cmd.Env = childEnv
	cmd.Stdin = nil // /dev/null; a child must never read our tty
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { // kill the whole group, not just the leader
		if cmd.Process != nil {
			return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
		return nil
	}
	var so, se bytes.Buffer
	cmd.Stdout, cmd.Stderr = &so, &se
	err := cmd.Run()

	res := Result{Argv: argv, Stdout: so.String(), Stderr: se.String()}
	switch {
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		res.Code = -1
		res.Err = fmt.Sprintf("timeout after %s: %s", timeout, argv[0])
	case err != nil:
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			res.Code = ee.ExitCode()
		} else {
			res.Code = -1
			res.Err = "exec failed: " + err.Error()
		}
	}
	return res
}

type Event struct {
	Token string
	Res   Result
}

type job struct {
	token   string
	argv    []string
	timeout time.Duration
}

// Runner is the fixed worker pool. Modules submit; results appear on
// Events; the UI goroutine drains them. Same meeting-at-a-queue model
// as v0.1, with goroutines instead of threads.
type Runner struct {
	Events chan Event
	jobs   chan job
}

func NewRunner(workers int) *Runner {
	r := &Runner{
		Events: make(chan Event, 256),
		jobs:   make(chan job, 64),
	}
	for i := 0; i < workers; i++ {
		go func() {
			for j := range r.jobs {
				r.Events <- Event{j.token, Do(j.argv, j.timeout)}
			}
		}()
	}
	return r
}

// Submit never blocks the UI goroutine: a saturated queue reports
// backpressure as a result instead of freezing the loop.
func (r *Runner) Submit(token string, argv []string, timeout time.Duration) {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	select {
	case r.jobs <- job{token, argv, timeout}:
	default:
		r.Events <- Event{token, Result{Argv: argv, Code: -1,
			Err: "job queue full — too many pending commands"}}
	}
}
