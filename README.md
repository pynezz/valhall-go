# stoker (v0.2 — Go)

Terminal control panel for a RHEL 8.10-based system. One static binary,
pure Go stdlib (raw termios + ANSI, no ncurses, no external modules).

    go build ./cmd/stoker          # go-toolset >= 1.22
    ./stoker                       # run the TUI
    ./stoker --attach              # SSH-app mode: tmux attach-or-create
    ./stoker --check               # headless load check (CI gate)
    go test ./...                  # parser / trust / quoting tests
    python3 tests/pty_smoke.py     # full keypress gauntlet in a pty

SSH-app install: see contrib/stoker-shell and contrib/sshd_config.d-stoker.conf.
Plugins: executable scripts with a `# stoker-plugin:` header in
/etc/stoker/plugins (root-owned, not group/world-writable) — format
unchanged from v0.1; Python-era plugins load as-is.

Design history and decision records: WORKLOG.html, plus v0.1's PLAN.md.
