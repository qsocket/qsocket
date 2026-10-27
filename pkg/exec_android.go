//go:build android
// +build android

package qsock

import (
	"io"
	"os"
	"os/exec"

	"github.com/creack/pty"
	"github.com/qsocket/qsocket/session"
	// _ "golang.org/x/mobile/app"
)

const (
	// SHELL is the shell spawned for the sessions carrying no command, it
	// also interprets the command lines sent by the connecting peers.
	SHELL = "sh"
)

// commandLine returns the process spawned for the given session specs.
//
// A command line sent by the connecting peer is handed over to a shell, so
// that its quoting, pipes and redirections behave the way they would over a
// regular ssh connection.
func commandLine(specs *session.SessionSpecs) []string {
	if specs.Exec == "" {
		return []string{SHELL}
	}
	return []string{SHELL, "-c", specs.Exec}
}

// ExecCommand runs the command of the given session specs and redirects its
// standard streams into the session.
func ExecCommand(conn Session, specs *session.SessionSpecs) error {
	defer conn.Close()
	params := commandLine(specs)

	ncDir, err := os.Executable() // Get the full path of the executalbe.
	if err != nil {
		return err
	}
	os.Setenv("qsock", ncDir)          // Set binary dir to env variable for easy access.
	os.Setenv("qs_netcat", ncDir)      // Kept for the scripts written against the old releases.
	os.Setenv("HISTFILE", "/dev/null") // Unset histfile for disabling logging.

	cmd := exec.Command(params[0], params[1:]...)

	if specs.Has(session.FLAG_PTY) {
		// Start the command with a pty.
		ptmx, err := pty.StartWithSize(
			cmd,
			&pty.Winsize{Cols: specs.TermSize.Cols, Rows: specs.TermSize.Rows},
		)
		if err != nil {
			return err
		}

		// Make sure to close the pty at the end.
		defer ptmx.Close() // Best effort.

		// Copy stdin to the pty and the pty to stdout.
		// NOTE: The goroutine will keep reading until the next keystroke before returning.
		go io.Copy(ptmx, conn)
		io.Copy(conn, ptmx)
		return nil
	}

	cmd.Stdin = conn
	cmd.Stdout = conn
	cmd.Stderr = conn
	return cmd.Run()
}
