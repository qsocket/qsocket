// Package session implements the QSocket session initiation protocol.
//
// Peers exchange a fixed size header followed by a list of type-length-value
// fields before any payload is streamed. Every multi byte integer is encoded
// in network byte order and all layouts are fixed, no language specific
// serialization is used so that implementations written in other languages
// stay wire compatible with the Go peers.
//
//	Session request (connecting peer -> listening peer)
//
//	 0                   1                   2                   3
//	 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
//	+-------+-------+-------+-------+-------+-------+-------+-------+
//	|  'Q'  |  'S'  |  VER  |  CMD  | FLAGS |NFIELDS|     ROWS      |
//	+-------+-------+-------+-------+-------+-------+-------+-------+
//	|     COLS      |    XPIXEL     |    YPIXEL     |
//	+-------+-------+-------+-------+-------+-------+
//
//	followed by NFIELDS fields of
//
//	+-------+-------+-------+ - - - - - - - +
//	| TYPE  |     LENGTH    |  VALUE...     |
//	+-------+-------+-------+ - - - - - - - +
//
//	Session response (listening peer -> connecting peer)
//
//	+-------+-------+-------+-------+-------+-------+ - - - - - - - +
//	|  'Q'  |  'S'  |  VER  |STATUS |    MSGLEN     |  MESSAGE...   |
//	+-------+-------+-------+-------+-------+-------+ - - - - - - - +
package session

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

const (
	// MAGIC_0 and MAGIC_1 are the two byte prefix of every protocol message.
	MAGIC_0 = 'Q'
	MAGIC_1 = 'S'
	// VERSION is the current revision of the session protocol, peers reject
	// messages carrying any other version.
	VERSION = 1
	// SPECS_HEADER_LEN is the length of the fixed part of a session request.
	SPECS_HEADER_LEN = 14
	// RESPONSE_HEADER_LEN is the length of the fixed part of a session response.
	RESPONSE_HEADER_LEN = 6
	// FIELD_HEADER_LEN is the length of a single field header. (type + length)
	FIELD_HEADER_LEN = 3
	// MAX_FIELD_LEN is the largest value a single field can carry.
	MAX_FIELD_LEN = 0xFFFF
	// MAX_FIELDS is the largest number of fields a session request can carry.
	MAX_FIELDS = 0xFF
)

// Command identifies the subcommand requested by the connecting peer, the
// listening peer dispatches the session based on this value.
type Command uint8

const (
	// CMD_NETCAT requests a raw data channel. (netcat subcommand)
	CMD_NETCAT Command = iota
	// CMD_SSH requests an interactive terminal session. (ssh subcommand)
	CMD_SSH
	// CMD_SCP requests a file transfer stream. (scp subcommand)
	CMD_SCP
	// CMD_FORWARD requests a TCP tunnel to the address carried in the
	// FIELD_ADDRESS field, it backs the local port forwarding of the ssh
	// subcommand and is never requested directly by the user.
	CMD_FORWARD
)

// Flag values are combined into the flags byte of a session request.
type Flag uint8

const (
	// FLAG_PTY requests a pseudo terminal for the session.
	FLAG_PTY Flag = 1 << iota
	// FLAG_RECURSIVE requests a recursive file transfer. (scp -r)
	FLAG_RECURSIVE
	// FLAG_UPLOAD marks a file transfer where the connecting peer is the
	// sender, without it the listening peer sends the files.
	FLAG_UPLOAD
)

// Field identifies the contents of a single session request field.
type Field uint8

const (
	// FIELD_COMMAND carries the command line executed on the remote peer.
	FIELD_COMMAND Field = 1
	// FIELD_ADDRESS carries a "host:port" tunnel destination.
	FIELD_ADDRESS Field = 2
	// FIELD_PATH carries the remote path of a file transfer.
	FIELD_PATH Field = 3
)

// Status is the result of a session request reported by the listening peer.
type Status uint8

const (
	// STATUS_OK reports an accepted session, the payload stream follows.
	STATUS_OK Status = iota
	// STATUS_ERROR reports a generic failure.
	STATUS_ERROR
	// STATUS_UNSUPPORTED reports an unknown or unimplemented command.
	STATUS_UNSUPPORTED
	// STATUS_NOT_FOUND reports a missing path.
	STATUS_NOT_FOUND
	// STATUS_NOT_REGULAR reports a path which is not a regular file.
	STATUS_NOT_REGULAR
	// STATUS_DENIED reports a rejected session.
	STATUS_DENIED
)

var (
	ErrInvalidMagic       = errors.New("Invalid session protocol magic.")
	ErrInvalidVersion     = errors.New("Unsupported session protocol version.")
	ErrInvalidField       = errors.New("Invalid session specs field.")
	ErrUnsupportedCommand = errors.New("Unsupported session command.")
	ErrSpecsTooLarge      = errors.New("Session specs exceed the protocol limits.")
)

// Winsize describes the terminal window size.
//
// The field order matches the kernel winsize structure, it is passed to the
// TIOCGWINSZ ioctl as is.
type Winsize struct {
	Rows uint16 // ws_row: Number of rows (in cells)
	Cols uint16 // ws_col: Number of columns (in cells)
	X    uint16 // ws_xpixel: Width in pixels
	Y    uint16 // ws_ypixel: Height in pixels
}

// IsZero reports whether the window size carries no dimensions.
func (ws Winsize) IsZero() bool {
	return ws.Rows == 0 && ws.Cols == 0
}

// SessionSpecs carries every parameter needed for starting a new session, it
// is sent by the connecting peer before any payload is streamed.
type SessionSpecs struct {
	// ID is the subcommand requested from the listening peer.
	ID Command
	// Flags are the session modifiers of the requested command.
	Flags Flag
	// TermSize is the terminal size of the connecting peer, a zero value
	// lets the listening peer pick its own dimensions.
	TermSize Winsize
	// Exec is the command line executed on the listening peer, an empty
	// value spawns the default shell.
	Exec string
	// Address is the "host:port" destination of a CMD_FORWARD session.
	Address string
	// Path is the remote path of a CMD_SCP session.
	Path string
}

// String returns the human readable name of a session command.
func (c Command) String() string {
	switch c {
	case CMD_NETCAT:
		return "netcat"
	case CMD_SSH:
		return "ssh"
	case CMD_SCP:
		return "scp"
	case CMD_FORWARD:
		return "forward"
	}
	return fmt.Sprintf("unknown(%d)", uint8(c))
}

// Has reports whether all of the given flags are set.
func (s *SessionSpecs) Has(flags Flag) bool {
	return s.Flags&flags == flags
}

// Set enables the given flags when v is true and clears them otherwise.
func (s *SessionSpecs) Set(flags Flag, v bool) {
	if v {
		s.Flags |= flags
		return
	}
	s.Flags &= ^flags
}

// fields returns the populated fields of the specs in a stable order.
func (s *SessionSpecs) fields() []struct {
	Type  Field
	Value string
} {
	all := []struct {
		Type  Field
		Value string
	}{
		{FIELD_COMMAND, s.Exec},
		{FIELD_ADDRESS, s.Address},
		{FIELD_PATH, s.Path},
	}
	set := all[:0]
	for _, f := range all {
		if f.Value != "" {
			set = append(set, f)
		}
	}
	return set
}

// Marshal encodes the session specs into its wire representation.
func (s *SessionSpecs) Marshal() ([]byte, error) {
	fields := s.fields()
	if len(fields) > MAX_FIELDS {
		return nil, ErrSpecsTooLarge
	}

	size := SPECS_HEADER_LEN
	for _, f := range fields {
		if len(f.Value) > MAX_FIELD_LEN {
			return nil, ErrSpecsTooLarge
		}
		size += FIELD_HEADER_LEN + len(f.Value)
	}

	buf := make([]byte, SPECS_HEADER_LEN, size)
	buf[0] = MAGIC_0
	buf[1] = MAGIC_1
	buf[2] = VERSION
	buf[3] = uint8(s.ID)
	buf[4] = uint8(s.Flags)
	buf[5] = uint8(len(fields))
	binary.BigEndian.PutUint16(buf[6:], s.TermSize.Rows)
	binary.BigEndian.PutUint16(buf[8:], s.TermSize.Cols)
	binary.BigEndian.PutUint16(buf[10:], s.TermSize.X)
	binary.BigEndian.PutUint16(buf[12:], s.TermSize.Y)

	for _, f := range fields {
		header := [FIELD_HEADER_LEN]byte{uint8(f.Type)}
		binary.BigEndian.PutUint16(header[1:], uint16(len(f.Value)))
		buf = append(buf, header[:]...)
		buf = append(buf, f.Value...)
	}
	return buf, nil
}

// WriteSpecs encodes the session specs and writes them in a single call so
// that the request lands in one frame of the underlying stream.
func WriteSpecs(w io.Writer, specs *SessionSpecs) error {
	data, err := specs.Marshal()
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

// ReadSpecs reads a session request from the given stream. Unknown fields are
// skipped so that peers speaking a later revision of the protocol stay
// readable as long as the header layout is unchanged.
func ReadSpecs(r io.Reader) (*SessionSpecs, error) {
	header := make([]byte, SPECS_HEADER_LEN)
	if _, err := io.ReadFull(r, header); err != nil {
		return nil, err
	}
	if header[0] != MAGIC_0 || header[1] != MAGIC_1 {
		return nil, ErrInvalidMagic
	}
	if header[2] != VERSION {
		return nil, ErrInvalidVersion
	}

	specs := &SessionSpecs{
		ID:    Command(header[3]),
		Flags: Flag(header[4]),
		TermSize: Winsize{
			Rows: binary.BigEndian.Uint16(header[6:]),
			Cols: binary.BigEndian.Uint16(header[8:]),
			X:    binary.BigEndian.Uint16(header[10:]),
			Y:    binary.BigEndian.Uint16(header[12:]),
		},
	}

	fieldHeader := make([]byte, FIELD_HEADER_LEN)
	for i := uint8(0); i < header[5]; i++ {
		if _, err := io.ReadFull(r, fieldHeader); err != nil {
			return nil, err
		}
		value := make([]byte, binary.BigEndian.Uint16(fieldHeader[1:]))
		if _, err := io.ReadFull(r, value); err != nil {
			return nil, err
		}
		switch Field(fieldHeader[0]) {
		case FIELD_COMMAND:
			specs.Exec = string(value)
		case FIELD_ADDRESS:
			specs.Address = string(value)
		case FIELD_PATH:
			specs.Path = string(value)
		}
	}
	return specs, nil
}

// Response is the reply of the listening peer to a session request.
type Response struct {
	Status  Status
	Message string
}

// WriteResponse writes a session response in a single call.
func WriteResponse(w io.Writer, status Status, msg string) error {
	if len(msg) > MAX_FIELD_LEN {
		msg = msg[:MAX_FIELD_LEN]
	}
	buf := make([]byte, RESPONSE_HEADER_LEN, RESPONSE_HEADER_LEN+len(msg))
	buf[0] = MAGIC_0
	buf[1] = MAGIC_1
	buf[2] = VERSION
	buf[3] = uint8(status)
	binary.BigEndian.PutUint16(buf[4:], uint16(len(msg)))
	buf = append(buf, msg...)
	_, err := w.Write(buf)
	return err
}

// WriteError reports a failed session request with the given status, the
// error text is sent to the connecting peer verbatim.
func WriteError(w io.Writer, status Status, err error) error {
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	return WriteResponse(w, status, msg)
}

// ReadResponse reads a session response from the given stream.
func ReadResponse(r io.Reader) (*Response, error) {
	header := make([]byte, RESPONSE_HEADER_LEN)
	if _, err := io.ReadFull(r, header); err != nil {
		return nil, err
	}
	if header[0] != MAGIC_0 || header[1] != MAGIC_1 {
		return nil, ErrInvalidMagic
	}
	if header[2] != VERSION {
		return nil, ErrInvalidVersion
	}

	msg := make([]byte, binary.BigEndian.Uint16(header[4:]))
	if _, err := io.ReadFull(r, msg); err != nil {
		return nil, err
	}
	return &Response{Status: Status(header[3]), Message: string(msg)}, nil
}

// Err returns the failure reported by the response, it returns nil when the
// session request has been accepted.
func (r *Response) Err() error {
	if r == nil {
		return ErrInvalidMagic
	}
	if r.Status == STATUS_OK {
		return nil
	}
	if r.Message != "" {
		return errors.New(r.Message)
	}
	switch r.Status {
	case STATUS_UNSUPPORTED:
		return ErrUnsupportedCommand
	case STATUS_NOT_FOUND:
		return errors.New("No such file or directory")
	case STATUS_NOT_REGULAR:
		return errors.New("Not a regular file")
	case STATUS_DENIED:
		return errors.New("Session request denied")
	}
	return errors.New("Session request failed")
}
