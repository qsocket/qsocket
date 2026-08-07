package config

import (
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/alecthomas/kong"
	"github.com/fatih/color"
	"github.com/mdp/qrterminal/v3"
	"github.com/qsocket/qs-netcat/log"
	"github.com/qsocket/qs-netcat/utils"
)

const (
	USAGE_EAMPLES = `
Example to forward traffic from port 2222 to 192.168.6.7:22:
  $ qs-netcat -s MyCecret -f 2222:192.168.6.7:22
Example file transfer:
	$ qs-netcat -l -s MyCecret > warez.tar.gz         # Server
	$ qs-netcat -s MyCecret < warez.tar.gz            # Client
Example for a reverse shell:
	$ qs-netcat -s MyCecret -l -i                     # Server
	$ qs-netcat -s MyCecret -i                        # Client
`
	DEFAULT_E2E_CIPHER = "SRP-AES-GCM-256-E2E (Prime: 4096)"
)

var (
	ForwardAddrRgx = regexp.MustCompile(`(?P<lport>([0-9]{1,5}):|)(?P<rhost>(?:[0-9]{1,3}\.){3}[0-9]{1,3}):(?P<rport>[0-9]{1,5})`)
	Version        = "?"
	// NoPrompt disables all interactive terminal prompts, it is used when the
	// process has no console attached such as when running as a Windows service.
	NoPrompt = false
)

// Main config struct for parsing the TOML file
type Options struct {
	Secret          string   `help:"Secret (e.g. password)." name:"secret" short:"s"`
	Execute         string   `help:"Execute command [e.g. \"bash -il\" or \"cmd.exe\"]" name:"exec" short:"e"`
	ForwardAddr     string   `help:"IP:PORT or PORT:IP:PORT for port forwarding." name:"forward" short:"f"`
	SocksAddr       string   `help:"User socks proxy address for connecting QSRN." name:"socks" short:"x"`
	CertFingerprint string   `help:"Hex encoded TLS certificate fingerprint for validation." name:"cert-fp"`
	ProbeInterval   int      `help:"Probe interval for connecting QSRN." name:"probe" short:"n" default:"5"`
	DisableEnc      bool     `help:"Disable all encryption." name:"plain" short:"C"`
	End2End         bool     `help:"Use E2E encryption. (default:true)" name:"e2e" default:"true"`
	Interactive     bool     `help:"Execute with a PTY shell." name:"interactive" short:"i"`
	Listen          bool     `help:"Server mode. (listen for connections)" name:"listen" short:"l"`
	RandomSecret    bool     `help:"Generate a Secret. (random)" name:"generate" short:"g"`
	CertPinning     bool     `help:"Enable certificate pinning on TLS connections." name:"pin" short:"K"`
	Quiet           bool     `help:"Quiet mode. (no stdout)" name:"quiet" short:"q"`
	UseTor          bool     `help:"Use TOR for connecting QSRN." name:"tor" short:"T"`
	GenerateQR      bool     `help:"Generate a QR code with given stdin and print on the terminal." name:"qr"`
	Verbose         bool     `help:"Verbose mode." name:"verbose" short:"v"`
	LocalPort       string   `kong:"-"`
	InPipe          *os.File `kong:"-"`
	OutPipe         *os.File `kong:"-"`
	Version         kong.VersionFlag
}

func HelpPrompt(options kong.HelpOptions, ctx *kong.Context) error {
	err := kong.DefaultHelpPrinter(options, ctx)
	if err != nil {
		return err
	}
	print(USAGE_EAMPLES)
	return nil
}

// ConfigureOptions accepts a flag set and augments it with agentgo-server
// specific flags. On success, an options structure is returned configured
// based on the selected flags.
func ConfigureOptions() (*Options, error) {
	if os.Args == nil {
		return nil, errors.New("os.Args is nil")
	}
	// If QS_ARGS exists overwrite the given arguments.
	qsArgs := os.Getenv("QS_ARGS")
	args := os.Args[1:]
	if qsArgs != "" {
		args = strings.Split(qsArgs, " ")
	}

	// Parse arguments and check for errors
	opts := &Options{}
	parser, err := kong.New(
		opts,
		kong.Help(HelpPrompt),
		kong.UsageOnError(),
		kong.Vars{"version": Version},
		kong.ConfigureHelp(kong.HelpOptions{
			Summary: true,
		}),
	)
	if err != nil {
		return nil, err
	}
	_, err = parser.Parse(args)
	if err != nil {
		return nil, err
	}

	// Disable smart pipe if parameters are being passed via env
	if utils.IsFilePiped(os.Stdin) && qsArgs == "" {
		opts.InPipe = os.Stdin
	}

	if utils.IsFilePiped(os.Stdout) && qsArgs == "" {
		opts.OutPipe = os.Stdout
		utils.EnableSmartPipe()
	}

	if opts.GenerateQR {
		in, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, err
		}

		qcfg := qrterminal.Config{
			Level:     qrterminal.M,
			Writer:    os.Stdout,
			BlackChar: qrterminal.WHITE,
			WhiteChar: qrterminal.BLACK,
			QuietZone: 1,
		}

		qrterminal.GenerateWithConfig(string(in), qcfg)
		os.Exit(0)
	}

	// Generate random secret
	if !opts.Listen && opts.RandomSecret {
		print(utils.RandomString(20))
		os.Exit(0)
	}

	if opts.ForwardAddr != "" && !ForwardAddrRgx.MatchString(opts.ForwardAddr) {
		subMatches := ForwardAddrRgx.FindStringSubmatch(opts.ForwardAddr)
		lport := ForwardAddrRgx.SubexpIndex("lport")
		rhost := ForwardAddrRgx.SubexpIndex("rhost")
		rport := ForwardAddrRgx.SubexpIndex("rport")
		opts.LocalPort = subMatches[lport]
		opts.ForwardAddr = fmt.Sprintf("%s:%s", subMatches[rhost], subMatches[rport])
		return nil, errors.New("Invalid forward address.")
	}

	if opts.UseTor {
		opts.SocksAddr = "127.0.0.1:9050"
	}

	if opts.DisableEnc {
		opts.End2End = false
	}

	if !opts.RandomSecret && opts.Secret == "" {
		// A randomly generated secret is useless when there is no console to
		// print it on, so a secret needs to be supplied via flags or QS_ARGS.
		if NoPrompt {
			return nil, errors.New("No secret specified.")
		}
		color.New(color.FgBlue).Add(color.Bold).Print("[>] ")
		print("Enter Secret (or press Enter to generate): ")
		n, _ := fmt.Scanln(&opts.Secret)
		if n == 0 {
			opts.RandomSecret = true
		}
	}

	if opts.Verbose {
		log.SetLevel(log.LOG_LEVEL_TRACE) // Show all the shit!
	}

	if opts.Quiet {
		log.SetLevel(log.LOG_LEVEL_FATAL) // Show nothing!
	}

	return opts, nil
}

func (opts *Options) IsPiped() bool {
	return opts.InPipe != nil || opts.OutPipe != nil
}

func (opts *Options) Summarize() {
	if opts == nil || opts.Quiet {
		return
	}
	encType := DEFAULT_E2E_CIPHER
	mode := "client"
	if opts.Listen {
		mode = "server"
	}

	fmt.Printf("%s %s\n", utils.BoldYellow.Sprintf("[#]"), utils.BoldBlue.Sprintf(".::Qsocket Netcat::."))
	fmt.Printf("%s Secret: %s\n", utils.Yellow.Sprintf(" ├──>"), utils.Red.Sprintf(opts.Secret))
	fmt.Printf("%s Mode: %s\n", utils.Yellow.Sprintf(" ├──>"), mode)
	fmt.Printf("%s TOR: %t\n", utils.Yellow.Sprintf(" ├──>"), opts.UseTor)
	if opts.ForwardAddr != "" {
		fmt.Printf("%s Forward: %s\n", utils.Yellow.Sprintf(" ├──>"), opts.ForwardAddr)
	}
	if opts.Listen {
		fmt.Printf("%s Probe Interval: %s\n", utils.Yellow.Sprintf(" ├──>"), time.Second*time.Duration(opts.ProbeInterval))
	} else {
		fmt.Printf("%s Probe Duration: %s\n", utils.Yellow.Sprintf(" ├──>"), time.Second*time.Duration(opts.ProbeInterval))
	}
	if opts.InPipe != nil || opts.OutPipe != nil {
		fmt.Printf("%s Pipe: true\n", utils.Yellow.Sprintf(" ├──>"))
	}
	if opts.DisableEnc {
		encType = utils.Red.Sprintf("DISABLED")
	} else {
		if !opts.End2End {
			encType = "TLS"
		}
	}
	fmt.Printf("%s Encryption: %s\n", utils.Yellow.Sprintf(" └──>"), encType)
	print("\n")
}
