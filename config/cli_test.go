package config

import (
	"strings"
	"testing"

	"github.com/qsocket/qsocket/session"
)

// configure resolves an argument list the way the command line does, without
// letting it reach the standard streams of the test process.
func configure(t *testing.T, args ...string) *Options {
	t.Helper()
	opts, err := Configure(args, false)
	if err != nil {
		t.Fatalf("qsock %s: %v", strings.Join(args, " "), err)
	}
	return opts
}

func TestConfigureSSH(t *testing.T) {
	secret, err := NewSecret("MySecret")
	if err != nil {
		t.Fatal(err)
	}

	opts := configure(t, "ssh", "qs://MySecret")
	if opts.Cmd != session.CMD_SSH {
		t.Errorf("got command %s, want ssh", opts.Cmd)
	}
	if opts.Secret != secret {
		t.Errorf("got secret %q, want %q", opts.Secret, secret)
	}
	if opts.Exec != "" {
		t.Errorf("got command %q, want the default shell", opts.Exec)
	}
	if opts.Listen {
		t.Error("a connecting peer was configured as a server")
	}

	// A command after the secret is joined back together for the shell of
	// the remote peer, the way an ssh client does it.
	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"ssh", "qs://MySecret", "app.exe"}, "app.exe"},
		{[]string{"ssh", "qs://MySecret", "uname -a; id"}, "uname -a; id"},
		{[]string{"ssh", "qs://MySecret", "ls", "-la", "/tmp"}, "ls -la /tmp"},
		// Flags past the command belong to the command itself.
		{[]string{"ssh", "qs://MySecret", "ls", "--color=always"}, "ls --color=always"},
		{[]string{"ssh", "qs://MySecret", "sh", "-c", "id"}, "sh -c id"},
	} {
		if got := configure(t, c.args...).Exec; got != c.want {
			t.Errorf("qsock %s: got command %q, want %q", strings.Join(c.args, " "), got, c.want)
		}
	}
}

func TestConfigureForwards(t *testing.T) {
	opts := configure(t,
		"ssh", "-N",
		"-L", "2222:localhost:22",
		"-L", "127.0.0.1:8080:intra.example.com:80",
		"qs://MySecret",
	)
	if !opts.NoExec {
		t.Error("-N did not suppress the remote command")
	}
	if len(opts.Forwards) != 2 {
		t.Fatalf("got %d forwards, want 2", len(opts.Forwards))
	}
	if got := opts.Forwards[0].ListenAddr(); got != ":2222" {
		t.Errorf("got listen address %q, want %q", got, ":2222")
	}
	if got := opts.Forwards[0].DialAddr(); got != "localhost:22" {
		t.Errorf("got dial address %q, want %q", got, "localhost:22")
	}
	if got := opts.Forwards[1].ListenAddr(); got != "127.0.0.1:8080" {
		t.Errorf("got listen address %q, want %q", got, "127.0.0.1:8080")
	}
}

func TestConfigureSocksIsRejected(t *testing.T) {
	// The flag is parsed, the feature is still in development.
	if _, err := Configure([]string{"ssh", "-D", "1080", "qs://MySecret"}, false); err == nil {
		t.Error("-D was accepted")
	}
}

func TestConfigureSCPUpload(t *testing.T) {
	opts := configure(t, "scp", "./local.txt", "qs://MySecret:/target/")
	if opts.Cmd != session.CMD_SCP {
		t.Errorf("got command %s, want scp", opts.Cmd)
	}
	if !opts.Upload {
		t.Error("a remote destination was not recognized as an upload")
	}
	if opts.LocalPath != "./local.txt" || opts.RemotePath != "/target/" {
		t.Errorf("got %q -> %q, want %q -> %q",
			opts.LocalPath, opts.RemotePath, "./local.txt", "/target/")
	}
}

func TestConfigureSCPDownload(t *testing.T) {
	opts := configure(t, "scp", "-r", "qs://MySecret:/target", ".")
	if opts.Upload {
		t.Error("a remote source was not recognized as a download")
	}
	if !opts.Recursive {
		t.Error("-r did not enable the recursive transfer")
	}
	if opts.RemotePath != "/target" || opts.LocalPath != "." {
		t.Errorf("got %q -> %q, want %q -> %q",
			opts.RemotePath, opts.LocalPath, "/target", ".")
	}
}

func TestConfigureSCPErrors(t *testing.T) {
	for _, args := range [][]string{
		{"scp", "local.txt"},                    // no destination
		{"scp", "a.txt", "b.txt"},               // no remote peer
		{"scp", "qs://a:/x", "qs://b:/y"},       // two remote peers
		{"scp", "-l", "qs://MySecret", "extra"}, // server mode takes one operand
	} {
		if _, err := Configure(args, false); err == nil {
			t.Errorf("qsock %s was accepted", strings.Join(args, " "))
		}
	}
}

func TestConfigureServerMode(t *testing.T) {
	secret, err := NewSecret("MySecret")
	if err != nil {
		t.Fatal(err)
	}

	// A listening peer serves every session type, so the subcommand is
	// optional and a raw channel is the default.
	opts := configure(t, "-l", "qs://MySecret")
	if !opts.Listen || opts.Cmd != session.CMD_NETCAT || opts.Secret != secret {
		t.Errorf("got %+v, want a listening netcat peer with secret %q", opts, secret)
	}

	// Naming one still works and its command becomes the server default.
	opts = configure(t, "-l", "ssh", "qs://MySecret", "/bin/zsh")
	if !opts.Listen || opts.Exec != "/bin/zsh" {
		t.Errorf("got listen=%t command=%q, want true and %q", opts.Listen, opts.Exec, "/bin/zsh")
	}

	// A bare URI connects as a raw channel, as the secret only
	// invocations of the previous releases did.
	opts = configure(t, "qs://MySecret")
	if opts.Listen || opts.Cmd != session.CMD_NETCAT {
		t.Errorf("got listen=%t command=%s, want false and netcat", opts.Listen, opts.Cmd)
	}

	// Scp only needs the secret in server mode.
	opts = configure(t, "-l", "scp", "qs://MySecret")
	if !opts.Listen || opts.Secret != secret {
		t.Errorf("got listen=%t secret=%q, want true and %q", opts.Listen, opts.Secret, secret)
	}
}

func TestConfigurePlainSecret(t *testing.T) {
	secret, err := NewSecret("MySecret")
	if err != nil {
		t.Fatal(err)
	}

	// The qs:// scheme is optional wherever a secret is expected, a bare
	// value is taken as a plain ASCII secret.
	for _, args := range [][]string{
		{"MySecret"},
		{"-l", "MySecret"},
		{"ssh", "MySecret"},
		{"netcat", "MySecret"},
		{"ssh", "-N", "-L", "2222:localhost:22", "MySecret"},
		{"-l", "scp", "MySecret"},
	} {
		if got := configure(t, args...).Secret; got != secret {
			t.Errorf("qsock %s: got secret %q, want %q",
				strings.Join(args, " "), got, secret)
		}
	}

	// A mistyped scheme is rejected instead of being hashed into a secret
	// of its own, and a mistyped subcommand is still reported as one.
	for _, args := range [][]string{
		{"qs:/MySecret"},
		{"http://MySecret"},
		{"ssh", "qs:/MySecret"},
		{"sshh", "MySecret"},
	} {
		if _, err := Configure(args, false); err == nil {
			t.Errorf("qsock %s was accepted", strings.Join(args, " "))
		}
	}
}

func TestConfigureGlobals(t *testing.T) {
	opts := configure(t, "-C", "-n", "11", "--cert-fp", "deadbeef", "netcat", "qs://MySecret")
	if !opts.DisableEnc {
		t.Error("-C did not disable the encryption")
	}
	if opts.End2End {
		t.Error("-C left the E2E layer on")
	}
	if opts.ProbeInterval != 11 {
		t.Errorf("got probe interval %d, want 11", opts.ProbeInterval)
	}
	if opts.CertFingerprint != "deadbeef" {
		t.Errorf("got fingerprint %q, want %q", opts.CertFingerprint, "deadbeef")
	}

	if opts = configure(t, "-T", "netcat", "qs://MySecret"); opts.SocksAddr != TOR_SOCKS_ADDR {
		t.Errorf("got proxy %q, want %q", opts.SocksAddr, TOR_SOCKS_ADDR)
	}
	if opts = configure(t, "netcat", "qs://MySecret"); !opts.End2End {
		t.Error("the E2E layer is not on by default")
	}
}

func TestConfigureUnknownCommand(t *testing.T) {
	_, err := Configure([]string{"sshh", "qs://MySecret"}, false)
	if err == nil {
		t.Fatal("an unknown command was accepted")
	}
	// The diagnostic points at both of the things it could have been.
	if !strings.Contains(err.Error(), "ssh, scp, netcat") {
		t.Errorf("got %q, want the subcommands listed", err)
	}
}

func TestConfigureRelay(t *testing.T) {
	// The value of a global flag is not mistaken for the subcommand.
	opts := configure(t,
		"--relay-server", "10.0.0.1:1337",
		"--relay-sni", "www.google.com",
		"ssh", "qs://MySecret",
	)
	if opts.Cmd != session.CMD_SSH {
		t.Errorf("got command %s, want ssh", opts.Cmd)
	}
	if got := opts.Relay.String(); got != "10.0.0.1:1337" {
		t.Errorf("got relay %q, want %q", got, "10.0.0.1:1337")
	}
	if opts.RelaySNI != "www.google.com" {
		t.Errorf("got SNI %q, want %q", opts.RelaySNI, "www.google.com")
	}

	// Both flags are optional, the public gate is dialed without them.
	opts = configure(t, "netcat", "qs://MySecret")
	if !opts.Relay.IsZero() || opts.RelaySNI != "" || opts.GateSNI() != "" {
		t.Errorf("got relay %+v and SNI %q, want the defaults of qsocket-go",
			opts.Relay, opts.RelaySNI)
	}

	// A gate given without a port keeps the default ports, and presents
	// the certificate of its own name unless a name is spoofed.
	opts = configure(t, "--relay-server", "gate.example.com", "-l", "qs://MySecret")
	if opts.Relay.Port != 0 {
		t.Errorf("got port %d, want the default ports of the network", opts.Relay.Port)
	}
	if got := opts.GateSNI(); got != "gate.example.com" {
		t.Errorf("got SNI %q, want %q", got, "gate.example.com")
	}

	// A spoofed name on its own still dials the public gate.
	opts = configure(t, "--relay-sni", "www.google.com", "qs://MySecret")
	if !opts.Relay.IsZero() {
		t.Errorf("got relay %+v, want the public gate", opts.Relay)
	}
	if opts.GateSNI() != "www.google.com" {
		t.Errorf("got SNI %q, want %q", opts.GateSNI(), "www.google.com")
	}

	for _, args := range [][]string{
		{"--relay-server", "10.0.0.1:0", "qs://MySecret"},
		{"--relay-server", "https://gate.example.com", "qs://MySecret"},
		{"--relay-sni", "www.google.com:443", "qs://MySecret"},
	} {
		if _, err := Configure(args, false); err == nil {
			t.Errorf("qsock %s was accepted", strings.Join(args, " "))
		}
	}
}
