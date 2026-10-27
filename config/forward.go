package config

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
)

var (
	ErrInvalidForward = errors.New("Invalid forwarding specification, expected [bind_address:]port:host:hostport")
	ErrInvalidPort    = errors.New("Invalid port number.")
)

// Forward describes a single local port forwarding specification given with
// the -L option of the ssh subcommand.
type Forward struct {
	// BindAddr is the local interface the listener is bound to, an empty
	// value binds every interface.
	BindAddr string
	// BindPort is the local port the listener is bound to.
	BindPort int
	// Host and Port are the destination dialed by the remote peer.
	Host string
	Port int
}

// ParseForward parses a "[bind_address:]port:host:hostport" specification.
//
// Without a bind address the listener accepts connections from any host, as
// an ssh client would, IPv6 literals are accepted in square brackets.
func ParseForward(spec string) (Forward, error) {
	fields, err := splitFields(spec)
	if err != nil {
		return Forward{}, err
	}

	fwd := Forward{}
	switch len(fields) {
	case 3:
		fwd.Host = fields[1]
	case 4:
		fwd.BindAddr, fwd.Host = fields[0], fields[2]
	default:
		return Forward{}, fmt.Errorf("%w (got %q)", ErrInvalidForward, spec)
	}

	// The local port always sits right before the destination host.
	fwd.BindPort, err = parsePort(fields[len(fields)-3])
	if err != nil {
		return Forward{}, err
	}
	fwd.Port, err = parsePort(fields[len(fields)-1])
	if err != nil {
		return Forward{}, err
	}
	if fwd.Host == "" {
		return Forward{}, fmt.Errorf("%w (no destination host in %q)", ErrInvalidForward, spec)
	}
	return fwd, nil
}

// ListenAddr returns the local address the forwarding listener binds to.
func (f Forward) ListenAddr() string {
	return net.JoinHostPort(f.BindAddr, strconv.Itoa(f.BindPort))
}

// DialAddr returns the destination address dialed by the remote peer.
func (f Forward) DialAddr() string {
	return net.JoinHostPort(f.Host, strconv.Itoa(f.Port))
}

// String returns the normalized form of the forwarding specification.
func (f Forward) String() string {
	return fmt.Sprintf("%s -> %s", f.ListenAddr(), f.DialAddr())
}

// splitFields splits a forwarding specification on its colons, the colons of
// a bracketed IPv6 literal are kept as part of the enclosing field.
func splitFields(spec string) ([]string, error) {
	fields := []string{}
	field := strings.Builder{}
	depth := 0
	for i := 0; i < len(spec); i++ {
		switch spec[i] {
		case '[':
			depth++
		case ']':
			depth--
			if depth < 0 {
				return nil, fmt.Errorf("%w (unbalanced brackets in %q)", ErrInvalidForward, spec)
			}
		case ':':
			if depth > 0 {
				field.WriteByte(':')
				continue
			}
			fields = append(fields, field.String())
			field.Reset()
		default:
			field.WriteByte(spec[i])
		}
	}
	if depth != 0 {
		return nil, fmt.Errorf("%w (unbalanced brackets in %q)", ErrInvalidForward, spec)
	}
	return append(fields, field.String()), nil
}

// parsePort parses and range checks a TCP port number.
func parsePort(v string) (int, error) {
	port, err := strconv.Atoi(v)
	if err != nil || port < 1 || port > 65535 {
		return 0, fmt.Errorf("%w (got %q)", ErrInvalidPort, v)
	}
	return port, nil
}
