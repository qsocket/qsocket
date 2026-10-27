package qsock

import (
	"context"
	"fmt"

	"github.com/qsocket/qsocket/config"
	"github.com/qsocket/qsocket/log"
	"github.com/qsocket/qsocket/session"
)

// PIPE_BUFFER_SIZE is the size of the chunks moved between a redirected
// standard stream and the session.
const PIPE_BUFFER_SIZE = 1024

// ConnectNetcat opens a raw data channel to the remote peer.
//
// Unlike an ssh session the channel carries no terminal, which makes it
// usable as a shell pipe for moving data across the relay network.
func ConnectNetcat(ctx context.Context, opts *config.Options) error {
	status := NewStatus(opts)
	defer status.Stop()
	status.Set("Dialing qsocket relay network...")

	qs, err := DialQSRN(opts)
	if err != nil {
		return err
	}
	defer qs.Close()

	if err := InitSession(qs, NewSpecs(opts)); err != nil {
		return err
	}
	status.Stop()

	if opts.IsPiped() {
		return AttachToPipe(qs, opts)
	}
	return AttachToSocket(qs, false)
}

// ServeNetcat serves a raw data channel requested by the connecting peer.
func ServeNetcat(qs Session, specs *session.SessionSpecs, opts *config.Options) error {
	if err := session.WriteResponse(qs, session.STATUS_OK, ""); err != nil {
		qs.Close()
		return err
	}

	// A redirected standard stream takes the place of the shell, the
	// session then moves the data of the pipe instead of a command.
	if opts.IsPiped() {
		return AttachToPipe(qs, opts)
	}

	go func() {
		// Execute command/program and redirect stdin/out/err
		if err := ExecCommand(qs, specs); err != nil {
			log.Error(err)
		}
	}()
	return nil
}

// AttachToPipe moves the data between the session and the redirected standard
// stream of the process.
//
// It returns when the pipe is drained or when the session ends, the number of
// bytes that made it across is reported on the terminal.
func AttachToPipe(conn Session, opts *config.Options) error {
	finalMsg := "No pipe is initialized..."
	defer func() { log.Info(finalMsg) }()
	defer conn.Close()

	status := NewStatus(opts)
	defer status.Stop()

	switch {
	case opts.InPipe != nil:
		total := 0
		status.Set("Reading from %s...", opts.InPipe.Name())
		for {
			data := make([]byte, PIPE_BUFFER_SIZE)
			n, err := opts.InPipe.Read(data)
			if err != nil {
				return err
			}
			if n == 0 {
				continue
			}
			total += n
			finalMsg = fmt.Sprintf("Sent %d bytes!", total)
			status.Set("Piping %d bytes from %s...", total, opts.InPipe.Name())
			if _, err = conn.Write(data[:n]); err != nil {
				return err
			}
		}
	case opts.OutPipe != nil:
		total := 0
		status.Set("Writing into %s...", opts.OutPipe.Name())
		for {
			data := make([]byte, PIPE_BUFFER_SIZE)
			n, err := conn.Read(data)
			if err != nil {
				return err
			}
			if n == 0 {
				continue
			}
			total += n
			finalMsg = fmt.Sprintf("Received %d bytes!", total)
			status.Set("Piping %d bytes to %s...", total, opts.OutPipe.Name())
			if _, err = opts.OutPipe.Write(data[:n]); err != nil {
				return err
			}
		}
	}
	return nil
}
