package qsock

import (
	"crypto/md5"
	"sync"

	qsocket "github.com/qsocket/qsocket-go"
)

// UID_CHAIN_MAX is the number of links of a UID chain a listener moves along,
// the relay follows a chain no further than this either. Sessions opened
// beyond it share the last link.
const UID_CHAIN_MAX = 64

// UIDChain tracks the links of a listener's UID chain carrying one of its
// sessions.
//
// The first link of the chain is the UID of the secret, every following one is
// the md5 of the previous link (qsocket.NextUID). Once a session is
// established on a link the listener moves on to the next free one, so a
// second listener started with the same secret takes the first link without
// colliding with the first listener and terminating its sessions. The relay
// sends a connecting peer down the same chain, past every link with an ongoing
// session, until it reaches a listener.
//
// A link becomes free again when its sessions end, the listener always probes
// the first free link so it stays reachable where the relay stops looking.
// A session ends for the relay as soon as its connection does, so the link is
// freed on the first failed read or write too, not only on Close: a handler
// whose process outlives the connection (a shell left running on Windows,
// where nothing hangs it up) must not keep the listener away from the link
// the relay sends the next peer to.
type UIDChain struct {
	mu     sync.Mutex
	links  [UID_CHAIN_MAX][16]byte
	active [UID_CHAIN_MAX]int
}

// NewUIDChain builds the UID chain of the given secret.
func NewUIDChain(secret string) *UIDChain {
	c := &UIDChain{}
	c.links[0] = md5.Sum([]byte(secret))
	for i := 1; i < UID_CHAIN_MAX; i++ {
		c.links[i] = qsocket.NextUID(c.links[i-1])
	}
	return c
}

// Next returns the first link carrying no session and its UID, the last link
// of the chain when every one of them is in use.
func (c *UIDChain) Next() (int, [16]byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i, n := range c.active {
		if n == 0 {
			return i, c.links[i]
		}
	}
	last := UID_CHAIN_MAX - 1
	return last, c.links[last]
}

// Active returns the number of sessions open on the links of the chain.
func (c *UIDChain) Active() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	total := 0
	for _, n := range c.active {
		total += n
	}
	return total
}

// Open marks a session established on the given link. The returned session
// frees the link again on its first Close or failed Read or Write.
func (c *UIDChain) Open(link int, qs Session) Session {
	c.mu.Lock()
	c.active[link]++
	c.mu.Unlock()
	return &chainSession{Session: qs, release: func() {
		c.mu.Lock()
		c.active[link]--
		c.mu.Unlock()
	}}
}

// chainSession is a session holding a link of a UIDChain until it is closed
// or its connection fails. No deadlines are set on sessions, so every read or
// write error means the relay connection is gone.
type chainSession struct {
	Session
	once    sync.Once
	release func()
}

func (s *chainSession) Read(b []byte) (int, error) {
	n, err := s.Session.Read(b)
	if err != nil {
		s.once.Do(s.release)
	}
	return n, err
}

func (s *chainSession) Write(b []byte) (int, error) {
	n, err := s.Session.Write(b)
	if err != nil {
		s.once.Do(s.release)
	}
	return n, err
}

func (s *chainSession) Close() {
	s.Session.Close()
	s.once.Do(s.release)
}
