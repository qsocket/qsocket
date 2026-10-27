package config

import (
	"errors"
	"fmt"
	"net"
	"strings"
)

var (
	ErrInvalidRelay = errors.New("Invalid relay server address, expected <host>[:port]")
	ErrInvalidSNI   = errors.New("Invalid TLS server name, expected a hostname.")
)

// Relay is the address of a QSRN gate given with the --relay-server option.
type Relay struct {
	// Host is the hostname or the IP address of the gate, an IPv6 literal
	// carries its square brackets since qsocket-go builds the address of
	// the gate by appending the port to it.
	Host string
	// Port is the TCP port of the gate, a zero value keeps the default
	// ports of the relay network. (443 with TLS, 80 without)
	Port int
}

// ParseRelay parses a "<host>[:port]" relay server address.
//
// Both a hostname and an IP address are accepted, the port is optional and
// defaults to the ports of the public relay network. IPv6 literals are
// accepted in square brackets, a bare one is bracketed on the way in.
func ParseRelay(addr string) (Relay, error) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return Relay{}, fmt.Errorf("%w (got %q)", ErrInvalidRelay, addr)
	}

	relay := Relay{Host: addr}
	// Neither a missing port nor the colons of a bare IPv6 literal are an
	// error, so the split only counts when it succeeds.
	if host, port, err := net.SplitHostPort(addr); err == nil {
		relay.Host = host
		if relay.Port, err = parsePort(port); err != nil {
			return Relay{}, err
		}
	}
	// A URL given instead of an address would only fail during the TLS
	// handshake, with a diagnostic pointing nowhere near the flag.
	if relay.Host == "" || strings.ContainsAny(relay.Host, `/\ `) {
		return Relay{}, fmt.Errorf("%w (got %q)", ErrInvalidRelay, addr)
	}
	// qsocket-go formats the gate address as "host:port", so a literal
	// needs its brackets for the result to stay parseable.
	if strings.Contains(relay.Host, ":") {
		relay.Host = "[" + strings.Trim(relay.Host, "[]") + "]"
	}
	return relay, nil
}

// ParseSNI validates the server name given with the --relay-sni option.
func ParseSNI(name string) (string, error) {
	name = strings.TrimSpace(name)
	// A server name indicator carries a bare hostname, it has no port and
	// no scheme of its own.
	if name == "" || strings.ContainsAny(name, `:/\ `) {
		return "", fmt.Errorf("%w (got %q)", ErrInvalidSNI, name)
	}
	return name, nil
}

// IsZero reports whether no custom gate was given.
func (r Relay) IsZero() bool {
	return r.Host == "" && r.Port == 0
}

// String returns the normalized address of the gate.
func (r Relay) String() string {
	if r.Port == 0 {
		return r.Host
	}
	return fmt.Sprintf("%s:%d", r.Host, r.Port)
}
