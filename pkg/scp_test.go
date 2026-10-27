package qsock

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/qsocket/qsocket/session"
)

// transferTree streams src into dest the way a transfer session does, with
// the sender and the receiver on the two ends of a pipe.
func transferTree(t *testing.T, src, dest string, recursive bool) error {
	t.Helper()
	reader, writer := io.Pipe()
	sent := make(chan error, 1)
	go func() {
		err := SendFiles(writer, src, recursive, false)
		writer.CloseWithError(err)
		sent <- err
	}()

	recvErr := RecvFiles(reader, dest, false)
	if err := <-sent; err != nil {
		return err
	}
	return recvErr
}

// sourceTree builds the directory layout used by the transfer tests.
func sourceTree(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "root")
	if err := os.MkdirAll(filepath.Join(root, "sub"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("hello"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "sub", "b.txt"), []byte("world"), 0644); err != nil {
		t.Fatal(err)
	}
	return root
}

// assertFile fails the test unless the given path holds the wanted contents.
func assertFile(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Errorf("reading %s: %v", path, err)
		return
	}
	if string(got) != want {
		t.Errorf("%s holds %q, want %q", path, got, want)
	}
}

func TestTransferFileIntoDirectory(t *testing.T) {
	root := sourceTree(t)
	dest := t.TempDir()
	if err := transferTree(t, filepath.Join(root, "a.txt"), dest, false); err != nil {
		t.Fatal(err)
	}
	// An existing directory receives the file under its own name.
	assertFile(t, filepath.Join(dest, "a.txt"), "hello")
}

func TestTransferFileRenames(t *testing.T) {
	root := sourceTree(t)
	dest := filepath.Join(t.TempDir(), "renamed.txt")
	if err := transferTree(t, filepath.Join(root, "a.txt"), dest, false); err != nil {
		t.Fatal(err)
	}
	// A destination which is not a directory names the file itself.
	assertFile(t, dest, "hello")
}

func TestTransferTreeIntoDirectory(t *testing.T) {
	root := sourceTree(t)
	dest := t.TempDir()
	if err := transferTree(t, root, dest, true); err != nil {
		t.Fatal(err)
	}
	// The tree keeps its own root inside an existing directory.
	assertFile(t, filepath.Join(dest, "root", "a.txt"), "hello")
	assertFile(t, filepath.Join(dest, "root", "sub", "b.txt"), "world")
}

func TestTransferTreeRenames(t *testing.T) {
	root := sourceTree(t)
	dest := filepath.Join(t.TempDir(), "newname")
	if err := transferTree(t, root, dest, true); err != nil {
		t.Fatal(err)
	}
	// A destination which does not exist takes the place of the root.
	assertFile(t, filepath.Join(dest, "a.txt"), "hello")
	assertFile(t, filepath.Join(dest, "sub", "b.txt"), "world")
}

func TestTransferDirectoryNeedsRecursive(t *testing.T) {
	root := sourceTree(t)
	err := transferTree(t, root, t.TempDir(), false)
	if err == nil {
		t.Fatal("a directory was transferred without the recursive flag")
	}
	// The wording is the one the classic scp utility uses.
	if !strings.HasSuffix(err.Error(), ": not a regular file") {
		t.Errorf("got %q, want a \"not a regular file\" diagnostic", err)
	}
}

func TestTransferPreservesPermissions(t *testing.T) {
	if os.PathSeparator != '/' {
		t.Skip("permission bits are not meaningful on this platform")
	}
	root := sourceTree(t)
	dest := t.TempDir()
	if err := transferTree(t, root, dest, true); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(dest, "root", "a.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0600 {
		t.Errorf("got mode %o, want %o", got, 0600)
	}
}

func TestTransferEmptyFile(t *testing.T) {
	src := filepath.Join(t.TempDir(), "empty.txt")
	if err := os.WriteFile(src, nil, 0644); err != nil {
		t.Fatal(err)
	}
	dest := t.TempDir()
	if err := transferTree(t, src, dest, false); err != nil {
		t.Fatal(err)
	}
	assertFile(t, filepath.Join(dest, "empty.txt"), "")
}

func TestRecvFilesRejectsEscapingName(t *testing.T) {
	base := t.TempDir()
	dest := filepath.Join(base, "inbox")
	if err := os.MkdirAll(dest, 0755); err != nil {
		t.Fatal(err)
	}

	reader, writer := io.Pipe()
	go func() {
		// A peer trying to plant a file next to the destination.
		session.WriteRecord(writer, &session.Record{
			Type: session.REC_FILE,
			Mode: 0644,
			Size: 5,
			Name: "../escaped.txt",
		})
		writer.Write([]byte("pwned"))
		session.WriteEndRecord(writer)
		writer.Close()
	}()

	if err := RecvFiles(reader, dest, false); err == nil {
		t.Error("accepted a record escaping the destination")
	}
	if _, err := os.Stat(filepath.Join(base, "escaped.txt")); !os.IsNotExist(err) {
		t.Error("a record escaped the destination directory")
	}
}

func TestResolveTarget(t *testing.T) {
	// Inside an existing directory the entries keep their own names.
	got, err := resolveTarget("/dest", true, "root/a.txt")
	if err != nil || got != filepath.Join("/dest", "root", "a.txt") {
		t.Errorf("got (%q, %v), want %q", got, err, filepath.Join("/dest", "root", "a.txt"))
	}
	// Otherwise the destination replaces the root of the transfer.
	got, err = resolveTarget("/dest/newname", false, "root/sub/b.txt")
	if err != nil || got != filepath.Join("/dest/newname", "sub", "b.txt") {
		t.Errorf("got (%q, %v), want %q", got, err, filepath.Join("/dest/newname", "sub", "b.txt"))
	}
	// A single file lands on the destination itself.
	got, err = resolveTarget("/dest/renamed.txt", false, "a.txt")
	if err != nil || got != "/dest/renamed.txt" {
		t.Errorf("got (%q, %v), want %q", got, err, "/dest/renamed.txt")
	}
	if _, err = resolveTarget("/dest", true, "../escaped"); err == nil {
		t.Error("resolveTarget accepted a name escaping the destination")
	}
}

// chdir moves into dir for the duration of the test.
func chdir(t *testing.T, dir string) {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(old) })
}

func TestTransferIntoRelativeDestination(t *testing.T) {
	// "." is how the classic scp utility is invoked most of the time, and
	// a relative destination shares no textual prefix with the cleaned
	// path a record name joins into.
	for _, dest := range []string{".", "./"} {
		root := sourceTree(t)
		work := t.TempDir()
		chdir(t, work)

		if err := transferTree(t, filepath.Join(root, "a.txt"), dest, false); err != nil {
			t.Errorf("destination %q: %v", dest, err)
			continue
		}
		assertFile(t, filepath.Join(work, "a.txt"), "hello")
	}
}

func TestTransferTreeIntoRelativeDestination(t *testing.T) {
	root := sourceTree(t)
	work := t.TempDir()
	chdir(t, work)

	if err := transferTree(t, root, ".", true); err != nil {
		t.Fatal(err)
	}
	assertFile(t, filepath.Join(work, "root", "a.txt"), "hello")
	assertFile(t, filepath.Join(work, "root", "sub", "b.txt"), "world")
}

func TestTransferTreeIntoRelativeName(t *testing.T) {
	root := sourceTree(t)
	work := t.TempDir()
	chdir(t, work)

	// A relative destination which does not exist yet renames the root.
	if err := transferTree(t, root, "newname", true); err != nil {
		t.Fatal(err)
	}
	assertFile(t, filepath.Join(work, "newname", "a.txt"), "hello")
	assertFile(t, filepath.Join(work, "newname", "sub", "b.txt"), "world")
}

func TestSafeJoinAcceptsEveryDestinationSpelling(t *testing.T) {
	// Each of these names the same directory, none of them may be read as
	// an attempt to escape it.
	for _, dest := range []string{".", "./", "sub", "./sub", "sub/", "/abs/dir", "/abs/dir/", ".."} {
		got, err := safeJoin(dest, "a.txt")
		if err != nil {
			t.Errorf("safeJoin(%q, \"a.txt\") failed: %v", dest, err)
			continue
		}
		if want := filepath.Join(dest, "a.txt"); got != want {
			t.Errorf("safeJoin(%q, \"a.txt\") = %q, want %q", dest, got, want)
		}
	}

	// Escaping is still refused whichever way it is spelled.
	for _, name := range []string{"../escaped", "a/../../escaped", "/etc/passwd"} {
		if got, err := safeJoin(".", name); err == nil {
			t.Errorf("safeJoin(\".\", %q) = %q, want an error", name, got)
		}
	}
}

func TestResolveTargetRelativeDestination(t *testing.T) {
	// Inside the working directory the entries keep their own names.
	got, err := resolveTarget(".", true, "o2")
	if err != nil || got != "o2" {
		t.Errorf("got (%q, %v), want %q", got, err, "o2")
	}
	got, err = resolveTarget(".", true, "root/a.txt")
	if err != nil || got != filepath.Join("root", "a.txt") {
		t.Errorf("got (%q, %v), want %q", got, err, filepath.Join("root", "a.txt"))
	}
	// A relative destination which is not a directory renames the root.
	got, err = resolveTarget("newname", false, "root/sub/b.txt")
	if err != nil || got != filepath.Join("newname", "sub", "b.txt") {
		t.Errorf("got (%q, %v), want %q", got, err, filepath.Join("newname", "sub", "b.txt"))
	}
}

func TestRecvFilesFailsOnRejectedName(t *testing.T) {
	dest := t.TempDir()
	reader, writer := io.Pipe()
	go func() {
		// A name the protocol layer cannot reject on its own, it only
		// becomes an escape once joined to the destination.
		session.WriteRecord(writer, &session.Record{Type: session.REC_DIR, Mode: 0755, Name: ".."})
		session.WriteEndRecord(writer)
		writer.Close()
	}()

	// Reporting a success having written nothing would be worse than the
	// transfer failing outright.
	if err := RecvFiles(reader, dest, false); err == nil {
		t.Error("a rejected record was reported as a successful transfer")
	}
}
