package config

import (
	"strings"
	"testing"
)

func TestParseURI(t *testing.T) {
	// A legacy secret stays usable, it is hashed into a UUID.
	legacy, err := ParseURI("qs://ElDdawDNV4Quz5SolGWT")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := ParseUUID(legacy); !ok {
		t.Errorf("ParseURI returned %q, which is not a UUID", legacy)
	}
	// The derivation needs to be stable across runs and peers.
	again, err := ParseURI("qs://ElDdawDNV4Quz5SolGWT")
	if err != nil || again != legacy {
		t.Errorf("ParseURI is not deterministic: %q != %q (%v)", again, legacy, err)
	}
	if other, _ := ParseURI("qs://ElDdawDNV4Quz5SolGWU"); other == legacy {
		t.Error("two different secrets derived the same UUID")
	}

	// A secret already in the UUID format is taken as it is.
	uuid := "f81d4fae-7dec-41d0-a765-00a0c91e6bf6"
	got, err := ParseURI("qs://" + uuid)
	if err != nil {
		t.Fatal(err)
	}
	if got != uuid {
		t.Errorf("got %q, want %q", got, uuid)
	}
	// Casing is normalized so that both peers derive the same identifier.
	if got, _ = ParseURI("qs://F81D4FAE-7DEC-41D0-A765-00A0C91E6BF6"); got != uuid {
		t.Errorf("got %q, want %q", got, uuid)
	}

	// The scheme is optional, a bare value is taken as a plain secret and
	// resolves to the same identifier as its URI form.
	for _, value := range []string{"ElDdawDNV4Quz5SolGWT", uuid, strings.ToUpper(uuid)} {
		want, err := ParseURI(URI_SCHEME + value)
		if err != nil {
			t.Fatal(err)
		}
		got, err := ParseURI(value)
		if err != nil {
			t.Errorf("ParseURI(%q) failed: %v", value, err)
			continue
		}
		if got != want {
			t.Errorf("ParseURI(%q) = %q, want %q", value, got, want)
		}
	}

	// A mistyped or foreign scheme is a mistake, not a secret, deriving
	// one from it would leave the peers under two identifiers.
	for _, uri := range []string{"", "qs:/secret", "qs:secret", "http://secret", "qs://"} {
		if _, err := ParseURI(uri); err == nil {
			t.Errorf("ParseURI(%q) accepted an invalid URI", uri)
		}
	}
}

func TestDerivedUUIDIsVersion8(t *testing.T) {
	// The derived identifiers need to stay valid RFC 9562 UUIDs so that
	// implementations in other languages can parse them.
	uuid := DeriveUUID("MySecret")
	if _, ok := ParseUUID(uuid); !ok {
		t.Fatalf("DeriveUUID returned %q, which is not a UUID", uuid)
	}
	if uuid[14] != '8' {
		t.Errorf("got version %q, want 8", uuid[14])
	}
	if variant := uuid[19]; variant != '8' && variant != '9' && variant != 'a' && variant != 'b' {
		t.Errorf("got variant %q, want one of 8, 9, a, b", variant)
	}
}

func TestRandomUUIDIsUnique(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 64; i++ {
		uuid, err := RandomUUID()
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := ParseUUID(uuid); !ok {
			t.Fatalf("RandomUUID returned %q, which is not a UUID", uuid)
		}
		if seen[uuid] {
			t.Fatalf("RandomUUID repeated %q", uuid)
		}
		seen[uuid] = true
	}
}

func TestParseRemote(t *testing.T) {
	cases := map[string]string{
		"qs://MySecret:/target/":           "/target/",
		"qs://MySecret:/target/remote.txt": "/target/remote.txt",
		"qs://MySecret:relative/path":      "relative/path",
		// A target carrying no path refers to the working directory.
		"qs://MySecret":  ".",
		"qs://MySecret:": ".",
	}
	want, err := NewSecret("MySecret")
	if err != nil {
		t.Fatal(err)
	}
	for target, wantPath := range cases {
		secret, path, err := ParseRemote(target)
		if err != nil {
			t.Errorf("ParseRemote(%q) failed: %v", target, err)
			continue
		}
		if secret != want {
			t.Errorf("ParseRemote(%q) secret = %q, want %q", target, secret, want)
		}
		if path != wantPath {
			t.Errorf("ParseRemote(%q) path = %q, want %q", target, path, wantPath)
		}
	}

	// A UUID carries no colon, so the first one always starts the path.
	uuid := "f81d4fae-7dec-41d0-a765-00a0c91e6bf6"
	secret, path, err := ParseRemote("qs://" + uuid + ":/tmp")
	if err != nil || secret != uuid || path != "/tmp" {
		t.Errorf("got (%q, %q, %v), want (%q, %q, nil)", secret, path, err, uuid, "/tmp")
	}

	if _, _, err := ParseRemote("./local/path"); err == nil {
		t.Error("ParseRemote accepted a local path")
	}
}
