package qsock

import (
	"errors"
	"io"
	"net"
	"os"

	"golang.org/x/term"
)

var (
	ErrNoOptions         = errors.New("Options are not initialized")
	ErrQsocketSessionEnd = errors.New("QSocket session has ended")
	ErrTtyFailed         = errors.New("TTY initialization failed")
	ErrUntrustedCert     = errors.New("Certificate fingerprint mismatch")
	ErrNothingToDo       = errors.New("Nothing to do, -N requires at least one -L forwarding specification")
	// ErrSilent is returned by the subcommands which already reported the
	// failure on the terminal, it keeps the message from being printed
	// twice.
	ErrSilent = errors.New("")
)

// Session is the connection carrying a qsock session.
//
// It is what a dialed QSocket provides, the handlers are written against the
// interface instead of the concrete socket so that the session layer stays
// independent of the transport underneath it.
type Session interface {
	io.ReadWriter
	Close()
}

// Pipe copies data between the two given streams until either direction ends
// and returns the error which ended it.
func Pipe(a, b io.ReadWriter) error {
	done := make(chan error, 2)
	go func() {
		_, err := io.Copy(a, b)
		done <- err
	}()
	go func() {
		_, err := io.Copy(b, a)
		done <- err
	}()
	return <-done
}

// AttachToSocket redirects the standard streams of the process into the given
// session.
//
// A raw session puts the local terminal in raw mode, every keystroke is then
// handed to the remote peer instead of being interpreted locally.
func AttachToSocket(conn io.ReadWriter, raw bool) error {
	if raw {
		oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
		if err != nil {
			return err
		}
		defer term.Restore(int(os.Stdin.Fd()), oldState)
	}
	// The input copy is left running, it would otherwise keep reading
	// until the next keystroke before noticing the closed session.
	go func() { io.Copy(conn, os.Stdin) }()
	_, err := io.Copy(os.Stdout, conn)
	if errors.Is(err, io.ErrClosedPipe) || errors.Is(err, net.ErrClosed) {
		// The session has ended, which is how it is supposed to go.
		return nil
	}
	return err
}
