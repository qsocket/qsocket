//go:build !windows && !android
// +build !windows,!android

package qsnetcat

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync/atomic"

	// "syscall"
	"unsafe"

	"github.com/creack/pty"
	"github.com/google/shlex"
	"github.com/qsocket/qs-netcat/log"
	qsocket "github.com/qsocket/qsocket-go"
	"golang.org/x/sys/unix"
)

const SHELL = "/bin/bash -il"

var (
	PtyHeight      int = 39
	PtyWidth       int = 157
	RawPty         atomic.Bool
	ErrRawPtyInUse = errors.New("Raw PTY is already in use.")
)

func ExecCommand(conn *qsocket.QSocket, specs SessionSpecs) error {
	// If non specified spawn OS shell...
	if specs.Command == "" {
		specs.Command = SHELL
	}

	defer conn.Close()
	params, err := shlex.Split(specs.Command)
	if err != nil {
		return err
	}
	ncDir, err := os.Executable() // Get the full path of the executalbe.
	if err != nil {
		return err
	}
	os.Setenv("qs_netcat", ncDir)      // Set binary dir to env variable for easy access.
	os.Setenv("HISTFILE", "/dev/null") // Unset histfile for disabling logging.
	cmd := exec.Command(params[0])
	if cmd == nil {
		return errors.New("exec.Command returned nil")
	}
	if len(params) > 1 {
		cmd = exec.Command(params[0], params[1:]...)
	}

	if !specs.Interactive && RawPty.Load() {
		return ErrRawPtyInUse
	}

	if specs.Interactive {
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
	} else {
		// Handle pty size.
		err = pty.Setsize(os.Stdin, &pty.Winsize{Rows: specs.TermSize.Rows, Cols: specs.TermSize.Cols})
		if err != nil {
			log.Error(err)
		}
	}

	RawPty.Store(true)
	cmd.Stdin = conn
	cmd.Stdout = conn
	cmd.Stderr = conn
	return cmd.Run()
}

func GetCurrentTermSize() (*Winsize, error) {
	ws := &Winsize{}
	retCode, _, errno := unix.Syscall(unix.SYS_IOCTL,
		uintptr(unix.Stdin),
		uintptr(unix.TIOCGWINSZ),
		uintptr(unsafe.Pointer(ws)))

	if int(retCode) == -1 {
		return nil, fmt.Errorf("TIOCGWINSZ syscall failed with %d!", errno)
	}
	return ws, nil
}
