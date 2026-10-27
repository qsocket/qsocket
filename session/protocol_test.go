package session

import (
	"bytes"
	"io"
	"testing"
)

func TestSpecsRoundTrip(t *testing.T) {
	specs := &SessionSpecs{
		ID:       CMD_SCP,
		TermSize: Winsize{Rows: 40, Cols: 157, X: 1, Y: 2},
		Exec:     "uname -a; id",
		Address:  "intra.example.com:80",
		Path:     "/etc/nginx",
	}
	specs.Set(FLAG_RECURSIVE|FLAG_UPLOAD, true)

	buf := &bytes.Buffer{}
	if err := WriteSpecs(buf, specs); err != nil {
		t.Fatal(err)
	}
	// Three fields are populated, the header carries nothing else.
	wantLen := SPECS_HEADER_LEN + 3*FIELD_HEADER_LEN +
		len(specs.Exec) + len(specs.Address) + len(specs.Path)
	if buf.Len() != wantLen {
		t.Errorf("encoded %d bytes, want %d", buf.Len(), wantLen)
	}

	got, err := ReadSpecs(buf)
	if err != nil {
		t.Fatal(err)
	}
	if *got != *specs {
		t.Errorf("got %+v, want %+v", *got, *specs)
	}
	if !got.Has(FLAG_RECURSIVE) || !got.Has(FLAG_UPLOAD) || got.Has(FLAG_PTY) {
		t.Errorf("flags did not survive the round trip: %08b", got.Flags)
	}
}

func TestSpecsSkipsEmptyFields(t *testing.T) {
	specs := &SessionSpecs{ID: CMD_SSH}
	specs.Set(FLAG_PTY, true)

	data, err := specs.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != SPECS_HEADER_LEN {
		t.Fatalf("encoded %d bytes, want %d", len(data), SPECS_HEADER_LEN)
	}
	got, err := ReadSpecs(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if *got != *specs {
		t.Errorf("got %+v, want %+v", *got, *specs)
	}
}

func TestReadSpecsRejectsGarbage(t *testing.T) {
	for name, input := range map[string][]byte{
		"magic":   append([]byte("XX"), make([]byte, SPECS_HEADER_LEN-2)...),
		"version": append([]byte{MAGIC_0, MAGIC_1, VERSION + 1}, make([]byte, SPECS_HEADER_LEN-3)...),
		"short":   {MAGIC_0, MAGIC_1, VERSION},
	} {
		if _, err := ReadSpecs(bytes.NewReader(input)); err == nil {
			t.Errorf("%s: accepted an invalid request", name)
		}
	}
}

func TestResponseRoundTrip(t *testing.T) {
	buf := &bytes.Buffer{}
	msg := "scp: download /target: not a regular file"
	if err := WriteResponse(buf, STATUS_NOT_REGULAR, msg); err != nil {
		t.Fatal(err)
	}
	resp, err := ReadResponse(buf)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != STATUS_NOT_REGULAR {
		t.Errorf("got status %d, want %d", resp.Status, STATUS_NOT_REGULAR)
	}
	// The message of the listening peer reaches the user verbatim.
	if err := resp.Err(); err == nil || err.Error() != msg {
		t.Errorf("got error %v, want %q", err, msg)
	}

	buf.Reset()
	if err := WriteResponse(buf, STATUS_OK, ""); err != nil {
		t.Fatal(err)
	}
	resp, err = ReadResponse(buf)
	if err != nil {
		t.Fatal(err)
	}
	if err := resp.Err(); err != nil {
		t.Errorf("an accepted session reported %v", err)
	}
}

func TestRecordRoundTrip(t *testing.T) {
	buf := &bytes.Buffer{}
	rec := &Record{Type: REC_FILE, Mode: 0644, Size: 4, Name: "nginx/nginx.conf"}
	if err := WriteRecord(buf, rec); err != nil {
		t.Fatal(err)
	}
	buf.WriteString("data")
	if err := WriteEndRecord(buf); err != nil {
		t.Fatal(err)
	}

	got, err := ReadRecord(buf)
	if err != nil {
		t.Fatal(err)
	}
	if *got != *rec {
		t.Errorf("got %+v, want %+v", *got, *rec)
	}
	payload := make([]byte, got.Size)
	if _, err := io.ReadFull(buf, payload); err != nil {
		t.Fatal(err)
	}
	if string(payload) != "data" {
		t.Errorf("got payload %q, want %q", payload, "data")
	}
	end, err := ReadRecord(buf)
	if err != nil {
		t.Fatal(err)
	}
	if end.Type != REC_END {
		t.Errorf("got record type %d, want REC_END", end.Type)
	}
}

func TestSafeName(t *testing.T) {
	for _, name := range []string{
		"../etc/passwd",
		"a/../../b",
		"/etc/passwd",
		"..",
		"",
		".",
		`windows\path`,
		"C:/windows/system32",
		"stream:$DATA",
	} {
		if got, err := SafeName(name); err == nil {
			t.Errorf("SafeName(%q) = %q, want an error", name, got)
		}
	}

	for name, want := range map[string]string{
		"a.txt":            "a.txt",
		"nginx/nginx.conf": "nginx/nginx.conf",
		"./a/b.txt":        "a/b.txt",
		"a//b.txt":         "a/b.txt",
		"a/./b.txt":        "a/b.txt",
	} {
		got, err := SafeName(name)
		if err != nil {
			t.Errorf("SafeName(%q) failed: %v", name, err)
			continue
		}
		if got != want {
			t.Errorf("SafeName(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestReadRecordRejectsUnsafeName(t *testing.T) {
	buf := &bytes.Buffer{}
	if err := WriteRecord(buf, &Record{Type: REC_FILE, Name: "../../etc/passwd"}); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadRecord(buf); err == nil {
		t.Error("accepted a record escaping the destination")
	}
}
