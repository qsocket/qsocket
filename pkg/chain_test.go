package qsock

import (
	"crypto/md5"
	"net"
	"testing"
)

func newPipeSession(t *testing.T) Session {
	t.Helper()
	a, b := net.Pipe()
	t.Cleanup(func() { b.Close() })
	return pipeSession{a}
}

func TestUIDChainLinks(t *testing.T) {
	secret := "qs://example-secret"
	c := NewUIDChain(secret)
	want := md5.Sum([]byte(secret))
	for i := 0; i < UID_CHAIN_MAX; i++ {
		if c.links[i] != want {
			t.Fatalf("link %d = %x, want %x", i, c.links[i], want)
		}
		want = md5.Sum(want[:])
	}
}

func TestUIDChainMovesOnAndFreesLinks(t *testing.T) {
	c := NewUIDChain("qs://example-secret")
	link, uid := c.Next()
	if link != 0 || uid != c.links[0] {
		t.Fatalf("fresh chain starts at link %d", link)
	}

	first := c.Open(0, newPipeSession(t))
	if link, uid = c.Next(); link != 1 || uid != c.links[1] {
		t.Fatalf("after a session on link 0, next = %d", link)
	}
	second := c.Open(1, newPipeSession(t))
	if link, _ = c.Next(); link != 2 {
		t.Fatalf("after sessions on links 0 and 1, next = %d", link)
	}
	if n := c.Active(); n != 2 {
		t.Fatalf("Active() = %d, want 2", n)
	}

	// The first link is free again once its session ends, closing the
	// session twice must not release it twice.
	first.Close()
	first.Close()
	if link, _ = c.Next(); link != 0 {
		t.Fatalf("after link 0 is freed, next = %d", link)
	}
	if n := c.Active(); n != 1 {
		t.Fatalf("Active() = %d, want 1", n)
	}
	second.Close()
	if n := c.Active(); n != 0 {
		t.Fatalf("Active() = %d, want 0", n)
	}
}

func TestUIDChainSaturates(t *testing.T) {
	c := NewUIDChain("qs://example-secret")
	for i := 0; i < UID_CHAIN_MAX; i++ {
		link, _ := c.Next()
		c.Open(link, newPipeSession(t))
	}
	if link, uid := c.Next(); link != UID_CHAIN_MAX-1 || uid != c.links[UID_CHAIN_MAX-1] {
		t.Fatalf("saturated chain returned link %d", link)
	}
}
