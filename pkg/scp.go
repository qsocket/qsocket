package qsock

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/qsocket/qsocket/config"
	"github.com/qsocket/qsocket/log"
	"github.com/qsocket/qsocket/session"
	"github.com/qsocket/qsocket/utils"
)

const (
	// DEFAULT_FILE_MODE is used for the files arriving without permissions.
	DEFAULT_FILE_MODE = 0o644
	// DEFAULT_DIR_MODE is used for the directories arriving without
	// permissions and for the parents created along the way.
	DEFAULT_DIR_MODE = 0o755
	// MODE_MASK are the permission bits honoured on an incoming entry, the
	// set-user-ID and set-group-ID bits of a remote peer are never applied.
	MODE_MASK = 0o777
)

// ConnectSCP copies files and folders to or from the remote peer.
func ConnectSCP(ctx context.Context, opts *config.Options) error {
	err := transferFiles(opts)
	if err == nil || err == io.EOF {
		// The progress bars only account for the files one by one, the
		// session as a whole gets a line of its own.
		if !opts.Quiet {
			utils.PrintSuccess("Transfer completed successfully.\n")
		}
		return nil
	}
	// The diagnostics of a failed transfer are already formatted the way
	// the classic scp utility words them, they are printed as they are
	// instead of being wrapped into a log line.
	if strings.HasPrefix(err.Error(), "scp: ") {
		fmt.Fprintln(os.Stderr, err.Error())
		return ErrSilent
	}
	return err
}

// transferFiles dials the relay network and runs the transfer in the
// direction picked by the command line.
func transferFiles(opts *config.Options) error {
	// A source which cannot be read is reported before dialing, there is
	// no reason to bother the remote peer with it.
	if opts.Upload {
		info, err := os.Stat(opts.LocalPath)
		if err != nil {
			return fmt.Errorf("scp: %s: %s", opts.LocalPath, statError(err))
		}
		if info.IsDir() && !opts.Recursive {
			return fmt.Errorf("scp: %s: not a regular file", opts.LocalPath)
		}
	}

	status := NewStatus(opts)
	status.Set("Dialing qsocket relay network...")
	qs, err := DialQSRN(opts)
	status.Stop()
	if err != nil {
		return err
	}
	defer qs.Close()

	if err := InitSession(qs, NewSpecs(opts)); err != nil {
		return err
	}

	if opts.Upload {
		return SendTransfer(qs, opts.LocalPath, opts.Recursive, ShowProgress(opts))
	}
	return RecvTransfer(qs, opts.LocalPath, ShowProgress(opts))
}

// ServeSCP serves a file transfer requested by the connecting peer.
func ServeSCP(qs Session, specs *session.SessionSpecs, opts *config.Options) error {
	// The connecting peer is the sender, every check that matters happens
	// on its side so the stream is simply accepted.
	if specs.Has(session.FLAG_UPLOAD) {
		if err := session.WriteResponse(qs, session.STATUS_OK, ""); err != nil {
			qs.Close()
			return err
		}
		go func() {
			defer qs.Close()
			if err := RecvTransfer(qs, specs.Path, ShowProgress(opts)); err != nil {
				log.Error(err)
				return
			}
			log.Info("Transfer completed successfully.")
		}()
		return nil
	}

	info, err := os.Stat(specs.Path)
	if err != nil {
		log.Error(err)
		session.WriteResponse(qs, session.STATUS_NOT_FOUND,
			fmt.Sprintf("scp: %s: %s", specs.Path, statError(err)))
		qs.Close()
		return nil
	}
	if info.IsDir() && !specs.Has(session.FLAG_RECURSIVE) {
		session.WriteResponse(qs, session.STATUS_NOT_REGULAR,
			fmt.Sprintf("scp: download %s: not a regular file", specs.Path))
		qs.Close()
		return nil
	}
	if err := session.WriteResponse(qs, session.STATUS_OK, ""); err != nil {
		qs.Close()
		return err
	}

	go func() {
		defer qs.Close()
		// Everything the transfer needs was asked for by the connecting
		// peer, the options of this one only say how to report it.
		err := SendTransfer(qs, specs.Path, specs.Has(session.FLAG_RECURSIVE), ShowProgress(opts))
		if err != nil {
			log.Error(err)
			return
		}
		log.Info("Transfer completed successfully.")
	}()
	return nil
}

// SendTransfer streams the given path as a record stream and waits for the
// receiving peer to confirm that everything made it to disk.
func SendTransfer(qs Session, src string, recursive, progress bool) error {
	if err := SendFiles(qs, src, recursive, progress); err != nil {
		return err
	}
	resp, err := session.ReadResponse(qs)
	if err != nil {
		return err
	}
	return resp.Err()
}

// RecvTransfer receives a record stream into the given destination and
// reports the result back to the sending peer.
func RecvTransfer(qs Session, dest string, progress bool) error {
	err := RecvFiles(qs, dest, progress)
	if err != nil {
		session.WriteError(qs, session.STATUS_ERROR, err)
		return err
	}
	return session.WriteResponse(qs, session.STATUS_OK, "")
}

// SendFiles walks the given source path and writes every entry of it into a
// file transfer record stream.
//
// Record names are relative to the parent of the source, so a recursive
// transfer of "/etc/nginx" arrives as "nginx/nginx.conf" on the receiving
// peer and keeps its directory layout.
func SendFiles(w io.Writer, src string, recursive bool, progress bool) error {
	root, err := filepath.Abs(src)
	if err != nil {
		return err
	}
	base := filepath.Dir(root)

	err = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		name, err := filepath.Rel(base, path)
		if err != nil {
			return err
		}
		// Record names are slash separated regardless of the separator
		// used by the sending peer.
		name = filepath.ToSlash(name)

		switch {
		case info.IsDir():
			if !recursive {
				return fmt.Errorf("scp: %s: not a regular file", src)
			}
			return session.WriteRecord(w, &session.Record{
				Type: session.REC_DIR,
				Mode: uint32(info.Mode().Perm()),
				Name: name,
			})
		case !info.Mode().IsRegular():
			// Sockets, devices and symbolic links have no meaningful
			// representation on the receiving peer.
			log.Warnf("Skipping %s, not a regular file.", path)
			return nil
		}
		return sendFile(w, path, name, info, progress)
	})
	if err != nil {
		return err
	}
	return session.WriteEndRecord(w)
}

// sendFile writes the record of a single regular file and streams exactly as
// many bytes as its header announced.
func sendFile(w io.Writer, path, name string, info os.FileInfo, progress bool) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	size := uint64(info.Size())
	err = session.WriteRecord(w, &session.Record{
		Type: session.REC_FILE,
		Mode: uint32(info.Mode().Perm()),
		Size: size,
		Name: name,
	})
	if err != nil {
		return err
	}

	bar := NewProgress(name, size, progress)
	defer bar.Close()

	sent, err := io.CopyN(io.MultiWriter(w, bar), file, int64(size))
	if err != nil && err != io.EOF {
		return err
	}
	if uint64(sent) < size {
		// The file shrank while it was being read, the receiving peer is
		// waiting for the announced number of bytes and the stream would
		// go out of sync without them.
		log.Warnf("%s shrank while being sent, padding %d bytes.", name, size-uint64(sent))
		return padStream(w, size-uint64(sent), bar)
	}
	return nil
}

// padStream writes n zero bytes, it keeps a record stream in sync after a
// short read.
func padStream(w io.Writer, n uint64, bar *Progress) error {
	zeros := make([]byte, PIPE_BUFFER_SIZE)
	for n > 0 {
		chunk := uint64(len(zeros))
		if chunk > n {
			chunk = n
		}
		if _, err := io.MultiWriter(w, bar).Write(zeros[:chunk]); err != nil {
			return err
		}
		n -= chunk
	}
	return nil
}

// RecvFiles reads a file transfer record stream and materializes it under the
// given destination.
//
// A destination naming an existing directory receives the entries under their
// own names, anything else takes the place of the transfer root, which
// renames a single file or a whole directory tree. This is how the classic
// scp utility picks its destinations.
func RecvFiles(r io.Reader, dest string, progress bool) error {
	// A destination written with a trailing separator is a directory by
	// definition, it is created when it does not exist yet.
	if endsWithSeparator(dest) {
		if err := os.MkdirAll(dest, DEFAULT_DIR_MODE); err != nil {
			return err
		}
	}
	// The destination may well become a directory during the transfer, so
	// the decision is made once, before anything is written.
	destIsDir := isDir(dest)

	for {
		rec, err := session.ReadRecord(r)
		if err != nil {
			return err
		}
		if rec.Type == session.REC_END {
			return nil
		}

		target, err := resolveTarget(dest, destIsDir, rec.Name)
		if err != nil {
			return fmt.Errorf("scp: %s: %w", rec.Name, err)
		}

		if rec.Type == session.REC_DIR {
			if err := os.MkdirAll(target, dirMode(rec.Mode)); err != nil {
				return err
			}
			continue
		}
		if err := recvFile(r, target, rec, progress); err != nil {
			return err
		}
	}
}

// recvFile stores the contents following a file record.
func recvFile(r io.Reader, target string, rec *session.Record, progress bool) error {
	if err := os.MkdirAll(filepath.Dir(target), DEFAULT_DIR_MODE); err != nil {
		return err
	}
	file, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, fileMode(rec.Mode))
	if err != nil {
		// The contents are on their way regardless of whether they can
		// be stored, draining them keeps the stream usable.
		rec.Discard(r)
		return err
	}
	defer file.Close()

	bar := NewProgress(rec.Name, rec.Size, progress)
	defer bar.Close()

	if _, err := io.CopyN(io.MultiWriter(file, bar), r, int64(rec.Size)); err != nil {
		return err
	}
	return nil
}

// resolveTarget maps the name of an incoming record onto a local path.
func resolveTarget(dest string, destIsDir bool, name string) (string, error) {
	if destIsDir {
		return safeJoin(dest, name)
	}
	// The destination replaces the root element of the transfer, so a
	// single file lands on it directly and a tree is renamed into it.
	if idx := strings.IndexByte(name, '/'); idx != -1 {
		return safeJoin(dest, name[idx+1:])
	}
	return dest, nil
}

// safeJoin joins the name of an incoming record to the destination and makes
// sure the result stays inside of it.
func safeJoin(dest, name string) (string, error) {
	// The protocol layer has already validated the name, this is the
	// second line of defense against a peer trying to plant files outside
	// of the destination the user picked.
	clean, err := session.SafeName(name)
	if err != nil {
		return "", err
	}
	target := filepath.Join(dest, filepath.FromSlash(clean))

	// Asking how to walk from the destination to the target is the only
	// comparison which holds for every way a destination can be written,
	// a relative one such as "." has no common textual prefix with the
	// cleaned path it joins into.
	rel, err := filepath.Rel(filepath.Clean(dest), target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", session.ErrInvalidRecName
	}
	return target, nil
}

// isDir reports whether the given path names an existing directory.
func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// endsWithSeparator reports whether the given path is written as a directory.
func endsWithSeparator(path string) bool {
	if path == "" {
		return false
	}
	last := path[len(path)-1]
	return last == '/' || last == os.PathSeparator
}

// fileMode returns the permissions applied to an incoming file.
func fileMode(mode uint32) os.FileMode {
	if mode&MODE_MASK == 0 {
		return DEFAULT_FILE_MODE
	}
	return os.FileMode(mode & MODE_MASK)
}

// dirMode returns the permissions applied to an incoming directory, the owner
// always keeps the access needed for populating it.
func dirMode(mode uint32) os.FileMode {
	if mode&MODE_MASK == 0 {
		return DEFAULT_DIR_MODE
	}
	return os.FileMode(mode&MODE_MASK) | 0o700
}

// statError words a failed stat the way the classic scp utility does.
func statError(err error) string {
	if os.IsNotExist(err) {
		return "No such file or directory"
	}
	if os.IsPermission(err) {
		return "Permission denied"
	}
	return err.Error()
}
