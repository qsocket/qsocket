package qsnetcat

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"time"

	"github.com/qsocket/qs-netcat/config"
	"github.com/qsocket/qs-netcat/log"
	qsocket "github.com/qsocket/qsocket-go"

	"github.com/briandowns/spinner"
	"golang.org/x/term"
)

var (
	ErrQsocketSessionEnd = errors.New("QSocket session has ended")
	ErrTtyFailed         = errors.New("TTY initialization failed")
	ErrUntrustedCert     = errors.New("Certificate fingerprint mismatch")
	spn                  = spinner.New(spinner.CharSets[9], 50*time.Millisecond)
)

// Run executes qs-netcat with the given options, either in server (listen)
// or client mode. It returns when the work is complete, when an unrecoverable
// error occurs, or when the given context is cancelled.
//
// This is the common entry point shared by the console and the Windows
// service execution modes.
func Run(ctx context.Context, opts *config.Options) error {
	if opts == nil {
		return errors.New("Options are not initialized")
	}
	if opts.Listen {
		return Serve(ctx, opts)
	}
	err := Connect(opts)
	if err != nil && err != io.EOF {
		return err
	}
	return nil
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

func ProbeQSRN(opts *config.Options) error {
	// This is nessesary for persistence on windows
	os.Unsetenv("QS_ARGS") // Remove this for allowing recursive qs-netcat usage
	qs := qsocket.NewSocket(GetPeerTag(opts), opts.Secret)
	err := qs.SetE2E(opts.End2End)
	if err != nil {
		return err
	}
	if opts.CertFingerprint != "" {
		err = qs.SetCertFingerprint(opts.CertFingerprint)
		if err != nil {
			return err
		}
	}
	if opts.SocksAddr != "" {
		err = qs.SetProxy(opts.SocksAddr)
		if err != nil {
			return err
		}
	}

	// Dial QSRN...
	err = qs.Dial(!opts.DisableEnc)
	if err != nil {
		return err
	}

	log.Debug("Recving session specs...")
	specs, err := RecvSessionSpecs(qs, opts)
	if err != nil {
		return err
	}

	log.Info("Starting new session...")
	// Resize terminal with client dimentions...
	log.Debugf("Got term size: %dx%d", specs.TermSize.Rows, specs.TermSize.Cols)
	log.Debugf("Got command: %s", specs.Command)
	log.Debugf("Got forward: %s", specs.ForwardAddr)

	// First check if forwarding enabled
	if specs.ForwardAddr != "" {
		// Redirect traffic to forward addr
		err = CreatePipeOnConnect(qs, specs.ForwardAddr)
		if err != nil {
			return err
		}
	}

	if opts.IsPiped() {
		return AttachToPipe(qs, opts)
	}

	go func() {
		// Execute command/program and redirect stdin/out/err
		err = ExecCommand(qs, *specs)
		if err != nil {
			log.Error(err)
		}
	}()
	return err
}

func CreatePipeOnConnect(qs *qsocket.QSocket, addr string) error {
	defer qs.Close()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return err
	}
	defer conn.Close()

	ch := make(chan bool, 1)
	go func() {
		_, err = io.Copy(conn, qs)
		ch <- true
	}()
	go func() {
		_, err = io.Copy(qs, conn)
		ch <- true
	}()
	<-ch
	return err
}

func InitLocalProxy(qs *qsocket.QSocket, opts *config.Options) {
	ln, err := net.Listen("tcp", ":"+opts.LocalPort)
	if err != nil {
		log.Fatal(err)
	}

	for {
		spn.Suffix = " Waiting for local connection..."
		spn.Start()
		conn, err := ln.Accept()
		if err != nil {
			log.Error(err)
			continue
		}
		spn.Suffix = " Dialing qsocket relay network..."
		if opts.SocksAddr != "" {
			err = qs.SetProxy(opts.SocksAddr)
			if err != nil {
				log.Fatal(err)
			}
		}

		err = qs.Dial(!opts.DisableEnc)
		if err != nil {
			spn.Stop()
			log.Error(err)
			continue
		}

		err = SendSessionSpecs(qs, opts)
		if err != nil {
			spn.Stop()
			log.Error(err)
			continue
		}

		spn.Suffix = " Forwarding local traffic..."
		go func() {
			_, err = io.Copy(conn, qs)
		}()
		_, err = io.Copy(qs, conn)
		if err != nil {
			log.Debug(err)
		}
		qs.Close()
		conn.Close()
	}
}

func Connect(opts *config.Options) error {
	defer spn.Stop()
	if opts == nil {
		return errors.New("Options are not initialized")
	}
	if !opts.Quiet {
		spn.Suffix = " Dialing qsocket relay network..."
		spn.Start()
	}

	qs := qsocket.NewSocket(GetPeerTag(opts), opts.Secret)
	err := qs.SetE2E(opts.End2End)
	if err != nil {
		log.Fatal(err)
	}
	if opts.CertFingerprint != "" {
		err = qs.SetCertFingerprint(opts.CertFingerprint)
		if err != nil {
			log.Fatal(err)
		}
	}

	if opts.LocalPort != "" {
		InitLocalProxy(qs, opts)
	}

	if opts.SocksAddr != "" {
		err = qs.SetProxy(opts.SocksAddr)
		if err != nil {
			log.Fatal(err)
		}
	}

	err = qs.Dial(!opts.DisableEnc)
	if err != nil {
		return err
	}

	log.Debug("Sending session specs...")
	err = SendSessionSpecs(qs, opts)
	if err != nil {
		return err
	}

	if opts.IsPiped() {
		return AttachToPipe(qs, opts)
	}
	return AttachToSocket(qs, opts.Interactive)
}

func AttachToPipe(conn *qsocket.QSocket, opts *config.Options) error {
	finalMsg := "No pipe is initialized..."
	defer func() { log.Info(finalMsg) }()
	defer conn.Close()
	if opts.InPipe != nil {
		if !opts.Listen {
			spn.Suffix = fmt.Sprintf(" Reading from %s...", opts.InPipe.Name())
			spn.Start()
			defer spn.Stop()
		}
		total := 0
		for {
			data := make([]byte, 1024)
			n, err := opts.InPipe.Read(data)
			if err != nil {
				return err
			}
			if n == 0 {
				continue
			}
			total += n
			finalMsg = fmt.Sprintf("Sent %d bytes!", total)
			spn.Suffix = fmt.Sprintf(" Piping %d bytes from %s...", total, opts.InPipe.Name())
			n, err = conn.Write(data[:n])
			if err != nil {
				return err
			}
		}
	} else if opts.OutPipe != nil {
		if !opts.Listen {
			spn = spinner.New(spinner.CharSets[9], 50*time.Millisecond, spinner.WithWriter(os.Stderr))
			spn.Suffix = fmt.Sprintf(" Writing into %s...", opts.OutPipe.Name())
			spn.Start()
			defer spn.Stop()
		}
		total := 0
		for {
			data := make([]byte, 1024)
			n, err := conn.Read(data)
			if err != nil {
				return err
			}
			if n == 0 {
				continue
			}
			total += n
			finalMsg = fmt.Sprintf("Received %d bytes!", total)
			spn.Suffix = fmt.Sprintf(" Piping %d bytes to %s...", total, opts.OutPipe.Name())
			n, err = opts.OutPipe.Write(data[:n])
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func AttachToSocket(conn *qsocket.QSocket, interactive bool) error {
	defer conn.Close()
	if interactive {
		spn.Suffix = " Setting up TTY terminal..."
		oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
		if err != nil {
			return err
		}
		defer term.Restore(int(os.Stdin.Fd()), oldState)
	}
	spn.Stop()
	go func() { io.Copy(conn, os.Stdin) }()
	io.Copy(os.Stdout, conn)
	return nil
}

func GetPeerTag(opts *config.Options) qsocket.SocketType {
	if opts.Listen {
		return qsocket.Server
	}
	return qsocket.Client
}
