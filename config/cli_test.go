package config

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/qsocket/qsocket/log"
	"github.com/qsocket/qsocket/session"
)

// testFP is a well formed SPKI fingerprint. (SHA-256 of "test")
const testFP = "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"

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

func TestConfigureSchemeIsMandatory(t *testing.T) {
	// The qs:// scheme is mandatory wherever a secret is expected, a UUID
	// or a plain secret carrying no scheme is rejected, as is a mistyped
	// scheme or subcommand.
	uuid := "f81d4fae-7dec-41d0-a765-00a0c91e6bf6"
	for _, args := range [][]string{
		{"MySecret"},
		{uuid},
		{"-l", "MySecret"},
		{"-l", uuid},
		{"ssh", "MySecret"},
		{"ssh", uuid},
		{"netcat", "MySecret"},
		{"ssh", "-N", "-L", "2222:localhost:22", "MySecret"},
		{"-l", "scp", "MySecret"},
		{"-l", "ssh", uuid, "/bin/zsh"},
		{"qs:/MySecret"},
		{"http://MySecret"},
		{"ssh", "qs:/MySecret"},
		{"sshh", "MySecret"},
	} {
		if _, err := Configure(args, false); err == nil {
			t.Errorf("qsock %s was accepted", strings.Join(args, " "))
		}
	}

	// The same secrets are accepted in their URI form.
	for _, value := range []string{"MySecret", uuid} {
		want, err := NewSecret(value)
		if err != nil {
			t.Fatal(err)
		}
		for _, args := range [][]string{
			{URI_SCHEME + value},
			{"-l", URI_SCHEME + value},
			{"ssh", URI_SCHEME + value},
			{"netcat", URI_SCHEME + value},
			{"-l", "scp", URI_SCHEME + value},
		} {
			if got := configure(t, args...).Secret; got != want {
				t.Errorf("qsock %s: got secret %q, want %q",
					strings.Join(args, " "), got, want)
			}
		}
	}
}

func TestConfigureGlobals(t *testing.T) {
	opts := configure(t, "-C", "-n", "11", "netcat", "qs://MySecret")
	if !opts.DisableEnc {
		t.Error("-C did not disable the encryption")
	}
	if opts.End2End {
		t.Error("-C left the E2E layer on")
	}
	if opts.TLS {
		t.Error("-C left the TLS layer on")
	}
	if opts.ProbeInterval != 11 {
		t.Errorf("got probe interval %d, want 11", opts.ProbeInterval)
	}
	if opts = configure(t, "--cert-fp", testFP, "netcat", "qs://MySecret"); opts.CertFingerprint != testFP {
		t.Errorf("got fingerprint %q, want %q", opts.CertFingerprint, testFP)
	}

	if opts = configure(t, "-T", "netcat", "qs://MySecret"); opts.SocksAddr != TOR_SOCKS_ADDR {
		t.Errorf("got proxy %q, want %q", opts.SocksAddr, TOR_SOCKS_ADDR)
	}
	if opts = configure(t, "netcat", "qs://MySecret"); !opts.End2End {
		t.Error("the E2E layer is not on by default")
	}
	if !opts.TLS {
		t.Error("the TLS layer is not on by default")
	}
}

func TestConfigureEncryptionLayers(t *testing.T) {
	for _, tc := range []struct {
		args     []string
		tls, e2e bool
	}{
		{[]string{"--no-tls"}, false, true},
		{[]string{"--tls=false"}, false, true},
		{[]string{"--no-e2e"}, true, false},
		{[]string{"--e2e=false"}, true, false},
		{[]string{"--no-tls", "--no-e2e"}, false, false},
		{[]string{"--tls", "--e2e"}, true, true},
	} {
		args := append(tc.args, "netcat", "qs://MySecret")
		opts := configure(t, args...)
		if opts.TLS != tc.tls || opts.End2End != tc.e2e {
			t.Errorf("qsock %s: got tls=%t e2e=%t, want tls=%t e2e=%t",
				strings.Join(args, " "), opts.TLS, opts.End2End, tc.tls, tc.e2e)
		}
	}

	for _, args := range [][]string{
		{"--no-tls", "--cert-fp", testFP, "netcat", "qs://MySecret"},
		{"-C", "--cert-fp", testFP, "netcat", "qs://MySecret"},
		{"--no-tls", "--insecure-cert", "netcat", "qs://MySecret"},
		{"-C", "--insecure-cert", "netcat", "qs://MySecret"},
	} {
		if _, err := Configure(args, false); err != ErrCertWithoutTLS {
			t.Errorf("qsock %s: got %v, want %v", strings.Join(args, " "), err, ErrCertWithoutTLS)
		}
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

// captureStderr runs fn and returns what it wrote to the standard error.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	saved := os.Stderr
	os.Stderr = w
	defer func() { os.Stderr = saved }()
	fn()
	w.Close()
	out, _ := io.ReadAll(r)
	return string(out)
}

func TestConfigureCertificateFlags(t *testing.T) {
	defer log.SetLevel(log.LOG_LEVEL_INFO)

	// -K/--pin is removed, the fingerprint alone enables the check.
	for _, flag := range []string{"-K", "--pin"} {
		if _, err := Configure([]string{flag, "--cert-fp", testFP, "netcat", "qs://MySecret"}, false); err == nil {
			t.Errorf("qsock %s was accepted", flag)
		}
	}

	for _, fp := range []string{
		"deadbeef",
		testFP[:63],
		testFP + "0",
		"zz" + testFP[2:],
	} {
		if _, err := Configure([]string{"--cert-fp", fp, "netcat", "qs://MySecret"}, false); err != ErrInvalidCertFP {
			t.Errorf("--cert-fp %s: got %v, want %v", fp, err, ErrInvalidCertFP)
		}
	}

	opts := configure(t, "--cert-fp", testFP, "--insecure-cert", "netcat", "qs://MySecret")
	if opts.CertFingerprint != testFP || !opts.InsecureCert {
		t.Errorf("--cert-fp with --insecure-cert: got fp=%q insecure=%t", opts.CertFingerprint, opts.InsecureCert)
	}
}

func TestConfigureInsecureCertWarnings(t *testing.T) {
	defer log.SetLevel(log.LOG_LEVEL_INFO)

	for _, tc := range []struct {
		args        []string
		warn, alert bool
	}{
		{[]string{"--insecure-cert"}, true, false},
		{[]string{"--insecure-cert", "--no-e2e"}, true, true},
		// Quiet mode keeps the FATAL level, only the alert gets through.
		{[]string{"-q", "--insecure-cert"}, false, false},
		{[]string{"-q", "--insecure-cert", "--no-e2e"}, false, true},
		// A pinned fingerprint takes precedence, nothing to warn about.
		{[]string{"--cert-fp", testFP, "--insecure-cert", "--no-e2e"}, false, false},
		{[]string{"--no-e2e"}, false, false},
	} {
		log.SetLevel(log.LOG_LEVEL_INFO)
		args := append(tc.args, "netcat", "qs://MySecret")
		out := captureStderr(t, func() {
			if _, err := Configure(args, false); err != nil {
				t.Errorf("qsock %s: %v", strings.Join(args, " "), err)
			}
		})
		if got := strings.Contains(out, MSG_INSECURE_CERT); got != tc.warn {
			t.Errorf("qsock %s: warning printed=%t, want %t", strings.Join(args, " "), got, tc.warn)
		}
		if got := strings.Contains(out, MSG_INSECURE_PLAIN); got != tc.alert {
			t.Errorf("qsock %s: alert printed=%t, want %t", strings.Join(args, " "), got, tc.alert)
		}
	}
}
