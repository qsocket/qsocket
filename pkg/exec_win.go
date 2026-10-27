//go:build windows
// +build windows

package qsock

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	conpty "github.com/qsocket/conpty-go"
	"github.com/qsocket/qsocket/log"
	"github.com/qsocket/qsocket/session"
)

// const ATTACH_PARENT_PROCESS = ^uintptr(0)
const (
	// SHELL is the shell spawned for the sessions carrying no command, it
	// also interprets the command lines sent by the connecting peers.
	SHELL = "cmd.exe"
)

// func init() {
// 	proc := syscall.MustLoadDLL("KERNEL32.dll").MustFindProc("AttachConsole")
// 	proc.Call(ATTACH_PARENT_PROCESS) // We need this to get console output when using windowsgui subsystem.
// }

// commandLine returns the command line spawned for the given session specs.
//
// A command line sent by the connecting peer is handed over to the shell, so
// that its quoting and redirections behave the way they would over a regular
// ssh connection.
func commandLine(specs *session.SessionSpecs) string {
	if specs.Exec == "" {
		return SHELL
	}
	return SHELL + " /c " + specs.Exec
}

// ExecCommand runs the command of the given session specs and redirects its
// standard streams into the session.
func ExecCommand(conn Session, specs *session.SessionSpecs) error {
	defer conn.Close()
	cmdLine := commandLine(specs)

	ncDir, err := filepath.Abs(os.Args[0]) // Get the full path of the executalbe.
	if err != nil {
		return err
	}
	os.Setenv("qsock", ncDir)     // Set binary dir to env variable for easy access.
	os.Setenv("qs_netcat", ncDir) // Kept for the scripts written against the old releases.

	if specs.Has(session.FLAG_PTY) {
		cpty, err := conpty.Start(cmdLine)
		if err != nil {
			return err
		}
		defer cpty.Close()
		err = cpty.Resize(int(specs.TermSize.Cols), int(specs.TermSize.Rows))
		if err != nil {
			log.Error(err)
		}

		go func() {
			go io.Copy(conn, cpty)
			io.Copy(cpty, conn)
		}()

		_, err = cpty.Wait(context.Background())
		return err
	}

	// The command line is passed to the shell verbatim, letting os/exec
	// quote it would change what the connecting peer asked for.
	cmd := exec.Command(SHELL)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow: true, // Hide new process window
		CmdLine:    cmdLine,
	}
	cmd.Stdin = conn
	cmd.Stdout = conn
	cmd.Stderr = conn
	return cmd.Run()
}
