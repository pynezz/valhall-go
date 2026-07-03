#!/usr/bin/env python3
"""Screenshot pipeline for the work log.

Drives ./stoker (optionally under tmux) in a pty, feeds keystrokes,
emulates the terminal with pyte, and renders named frames to PNG with
a dark-terminal palette. Build-container tooling only — pyte and PIL
are not stoker dependencies.
"""
import fcntl, os, pty, select, struct, subprocess, sys, termios, time
import pyte
from PIL import Image, ImageDraw, ImageFont

COLS, ROWS = 110, 32
FONT = ImageFont.truetype(
    "/usr/share/fonts/truetype/dejavu/DejaVuSansMono.ttf", 15)
BOLD = ImageFont.truetype(
    "/usr/share/fonts/truetype/dejavu/DejaVuSansMono-Bold.ttf", 15)

PAL = {
    "default_bg": (13, 17, 23), "default_fg": (201, 209, 217),
    "black": (13, 17, 23), "red": (248, 81, 73), "green": (63, 185, 80),
    "brown": (210, 153, 34), "yellow": (210, 153, 34),
    "blue": (88, 166, 255), "magenta": (188, 140, 255),
    "cyan": (57, 197, 207), "white": (201, 209, 217),
    "brightblack": (110, 118, 129), "brightred": (255, 123, 114),
    "brightgreen": (86, 211, 100), "brightyellow": (227, 179, 65),
    "brightblue": (121, 192, 255), "brightmagenta": (210, 168, 255),
    "brightcyan": (86, 214, 223), "brightwhite": (240, 246, 252),
}

def color(name, fg=True, bold=False):
    if name in ("default", None):
        c = PAL["default_fg" if fg else "default_bg"]
    else:
        c = PAL.get(name)
        if c is None and len(name) == 6:  # hex from 256-color SGR
            try:
                c = tuple(int(name[i:i+2], 16) for i in (0, 2, 4))
            except ValueError:
                c = PAL["default_fg"]
        c = c or PAL["default_fg"]
    if bold and fg:
        c = tuple(min(255, int(v * 1.18) + 12) for v in c)
    return c

CW, CH = 9, 19  # cell size

def render(screen, path, title):
    pad, bar = 14, 34
    img = Image.new("RGB", (COLS*CW + 2*pad, ROWS*CH + 2*pad + bar),
                    (22, 27, 34))
    d = ImageDraw.Draw(img)
    # window chrome
    for i, c in enumerate([(248, 81, 73), (210, 153, 34), (63, 185, 80)]):
        d.ellipse([pad + i*22, 11, pad + 12 + i*22, 23], fill=c)
    d.text((pad + 76, 8), title, font=FONT, fill=(139, 148, 158))
    ox, oy = pad, bar + pad - 6
    d.rectangle([ox-6, oy-6, ox + COLS*CW + 6, oy + ROWS*CH + 6],
                fill=PAL["default_bg"])
    for row in range(ROWS):
        line = screen.buffer[row]
        for col in range(COLS):
            ch = line[col]
            fg = color(ch.fg, True, ch.bold)
            bg = color(ch.bg, False)
            if ch.reverse:
                fg, bg = bg if ch.bg != "default" else PAL["default_bg"], fg
                if ch.fg == "default":
                    bg = PAL["default_fg"]
                    fg = PAL["default_bg"]
            x, y = ox + col*CW, oy + row*CH
            if bg != PAL["default_bg"]:
                d.rectangle([x, y, x+CW, y+CH], fill=bg)
            if ch.data and ch.data != " ":
                d.text((x, y + 1), ch.data, font=BOLD if ch.bold else FONT,
                       fill=fg)
    img.save(path)
    print("wrote", path)

class Session:
    def __init__(self, argv, env=None):
        e = dict(os.environ, TERM="xterm-256color")
        e.pop("TMUX", None)
        if env:
            e.update(env)
        self.master, slave = pty.openpty()
        fcntl.ioctl(slave, termios.TIOCSWINSZ,
                    struct.pack("HHHH", ROWS, COLS, 0, 0))
        self.proc = subprocess.Popen(argv, stdin=slave, stdout=slave,
                                     stderr=subprocess.DEVNULL, env=e,
                                     cwd=os.path.dirname(__file__) + "/..")
        os.close(slave)
        self.screen = pyte.Screen(COLS, ROWS)
        self.stream = pyte.ByteStream(self.screen)

    def pump(self, t):
        end = time.time() + t
        while time.time() < end:
            r, _, _ = select.select([self.master], [], [], 0.05)
            if r:
                try:
                    self.stream.feed(os.read(self.master, 65536))
                except OSError:
                    return

    def key(self, b, wait=0.35):
        os.write(self.master, b)
        self.pump(wait)

    def shot(self, name, title):
        render(self.screen, f"shots/{name}.png", title)

    def close(self):
        try:
            self.proc.kill()
        except Exception:
            pass
        os.close(self.master)

os.makedirs("shots", exist_ok=True)
os.chdir(os.path.dirname(os.path.abspath(__file__)) + "/..")

# ---- plain session ----------------------------------------------------
s = Session(["./stoker"], env={"STOKER_PLUGIN_DIR": os.path.abspath("plugins")})
s.pump(1.2)
s.shot("01-overview", "stoker — Overview (degraded state, failed unit surfaced)")
s.key(b"2", 1.0)
s.shot("02-units", "stoker — Units (failed-first sort)")
s.key(b"/", 0.3); s.key(b"ss", 0.2); s.key(b"\r", 0.4)
s.shot("03-units-filter", "stoker — Units, live filter")
s.key(b"\x1b", 0.3)
s.key(b"\r", 0.9)
s.shot("04-unit-detail", "stoker — Unit detail (systemctl status)")
s.key(b"\x1b", 0.3)
s.key(b"3", 1.0)
s.shot("05-journal", "stoker — Journal (short-iso, priority filter)")
s.key(b"4", 1.0)
s.shot("06-storage", "stoker — Storage, block view")
s.key(b"5", 1.0); s.key(b"]", 0.8)
s.shot("07-network", "stoker — Network, routes view")
s.key(b"7", 1.2); s.key(b"]", 1.0)
s.shot("08-packages", "stoker — Packages, pending updates")
s.key(b"9", 1.5)
s.shot("09-plugin", "stoker — script plugin (AVC denials, v0.1 header format)")
s.key(b"?", 0.4)
s.shot("10-help", "stoker — help overlay")
s.close()

# ---- tmux session: F-split live journal, U detached update ------------
t = Session(["./stoker", "--attach"],
            env={"STOKER_PLUGIN_DIR": os.path.abspath("plugins")})
t.pump(1.5)
t.key(b"3", 1.0)
t.key(b"F", 2.8)  # split-window journalctl -f
t.shot("11-tmux-follow", "stoker inside tmux — F: live journalctl -f split")
t.key(b"\x1b", 0.2)
# focus back: the split stole tmux focus; send tmux prefix+up? Use tmux CLI.
subprocess.run(["tmux", "select-pane", "-t", "stoker", "-U"],
               stderr=subprocess.DEVNULL)
t.pump(0.5)
t.key(b"7", 1.2)
t.key(b"U", 0.5)
t.key(b"y", 1.5)  # confirm
t.shot("12-tmux-update", "stoker — U: dnf update in a detached tmux window")
subprocess.run(["tmux", "kill-server"], stderr=subprocess.DEVNULL)
t.close()
print("done")
