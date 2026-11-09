package twitch

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePrivmsg(t *testing.T) {
	login, text, ok := parsePrivmsg(":alice!alice@alice.tmi.twitch.tv PRIVMSG #chan :hello world")
	assert.True(t, ok)
	assert.Equal(t, "alice", login)
	assert.Equal(t, "hello world", text)

	_, _, ok = parsePrivmsg(":alice!alice@alice.tmi.twitch.tv JOIN #chan")
	assert.False(t, ok)
}

func TestChatMonitorTracksUniquesAndPrunes(t *testing.T) {
	now := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	m := NewChatMonitor()
	m.now = func() time.Time { return now }
	m.record("alice", "hi")
	m.record("bob", "yo")
	m.record("alice", "again")
	assert.Len(t, m.ActiveChatters(5*time.Minute), 2)

	now = now.Add(6 * time.Minute)
	assert.Empty(t, m.ActiveChatters(5*time.Minute))
}

func TestChatMonitorCapsMessages(t *testing.T) {
	m := NewChatMonitor()
	for i := 0; i < 120; i++ {
		m.record("user", fmt.Sprintf("m%d", i))
	}
	msgs := m.RecentMessages(50)
	require.Len(t, msgs, 50)
	assert.Equal(t, "m119", msgs[0].Text)
}

// waitUntil polls cond until it holds or the timeout elapses.
func waitUntil(t *testing.T, timeout time.Duration, cond func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out: %s", msg)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// readHandshake reads the three handshake lines the monitor must send on join.
// The read runs in a goroutine so a stuck handshake fails the test instead of
// hanging it.
func readHandshake(t *testing.T, server net.Conn) []string {
	t.Helper()
	type result struct {
		lines []string
		err   error
	}
	ch := make(chan result, 1)
	go func() {
		var lines []string
		reader := bufio.NewReader(server)
		for i := 0; i < 3; i++ {
			line, err := reader.ReadString('\n')
			if err != nil {
				ch <- result{lines, err}
				return
			}
			lines = append(lines, strings.TrimSpace(line))
		}
		ch <- result{lines, nil}
	}()
	select {
	case r := <-ch:
		require.NoError(t, r.err)
		return r.lines
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for handshake")
		return nil
	}
}

func TestChatMonitorSetChannelStopsPrevious(t *testing.T) {
	m := NewChatMonitor()

	type pipePair struct {
		client net.Conn
		server net.Conn
	}
	dialed := make(chan pipePair, 8)
	m.dial = func(ctx context.Context, address string) (net.Conn, error) {
		client, server := net.Pipe()
		dialed <- pipePair{client: client, server: server}
		return client, nil
	}

	// Join channel a: fake dial succeeds, monitor performs the anonymous
	// handshake, and a PRIVMSG from the server side is recorded.
	m.SetChannel("a")

	var pairA pipePair
	select {
	case pairA = <-dialed:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for first dial")
	}

	handshake := readHandshake(t, pairA.server)
	require.Len(t, handshake, 3)
	assert.Equal(t, "PASS SCHMOOPIIE", handshake[0])
	assert.True(t, strings.HasPrefix(handshake[1], "NICK justinfan"), "nick %q should be anonymous justinfan", handshake[1])
	assert.Equal(t, "JOIN #a", handshake[2])

	waitUntil(t, 2*time.Second, func() bool { return m.Connected() }, "connected after handshake on channel a")

	_, err := pairA.server.Write([]byte(":alice!alice@alice.tmi.twitch.tv PRIVMSG #a :hello\r\n"))
	require.NoError(t, err)
	waitUntil(t, 2*time.Second, func() bool {
		return len(m.RecentMessages(10)) == 1
	}, "message recorded for channel a")

	// Remember the session-exit signal for the old connection, then switch.
	m.mu.Lock()
	doneA := m.done
	m.mu.Unlock()
	require.NotNil(t, doneA)

	m.SetChannel("#b")

	// The old session goroutine must exit (bounded wait, no goroutine leak).
	select {
	case <-doneA:
	case <-time.After(2 * time.Second):
		t.Fatal("old monitor goroutine did not stop after channel switch")
	}

	// The old connection is closed: writes from the server side must fail.
	leakErr := make(chan error, 1)
	go func() {
		_, werr := pairA.server.Write([]byte(":bob!bob@bob.tmi.twitch.tv PRIVMSG #a :leak\r\n"))
		leakErr <- werr
	}()
	select {
	case werr := <-leakErr:
		require.Error(t, werr, "old connection should be closed after channel switch")
	case <-time.After(2 * time.Second):
		t.Fatal("write to old connection did not fail; connection was not closed")
	}

	// Nothing recorded after the switch: state was cleared with the old channel.
	assert.False(t, m.Connected(), "old connection should no longer be reported as connected")
	assert.Empty(t, m.RecentMessages(50), "no messages should leak after channel switch")

	// The new channel joins and records independently of the old one.
	var pairB pipePair
	select {
	case pairB = <-dialed:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for second dial")
	}
	handshakeB := readHandshake(t, pairB.server)
	require.Len(t, handshakeB, 3)
	assert.Equal(t, "PASS SCHMOOPIIE", handshakeB[0])
	assert.Equal(t, "JOIN #b", handshakeB[2])

	waitUntil(t, 2*time.Second, func() bool { return m.Connected() }, "connected after handshake on channel b")

	_, err = pairB.server.Write([]byte(":carol!carol@carol.tmi.twitch.tv PRIVMSG #b :hi b\r\n"))
	require.NoError(t, err)
	waitUntil(t, 2*time.Second, func() bool {
		msgs := m.RecentMessages(10)
		return len(msgs) == 1 && msgs[0].Text == "hi b"
	}, "message recorded for channel b")

	// Close and confirm the remaining goroutine stops (bounded wait).
	m.mu.Lock()
	doneB := m.done
	m.mu.Unlock()
	m.Close()
	select {
	case <-doneB:
	case <-time.After(2 * time.Second):
		t.Fatal("monitor goroutine did not stop after Close")
	}
	assert.False(t, m.Connected())
}
