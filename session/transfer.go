package session

import (
	"encoding/binary"
	"errors"
	"io"
	"path"
	"strings"
)

// The file transfer stream of a CMD_SCP session is a flat sequence of records
// terminated by a single REC_END record. Directories carry no payload, files
// are followed by exactly SIZE bytes of contents.
//
//	 0                   1                   2                   3
//	 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
//	+-------+-------+-------+-------+-------+-------+-------+-------+
//	| TYPE  |             MODE              |          SIZE
//	+-------+-------+-------+-------+-------+-------+-------+-------+
//	        SIZE (cont.)                    |    NAMELEN    | NAME...
//	+-------+-------+-------+-------+-------+-------+-------+ - - - +
const (
	// RECORD_HEADER_LEN is the length of the fixed part of a record.
	RECORD_HEADER_LEN = 15
	// MAX_NAME_LEN is the longest relative path a record can carry.
	MAX_NAME_LEN = 0xFFFF
)

// RecordType identifies the kind of filesystem entry a record describes.
type RecordType uint8

const (
	// REC_END terminates the record stream.
	REC_END RecordType = iota
	// REC_FILE describes a regular file, its contents follow the header.
	REC_FILE
	// REC_DIR describes a directory and carries no contents.
	REC_DIR
)

var (
	ErrInvalidRecord   = errors.New("Invalid file transfer record.")
	ErrInvalidRecName  = errors.New("Invalid file transfer record name.")
	ErrRecordTooLarge  = errors.New("File transfer record exceeds the protocol limits.")
	ErrNotRegularFile  = errors.New("Not a regular file")
	ErrNoSuchFile      = errors.New("No such file or directory")
	ErrUnexpectedEntry = errors.New("Unexpected file transfer record type.")
)

// Record is the header of a single entry of a file transfer stream.
type Record struct {
	// Type is the kind of the filesystem entry.
	Type RecordType
	// Mode carries the permission bits of the entry, the receiving peer
	// applies them masked with the local umask.
	Mode uint32
	// Size is the length of the contents following a REC_FILE header.
	Size uint64
	// Name is the slash separated path of the entry, relative to the root
	// of the transfer. It never escapes that root, see SafeName.
	Name string
}

// WriteRecord writes a record header in a single call, the contents of a
// REC_FILE record are streamed by the caller right after the header.
func WriteRecord(w io.Writer, rec *Record) error {
	if len(rec.Name) > MAX_NAME_LEN {
		return ErrRecordTooLarge
	}
	buf := make([]byte, RECORD_HEADER_LEN, RECORD_HEADER_LEN+len(rec.Name))
	buf[0] = uint8(rec.Type)
	binary.BigEndian.PutUint32(buf[1:], rec.Mode)
	binary.BigEndian.PutUint64(buf[5:], rec.Size)
	binary.BigEndian.PutUint16(buf[13:], uint16(len(rec.Name)))
	buf = append(buf, rec.Name...)
	_, err := w.Write(buf)
	return err
}

// WriteEndRecord terminates a file transfer stream.
func WriteEndRecord(w io.Writer) error {
	return WriteRecord(w, &Record{Type: REC_END})
}

// ReadRecord reads a single record header from the given stream.
func ReadRecord(r io.Reader) (*Record, error) {
	header := make([]byte, RECORD_HEADER_LEN)
	if _, err := io.ReadFull(r, header); err != nil {
		return nil, err
	}
	rec := &Record{
		Type: RecordType(header[0]),
		Mode: binary.BigEndian.Uint32(header[1:]),
		Size: binary.BigEndian.Uint64(header[5:]),
	}
	switch rec.Type {
	case REC_END, REC_FILE, REC_DIR:
	default:
		return nil, ErrInvalidRecord
	}

	name := make([]byte, binary.BigEndian.Uint16(header[13:]))
	if _, err := io.ReadFull(r, name); err != nil {
		return nil, err
	}
	rec.Name = string(name)
	if rec.Type == REC_END {
		return rec, nil
	}

	// The name of an incoming record ends up being joined to a local
	// destination, so it is validated before the caller gets to use it.
	safe, err := SafeName(rec.Name)
	if err != nil {
		return nil, err
	}
	rec.Name = safe
	return rec, nil
}

// SafeName validates the relative path of an incoming record and returns it
// in its cleaned form.
//
// A record name is always interpreted relative to the destination picked by
// the receiving peer, so anything able to escape that destination is rejected
// rather than sanitized, a peer sending such a name is not to be trusted.
func SafeName(name string) (string, error) {
	// Windows peers may send either separator, neither is a valid character
	// of a path element on the receiving side.
	if strings.ContainsRune(name, '\\') {
		return "", ErrInvalidRecName
	}
	if name == "" || path.IsAbs(name) {
		return "", ErrInvalidRecName
	}
	// Reject volume names and alternate data streams before cleaning, a
	// Windows receiver would otherwise resolve them outside the destination.
	if strings.ContainsRune(name, ':') {
		return "", ErrInvalidRecName
	}

	clean := path.Clean(name)
	if clean == "." || path.IsAbs(clean) {
		return "", ErrInvalidRecName
	}
	for _, elem := range strings.Split(clean, "/") {
		if elem == ".." {
			return "", ErrInvalidRecName
		}
	}
	return clean, nil
}

// Discard drains the contents of a record from the stream, it keeps the
// stream usable after an entry has been rejected by the receiving peer.
func (rec *Record) Discard(r io.Reader) error {
	if rec.Type != REC_FILE || rec.Size == 0 {
		return nil
	}
	_, err := io.CopyN(io.Discard, r, int64(rec.Size))
	return err
}
