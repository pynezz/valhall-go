#!/usr/bin/env python3
"""Generate WORKLOG.html with screenshots embedded as base64."""
import base64
import os
import sys

os.chdir(os.path.dirname(os.path.abspath(__file__)) + "/..")

def img(name, caption):
    with open(f"shots/{name}.png", "rb") as f:
        b64 = base64.b64encode(f.read()).decode()
    return (f'<figure><img src="data:image/png;base64,{b64}" '
            f'alt="{caption}" loading="lazy">'
            f'<figcaption>{caption}</figcaption></figure>')

ASCII = r"""███████╗████████╗ ██████╗ ██╗  ██╗███████╗██████╗
██╔════╝╚══██╔══╝██╔═══██╗██║ ██╔╝██╔════╝██╔══██╗
███████╗   ██║   ██║   ██║█████╔╝ █████╗  ██████╔╝
╚════██║   ██║   ██║   ██║██╔═██╗ ██╔══╝  ██╔══██╗
███████║   ██║   ╚██████╔╝██║  ██╗███████╗██║  ██║
╚══════╝   ╚═╝    ╚═════╝ ╚═╝  ╚═╝╚══════╝╚═╝  ╚═╝"""

HTML = f"""<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>stoker v0.2 — work log</title>
<style>
:root {{
  --bg: #171412; --panel: #201b16; --rule: #382f26;
  --amber: #ffb000; --amber-dim: #b87f0a;
  --text: #d8d1c4; --dim: #968b7c;
  --ok: #4cbf5e; --warn: #e5534b;
  --mono: ui-monospace, "Cascadia Mono", "JetBrains Mono", Menlo, Consolas, monospace;
  --sans: system-ui, "Segoe UI", Roboto, sans-serif;
}}
* {{ box-sizing: border-box; }}
body {{
  margin: 0; background: var(--bg); color: var(--text);
  font: 16px/1.65 var(--sans);
}}
main {{ max-width: 920px; margin: 0 auto; padding: 48px 24px 96px; }}
header pre {{
  color: var(--amber); font: 700 min(1.55vw, 13px)/1.15 var(--mono);
  text-shadow: 0 0 18px rgba(255,176,0,.35); margin: 0 0 10px; overflow-x: auto;
}}
.meta {{ font: 13px var(--mono); color: var(--dim); letter-spacing: .06em; }}
.thesis {{
  border-left: 3px solid var(--amber); padding: 4px 0 4px 18px;
  margin: 28px 0 8px; font-size: 18px; color: var(--text);
}}
h2 {{
  font: 700 13px var(--mono); color: var(--amber);
  letter-spacing: .18em; text-transform: uppercase;
  border-bottom: 1px solid var(--rule); padding-bottom: 8px; margin: 56px 0 18px;
}}
h2 .n {{ color: var(--dim); margin-right: 10px; }}
p {{ margin: 0 0 14px; }}
code, .k {{
  font: 13.5px var(--mono); background: var(--panel);
  padding: 1px 6px; border-radius: 3px; color: var(--amber);
}}
pre.block {{
  background: var(--panel); border: 1px solid var(--rule); border-radius: 6px;
  padding: 14px 16px; overflow-x: auto; font: 13px/1.5 var(--mono);
  color: var(--text); margin: 14px 0 20px;
}}
/* signature element: boot-log timeline */
.boot {{ font: 14px/1.9 var(--mono); margin: 18px 0 26px; }}
.boot .row {{ display: flex; gap: 14px; align-items: baseline; }}
.boot .tag {{ flex: 0 0 74px; text-align: center; }}
.boot .ok .tag {{ color: var(--ok); }}
.boot .warn .tag {{ color: var(--warn); }}
.boot .note .tag {{ color: var(--amber); }}
.boot .msg {{ color: var(--text); }}
.boot .msg small {{ color: var(--dim); display: block; font-size: 12.5px; }}
/* decision records */
.dr {{
  background: var(--panel); border: 1px solid var(--rule);
  border-left: 3px solid var(--amber); border-radius: 0 6px 6px 0;
  padding: 16px 20px; margin: 0 0 18px;
}}
.dr b.id {{ font: 700 12.5px var(--mono); color: var(--amber); letter-spacing: .1em; }}
.dr h3 {{ margin: 4px 0 8px; font: 600 17px var(--sans); color: var(--text); }}
.dr p {{ font-size: 15px; }}
.dr p:last-child {{ margin-bottom: 0; }}
.dr .against {{ color: var(--dim); font-size: 14px; }}
.dr .against b {{ color: var(--warn); font-weight: 600; }}
figure {{ margin: 22px 0 30px; }}
figure img {{
  width: 100%; border: 1px solid var(--rule); border-radius: 8px; display: block;
}}
figcaption {{ font: 12.5px var(--mono); color: var(--dim); padding-top: 8px; }}
table {{ border-collapse: collapse; width: 100%; font-size: 14.5px; margin: 12px 0 20px; }}
th {{ font: 700 12px var(--mono); color: var(--dim); text-transform: uppercase;
     letter-spacing: .1em; text-align: left; }}
th, td {{ border-bottom: 1px solid var(--rule); padding: 8px 12px 8px 0; vertical-align: top; }}
td code {{ white-space: nowrap; }}
.foot {{ margin-top: 64px; border-top: 1px solid var(--rule); padding-top: 18px;
         font: 12.5px var(--mono); color: var(--dim); }}
a {{ color: var(--amber); }}
</style>
</head>
<body>
<main>

<header>
<pre>{ASCII}</pre>
<div class="meta">WORK LOG · v0.1 → v0.2 · PYTHON → GO · TARGET RHEL 8.10 · 2026-07-02</div>
<p class="thesis">One static binary, one persistent session: the control panel now
survives everything short of the tmux server — including the updates it starts.</p>
</header>

<h2><span class="n">§0</span> What changed this cycle</h2>
<p>Three inputs landed since the v0.1 planning document: the target moved from a
Fedora base to a <b>RHEL 8.10</b> derivative, the <b>go-toolset</b> module and
<b>EPEL</b> are available on it, and stoker should behave as an <b>SSH-app</b> —
log in over SSH, land in the TUI, and don't lose state when the connection or
sshd itself blinks during maintenance. The LAN is fast and reliable, so latency
tricks (mosh-style prediction) are out of scope; session <em>persistence</em> is
the actual requirement. Everything below follows from those three inputs.</p>
<p>Toolchain fact check before committing: Red Hat's Go Toolset status page
confirms Go 1.25.3 shipped for RHEL 8.10 in December 2025, so
<code>go.mod</code> pins a conservative <code>go 1.22</code> floor and the
target's compiler clears it comfortably. RHEL 8 ships tmux <b>2.7</b> — and EPEL
never replaces base packages, so 2.7 it stays: no <code>display-popup</code>
(3.2+), splits and windows only. The integration is written to that floor.</p>

<h2><span class="n">§1</span> Decision records</h2>

<div class="dr"><b class="id">DR-01</b>
<h3>Port the shell to Go; retire the Python implementation to reference status</h3>
<p>v0.1 chose Python because the toolchain <em>was</em> the base OS. RHEL 8
inverts that argument twice: platform Python is <b>3.6</b>, which the v0.1 code
does not run on (deferred annotations are 3.7+), while go-toolset is one
<code>yum module install</code> away and produces a <b>2.2&nbsp;MB static
binary</b> with zero runtime dependencies — it runs on the sickest possible box,
which is exactly when a recovery tool earns its keep. The build stays pure
stdlib: raw termios via ioctl, hand-rolled ANSI renderer, no ncurses, no
tcell/bubbletea, nothing vendored.</p>
<p class="against"><b>Held against it:</b> ~1,900 lines rewritten, and losing
in-process Python plugins (see DR-04). The script-plugin header format is the
frozen boundary that made the rewrite safe — v0.1 plugins load unchanged.</p>
</div>

<div class="dr"><b class="id">DR-02</b>
<h3>EPEL: detected, never depended on</h3>
<p>EPEL widens what <em>can</em> be on the box; it must not narrow where stoker
works. The static binary already removed runtime dependencies, so EPEL's role is
optional richness: <code>run.Resolve()</code> probes fixed system dirs and every
view degrades per-tool, so a box with htop/iotop/ncdu gets richer plugin screens
and a minimal one loses nothing. Packaging-wise EPEL (or a COPR-style repo) is a
sensible distribution channel for the stoker RPM itself.</p>
<p class="against"><b>Held against it:</b> nothing — this is the discipline that
keeps the tool trustworthy on broken systems, which is the founding premise.</p>
</div>

<div class="dr"><b class="id">DR-03</b>
<h3>SSH-app = login wrapper + tmux attach-or-create; mutations detach</h3>
<p>The mechanism is <code>tmux new-session -A -s stoker</code>: attach if the
session exists, create it if not. <code>stoker --attach</code> self-execs
through tmux so the pattern is one flag; <code>contrib/stoker-shell</code> makes
it a login shell, and <code>contrib/sshd_config.d-stoker.conf</code> shows the
ForceCommand variant for a group. This is what makes "updates that would impair
its function through SSH" safe: the session — and any transaction in it —
belongs to the tmux server, not to sshd. Disconnect, sshd restart mid-update,
closed lid: reattach into the same state. The Packages module's <code>U</code>
verb goes one step further and runs <code>dnf -y update</code> in a
<em>detached</em> window, so the transaction survives stoker too. tmux also
absorbed live streaming: <code>F</code> in the Journal splits a real
<code>journalctl -f</code> below stoker, with pipe lifecycle owned by tmux
instead of the TUI (v0.1 open question, now closed).</p>
<p class="against"><b>Held against it:</b> this is convenience, not confinement —
<code>!</code> still opens a shell. A restricted ops account is a different
design: ForceCommand plus a build with the escape removed. Flagged, not built.</p>
</div>

<div class="dr"><b class="id">DR-04</b>
<h3>One plugin tier: executable scripts with the v0.1 header</h3>
<p>The Go port deliberately drops in-process Python plugins. The script tier
carried 90% of the value at 10% of the surface, and the TUI process now contains
no foreign code — the trust model reduces cleanly to "who can write the plugin
directory" (root/user-owned, no group/world write, enforced and reported).
Anything needing real logic can be a compiled helper a script invokes. The
header (<code># stoker-plugin:</code>, <code>-order</code>,
<code>-timeout</code>, <code>-root</code>) is implementation-neutral and now
proven across two implementations.</p>
</div>

<h2><span class="n">§2</span> Build log</h2>
<div class="boot">
<div class="row ok"><span class="tag">[  OK  ]</span><div class="msg">Verified toolchain floor
<small>Go 1.25.3 on RHEL 8.10 via go-toolset (Red Hat status page, 2025-12-05) · go.mod pins 1.22</small></div></div>
<div class="row ok"><span class="tag">[  OK  ]</span><div class="msg">internal/term — raw mode, keys
<small>termios via ioctl (TCGETS/TCSETS/TIOCGWINSZ), alt screen, SIGWINCH, incremental CSI/SS3 parser with 25 ms lone-Esc window, Suspend() hands the tty to a child and pauses the reader goroutine</small></div></div>
<div class="row ok"><span class="tag">[  OK  ]</span><div class="msg">internal/screen — renderer
<small>cell buffer, semantic styles with monochrome fallback, ANSI/control sanitisation of all displayed text; frames trim trailing blanks per row + EL — a serial console at 115200 baud gets hundreds of bytes per frame, not W×H</small></div></div>
<div class="row ok"><span class="tag">[  OK  ]</span><div class="msg">internal/run — safe exec
<small>argv-only, fixed-dir resolution, scrubbed env, mandatory timeouts; new vs v0.1: Setpgid + process-group SIGKILL on timeout, so a timed-out script can't leave grandchildren</small></div></div>
<div class="row ok"><span class="tag">[  OK  ]</span><div class="msg">Modules ported: Overview · Units · Journal · Storage · Network · Processes · Packages · SELinux
<small>same Host/Module contract; CmdModule keeps the five-line cost per new screen; RHEL 8 adjustments: nmcli view instead of resolvectl, vmstat beside PSI (4.18 kernel usually lacks /proc/pressure)</small></div></div>
<div class="row ok"><span class="tag">[  OK  ]</span><div class="msg">tmux integration (2.7 floor)
<small>split-window and new-window only; every argument reject-then-quoted against an allowlist regex before touching tmux's shell-command string — unit names are untrusted input</small></div></div>
<div class="row ok"><span class="tag">[  OK  ]</span><div class="msg">SSH-app mode
<small>--attach self-exec through tmux new -A; stoker-shell wrapper + sshd Match/ForceCommand snippet in contrib/</small></div></div>
<div class="row note"><span class="tag">[ NOTE ]</span><div class="msg">Screenshots below come from a mock harness
<small>the build container isn't systemd-booted, so systemctl/journalctl/dnf/… are shims emitting RHEL 8.10-flavoured output (tests/mock-setup.sh); layout, colours, and behaviour are the real binary</small></div></div>
</div>

{img("01-overview", "01 · Overview — /proc-direct facts render instantly; systemd state arrives async. Degraded state and the failed unit are red and above the fold.")}
{img("02-units", "02 · Units — failed-first sort; active green, failed red; verbs on the status bar.")}
{img("03-units-filter", "03 · Units — live substring filter (/), applied against the full unit list.")}
{img("04-unit-detail", "04 · Unit detail — systemctl status inline; Esc back, j/k scroll.")}
{img("05-journal", "05 · Journal — short-iso tail with priority/boot/unit/grep filters; f re-polls, F splits (next shot).")}
{img("06-storage", "06 · Storage — lsblk tree; [ and ] cycle mounts/btrfs/luks/diskstats views.")}
{img("07-network", "07 · Network — ip/ss/nmcli views; resolv.conf view replaces resolvectl on RHEL 8.")}
{img("08-packages", "08 · Packages — pending updates via dnf check-update (120 s budget, honest about being slow).")}
{img("09-plugin", "09 · Script plugin — the v0.1 selinux-denials.sh, byte-identical header, loaded by the Go binary.")}
{img("10-help", "10 · Help overlay — the status bar is the contract; ? is the map.")}
{img("11-tmux-follow", "11 · Inside tmux: F in the Journal splits a live journalctl -f under stoker. Pipe lifecycle belongs to tmux.")}
{img("12-tmux-update", "12 · Packages U — dnf update running in the detached 'dnf-update' window (tmux status bar); survives SSH drop, sshd restart, and stoker itself.")}

<h2><span class="n">§3</span> Test evidence</h2>
<pre class="block">$ go vet ./...                                # clean
$ go test ./...                               # key parser · plugin trust/header · tmux quoting
ok  internal/plugin   ok  internal/term   ok  internal/tmuxx
$ ./stoker --check                            # headless module+plugin load (CI gate)   exit 0
$ python3 tests/pty_smoke.py                  # 30-key gauntlet across every module
OK rc=0, 389267 bytes of frames, no panic
$ CGO_ENABLED=0 go build -trimpath -ldflags "-s -w"
$ ldd stoker
        not a dynamic executable              # 2.2 MB, runs on a base install</pre>
<p>The tmux-quoting test is the one to keep honest: it is the security boundary
between untrusted unit names and tmux's shell-command argument
(reject-then-quote; <code>$(reboot)</code>, backticks, quotes, pipes all
refused; escaped systemd names like <code>dev-disk-by\\x2duuid.device</code>
accepted).</p>

<h2><span class="n">§4</span> Carried over vs. changed</h2>
<table>
<tr><th>Area</th><th>v0.1 (Python)</th><th>v0.2 (Go)</th></tr>
<tr><td>Concurrency</td><td>3 worker threads + queue</td><td>3 goroutines + channels; select loop; results drained per frame</td></tr>
<tr><td>Rendering</td><td>curses (library diffs)</td><td>own ANSI frames; trailing-blank trim + EL for serial links</td></tr>
<tr><td>Timeout kill</td><td>process only</td><td><b>process group</b> (Setpgid + SIGKILL to -pgid)</td></tr>
<tr><td>Live journal</td><td>2 s re-poll (open question)</td><td>re-poll kept for in-pane; <b>F → tmux split</b> for real streaming</td></tr>
<tr><td>Plugins</td><td>scripts + Python modules</td><td>scripts only; header frozen; helpers may be compiled</td></tr>
<tr><td>Privilege</td><td colspan="2">unchanged: per-action <code>sudo -n</code>, probe cached, never a hidden password prompt; destructive verbs confirm</td></tr>
<tr><td>Deploy</td><td>source tree + python3</td><td>one static binary; <code>--attach</code> SSH-app mode; contrib shell/sshd snippets</td></tr>
</table>

<h2><span class="n">§5</span> Next</h2>
<p><b>v0.3 candidates:</b> RPM spec (noarch is gone — arch-specific now) and a
tiny COPR-style repo, journal→units cross-jump, firewalld/nftables module,
<code>systemctl edit</code> via Suspend + $EDITOR, and a <code>--restricted</code>
build tag that removes the shell escape for ForceCommand accounts.
<b>Open questions carried:</b> enable/disable verbs in Units (still leaning
"type the command" for persistent state changes); whether the detached-update
pattern should grow a completion notification back into stoker (tmux
<code>wait-for</code> exists in 2.7 — plumbing is there if wanted).</p>

<div class="foot">stoker v0.2.0 · git.pynezz.dev/pynezz/stoker · reproduce:
tar xzf stoker-go-0.2.tar.gz && cd stoker-go && go build ./cmd/stoker &&
./stoker --attach · plugins: /etc/stoker/plugins (root-owned, 0755)</div>

</main>
</body>
</html>
"""

with open("WORKLOG.html", "w") as f:
    f.write(HTML)
print("WORKLOG.html", os.path.getsize("WORKLOG.html") // 1024, "KiB")
