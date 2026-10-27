package qsock

import (
	"context"
	"io"
	"os"
	"time"

	"github.com/qsocket/qsocket/config"
	"github.com/qsocket/qsocket/log"
	"github.com/qsocket/qsocket/session"
	qsocket "github.com/qsocket/qsocket-go"
)

// Run executes qsock with the given options, either in server (listen) or
// client mode. It returns when the work is complete, when an unrecoverable
// error occurs, or when the given context is cancelled.
//
// This is the common entry point shared by the console and the Windows
// service execution modes.
func Run(ctx context.Context, opts *config.Options) error {
	if opts == nil {
		return ErrNoOptions
	}
	if opts.Listen {
		return Serve(ctx, opts)
	}
	err := Connect(ctx, opts)
	if err != nil && err != io.EOF {
		return err
	}
	return nil
}

// Connect starts the session requested on the command line.
func Connect(ctx context.Context, opts *config.Options) error {
	switch opts.Cmd {
	case session.CMD_SSH:
		return ConnectSSH(ctx, opts)
	case session.CMD_SCP:
		return ConnectSCP(ctx, opts)
	case session.CMD_NETCAT:
		return ConnectNetcat(ctx, opts)
	}
	return session.ErrUnsupportedCommand
}

// Serve probes the QSRN for incoming connections until the given context is
// cancelled. Recoverable errors are logged and retried on the next probe
// interval, unrecoverable ones (such as a server tag collision) are returned.
//
// Cancelling the context stops the probe loop, it does not interrupt an
// in-flight probe or terminate the sessions that are already established.
func Serve(ctx context.Context, opts *config.Options) error {
	log.Info("Listening for connections...")
	for firstRun := true; ; firstRun = false {
		if !firstRun {
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(time.Duration(opts.ProbeInterval) * time.Second):
			}
		}
		if ctx.Err() != nil {
			return nil
		}

		err := ProbeQSRN(opts)
		switch err {
		case nil:
		case qsocket.ErrPeerNotFound, io.EOF:
			log.Debug(err)
		case qsocket.ErrServerCollision:
			return err
		default:
			log.Error(err)
		}
	}
}

// ProbeQSRN dials the relay network once and serves the session of the peer
// waiting on the other end, if there is one.
func ProbeQSRN(opts *config.Options) error {
	// This is nessesary for persistence on windows
	os.Unsetenv("QS_ARGS") // Remove this for allowing recursive qsock usage
	qs, err := DialQSRN(opts)
	if err != nil {
		return err
	}

	log.Debug("Recving session specs...")
	specs, err := session.ReadSpecs(qs)
	if err != nil {
		qs.Close()
		return err
	}
	return ServeSession(qs, specs, opts)
}

// ServeSession dispatches an accepted session based on the command ID of its
// specs.
//
// The handlers either take the session over in a goroutine of their own or
// close it, in both cases the caller is free to probe for the next peer as
// soon as this returns.
func ServeSession(qs Session, specs *session.SessionSpecs, opts *config.Options) error {
	applyDefaults(specs, opts)
	log.Infof("Starting new %s session...", specs.ID)
	log.Debugf("Got term size: %dx%d", specs.TermSize.Rows, specs.TermSize.Cols)
	log.Debugf("Got command: %s", specs.Exec)
	log.Debugf("Got address: %s", specs.Address)
	log.Debugf("Got path: %s", specs.Path)

	switch specs.ID {
	case session.CMD_SSH:
		return ServeSSH(qs, specs)
	case session.CMD_NETCAT:
		return ServeNetcat(qs, specs, opts)
	case session.CMD_FORWARD:
		return ServeForward(qs, specs)
	case session.CMD_SCP:
		return ServeSCP(qs, specs, opts)
	}

	log.Errorf("Unsupported session command: %s", specs.ID)
	session.WriteError(qs, session.STATUS_UNSUPPORTED, session.ErrUnsupportedCommand)
	qs.Close()
	return nil
}

// applyDefaults fills in the session parameters the connecting peer left
// unset with the ones configured on the listening peer.
func applyDefaults(specs *session.SessionSpecs, opts *config.Options) {
	if specs.Exec == "" {
		specs.Exec = opts.Exec
	}
	if specs.TermSize.IsZero() {
		ws, err := GetCurrentTermSize()
		if err != nil {
			// A listening peer with no terminal of its own still has
			// to give the shell usable dimensions.
			log.Debug(err)
			specs.TermSize = DefaultTermSize()
			return
		}
		specs.TermSize = *ws
	}
}
