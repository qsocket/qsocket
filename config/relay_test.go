package config

import "testing"

func TestParseRelay(t *testing.T) {
	for _, c := range []struct {
		addr string
		want Relay
	}{
		{"10.0.0.1:443", Relay{Host: "10.0.0.1", Port: 443}},
		{"gate.example.com:8443", Relay{Host: "gate.example.com", Port: 8443}},
		// Without a port the default ports of the network are kept.
		{"gate.example.com", Relay{Host: "gate.example.com"}},
		{"10.0.0.1", Relay{Host: "10.0.0.1"}},
		{" 10.0.0.1:443 ", Relay{Host: "10.0.0.1", Port: 443}},
		// An IPv6 literal keeps its brackets, qsocket-go appends the
		// port to the gate address.
		{"[::1]:443", Relay{Host: "[::1]", Port: 443}},
		{"[fd00::1]", Relay{Host: "[fd00::1]"}},
		{"fd00::1", Relay{Host: "[fd00::1]"}},
	} {
		got, err := ParseRelay(c.addr)
		if err != nil {
			t.Errorf("--relay-server %s: %v", c.addr, err)
			continue
		}
		if got != c.want {
			t.Errorf("--relay-server %s: got %+v, want %+v", c.addr, got, c.want)
		}
	}
}

func TestParseRelayErrors(t *testing.T) {
	for _, addr := range []string{
		"",                         // nothing to dial
		"   ",                      // nothing but spaces
		":443",                     // no host
		"10.0.0.1:",                // no port after the colon
		"10.0.0.1:https",           // port is a number
		"10.0.0.1:0",               // out of range
		"10.0.0.1:65536",           // out of range
		"https://gate.example.com", // a URL is not an address
		"gate.example.com/path",
	} {
		if relay, err := ParseRelay(addr); err == nil {
			t.Errorf("--relay-server %q was accepted as %+v", addr, relay)
		}
	}
}

func TestRelayString(t *testing.T) {
	for _, c := range []struct {
		relay Relay
		want  string
	}{
		{Relay{Host: "10.0.0.1", Port: 443}, "10.0.0.1:443"},
		{Relay{Host: "gate.example.com"}, "gate.example.com"},
		{Relay{Host: "[::1]", Port: 80}, "[::1]:80"},
	} {
		if got := c.relay.String(); got != c.want {
			t.Errorf("got %q, want %q", got, c.want)
		}
	}
	if !(Relay{}).IsZero() {
		t.Error("the zero relay is not zero")
	}
	if (Relay{Host: "10.0.0.1"}).IsZero() {
		t.Error("a configured relay is reported as zero")
	}
}

func TestParseSNI(t *testing.T) {
	for _, c := range []struct {
		name string
		want string
	}{
		{"www.google.com", "www.google.com"},
		{" cdn.example.com ", "cdn.example.com"},
	} {
		got, err := ParseSNI(c.name)
		if err != nil {
			t.Errorf("--relay-sni %s: %v", c.name, err)
			continue
		}
		if got != c.want {
			t.Errorf("--relay-sni %s: got %q, want %q", c.name, got, c.want)
		}
	}

	for _, name := range []string{
		"",                       // nothing to indicate
		"   ",                    // nothing but spaces
		"www.google.com:443",     // a server name carries no port
		"https://www.google.com", // nor a scheme
	} {
		if sni, err := ParseSNI(name); err == nil {
			t.Errorf("--relay-sni %q was accepted as %q", name, sni)
		}
	}
}

func TestGateSNI(t *testing.T) {
	// Without a configured gate the default of qsocket-go is kept.
	opts := &Options{}
	if got := opts.GateSNI(); got != "" {
		t.Errorf("got SNI %q, want the library default", got)
	}
	// A custom gate is asked for the certificate of its own name.
	opts = &Options{Relay: Relay{Host: "gate.example.com", Port: 8443}}
	if got := opts.GateSNI(); got != "gate.example.com" {
		t.Errorf("got SNI %q, want %q", got, "gate.example.com")
	}
	// A spoofed name takes precedence over both.
	opts.RelaySNI = "www.google.com"
	if got := opts.GateSNI(); got != "www.google.com" {
		t.Errorf("got SNI %q, want %q", got, "www.google.com")
	}
}
