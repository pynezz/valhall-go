#!/usr/bin/env python3
"""Drive the Go stoker binary in a pty: keypress gauntlet across every
module, prompt entry, help overlay, detail view. Fails on nonzero exit,
traceback-equivalent (panic) on stderr, or unsent keys."""
import fcntl, os, pty, select, struct, subprocess, sys, termios, time

KEYS = [
    (0.7, b"\t"), (0.2, b"2"),                 # Units, content focus
    (0.6, b"j"), (0.1, b"j"), (0.1, b"k"),
    (0.2, b"f"), (0.5, b"f"), (0.5, b"f"), (0.4, b"f"), (0.4, b"f"),  # cycle scopes back to all
    (0.5, b"/"), (0.2, b"ssh"), (0.1, b"\r"),  # filter prompt
    (0.4, b"\x1b"),                            # clear filter
    (0.3, b"\r"),                              # open detail
    (0.6, b"j"), (0.1, b"\x1b"),               # scroll, back
    (0.3, b"3"), (0.6, b"p"), (0.4, b"b"), (0.4, b"b"),  # journal filters
    (0.3, b"F"),                               # tmux follow (msg: needs tmux)
    (0.3, b"4"), (0.5, b"]"), (0.4, b"]"), (0.3, b"["),  # storage tabs
    (0.3, b"5"), (0.5, b"]"),                  # network
    (0.3, b"7"), (0.6, b"U"),                  # packages: U without tmux -> msg
    (0.3, b"8"), (0.5, b"]"),                  # selinux
    (0.3, b"?"), (0.4, b" "),                  # help
    (0.3, b"1"), (0.5, b"Q"),                  # overview, quit
]

def main():
    env = dict(os.environ, TERM="xterm-256color")
    env.pop("TMUX", None)
    master, slave = pty.openpty()
    fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack("HHHH", 32, 110, 0, 0))
    proc = subprocess.Popen(["./stoker"], stdin=slave, stdout=slave,
                            stderr=subprocess.PIPE, env=env,
                            cwd=os.path.join(os.path.dirname(__file__), ".."))
    os.close(slave)
    out = bytearray(); keys = list(KEYS)
    nxt = time.time() + keys[0][0]; deadline = time.time() + 40
    try:
        while time.time() < deadline and proc.poll() is None:
            r, _, _ = select.select([master], [], [], 0.05)
            if r:
                try: out += os.read(master, 65536)
                except OSError: break
            if keys and time.time() >= nxt:
                _, k = keys.pop(0); os.write(master, k)
                if keys: nxt = time.time() + keys[0][0]
        proc.wait(timeout=5)
    except subprocess.TimeoutExpired:
        proc.kill(); print("FAIL: no exit after Q"); return 1
    finally:
        os.close(master)
    err = proc.stderr.read().decode(errors="replace")
    if proc.returncode != 0 or "panic" in err:
        print(f"FAIL rc={proc.returncode}\n{err}"); return 1
    if keys:
        print(f"FAIL: {len(keys)} keys unsent"); return 1
    print(f"OK rc=0, {len(out)} bytes of frames, no panic")
    return 0

if __name__ == "__main__":
    raise SystemExit(main())
