package qsock

import (
	"errors"
	"os"

	"github.com/qsocket/qsocket/session"
	"golang.org/x/term"
)

const (
	// DEFAULT_COLS and DEFAULT_ROWS are the dimensions handed to a shell
	// when neither peer has a terminal to measure.
	DEFAULT_COLS = 80
	DEFAULT_ROWS = 40
)

// ErrNoTerminal is returned when the process has no terminal attached.
var ErrNoTerminal = errors.New("No terminal attached to the process.")

// HasTerminal reports whether the standard input of the process is a
// terminal.
//
// A session is only worth a pseudo terminal when there is a real one on this
// end to drive it, an ssh client makes the same call.
func HasTerminal() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

// GetCurrentTermSize returns the window size of the terminal attached to the
// process.
//
// The standard output is measured as well, a session started with its input
// redirected still has a terminal worth reporting to the remote peer. This
// goes through x/term rather than a TIOCGWINSZ ioctl of its own, which keeps
// it building on every supported platform.
func GetCurrentTermSize() (*session.Winsize, error) {
	for _, file := range []*os.File{os.Stdin, os.Stdout} {
		cols, rows, err := term.GetSize(int(file.Fd()))
		if err != nil || cols <= 0 || rows <= 0 {
			continue
		}
		return &session.Winsize{Cols: uint16(cols), Rows: uint16(rows)}, nil
	}
	return nil, ErrNoTerminal
}

// DefaultTermSize returns the window size used when there is nothing to
// measure.
func DefaultTermSize() session.Winsize {
	return session.Winsize{Cols: DEFAULT_COLS, Rows: DEFAULT_ROWS}
}
