package qsock

import (
	"context"
	"io"
	"net"
	"os"
	"time"

	"github.com/qsocket/qsocket/config"
	"github.com/qsocket/qsocket/log"
	"github.com/qsocket/qsocket/session"
	"github.com/qsocket/qsocket/utils"
)

// FORWARD_DIAL_TIMEOUT bounds the time the listening peer spends connecting
// to a forwarding destination, a destination behind a dropped packet would
// otherwise stall the probe loop.
const FORWARD_DIAL_TIMEOUT = 10 * time.Second

// ConnectSSH opens an interactive session on the remote peer.
//
// Local port forwarding takes over the session entirely, a -L invocation
// tunnels TCP traffic instead of opening a terminal, just like an ssh client
// given -N. The forwarding listeners keep running for as long as the process
// does.
func ConnectSSH(ctx context.Context, opts *config.Options) error {
	if len(opts.Forwards) != 0 {
		return ForwardPorts(ctx, opts)
	}
	if opts.NoExec {
		return ErrNothingToDo
	}

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

	status.Set("Setting up TTY terminal...")
	status.Stop()
	return AttachToSocket(qs, HasTerminal())
}

// ForwardPorts binds every forwarding specification of the session and
// tunnels the accepted connections until the process is interrupted.
//
// The listeners are bound before any of them starts accepting so that a
// failing bind is reported instead of being announced as forwarded.
func ForwardPorts(ctx context.Context, opts *config.Options) error {
	for _, fwd := range opts.Forwards {
		ln, err := net.Listen("tcp", fwd.ListenAddr())
		if err != nil {
			return err
		}
		defer ln.Close()

		// Logged while nothing else is drawing on the terminal, a line
		// printed underneath a spinner garbles both of them.
		log.Infof("Forwarding %s", fwd)
		go ForwardListener(ctx, opts, ln, fwd)
	}

	log.Info("Forwarding ports, press Ctrl-C to quit...")
	utils.WaitForExitSignal(os.Interrupt)
	return nil
}

// ServeSSH serves an interactive session requested by the connecting peer.
func ServeSSH(qs Session, specs *session.SessionSpecs) error {
	if err := session.WriteResponse(qs, session.STATUS_OK, ""); err != nil {
		qs.Close()
		return err
	}
	go func() {
		// Execute command/program and redirect stdin/out/err
		if err := ExecCommand(qs, specs); err != nil {
			log.Error(err)
		}
	}()
	return nil
}

// ForwardListener accepts connections on the given listener and tunnels each
// one of them to the forwarding destination through the remote peer.
//
// Every accepted connection gets a channel of its own on the relay network,
// so a single forwarded port serves as many connections as it is given.
func ForwardListener(ctx context.Context, opts *config.Options, ln net.Listener, fwd config.Forward) {
	defer ln.Close()

	// Unblock the accept loop when the session is torn down.
	go func() {
		<-ctx.Done()
		ln.Close()
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Error(err)
			continue
		}
		go func() {
			err := forwardConn(opts, fwd, conn)
			if err != nil && err != io.EOF {
				log.Error(err)
			}
		}()
	}
}

// forwardConn tunnels an accepted local connection to the forwarding
// destination through the remote peer.
func forwardConn(opts *config.Options, fwd config.Forward, conn net.Conn) error {
	defer conn.Close()
	log.Debugf("Forwarding %s -> %s", conn.RemoteAddr(), fwd.DialAddr())

	qs, err := DialQSRN(opts)
	if err != nil {
		return err
	}
	defer qs.Close()

	specs := &session.SessionSpecs{
		ID:      session.CMD_FORWARD,
		Address: fwd.DialAddr(),
	}
	if err := InitSession(qs, specs); err != nil {
		return err
	}
	return Pipe(qs, conn)
}

// ServeForward tunnels the session to the TCP destination requested by the
// connecting peer.
func ServeForward(qs Session, specs *session.SessionSpecs) error {
	conn, err := net.DialTimeout("tcp", specs.Address, FORWARD_DIAL_TIMEOUT)
	if err != nil {
		session.WriteError(qs, session.STATUS_ERROR, err)
		qs.Close()
		return err
	}
	if err := session.WriteResponse(qs, session.STATUS_OK, ""); err != nil {
		conn.Close()
		qs.Close()
		return err
	}

	go func() {
		defer qs.Close()
		defer conn.Close()
		if err := Pipe(qs, conn); err != nil && err != io.EOF {
			log.Error(err)
		}
	}()
	return nil
}
