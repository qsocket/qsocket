package config

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/mdp/qrterminal/v3"
	"github.com/qsocket/qsocket/log"
	"github.com/qsocket/qsocket/session"
	"github.com/qsocket/qsocket/utils"
)

const (
	// DESCRIPTION is the summary printed on the top level help page.
	DESCRIPTION = "Reads and writes E2E encrypted data across systems using the QSocket relay network. (QSRN)"
	// DEFAULT_E2E_CIPHER names the cipher suite of E2E encrypted sessions.
	DEFAULT_E2E_CIPHER = "SRP-AES-GCM-256-E2E (Prime: 4096)"
	// TOR_SOCKS_ADDR is the local TOR SOCKS proxy used by the --tor flag.
	TOR_SOCKS_ADDR = "127.0.0.1:9050"
)

var (
	// Version is the release of the binary, it is set on build time.
	Version = "?"
	// NoPrompt disables all interactive terminal prompts, it is used when
	// the process has no console attached such as when running as a
	// Windows service.
	NoPrompt = false

	ErrNoSecret          = errors.New("No secret specified.")
	ErrSocksNotSupported = errors.New("SOCKS proxy support is not implemented yet, this feature is still in development!")
	ErrSCPMissingArgs    = errors.New("scp: missing source or destination operand")
	ErrSCPBothRemote     = errors.New("scp: copying between two remote peers is not supported")
	ErrSCPNoRemote       = errors.New("scp: either the source or the destination needs to be a qs:// URI")
	ErrSCPServerArgs     = errors.New("scp: server mode takes a single qs:// URI, the paths are chosen by the connecting peer")
)

// Options is the resolved configuration of a single qsock execution.
type Options struct {
	// Cmd is the session type requested from the remote peer, in server
	// mode it is the session type served by default.
	Cmd session.Command
	// Secret is the canonical UUID form of the QSocket secret.
	Secret string
	// Exec is the command line executed on the remote peer, an empty value
	// spawns the default shell of the remote peer.
	Exec string

	// Forwards are the local port forwarding specifications. (ssh -L)
	Forwards []Forward
	// NoExec keeps an ssh session up for its forwarded ports only. (ssh -N)
	NoExec bool

	// Recursive enables recursive file transfers. (scp -r)
	Recursive bool
	// Upload marks a file transfer where this peer is the sender.
	Upload bool
	// RemotePath is the file transfer path on the remote peer.
	RemotePath string
	// LocalPath is the file transfer path on this peer.
	LocalPath string

	// CertFingerprint is the expected TLS certificate fingerprint of the
	// relay network gate.
	CertFingerprint string
	// Relay is the address of a custom relay network gate, a zero value
	// dials the public QSRN gate.
	Relay Relay
	// RelaySNI is the server name indicated to the gate during the TLS
	// handshake, it does not need to be the name of the gate itself.
	RelaySNI string
	// ProbeInterval is the number of seconds waited between two probes.
	ProbeInterval int
	// DisableEnc turns off every layer of encryption.
	DisableEnc bool
	// End2End turns on the end to end encrypted session layer.
	End2End bool
	// Listen runs the process in server mode.
	Listen bool
	// RandomAddress requests a randomly generated secret.
	RandomAddress bool
	// CertPinning enables certificate pinning on the TLS connections.
	CertPinning bool
	// Quiet silences every non fatal message.
	Quiet bool
	// UseTor reaches the relay network over the local TOR proxy.
	UseTor bool
	// Verbose turns on the trace level logging.
	Verbose bool
	// SocksAddr is the proxy address used for reaching the relay network.
	SocksAddr string

	// InPipe and OutPipe carry the redirected standard streams of a netcat
	// session started with a shell pipe.
	InPipe  *os.File
	OutPipe *os.File
}

// ConfigureOptions parses the command line and returns the resolved options.
//
// The QS_ARGS environment variable overrides the process arguments, it lets
// the binary carry its configuration when the command line is not under the
// control of the user, such as a service started by the Windows SCM.
func ConfigureOptions() (*Options, error) {
	if os.Args == nil {
		return nil, errors.New("os.Args is nil")
	}

	qsArgs := os.Getenv("QS_ARGS")
	args := os.Args[1:]
	if qsArgs != "" {
		args = strings.Fields(qsArgs)
	}
	// Smart pipes are disabled when the arguments arrive over the
	// environment, a detached process has no meaningful standard streams.
	return Configure(args, qsArgs == "")
}

// Configure resolves the options of the given argument list.
//
// Pipes are only picked up when allowPipes is set, the standard streams of a
// process which was handed its arguments over the environment carry nothing
// of interest.
func Configure(args []string, allowPipes bool) (*Options, error) {
	if len(args) == 0 {
		if NoPrompt {
			return nil, ErrNoSecret
		}
		// Nothing to work with, print the help page and quit.
		args = []string{"--help"}
	}

	// The full grammar is only needed when a subcommand is named, the help
	// page is an exception since it is the listing of those subcommands.
	bare := firstBareArg(args)
	if IsCommand(bare) || (bare == "" && hasHelpFlag(args)) {
		return configureCommand(args, allowPipes)
	}
	// Anything else is the secret of a subcommand-less invocation, either
	// a qs:// URI or a plain secret. A subcommand-less invocation carries
	// nothing but its secret, so a second bare argument means the first
	// one was meant to name a subcommand.
	if bare != "" && !IsURI(bare) && countBareArgs(args) > 1 {
		return nil, fmt.Errorf(
			"Unexpected argument %q, expected a QSocket secret (%s<secret>) or one of: %s",
			bare, URI_SCHEME, strings.Join(CommandNames(), ", "),
		)
	}
	return configureServer(args, allowPipes)
}

// configureCommand resolves an invocation naming one of the subcommands.
func configureCommand(args []string, allowPipes bool) (*Options, error) {
	cli := &CLI{}
	parser, err := newParser(cli, DESCRIPTION)
	if err != nil {
		return nil, err
	}
	ctx, err := parser.Parse(args)
	if err != nil {
		return nil, err
	}

	opts, err := newOptions(&cli.Globals)
	if err != nil {
		return nil, err
	}
	switch ctx.Command() {
	case "ssh", "ssh <uri>", "ssh <uri> <command>":
		err = opts.resolveSSH(&cli.SSH)
	case "netcat", "netcat <uri>", "netcat <uri> <command>":
		err = opts.resolveNetcat(&cli.Netcat)
	case "scp", "scp <source>", "scp <source> <destination>":
		err = opts.resolveSCP(&cli.SCP)
	default:
		err = fmt.Errorf("Unexpected command: %s", ctx.Command())
	}
	if err != nil {
		return nil, err
	}
	return opts, opts.finalize(allowPipes)
}

// configureServer resolves an invocation carrying no subcommand.
//
// The session type is picked by the connecting peer, so a listening peer
// needs nothing but the secret. A connecting peer defaults to a raw netcat
// channel, which is the behaviour of the secret only invocations of the
// previous releases.
func configureServer(args []string, allowPipes bool) (*Options, error) {
	cli := &ServerCLI{}
	parser, err := newParser(cli, DESCRIPTION)
	if err != nil {
		return nil, err
	}
	if _, err = parser.Parse(args); err != nil {
		return nil, err
	}

	opts, err := newOptions(&cli.Globals)
	if err != nil {
		return nil, err
	}
	opts.Cmd = session.CMD_NETCAT
	if err = opts.setSecret(cli.URI); err != nil {
		return nil, err
	}
	return opts, opts.finalize(allowPipes)
}

// newOptions copies the global flags into a new options structure and applies
// the ones changing the behaviour of the process itself.
func newOptions(g *Globals) (*Options, error) {
	opts := &Options{
		CertFingerprint: g.CertFingerprint,
		ProbeInterval:   g.ProbeInterval,
		DisableEnc:      g.DisableEnc,
		End2End:         g.End2End,
		Listen:          g.Listen,
		RandomAddress:   g.RandomAddress,
		CertPinning:     g.CertPinning,
		Quiet:           g.Quiet,
		UseTor:          g.UseTor,
		Verbose:         g.Verbose,
	}

	if opts.Verbose {
		log.SetLevel(log.LOG_LEVEL_TRACE) // Show all the shit!
	}
	if opts.Quiet {
		log.SetLevel(log.LOG_LEVEL_FATAL) // Show nothing!
	}
	if opts.UseTor {
		opts.SocksAddr = TOR_SOCKS_ADDR
	}
	if opts.DisableEnc {
		opts.End2End = false
	}

	if g.RelayServer != "" {
		relay, err := ParseRelay(g.RelayServer)
		if err != nil {
			return nil, err
		}
		opts.Relay = relay
	}
	if g.RelaySNI != "" {
		sni, err := ParseSNI(g.RelaySNI)
		if err != nil {
			return nil, err
		}
		opts.RelaySNI = sni
	}

	// A secret generated for a connecting peer is of no use to anyone, so
	// the flag only prints one and quits. The QR code is printed alongside
	// it instead of consuming the standard input.
	if opts.RandomAddress && !opts.Listen {
		secret, err := RandomUUID()
		if err != nil {
			log.Error(err)
			os.Exit(1)
		}
		printSecret(secret, g.GenerateQR)
		os.Exit(0)
	}
	if g.GenerateQR {
		printStdinQR()
		os.Exit(0)
	}
	return opts, nil
}

// GateSNI returns the server name indicated to the relay network gate during
// the TLS handshake.
//
// A spoofed name takes precedence, otherwise a custom gate is asked for the
// certificate of its own name and an empty value keeps the default of
// qsocket-go for the public network.
func (opts *Options) GateSNI() string {
	if opts.RelaySNI != "" {
		return opts.RelaySNI
	}
	return opts.Relay.Host
}

// resolveSSH resolves the parameters of the ssh subcommand.
func (opts *Options) resolveSSH(cmd *SSHCmd) error {
	opts.Cmd = session.CMD_SSH
	opts.NoExec = cmd.NoExec
	if cmd.SocksPort != 0 {
		return ErrSocksNotSupported
	}
	for _, spec := range cmd.Forwards {
		fwd, err := ParseForward(spec)
		if err != nil {
			return err
		}
		opts.Forwards = append(opts.Forwards, fwd)
	}
	// The command is joined back together and executed by the shell of the
	// remote peer, just as an ssh client would.
	opts.Exec = strings.Join(cmd.Exec, " ")
	return opts.setSecret(cmd.URI)
}

// resolveNetcat resolves the parameters of the netcat subcommand.
func (opts *Options) resolveNetcat(cmd *NetcatCmd) error {
	opts.Cmd = session.CMD_NETCAT
	opts.Exec = strings.Join(cmd.Exec, " ")
	return opts.setSecret(cmd.URI)
}

// resolveSCP resolves the parameters of the scp subcommand, the position of
// the qs:// URI picks the direction of the transfer.
func (opts *Options) resolveSCP(cmd *SCPCmd) error {
	opts.Cmd = session.CMD_SCP
	opts.Recursive = cmd.Recursive

	if opts.Listen {
		if cmd.Destination != "" {
			return ErrSCPServerArgs
		}
		return opts.setSecret(cmd.Source)
	}

	if cmd.Source == "" || cmd.Destination == "" {
		return ErrSCPMissingArgs
	}

	var (
		err                  error
		srcRemote, dstRemote = IsURI(cmd.Source), IsURI(cmd.Destination)
	)
	switch {
	case srcRemote && dstRemote:
		return ErrSCPBothRemote
	case !srcRemote && !dstRemote:
		return ErrSCPNoRemote
	case srcRemote:
		opts.Upload = false
		opts.LocalPath = cmd.Destination
		opts.Secret, opts.RemotePath, err = ParseRemote(cmd.Source)
	default:
		opts.Upload = true
		opts.LocalPath = cmd.Source
		opts.Secret, opts.RemotePath, err = ParseRemote(cmd.Destination)
	}
	return err
}

// setSecret resolves the QSocket secret of the session. Without a URI the
// secret is either generated or read from the terminal.
func (opts *Options) setSecret(uri string) error {
	if uri != "" {
		secret, err := ParseURI(uri)
		if err != nil {
			return err
		}
		opts.Secret = secret
		return nil
	}

	if opts.RandomAddress {
		secret, err := RandomUUID()
		if err != nil {
			return err
		}
		opts.Secret = secret
		return nil
	}

	// A randomly generated secret is useless when there is no console to
	// print it on, so it needs to be supplied via arguments or QS_ARGS.
	if NoPrompt || utils.IsFilePiped(os.Stdin) {
		return ErrNoSecret
	}
	return opts.promptSecret()
}

// promptSecret reads the secret from the terminal, an empty answer generates
// a random one.
func (opts *Options) promptSecret() error {
	color.New(color.FgBlue).Add(color.Bold).Print("[>] ")
	print("Enter secret (or press Enter to generate): ")
	answer := ""
	if n, _ := fmt.Scanln(&answer); n == 0 {
		secret, err := RandomUUID()
		if err != nil {
			return err
		}
		opts.Secret = secret
		opts.RandomAddress = true
		return nil
	}

	// Both the bare secret and the full URI are accepted, the prompt is
	// already unambiguous about what it is asking for.
	secret, err := ParseURI(strings.TrimSpace(answer))
	opts.Secret = secret
	return err
}

// finalize applies the option processing shared by every invocation.
func (opts *Options) finalize(allowPipes bool) error {
	if opts.Secret == "" {
		return ErrNoSecret
	}
	// Only a raw data channel carries the standard streams of the process,
	// the other sessions use the terminal or the local filesystem.
	if allowPipes && opts.Cmd == session.CMD_NETCAT {
		if utils.IsFilePiped(os.Stdin) {
			opts.InPipe = os.Stdin
		}
		if utils.IsFilePiped(os.Stdout) {
			opts.OutPipe = os.Stdout
			utils.EnableSmartPipe()
		}
	}
	return nil
}

// printSecret prints a generated secret in its URI form.
func printSecret(secret string, qr bool) {
	uri := URI(secret)
	fmt.Print(uri)
	if qr {
		printQR(uri)
	}
}

// printStdinQR renders the standard input as a QR code.
func printStdinQR() {
	in, err := io.ReadAll(os.Stdin)
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
	printQR(string(in))
}

// printQR renders the given value as a QR code on the terminal.
func printQR(value string) {
	qrterminal.GenerateWithConfig(value, qrterminal.Config{
		Level:     qrterminal.M,
		Writer:    os.Stdout,
		BlackChar: qrterminal.WHITE,
		WhiteChar: qrterminal.BLACK,
		QuietZone: 1,
	})
}

// IsPiped reports whether the session carries a redirected standard stream.
func (opts *Options) IsPiped() bool {
	return opts.InPipe != nil || opts.OutPipe != nil
}

// URI returns the qs:// URI of the session secret.
func (opts *Options) URI() string {
	return URI(opts.Secret)
}

// Summarize prints the session parameters on the terminal.
func (opts *Options) Summarize() {
	if opts == nil || opts.Quiet {
		return
	}

	mode := "client"
	if opts.Listen {
		mode = "server"
	}
	encType := DEFAULT_E2E_CIPHER
	if opts.DisableEnc {
		encType = utils.Red.Sprintf("DISABLED")
	} else if !opts.End2End {
		encType = "TLS"
	}

	fmt.Printf("%s %s\n", utils.BoldYellow.Sprintf("[#]"), utils.BoldBlue.Sprintf(".::QSocket::."))
	fmt.Printf("%s %s\n", utils.Yellow.Sprintf(" ├──>"), utils.Red.Sprintf("%s", opts.URI()))
	fmt.Printf("%s Mode: %s\n", utils.Yellow.Sprintf(" ├──>"), mode)
	if !opts.Listen {
		fmt.Printf("%s Session: %s\n", utils.Yellow.Sprintf(" ├──>"), opts.Cmd)
	}
	fmt.Printf("%s TOR: %t\n", utils.Yellow.Sprintf(" ├──>"), opts.UseTor)
	if !opts.Relay.IsZero() {
		fmt.Printf("%s Relay: %s\n", utils.Yellow.Sprintf(" ├──>"), opts.Relay)
	}
	if opts.RelaySNI != "" {
		fmt.Printf("%s SNI: %s\n", utils.Yellow.Sprintf(" ├──>"), opts.RelaySNI)
	}
	for _, fwd := range opts.Forwards {
		fmt.Printf("%s Forward: %s\n", utils.Yellow.Sprintf(" ├──>"), fwd)
	}
	if opts.Cmd == session.CMD_SCP && !opts.Listen {
		direction := fmt.Sprintf("%s <- %s", opts.LocalPath, opts.RemotePath)
		if opts.Upload {
			direction = fmt.Sprintf("%s -> %s", opts.LocalPath, opts.RemotePath)
		}
		fmt.Printf("%s Transfer: %s\n", utils.Yellow.Sprintf(" ├──>"), direction)
	}
	if opts.Listen {
		fmt.Printf("%s Probe Interval: %s\n", utils.Yellow.Sprintf(" ├──>"), time.Second*time.Duration(opts.ProbeInterval))
	} else {
		fmt.Printf("%s Probe Duration: %s\n", utils.Yellow.Sprintf(" ├──>"), time.Second*time.Duration(opts.ProbeInterval))
	}
	if opts.IsPiped() {
		fmt.Printf("%s Pipe: true\n", utils.Yellow.Sprintf(" ├──>"))
	}
	fmt.Printf("%s Encryption: %s\n", utils.Yellow.Sprintf(" └──>"), encType)
	print("\n")
}
