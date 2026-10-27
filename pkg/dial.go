package qsock

import (
	"sync"

	"github.com/qsocket/qsocket/config"
	"github.com/qsocket/qsocket/log"
	"github.com/qsocket/qsocket/session"
	qsocket "github.com/qsocket/qsocket-go"
)

// gateOnce guards the gate globals of qsocket-go, every session dials
// through the same values and a forwarded port dials from a goroutine of its
// own.
var gateOnce sync.Once

// setGate points qsocket-go at the relay network gate given on the command
// line, it is a no-op without the --relay-server and --relay-sni options.
func setGate(opts *config.Options) {
	gateOnce.Do(func() {
		if !opts.Relay.IsZero() {
			qsocket.QSRN_GATE = opts.Relay.Host
			// A named gate is the one to dial, reaching the relay
			// network over TOR does not bring back the onion gate.
			qsocket.QSRN_TOR_GATE = opts.Relay.Host
			// The library picks the port by the presence of the TLS
			// layer, an explicitly given one is meant for both.
			if opts.Relay.Port != 0 {
				qsocket.QSRN_GATE_TLS_PORT = opts.Relay.Port
				qsocket.QSRN_GATE_PORT = opts.Relay.Port
			}
		}
		// The gate of the public network presents the certificate of
		// its own name, so only a configured value is applied.
		if sni := opts.GateSNI(); sni != "" {
			qsocket.QSRN_GATE_SNI = sni
		}
		port := qsocket.QSRN_GATE_TLS_PORT
		if opts.DisableEnc {
			port = qsocket.QSRN_GATE_PORT
		}
		log.Debugf("Using QSRN gate %s:%d (SNI: %s)",
			qsocket.QSRN_GATE, port, qsocket.QSRN_GATE_SNI)
	})
}

// NewSocket builds a QSocket configured with the given options.
func NewSocket(opts *config.Options) (*qsocket.QSocket, error) {
	setGate(opts)

	sType := qsocket.Client
	if opts.Listen {
		sType = qsocket.Server
	}

	qs := qsocket.NewSocket(sType, opts.Secret)
	if err := qs.SetE2E(opts.End2End); err != nil {
		return nil, err
	}
	if opts.CertFingerprint != "" {
		if err := qs.SetCertFingerprint(opts.CertFingerprint); err != nil {
			return nil, err
		}
	}
	if opts.SocksAddr != "" {
		if err := qs.SetProxy(opts.SocksAddr); err != nil {
			return nil, err
		}
	}
	return qs, nil
}

// DialQSRN dials the relay network and returns a connected socket.
func DialQSRN(opts *config.Options) (*qsocket.QSocket, error) {
	qs, err := NewSocket(opts)
	if err != nil {
		return nil, err
	}
	if err := qs.Dial(!opts.DisableEnc); err != nil {
		return nil, err
	}
	return qs, nil
}

// NewSpecs builds the session specs requested from the remote peer.
func NewSpecs(opts *config.Options) *session.SessionSpecs {
	specs := &session.SessionSpecs{
		ID:   opts.Cmd,
		Exec: opts.Exec,
		Path: opts.RemotePath,
	}
	// Without a terminal to drive it a pseudo terminal only garbles the
	// session, so a piped or redirected ssh runs the command raw, which is
	// how an ssh client behaves as well.
	specs.Set(session.FLAG_PTY, opts.Cmd == session.CMD_SSH && HasTerminal())
	specs.Set(session.FLAG_RECURSIVE, opts.Recursive)
	specs.Set(session.FLAG_UPLOAD, opts.Upload)

	// Only a terminal session has a window size to negotiate, the remote
	// peer falls back to its own dimensions for a zero value.
	if specs.Has(session.FLAG_PTY) {
		ws, err := GetCurrentTermSize()
		if err != nil {
			log.Debug(err)
		} else {
			specs.TermSize = *ws
		}
	}
	return specs
}

// InitSession sends the session specs to the listening peer and waits for it
// to accept the session.
func InitSession(qs Session, specs *session.SessionSpecs) error {
	log.Debugf("Sending %s session specs...", specs.ID)
	if err := session.WriteSpecs(qs, specs); err != nil {
		return err
	}
	resp, err := session.ReadResponse(qs)
	if err != nil {
		return err
	}
	return resp.Err()
}
