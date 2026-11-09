package twitch

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	// anonymousPass is the well-known password Twitch expects from read-only
	// justinfan connections; it grants no write access.
	anonymousPass = "SCHMOOPIIE"

	chatterWindow   = 5 * time.Minute  // active = chatted within the last 300 s
	messageWindow   = 10 * time.Minute // drop chat lines older than 10 min
	messageMaxCount = 100              // keep at most 100 recent messages

	reconnectBase   = 5 * time.Second
	reconnectMax    = 60 * time.Second
	reconnectJitter = time.Second
)

// Chatter is a login seen chatting recently.
type Chatter struct {
	Login         string    `json:"login"`
	LastMessageAt time.Time `json:"last_message_at"`
}

// ChatMessage is a single read-only chat line captured from IRC.
type ChatMessage struct {
	Login string    `json:"login"`
	Text  string    `json:"text"`
	At    time.Time `json:"at"`
	IsBot bool      `json:"is_bot"`
}

// privmsgRe matches Twitch PRIVMSG lines without IRCv3 tags:
// :nick!nick@host.tmi.twitch.tv PRIVMSG #channel :message text
var privmsgRe = regexp.MustCompile(`^:([^!]+)![^@]*@[^ ]+ PRIVMSG #[^ ]+ :(.*)$`)

// parsePrivmsg extracts the sender login and message text from a PRIVMSG line.
func parsePrivmsg(line string) (login, text string, ok bool) {
	match := privmsgRe.FindStringSubmatch(line)
	if match == nil {
		return "", "", false
	}
	return match[1], match[2], true
}

// pongReply builds a well-formed PONG for a Twitch IRC PING line. It accepts
// both "PING :payload" and "PING payload" forms and returns "PONG :<payload>".
// For non-PING lines it returns ("", false).
func pongReply(line string) (reply string, ok bool) {
	rest, found := strings.CutPrefix(line, "PING")
	if !found {
		return "", false
	}
	payload := strings.TrimPrefix(strings.TrimSpace(rest), ":")
	return "PONG :" + payload, true
}

// ChatMonitor anonymously joins a Twitch channel over IRC and records chat
// traffic. It is strictly read-only: the only lines it ever writes are the
// anonymous handshake (PASS/NICK/JOIN) and PONG replies.
type ChatMonitor struct {
	dial func(ctx context.Context, address string) (net.Conn, error)
	now  func() time.Time

	mu        sync.Mutex
	chatters  map[string]time.Time
	messages  []ChatMessage
	channel   string
	gen       int
	cancel    context.CancelFunc
	conn      net.Conn
	connected bool
	done      chan struct{}
}

// NewChatMonitor creates a stopped monitor; call SetChannel to join a channel.
func NewChatMonitor() *ChatMonitor {
	return &ChatMonitor{
		dial:     dialTwitchIRC,
		now:      time.Now,
		chatters: make(map[string]time.Time),
	}
}

// dialTwitchIRC connects to the Twitch IRC server over TLS.
func dialTwitchIRC(ctx context.Context, address string) (net.Conn, error) {
	d := &tls.Dialer{
		NetDialer: &net.Dialer{Timeout: 10 * time.Second},
		Config:    &tls.Config{MinVersion: tls.VersionTLS12},
	}
	return d.DialContext(ctx, "tcp", address)
}

// SetChannel joins the given channel, stopping any previous connection first.
// An empty channel stops the monitor; the same channel is a no-op. Stored
// chatters and messages are cleared on every switch so data never leaks
// between channels.
func (m *ChatMonitor) SetChannel(channel string) {
	login := NormalizeChannel(channel)

	m.mu.Lock()
	if login == m.channel {
		m.mu.Unlock()
		return
	}

	m.stopLocked()

	m.channel = login
	m.chatters = make(map[string]time.Time)
	m.messages = nil
	m.connected = false
	m.gen++

	if login == "" {
		m.mu.Unlock()
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	m.cancel = cancel
	m.done = done
	gen := m.gen
	m.mu.Unlock()

	go m.runSession(ctx, gen, login, done)
}

// Connected reports whether the current read loop has a live connection.
func (m *ChatMonitor) Connected() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.connected
}

// ActiveChatters returns the unique logins that chatted within the window.
func (m *ChatMonitor) ActiveChatters(window time.Duration) []Chatter {
	m.mu.Lock()
	defer m.mu.Unlock()

	cutoff := m.now().Add(-window)
	for login, at := range m.chatters {
		if at.Before(cutoff) {
			delete(m.chatters, login)
		}
	}

	out := make([]Chatter, 0, len(m.chatters))
	for login, at := range m.chatters {
		out = append(out, Chatter{Login: login, LastMessageAt: at})
	}
	return out
}

// RecentMessages returns up to limit chat lines, newest first.
func (m *ChatMonitor) RecentMessages(limit int) []ChatMessage {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.pruneLocked()

	if limit <= 0 || len(m.messages) == 0 {
		return []ChatMessage{}
	}

	n := min(limit, len(m.messages))
	out := make([]ChatMessage, 0, n)
	for i := len(m.messages) - 1; i >= 0 && len(out) < n; i-- {
		out = append(out, m.messages[i])
	}
	return out
}

// Close stops the monitor and releases its connection.
func (m *ChatMonitor) Close() {
	m.SetChannel("")
}

// stopLocked cancels the running session and closes its connection.
// Callers must hold m.mu.
func (m *ChatMonitor) stopLocked() {
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
	if m.conn != nil {
		_ = m.conn.Close()
		m.conn = nil
	}
	m.done = nil
}

// runSession connects and reconnects with capped backoff until ctx is canceled.
func (m *ChatMonitor) runSession(ctx context.Context, gen int, channel string, done chan struct{}) {
	defer close(done)

	backoff := reconnectBase
	for {
		if ctx.Err() != nil {
			return
		}

		err := m.runConnection(ctx, gen, channel)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			log.Printf("[chat-monitor %s] disconnected, retrying: %v", channel, err)
		}

		delay := backoff + time.Duration(rand.Int63n(int64(reconnectJitter)))
		backoff = min(backoff*2, reconnectMax)

		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
	}
}

// runConnection performs the anonymous handshake and reads one connection
// until it drops or ctx is canceled.
func (m *ChatMonitor) runConnection(ctx context.Context, gen int, channel string) error {
	conn, err := m.dial(ctx, twitchAddr)
	if err != nil {
		return fmt.Errorf("dial %s: %w", twitchAddr, err)
	}
	defer conn.Close()

	m.mu.Lock()
	if m.gen != gen {
		m.mu.Unlock()
		return nil
	}
	m.conn = conn
	m.mu.Unlock()

	defer m.releaseConn(gen)

	// Anonymous read-only handshake: no capabilities, no write access.
	for _, line := range []string{
		"PASS " + anonymousPass,
		"NICK " + anonymousNick(),
		"JOIN #" + channel,
	} {
		if err := writeLine(conn, line); err != nil {
			return fmt.Errorf("send %s: %w", strings.Fields(line)[0], err)
		}
	}

	m.mu.Lock()
	if m.gen != gen {
		m.mu.Unlock()
		return nil
	}
	m.connected = true
	m.mu.Unlock()

	scanner := bufio.NewScanner(conn)
	for {
		if ctx.Err() != nil {
			return nil
		}
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return fmt.Errorf("read irc: %w", err)
			}
			return fmt.Errorf("connection closed")
		}

		line := strings.TrimSpace(scanner.Text())
		if reply, ok := pongReply(line); ok {
			if err := writeLine(conn, reply); err != nil {
				return fmt.Errorf("send PONG: %w", err)
			}
			continue
		}
		switch {
		case line == "":
		default:
			if login, text, ok := parsePrivmsg(line); ok {
				m.recordSession(gen, login, text)
			}
		}
	}
}

// releaseConn clears connection state if this session is still current.
func (m *ChatMonitor) releaseConn(gen int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.gen != gen {
		return
	}
	m.conn = nil
	m.connected = false
}

// recordSession records a chat line only if the session is still current.
func (m *ChatMonitor) recordSession(gen int, login, text string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.gen != gen {
		return
	}
	m.storeLocked(login, text, m.now())
}

// record stores a chat line and updates the chatter map.
func (m *ChatMonitor) record(login, text string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.storeLocked(login, text, m.now())
}

// storeLocked appends a chat line and refreshes the chatter map.
// Callers must hold m.mu.
func (m *ChatMonitor) storeLocked(login, text string, at time.Time) {
	m.chatters[login] = at
	m.messages = append(m.messages, ChatMessage{Login: login, Text: text, At: at})
	m.pruneLocked()
}

// pruneLocked drops stale chatters and messages and caps the message buffer.
// Callers must hold m.mu.
func (m *ChatMonitor) pruneLocked() {
	now := m.now()

	chatterCutoff := now.Add(-chatterWindow)
	for login, at := range m.chatters {
		if at.Before(chatterCutoff) {
			delete(m.chatters, login)
		}
	}

	messageCutoff := now.Add(-messageWindow)
	drop := 0
	for drop < len(m.messages) && m.messages[drop].At.Before(messageCutoff) {
		drop++
	}
	m.messages = m.messages[drop:]

	if len(m.messages) > messageMaxCount {
		m.messages = m.messages[len(m.messages)-messageMaxCount:]
	}
}

// anonymousNick generates a justinfan<random> nickname for anonymous login.
func anonymousNick() string {
	return fmt.Sprintf("justinfan%d", 100000+rand.Intn(900000))
}

// writeLine sends one IRC line terminated with CRLF.
func writeLine(conn net.Conn, line string) error {
	_, err := io.WriteString(conn, line+"\r\n")
	return err
}
