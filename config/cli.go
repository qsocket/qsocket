package config

import (
	"strings"

	"github.com/alecthomas/kong"
)

const USAGE_EXAMPLES = `
Examples:
  # Generate a new address.
  $ qsock -g

  # Listen for incoming sessions, serves every subcommand. (remote peer)
  $ qsock -l qs://<secret>

  # Open an interactive session. (the qs:// scheme is optional)
  $ qsock ssh qs://<secret>
  $ qsock ssh <secret>

  # Run a single command on the remote peer.
  $ qsock ssh qs://<secret> "uname -a; id"

  # Forward the local port 2222 to 192.168.6.7:22 through the remote peer.
  $ qsock ssh -L 2222:192.168.6.7:22 qs://<secret>

  # Copy a local file into the /tmp directory of the remote peer.
  $ qsock scp ./warez.tar.gz qs://<secret>:/tmp/

  # Copy a remote folder into the working directory.
  $ qsock scp -r qs://<secret>:/etc/nginx .

  # Open a raw data channel. (no PTY)
  $ qsock netcat qs://<secret> < warez.tar.gz

  # Dial a custom relay server, spoofing the TLS server name. (SNI)
  $ qsock --relay-server 10.0.0.1:443 --relay-sni www.google.com ssh qs://<secret>
`

// Globals are the options shared by every subcommand, they are passed before
// the subcommand name.
type Globals struct {
	CertFingerprint string           `help:"Hex encoded TLS certificate fingerprint for validation." name:"cert-fp" placeholder:"HEX"`
	RelayServer     string           `help:"Custom QSRN relay server address. (default:relay.qsocket.io)" name:"relay-server" placeholder:"IP:PORT"`
	RelaySNI        string           `help:"Spoof the TLS server name indicated to the relay server." name:"relay-sni" placeholder:"HOST"`
	ProbeInterval   int              `help:"Probe interval for connecting QSRN." name:"probe" short:"n" default:"5"`
	DisableEnc      bool             `help:"Disable all encryption." name:"plain" short:"C"`
	End2End         bool             `help:"Use E2E encryption. (default:true)" name:"e2e" default:"true"`
	Listen          bool             `help:"Server mode. (listen for connections)" name:"listen" short:"l"`
	RandomAddress   bool             `help:"Generate a new address. (random)" name:"generate" short:"g"`
	CertPinning     bool             `help:"Enable certificate pinning on TLS connections." name:"pin" short:"K"`
	Quiet           bool             `help:"Quiet mode. (no stdout)" name:"quiet" short:"q"`
	UseTor          bool             `help:"Use TOR for connecting QSRN." name:"tor" short:"T"`
	GenerateQR      bool             `help:"Generate a QR code with given stdin and print on the terminal." name:"qr"`
	Verbose         bool             `help:"Verbose mode." name:"verbose" short:"v"`
	Version         kong.VersionFlag `help:"Print the version and quit." name:"version"`
}

// SSHCmd is the grammar of the ssh subcommand.
type SSHCmd struct {
	Forwards  []string `help:"Forward a local port to a destination reached by the remote peer. (repeatable)" short:"L" name:"local-forward" sep:"none" placeholder:"[bind:]port:host:hostport"`
	SocksPort int      `help:"Open a SOCKS proxy on the given local port. (in development)" short:"D" name:"dynamic-forward" placeholder:"PORT"`
	NoExec    bool     `help:"Do not execute a remote command, implied by -L. (port forwarding only)" short:"N" name:"no-exec"`
	URI       string   `arg:"" optional:"" name:"uri" help:"QSocket secret URI or plain secret. (qs://<secret>)"`
	Exec      []string `arg:"" optional:"" passthrough:"" name:"command" help:"Command to execute on the remote peer."`
}

// SCPCmd is the grammar of the scp subcommand.
//
// Exactly one of the source and the destination carries a qs:// URI, it picks
// the direction of the transfer. In server mode the paths are chosen by the
// connecting peer, hence only the secret is given.
type SCPCmd struct {
	Recursive   bool   `help:"Recursively copy entire directories." short:"r" name:"recursive"`
	Source      string `arg:"" optional:"" name:"source" help:"Local path or qs://<secret>:/path"`
	Destination string `arg:"" optional:"" name:"destination" help:"Local path or qs://<secret>:/path"`
}

// NetcatCmd is the grammar of the netcat subcommand.
type NetcatCmd struct {
	URI  string   `arg:"" optional:"" name:"uri" help:"QSocket secret URI or plain secret. (qs://<secret>)"`
	Exec []string `arg:"" optional:"" passthrough:"" name:"command" help:"Command to execute on the remote peer."`
}

// CLI is the full command line grammar of the qsock binary.
type CLI struct {
	Globals

	SSH    SSHCmd    `cmd:"" name:"ssh" help:"Start an interactive session on the remote peer."`
	SCP    SCPCmd    `cmd:"" name:"scp" help:"Copy files and folders to/from the remote peer."`
	Netcat NetcatCmd `cmd:"" name:"netcat" help:"Open a raw data channel to the remote peer."`
}

// ServerCLI is the reduced grammar used when no subcommand is given.
//
// The listening peer serves whichever session the connecting peer asks for,
// so naming a subcommand carries no meaning there and `qsock -l qs://<secret>`
// is the canonical way of starting a server.
type ServerCLI struct {
	Globals

	URI string `arg:"" optional:"" name:"uri" help:"QSocket secret URI or plain secret. (qs://<secret>)"`
}

// commandNames are the subcommands accepted on the command line, in the
// order they are listed on the help page.
var commandNames = []string{"ssh", "scp", "netcat"}

// CommandNames returns the names of the subcommands.
func CommandNames() []string {
	return commandNames
}

// valueFlags are the global flags consuming the argument following them, they
// are skipped while looking for the subcommand.
var valueFlags = map[string]bool{
	"--cert-fp":      true,
	"--probe":        true,
	"-n":             true,
	"--relay-server": true,
	"--relay-sni":    true,
}

// firstBareArg returns the first argument which is neither a flag nor the
// value of one, it is the subcommand name when the invocation names one.
func firstBareArg(args []string) string {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "-") && arg != "-" {
			if valueFlags[arg] {
				i++
			}
			continue
		}
		return arg
	}
	return ""
}

// countBareArgs returns the number of arguments which are neither a flag nor
// the value of one.
func countBareArgs(args []string) int {
	count := 0
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "-") && arg != "-" {
			if valueFlags[arg] {
				i++
			}
			continue
		}
		count++
	}
	return count
}

// IsCommand reports whether the given value names one of the subcommands.
func IsCommand(name string) bool {
	for _, cmd := range commandNames {
		if cmd == name {
			return true
		}
	}
	return false
}

// hasHelpFlag reports whether the arguments ask for the help page.
func hasHelpFlag(args []string) bool {
	for _, arg := range args {
		if arg == "-h" || arg == "--help" {
			return true
		}
	}
	return false
}

// helpPrinter prints the standard kong help page followed by the usage
// examples of the qsock binary.
func helpPrinter(options kong.HelpOptions, ctx *kong.Context) error {
	if err := kong.DefaultHelpPrinter(options, ctx); err != nil {
		return err
	}
	// The examples are only useful on the top level help page, the
	// subcommand pages document their own options.
	if ctx.Selected() == nil {
		print(USAGE_EXAMPLES)
	}
	return nil
}

// newParser builds a kong parser for the given grammar.
func newParser(grammar any, description string) (*kong.Kong, error) {
	return kong.New(
		grammar,
		kong.Name("qsock"),
		kong.Description(description),
		kong.Help(helpPrinter),
		kong.UsageOnError(),
		kong.Vars{"version": Version},
		kong.ConfigureHelp(kong.HelpOptions{Summary: true}),
	)
}
