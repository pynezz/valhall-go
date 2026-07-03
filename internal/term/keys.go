package term

import (
	"syscall"
	"time"
	"unicode/utf8"
)

type Code int

const (
	KRune Code = iota
	KEnter
	KTab
	KEsc
	KBackspace
	KUp
	KDown
	KLeft
	KRight
	KPgUp
	KPgDn
	KHome
	KEnd
	KF5
	KCtrlC
)

type Key struct {
	Code Code
	R    rune
}

// ReadKeys starts the single input goroutine. Escape-sequence
// disambiguation (lone Esc vs. arrow-key prefix) uses a 25 ms grace
// window instead of read deadlines, because stdin is a shared
// nonblocking fd that Suspend() temporarily hands to child processes.
func ReadKeys() <-chan Key {
	ch := make(chan Key, 64)
	go reader(ch)
	return ch
}

func reader(ch chan<- Key) {
	buf := make([]byte, 0, 128)
	tmp := make([]byte, 128)
	var escSince time.Time
	for {
		if paused.Load() { // a child owns the tty; don't steal its input
			time.Sleep(50 * time.Millisecond)
			continue
		}
		n, err := syscall.Read(0, tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		} else if err != nil && err != syscall.EAGAIN && err != syscall.EINTR {
			time.Sleep(50 * time.Millisecond)
			continue
		}
		for len(buf) > 0 {
			k, adv, needMore := parse(buf)
			if needMore {
				if buf[0] == 0x1b {
					if escSince.IsZero() {
						escSince = time.Now()
					} else if time.Since(escSince) > 25*time.Millisecond {
						ch <- Key{Code: KEsc}
						buf = buf[1:]
						escSince = time.Time{}
						continue
					}
				}
				break
			}
			escSince = time.Time{}
			buf = buf[adv:]
			if k != nil {
				ch <- *k
			}
		}
		if n <= 0 {
			time.Sleep(5 * time.Millisecond)
		}
	}
}

// parse returns (key, bytesConsumed, needMoreInput).
func parse(b []byte) (*Key, int, bool) {
	c := b[0]
	if c == 0x1b {
		if len(b) == 1 {
			return nil, 0, true
		}
		if b[1] == '[' || b[1] == 'O' {
			for i := 2; i < len(b); i++ {
				if b[i] >= 0x40 && b[i] <= 0x7e { // CSI final byte
					return csiKey(b[2:i], b[i]), i + 1, false
				}
			}
			return nil, 0, true
		}
		return &Key{Code: KEsc}, 1, false // ESC + unrelated byte
	}
	switch c {
	case '\r', '\n':
		return &Key{Code: KEnter}, 1, false
	case '\t':
		return &Key{Code: KTab}, 1, false
	case 127, 8:
		return &Key{Code: KBackspace}, 1, false
	case 3:
		return &Key{Code: KCtrlC}, 1, false
	}
	if c < 32 {
		return nil, 1, false // swallow other control bytes
	}
	if !utf8.FullRune(b) && len(b) < utf8.UTFMax {
		return nil, 0, true
	}
	r, sz := utf8.DecodeRune(b)
	if r == utf8.RuneError && sz <= 1 {
		return nil, 1, false
	}
	return &Key{Code: KRune, R: r}, sz, false
}

func csiKey(params []byte, final byte) *Key {
	switch final {
	case 'A':
		return &Key{Code: KUp}
	case 'B':
		return &Key{Code: KDown}
	case 'C':
		return &Key{Code: KRight}
	case 'D':
		return &Key{Code: KLeft}
	case 'H':
		return &Key{Code: KHome}
	case 'F':
		return &Key{Code: KEnd}
	case '~':
		switch string(params) {
		case "1", "7":
			return &Key{Code: KHome}
		case "4", "8":
			return &Key{Code: KEnd}
		case "5":
			return &Key{Code: KPgUp}
		case "6":
			return &Key{Code: KPgDn}
		case "15":
			return &Key{Code: KF5}
		}
	}
	return nil
}
