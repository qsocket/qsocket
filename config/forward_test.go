package config

import "testing"

func TestParseForward(t *testing.T) {
	cases := map[string]Forward{
		"80:intra.example.com:80":           {BindPort: 80, Host: "intra.example.com", Port: 80},
		"127.0.0.1:80:intra.example.com:80": {BindAddr: "127.0.0.1", BindPort: 80, Host: "intra.example.com", Port: 80},
		"2222:192.168.6.7:22":               {BindPort: 2222, Host: "192.168.6.7", Port: 22},
		"[::1]:8080:[fe80::1]:80":           {BindAddr: "::1", BindPort: 8080, Host: "fe80::1", Port: 80},
	}
	for spec, want := range cases {
		got, err := ParseForward(spec)
		if err != nil {
			t.Errorf("ParseForward(%q) failed: %v", spec, err)
			continue
		}
		if got != want {
			t.Errorf("ParseForward(%q) = %+v, want %+v", spec, got, want)
		}
	}

	for _, spec := range []string{
		"",
		"80",
		"80:host",
		"80:host:22:extra:more",
		"notaport:host:22",
		"80:host:notaport",
		"0:host:22",
		"80:host:70000",
		"80::22",
		"[::1:8080:host:80",
	} {
		if got, err := ParseForward(spec); err == nil {
			t.Errorf("ParseForward(%q) = %+v, want an error", spec, got)
		}
	}
}

func TestForwardAddrs(t *testing.T) {
	fwd, err := ParseForward("80:intra.example.com:8080")
	if err != nil {
		t.Fatal(err)
	}
	// An empty bind address accepts connections on every interface.
	if got := fwd.ListenAddr(); got != ":80" {
		t.Errorf("ListenAddr() = %q, want %q", got, ":80")
	}
	if got := fwd.DialAddr(); got != "intra.example.com:8080" {
		t.Errorf("DialAddr() = %q, want %q", got, "intra.example.com:8080")
	}

	fwd, err = ParseForward("[::1]:80:[fe80::1]:8080")
	if err != nil {
		t.Fatal(err)
	}
	// IPv6 literals need their brackets back when joined to a port.
	if got := fwd.ListenAddr(); got != "[::1]:80" {
		t.Errorf("ListenAddr() = %q, want %q", got, "[::1]:80")
	}
	if got := fwd.DialAddr(); got != "[fe80::1]:8080" {
		t.Errorf("DialAddr() = %q, want %q", got, "[fe80::1]:8080")
	}
}
