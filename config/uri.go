package config

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

const (
	// URI_SCHEME is the scheme prefix of the QSocket secret URI format.
	URI_SCHEME = "qs://"
	// URI_SCHEME_NAME is the scheme name on its own, without the
	// separator which follows it.
	URI_SCHEME_NAME = "qs:"
	// SCHEME_SEPARATOR separates the scheme name of a URI from the rest.
	SCHEME_SEPARATOR = "://"
	// UUID_LEN is the length of a canonical, dash separated UUID string.
	UUID_LEN = 36
)

var (
	ErrInvalidURI    = errors.New("Invalid QSocket secret, expected qs://<secret> or a plain secret")
	ErrInvalidRemote = errors.New("Invalid remote target, expected qs://<secret>:<path>")
	ErrEmptySecret   = errors.New("QSocket URI carries no secret.")
)

// uuidDashes are the offsets of the dashes inside a canonical UUID string.
var uuidDashes = [...]int{8, 13, 18, 23}

// IsURI reports whether the given value carries the QSocket URI scheme.
func IsURI(v string) bool {
	return strings.HasPrefix(v, URI_SCHEME)
}

// ParseURI resolves a secret given on the command line and normalizes it
// into the canonical UUID form.
//
// The qs:// scheme is optional, a value carrying no scheme at all is taken as
// a plain ASCII secret, which keeps the secret only invocations of the
// previous releases working. Secrets which are not already a UUID are hashed
// into one.
func ParseURI(uri string) (string, error) {
	if !IsURI(uri) {
		// A mistyped scheme is a mistake rather than a secret, deriving
		// one from it would leave the peers waiting for each other
		// under two identifiers which never match.
		if strings.Contains(uri, SCHEME_SEPARATOR) || strings.HasPrefix(uri, URI_SCHEME_NAME) {
			return "", fmt.Errorf("%w (got %q)", ErrInvalidURI, uri)
		}
		return NewSecret(uri)
	}
	return NewSecret(strings.TrimPrefix(uri, URI_SCHEME))
}

// ParseRemote splits a scp style remote target into its secret and path.
//
// The expected format is "qs://<secret>:<path>", a target carrying no path
// refers to the working directory of the remote peer. A UUID never contains a
// colon, hence the first one always separates the secret from the path.
//
// Unlike a bare secret the scheme is mandatory here, it is what tells the
// remote end of a transfer apart from the local path it is copied to.
func ParseRemote(target string) (secret string, path string, err error) {
	if !IsURI(target) {
		return "", "", fmt.Errorf("%w (got %q)", ErrInvalidRemote, target)
	}
	value := strings.TrimPrefix(target, URI_SCHEME)
	path = "."
	if idx := strings.IndexByte(value, ':'); idx != -1 {
		value, path = value[:idx], value[idx+1:]
		if path == "" {
			path = "."
		}
	}
	secret, err = NewSecret(value)
	return secret, path, err
}

// NewSecret normalizes a user supplied secret into the canonical UUID form,
// values which are not a valid UUID are hashed into one.
func NewSecret(value string) (string, error) {
	if value == "" {
		return "", ErrEmptySecret
	}
	if uuid, ok := ParseUUID(value); ok {
		return uuid, nil
	}
	return DeriveUUID(value), nil
}

// URI returns the qs:// URI representation of a canonical secret.
func URI(secret string) string {
	return URI_SCHEME + secret
}

// ParseUUID validates a canonical, dash separated UUID and returns it in its
// normalized (lower case) form.
func ParseUUID(v string) (string, bool) {
	if len(v) != UUID_LEN {
		return "", false
	}
	for _, i := range uuidDashes {
		if v[i] != '-' {
			return "", false
		}
	}
	// The dashes are at fixed offsets, so dropping them leaves the 32 hex
	// digits of the UUID behind.
	var hexDigits strings.Builder
	hexDigits.Grow(32)
	for i := 0; i < len(v); i++ {
		if v[i] == '-' {
			continue
		}
		hexDigits.WriteByte(v[i])
	}
	raw, err := hex.DecodeString(hexDigits.String())
	if err != nil || len(raw) != 16 {
		return "", false
	}
	return FormatUUID(raw), true
}

// DeriveUUID hashes an arbitrary secret into a deterministic UUID.
//
// The first 16 bytes of the SHA-256 digest of the secret are tagged as a
// version 8 (custom) UUID as described by RFC 9562. Both peers derive the
// same identifier from the same secret without exchanging anything, and the
// derivation is reproducible by implementations in other languages.
func DeriveUUID(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	raw := make([]byte, 16)
	copy(raw, sum[:16])
	raw[6] = (raw[6] & 0x0F) | 0x80 // Version 8. (custom)
	raw[8] = (raw[8] & 0x3F) | 0x80 // Variant 1. (RFC 4122/9562)
	return FormatUUID(raw)
}

// RandomUUID generates a cryptographically random version 4 UUID.
func RandomUUID() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	raw[6] = (raw[6] & 0x0F) | 0x40 // Version 4. (random)
	raw[8] = (raw[8] & 0x3F) | 0x80 // Variant 1. (RFC 4122/9562)
	return FormatUUID(raw), nil
}

// FormatUUID renders 16 raw bytes in the canonical UUID form.
func FormatUUID(raw []byte) string {
	if len(raw) != 16 {
		return ""
	}
	hexDigits := hex.EncodeToString(raw)
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		hexDigits[0:8],
		hexDigits[8:12],
		hexDigits[12:16],
		hexDigits[16:20],
		hexDigits[20:32],
	)
}
