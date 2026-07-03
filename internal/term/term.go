// Package term owns the tty: raw mode, alternate screen, size, and
// suspend/resume around external programs (shell, $EDITOR).
//
// Pure stdlib by design constraint: termios via ioctl through the
// syscall package instead of golang.org/x/term. The flag constants are
// asm-generic and identical on x86_64 and aarch64, which covers every
// architecture the target distro ships.
package term

import (
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"unsafe"
)

// kernel struct termios (asm-generic/termbits.h), NCCS=19.
type termios struct {
	Iflag, Oflag, Cflag, Lflag uint32
	Line                       uint8
	Cc                         [19]uint8
	_                          [3]uint8
	Ispeed, Ospeed             uint32
}

type winsize struct{ Row, Col, X, Y uint16 }

const (
	reqTCGETS     = 0x5401
	reqTCSETS     = 0x5402
	reqTIOCGWINSZ = 0x5413
)

var (
	saved  termios
	rawOn  bool
	paused atomic.Bool // reader goroutine yields stdin while a child owns the tty
)

func ioctl(fd, req uintptr, arg unsafe.Pointer) error {
	_, _, e := syscall.Syscall(syscall.SYS_IOCTL, fd, req, uintptr(arg))
	if e != 0 {
		return e
	}
	return nil
}

// EnterRaw switches the tty to raw mode and the alternate screen.
func EnterRaw() error {
	if err := ioctl(0, reqTCGETS, unsafe.Pointer(&saved)); err != nil {
		return err
	}
	raw := saved
	raw.Iflag &^= syscall.IGNBRK | syscall.BRKINT | syscall.PARMRK |
		syscall.ISTRIP | syscall.INLCR | syscall.IGNCR | syscall.ICRNL | syscall.IXON
	raw.Oflag &^= syscall.OPOST
	raw.Lflag &^= syscall.ECHO | syscall.ECHONL | syscall.ICANON |
		syscall.ISIG | syscall.IEXTEN
	raw.Cflag &^= syscall.CSIZE | syscall.PARENB
	raw.Cflag |= syscall.CS8
	raw.Cc[syscall.VMIN] = 0
	raw.Cc[syscall.VTIME] = 0
	if err := ioctl(0, reqTCSETS, unsafe.Pointer(&raw)); err != nil {
		return err
	}
	_ = syscall.SetNonblock(0, true) // lets the key reader poll without stealing blocking reads
	os.Stdout.WriteString("\x1b[?1049h\x1b[?25l") // alt screen, hide cursor
	rawOn = true
	return nil
}

// Restore undoes EnterRaw. Safe to call multiple times; must run on
// every exit path, including panic (see main's deferred recover).
func Restore() {
	if !rawOn {
		return
	}
	os.Stdout.WriteString("\x1b[0m\x1b[?25h\x1b[?1049l")
	_ = syscall.SetNonblock(0, false)
	_ = ioctl(0, reqTCSETS, unsafe.Pointer(&saved))
	rawOn = false
}

// Suspend hands the tty to fn (a shell, an editor) and takes it back.
func Suspend(fn func()) {
	paused.Store(true)
	Restore()
	fn()
	_ = EnterRaw()
	paused.Store(false)
}

// Size returns terminal columns, rows with an 80x24 fallback.
func Size() (w, h int) {
	var ws winsize
	if err := ioctl(1, reqTIOCGWINSZ, unsafe.Pointer(&ws)); err != nil ||
		ws.Col == 0 || ws.Row == 0 {
		return 80, 24
	}
	return int(ws.Col), int(ws.Row)
}

// WatchResize delivers SIGWINCH.
func WatchResize() <-chan os.Signal {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGWINCH)
	return ch
}
