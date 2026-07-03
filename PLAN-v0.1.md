# stoker — planning & design document

*A terminal control panel for maintaining, developing, and debugging a
Fedora-based custom distribution. No GUI stack, no dependencies beyond
a base Fedora install.*

Working name: **stoker** — the one who tends the fire. It fits the
hearth ecosystem, it's short, and `dnf search stoker` comes up empty.
Alternatives considered: `bellows`, `poker` (taken by card games),
`tongs`. Rename is a `sed` away; nothing in the architecture cares.

---

## 1. Problem statement

Debugging a custom distro today means juggling `systemctl`, `journalctl`,
`lsblk`, `ip`, `ausearch`, `dnf history`, and a dozen ad-hoc one-liners
across shell history. Each is fine alone; the cost is *context
switching* and *recall* — remembering the exact incantation for
"AVC denials since boot, previous boot, for this unit" while the box is
misbehaving. The goal is a single screen where system state is visible
at a glance and every inspection surface is two keystrokes away, on a
machine that may have nothing but a serial console.

Non-goals, stated early because they shape everything:

* Not a replacement for the underlying tools. stoker orchestrates
  `systemctl`; it does not talk D-Bus and reimplement it. When stoker
  is broken, the tools still work; when the tools change, stoker
  mostly doesn't care.
* Not a remote fleet manager. One box, one terminal. Fleet concerns
  belong to hearth's control plane, not this tool.
* Not a general-purpose file manager or editor. `vim` exists.

## 2. Hard constraints and what they force

**Only what ships in base Fedora.** This is the decision that decides
everything else. Base Fedora (Server, Everything minimal) gives us:
`python3` + full stdlib including `curses`, bash, coreutils,
systemd tooling, `iproute2`, `rpm`/`dnf`, `util-linux`. It does not
give us: Go toolchain, Rust, pip packages, `ncurses` C headers, `tmux`,
or any TUI library (no textual, no urwid, no blessed).

**Language decision: Python 3 stdlib + curses.** This deserves honesty
because it goes against my read of your instincts — lazyquadlet is Go,
and Go is the better language for a tool like this in most respects
(static binary, real concurrency, type system that catches the bugs
curses code loves to hide). Two arguments won:

1. *The constraint is about the development loop, not just runtime.*
   A static Go binary has no runtime deps, true — but then the target
   box can run stoker and not modify it. The whole premise of the
   plugin system is that an engineer on the target machine promotes a
   shell one-liner into a UI screen in five minutes, with `vi` and
   nothing else. Python source is the artifact; there is no build step
   to be missing.
2. *Script plugins keep the escape hatch open.* If a screen ever needs
   real performance, it can be a compiled helper invoked by a script
   plugin. The TUI shell itself is I/O-bound; Python is nowhere near
   the bottleneck.

The counterargument worth keeping alive: if stoker grows past ~5 kloc
or needs event-driven journal streaming, revisit Go with a vendored
TUI lib, and keep the script-plugin protocol as the compatibility
boundary. The plugin header format is deliberately
implementation-neutral for exactly this reason.

**No X/Wayland, serial-console friendly.** Consequences: degrade to
monochrome (`theme.py` checks `has_colors()`), no mouse dependency
(mouse is a possible later nicety, never a requirement), box-drawing
kept to a single `│`, ASCII fallback trivial, and the layout must stay
usable at 80×24.

## 3. Architecture

```
stoker/
├── __main__.py        entry, arg parsing, --check headless mode
├── app.py             layout, event loop, key routing
├── core/
│   ├── execsafe.py    argv-only exec, allowlisted PATH, worker pool
│   ├── privsep.py     euid detection, sudo -n probe, action wrapping
│   └── config.py      layered INI (/etc → ~/.config)
├── ui/
│   ├── theme.py       semantic color slots, graceful mono fallback
│   └── widgets.py     ListPane, TextPane, modal, confirm, prompt
├── modules/
│   ├── __init__.py    Module base, CommandModule, registry
│   ├── overview.py    /proc-direct dashboard + async systemd state
│   ├── units.py       interactive unit list + status + verbs
│   ├── journal.py     tail/follow/filter viewer
│   └── builtin_views.py  storage, network, packages, selinux, procs
└── plugins.py         loader: python + script plugins, trust checks
```

### Threading model

One rule: **the curses thread owns the terminal; workers own
subprocesses; they meet only at a queue.** Modules call
`app.submit(token, argv)`; a fixed pool of three workers runs commands
with mandatory timeouts; results arrive as `(token, Result)` on a
`queue.Queue` drained once per UI tick (80 ms `getch` timeout doubles
as the frame clock). Stale results — a module switched away before its
command finished — are dropped by token routing. There is no locking
anywhere in module code because modules only mutate their own state on
the UI thread.

This is the piece I'd defend hardest in review. Every TUI that blocks
on `subprocess.run` in its key handler feels fine in the demo and
freezes the first time `dnf check-update` takes 40 seconds on a
congested mirror. The `--check` flag exists so CI can verify the whole
module tree loads without a terminal.

### Rendering model

Immediate mode: every frame, `erase()` and redraw everything from
module state. No retained widget tree, no damage tracking, no dirty
flags. At terminal sizes this costs nothing (curses itself diffs
against the previous screen), and it makes resize handling a non-event:
`KEY_RESIZE` just falls through to the next frame. Widget "objects"
(`ListPane`, `TextPane`) are state holders with a `draw(scr, y, x, h, w)`
method — the geometry is decided by the caller every frame, so panes
never hold stale dimensions.

Two hygiene rules baked into `widgets.py`:

* `put()` clips instead of letting `addstr` raise at screen edges
  (the classic bottom-right-cell crash).
* `clean()` strips ANSI escapes and control characters from **all**
  displayed text. Tool output and plugin output are untrusted; a log
  line must never be able to inject escape sequences into the
  operator's terminal. This is a real attack surface on a debugging
  tool — attacker-influenced strings end up in journals constantly.

### Module contract

A module is one screen. Built-ins and plugins implement the identical
interface — there is deliberately no privileged internal API, so the
plugin system can't rot into a second-class citizen:

```python
class Module:
    name, title, order, interval
    def activate(app)                 # entered; usually refresh()
    def refresh(app)                  # submit async work
    def tick(app)                     # auto-refresh clock
    def on_result(app, token, res)    # worker results routed back
    def handle_key(app, ch, h, w)     # True if consumed
    def render(scr, y, x, h, w, focused)
    def footer() -> str               # context key hints
```

`CommandModule` is the workhorse: a list of `(name, argv)` views cycled
with `[`/`]`. Storage, network, packages, SELinux, and processes are
each a single declarative `register(CommandModule(...))` call in
`builtin_views.py`. That file is the proof of the design goal: **a new
inspection screen costs five lines.** If adding a screen ever requires
touching `app.py`, the design has failed.

## 4. Security model

Threat model: stoker frequently runs as root on a box you care about,
displays attacker-influenced text (logs), and executes pluggable code.
Priorities in order:

1. **No shell, ever.** `execsafe.run` takes argv lists,
   `shell=False` explicit. Binaries resolve against a fixed
   `/usr/bin:/usr/sbin:/bin:/sbin` — never the caller's `$PATH`, never
   CWD, never relative paths. Child env is constructed from scratch
   (`LANG=C.UTF-8`, `NO_COLOR`, `SYSTEMD_PAGER=`), not inherited.
2. **Privilege is per-action, not per-process.** stoker never elevates
   itself. Read paths work unprivileged where the tools allow
   (journal via adm/wheel group, `systemctl status`, `ip`, `rpm -qa`).
   Mutating verbs go through `Priv.wrap()`: as root, run directly;
   otherwise `sudo -n` (probed once, cached) or refuse with an
   actionable message. `-n` matters: a hidden interactive password
   prompt under curses is a hang, so we forbid it structurally.
3. **Destructive actions confirm.** stop/restart get a modal y/N
   (config-disableable for the impatient). Start/reload don't — the
   confirmation must stay meaningful, and confirming everything trains
   the operator to mash `y`.
4. **Plugins are code; the control is *who can write the directory*.**
   Plugin dirs and files must be owned by root or the invoking user
   and not group/world-writable, or they're skipped and reported.
   `~/.config/stoker/plugins` is opt-in (`allow_user_plugins`), so
   root-run stoker doesn't execute user-writable files by default.
   No signing, no sandboxing — a Python plugin runs in-process and a
   sandbox here would be theater. The honest statement is "a plugin is
   root code; treat the plugin dir like /etc/systemd/system", and the
   permission gate enforces the minimum that statement implies.
5. **Timeouts on everything.** No subprocess may outlive its budget;
   follow-mode is periodic re-poll rather than a held pipe partly for
   this reason (bounded, short-lived children only — no orphan
   `journalctl -f` after a crash).
6. **Output sanitisation** as described above — applies equally to
   plugin output.

Known accepted risks, written down so they're decisions rather than
oversights: `sudo -n` on a broad rule is effectively root (mitigate
with per-command sudoers rules if it matters on a given box);
`rpm -Va` in the packages module is slow and noisy by nature; a
malicious *trusted* plugin is game over by definition.

## 5. Module inventory (v0.1 — implemented)

| # | Module    | Source of truth | Notes |
|---|-----------|-----------------|-------|
| 1 | Overview  | /proc, os.*, systemctl | Instant render (no subprocess for local facts), failed units float up, 5 s auto-refresh |
| 2 | Units     | systemctl list-units/status | Failed-first sort, scope cycling (all/failed/service/timer/socket), filter, start/stop/restart/reload with privsep + confirm |
| 3 | Journal   | journalctl | Priority cycling, current/previous boot, `-u` and `-g` filters, on-screen search, follow via 2 s re-poll |
| 4 | Storage   | lsblk, findmnt, btrfs | btrfs/LUKS-aware views; absent tools degrade to a message, not a crash |
| 5 | Network   | ip, ss, resolvectl | 10 s auto-refresh |
| 6 | Processes | ps, systemd-cgls, PSI | pressure-stall view is the underrated one for "why is it slow" |
| 7 | Packages  | rpm, dnf | 120 s timeout budget; check-update and rpm -Va are honest about being slow |
| 8 | SELinux   | sestatus, ausearch, semodule | denials view pairs with the script-plugin example |

Deliberately absent for now: firewalld (nftables view is a fast-follow),
bootc/ostree status (belongs in v0.2 once hearth's update flow settles),
users/logins (`loginctl` view, cheap to add when needed).

## 6. Plugin system

Two tiers, chosen so the on-ramp is a gradient rather than a cliff.

**Script plugins** — any executable with a metadata header:

```bash
#!/usr/bin/env bash
# stoker-plugin: AVC denials (24h)
# stoker-order: 81
# stoker-timeout: 30
# stoker-root: yes
ausearch -m AVC,USER_AVC -ts today
```

Drop it in `/etc/stoker/plugins`, `chmod 755`, done: it's a screen.
Runs through the same execsafe path (clean env, timeout, sanitised
output), reruns on `r`, requests privilege declaratively via
`stoker-root` rather than embedding sudo. This is where 90 % of
plugins should live, because a screen that's "run this diagnostic,
show me the output, let me search it" covers most real debugging.

**Python plugins** — a `.py` file defining `register(api)`:

```python
def register(api):
    if api.resolve("podman") is None:
        return
    api.add_module(api.CommandModule("podman", "Containers", views=[...]))
```

Full Module subclassing available for genuinely interactive screens.
Loaded under a namespaced module name (`stoker_plugin_*`) so a plugin
filename can't shadow stdlib or stoker internals. A plugin that throws
during load is skipped and reported — one broken plugin must never
take down the recovery tool, since the moments you need stoker most
are the moments something is already broken.

Interface stability promise (to my future self): the script header
format and the `PluginAPI` names (`add_module`, `CommandModule`,
`Module`, `resolve`, `run`) are the public contract. Everything else
may change.

Future tier, explicitly deferred: an *interactive* script protocol
(script emits JSON menu, stoker renders it, selection re-invokes with
args). Sketched, not built — I want evidence real plugins need it
before committing to a wire format that then has to live forever.

## 7. UX rules

Navigation should be muscle memory within an hour: number keys jump
straight to modules, `Tab` toggles nav/content focus, vim keys and
arrows both work everywhere, `Esc` always means "back/out", `?` shows
global keys and the status bar shows the *current module's* keys at all
times. The status bar is the contract — a keybinding that isn't
discoverable there doesn't exist.

Problems float upward: failed units sort first and render red, the
overview leads with `is-system-running`, thresholds (load > ncpu,
mem/disk > 85 %) turn lines yellow. The operator should never have to
scroll to find out the box is unhappy.

Feedback beats spinners: pending job count shows in the status bar,
long views say "running…", and action results ("restart: ok" /
"restart failed: <stderr>") land in the module's message line rather
than a modal that interrupts flow.

### Global keys

| Key | Action |
|-----|--------|
| `1–9` | jump to module |
| `Tab` | toggle nav / content focus |
| `j/k`, arrows, PgUp/PgDn, `g/G` | move |
| `Enter` | focus content / open item |
| `R`, `F5` | refresh active module |
| `/` | filter or search (module-defined) |
| `Esc` | back / clear filter / to nav |
| `?` | help overlay |
| `q` (nav) / `Q` (anywhere) | quit |

## 8. Testing & code health

What exists now: `python3 -m stoker --check` loads the entire module
and plugin tree headlessly and exits nonzero on any skipped plugin —
this is the CI gate. `tests/pty_smoke.py` spawns the real TUI in a
pty, drives a keypress gauntlet across every module (navigation, scope
cycling, detail open/close, help overlay, quit), and fails on nonzero
exit or any traceback on stderr. Both pass in a container that isn't
even systemd-booted, which doubles as the graceful-degradation test.

Planned next: unit tests for the pure parts (unit-list parsing, script
header parsing, `_trusted()` permission logic, `clean()` against an
ANSI-injection corpus), frame-snapshot tests via a terminal emulator
for layout regressions, and running under `python3 -X dev -W error`
in CI. Style gate: `python3 -m compileall` plus whatever linting the
target box carries — pyflakes ships in Fedora repos but not base, so
lint runs on the dev machine, not the target.

Manual test matrix worth keeping: 80×24 serial console, monochrome
TERM, non-root user without sudo, non-root with NOPASSWD sudo, mid-
command module switching, and resize-during-modal.

## 9. Packaging & deployment

Target form: a single RPM (`stoker.spec`), `noarch`, requiring only
`python3`. Files land in `/usr/lib/python3.x/site-packages/stoker/` (or
zipapp — see below), `/usr/bin/stoker` wrapper, `/usr/share/stoker/plugins`
for vendored plugins, `/etc/stoker/` for config and local plugins.
Fits the hearth bootc image as a layered package; the spec gets the
`findempty.awk` + shellcheck treatment like everything else.

Attractive alternative for the interim: `zipapp` — the whole tree as a
single `stoker.pyz`, runnable anywhere with python3, scp-able to a
sick machine. Costs nothing to support both.

## 10. Roadmap

**v0.1 (done, this tree):** core loop, widgets, privsep, 8 built-in
modules, both plugin tiers, trust checks, headless check, pty smoke
test.

**v0.2:** firewalld/nftables module; `loginctl` sessions;
journal→units cross-jump (open journal pre-filtered to selected unit —
the single highest-value UX addition, wiring exists via module switch +
state set); bootc/ostree status once hearth's update flow is settled;
RPM spec + zipapp build script; unit tests for parsers.

**v0.3:** command palette (`:` fuzzy action search across modules);
interactive script-plugin protocol *if* plugin experience demands it;
mouse support (click nav, wheel scroll) as pure additive; optional
persistent journal streaming behind the same TextPane if 2 s polling
ever chafes.

**Explicitly parked:** remote/fleet anything, D-Bus native bindings,
config editing UIs (editing units belongs in `$EDITOR` — a v0.3
candidate is "open selected unit's override in vim via
`systemctl edit`", suspending curses around the editor).

## 11. Open questions (for future sessions)

* Should Units grow a `d`/`D` for daemon-reload and enable/disable?
  Leaning yes for reload, hesitant on enable/disable — persistent
  state changes feel like they deserve the extra friction of typing
  the command.
* Journal follow at 2 s poll: good enough, or does live tailing during
  an incident justify the pipe-lifecycle complexity? Decide from use,
  not speculation.
* Is `allow_user_plugins` a footgun even as opt-in? Alternative:
  require user plugin dir to be listed explicitly in the *system*
  config, so root has signed off on the location.
* Name collision check before publishing: `stoker` on PyPI/copr.
