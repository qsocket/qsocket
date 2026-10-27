package qsock

import (
	"bufio"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/qsocket/qsocket/config"
	"github.com/qsocket/qsocket/log"
	"github.com/qsocket/qsocket/session"
)

func TestMain(m *testing.M) {
	// The handlers log what they are serving, it is noise in a test run.
	log.SetLevel(log.LOG_LEVEL_FATAL)
	os.Exit(m.Run())
}

// pipeSession adapts a net.Conn to the Session interface, it stands in for a
// dialed QSocket so that the session layer can be exercised without the relay
// network in between.
type pipeSession struct {
	net.Conn
}

func (s pipeSession) Close() { s.Conn.Close() }

// serveOnce reads a session request off the given connection and serves it,
// exactly as the probe loop of a listening peer does.
func serveOnce(t *testing.T, conn net.Conn, opts *config.Options) {
	t.Helper()
	qs := pipeSession{conn}
	specs, err := session.ReadSpecs(qs)
	if err != nil {
		t.Errorf("reading the session specs: %v", err)
		conn.Close()
		return
	}
	if err := ServeSession(qs, specs, opts); err != nil {
		t.Errorf("serving a %s session: %v", specs.ID, err)
	}
}

// sessionPipe returns the two ends of a session, with the listening peer
// already serving on one of them.
func sessionPipe(t *testing.T, server *config.Options) Session {
	t.Helper()
	client, listener := net.Pipe()
	go serveOnce(t, listener, server)
	t.Cleanup(func() { client.Close() })
	return pipeSession{client}
}

func TestServeSCPDownload(t *testing.T) {
	src := filepath.Join(t.TempDir(), "remote.txt")
	if err := os.WriteFile(src, []byte("payload"), 0644); err != nil {
		t.Fatal(err)
	}
	dest := t.TempDir()

	server := &config.Options{Listen: true, Quiet: true}
	client := &config.Options{Quiet: true, Cmd: session.CMD_SCP, RemotePath: src, LocalPath: dest}
	qs := sessionPipe(t, server)

	if err := InitSession(qs, NewSpecs(client)); err != nil {
		t.Fatalf("the listening peer rejected the transfer: %v", err)
	}
	if err := RecvTransfer(qs, dest, false); err != nil {
		t.Fatal(err)
	}
	assertFile(t, filepath.Join(dest, "remote.txt"), "payload")
}

func TestServeSCPDownloadRejectsFolder(t *testing.T) {
	src := t.TempDir()
	server := &config.Options{Listen: true, Quiet: true}
	client := &config.Options{Quiet: true, Cmd: session.CMD_SCP, RemotePath: src}
	qs := sessionPipe(t, server)

	err := InitSession(qs, NewSpecs(client))
	if err == nil {
		t.Fatal("a folder was downloaded without the recursive flag")
	}
	// The diagnostic of the listening peer reaches the user verbatim.
	want := "scp: download " + src + ": not a regular file"
	if err.Error() != want {
		t.Errorf("got %q, want %q", err, want)
	}
}

func TestServeSCPDownloadFolder(t *testing.T) {
	root := sourceTree(t)
	dest := t.TempDir()

	server := &config.Options{Listen: true, Quiet: true}
	client := &config.Options{
		Quiet: true, Cmd: session.CMD_SCP,
		RemotePath: root, LocalPath: dest, Recursive: true,
	}
	qs := sessionPipe(t, server)

	if err := InitSession(qs, NewSpecs(client)); err != nil {
		t.Fatalf("the listening peer rejected the transfer: %v", err)
	}
	if err := RecvTransfer(qs, dest, false); err != nil {
		t.Fatal(err)
	}
	assertFile(t, filepath.Join(dest, "root", "a.txt"), "hello")
	assertFile(t, filepath.Join(dest, "root", "sub", "b.txt"), "world")
}

func TestServeSCPDownloadMissingPath(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nope.txt")
	server := &config.Options{Listen: true, Quiet: true}
	client := &config.Options{Quiet: true, Cmd: session.CMD_SCP, RemotePath: missing}
	qs := sessionPipe(t, server)

	err := InitSession(qs, NewSpecs(client))
	if err == nil {
		t.Fatal("a missing path was accepted")
	}
	want := "scp: " + missing + ": No such file or directory"
	if err.Error() != want {
		t.Errorf("got %q, want %q", err, want)
	}
}

func TestServeSCPUpload(t *testing.T) {
	root := sourceTree(t)
	dest := t.TempDir()

	server := &config.Options{Listen: true, Quiet: true}
	client := &config.Options{
		Quiet: true, Cmd: session.CMD_SCP, Upload: true,
		LocalPath: root, RemotePath: dest, Recursive: true,
	}
	qs := sessionPipe(t, server)

	if err := InitSession(qs, NewSpecs(client)); err != nil {
		t.Fatalf("the listening peer rejected the transfer: %v", err)
	}
	if err := SendTransfer(qs, root, true, false); err != nil {
		t.Fatal(err)
	}
	assertFile(t, filepath.Join(dest, "root", "a.txt"), "hello")
	assertFile(t, filepath.Join(dest, "root", "sub", "b.txt"), "world")
}

func TestServeForward(t *testing.T) {
	// A local listener standing in for the forwarding destination reached
	// by the listening peer.
	echo, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer echo.Close()
	go func() {
		conn, err := echo.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		io.Copy(conn, conn)
	}()

	server := &config.Options{Listen: true, Quiet: true}
	qs := sessionPipe(t, server)

	specs := &session.SessionSpecs{ID: session.CMD_FORWARD, Address: echo.Addr().String()}
	if err := InitSession(qs, specs); err != nil {
		t.Fatalf("the listening peer rejected the tunnel: %v", err)
	}
	if _, err := qs.Write([]byte("ping")); err != nil {
		t.Fatal(err)
	}
	got := make([]byte, 4)
	if _, err := io.ReadFull(qs, got); err != nil {
		t.Fatal(err)
	}
	if string(got) != "ping" {
		t.Errorf("the tunnel returned %q, want %q", got, "ping")
	}
}

func TestServeForwardRejectsDeadDestination(t *testing.T) {
	// A port nobody is listening on, the failure needs to reach the user.
	dead, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := dead.Addr().String()
	dead.Close()

	client, listener := net.Pipe()
	defer client.Close()
	go func() {
		qs := pipeSession{listener}
		specs, err := session.ReadSpecs(qs)
		if err != nil {
			listener.Close()
			return
		}
		// The dial failure is reported to the peer and returned.
		ServeForward(qs, specs)
	}()

	qs := pipeSession{client}
	specs := &session.SessionSpecs{ID: session.CMD_FORWARD, Address: addr}
	if err := InitSession(qs, specs); err == nil {
		t.Error("a tunnel to a closed port was accepted")
	}
}

func TestServeSessionRejectsUnknownCommand(t *testing.T) {
	server := &config.Options{Listen: true, Quiet: true}
	client, listener := net.Pipe()
	defer client.Close()
	go func() {
		qs := pipeSession{listener}
		specs, err := session.ReadSpecs(qs)
		if err != nil {
			listener.Close()
			return
		}
		ServeSession(qs, specs, server)
	}()

	qs := pipeSession{client}
	// A command only a later revision of the protocol knows about.
	err := InitSession(qs, &session.SessionSpecs{ID: session.Command(99)})
	if err == nil {
		t.Fatal("an unknown command was accepted")
	}
	if !strings.Contains(err.Error(), "Unsupported session command") {
		t.Errorf("got %q, want an unsupported command error", err)
	}
}

func TestServeShellRunsCommand(t *testing.T) {
	if _, err := os.Stat(EXEC_SHELL); err != nil {
		t.Skipf("%s is unavailable: %v", EXEC_SHELL, err)
	}

	server := &config.Options{Listen: true, Quiet: true}
	qs := sessionPipe(t, server)

	// A raw channel carries the output of the command without a PTY.
	specs := &session.SessionSpecs{ID: session.CMD_NETCAT, Exec: "echo qsock_works"}
	if err := InitSession(qs, specs); err != nil {
		t.Fatal(err)
	}

	// The session stays open until the connecting peer hangs up, just as
	// a raw netcat channel does, so the output is read a line at a time
	// instead of waiting for the end of the stream.
	done := make(chan string, 1)
	go func() {
		line, _ := bufio.NewReader(qs).ReadString('\n')
		done <- line
	}()
	select {
	case out := <-done:
		if !strings.Contains(out, "qsock_works") {
			t.Errorf("the command returned %q, want it to contain %q", out, "qsock_works")
		}
	case <-time.After(10 * time.Second):
		t.Error("the command produced no output")
	}
}
