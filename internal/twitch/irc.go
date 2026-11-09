package twitch

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"math/rand"
	"net"
	"strings"
	"sync"
	"time"
)

const twitchAddr = "irc.chat.twitch.tv:6697"

// P0-2: Rate limit handling
type IRCClient struct {
	login    string
	password string
	channel  string

	mu          sync.Mutex
	conn        net.Conn
	writer      *bufio.Writer
	connected   bool
	reconnecting bool
	lastPing    time.Time
	cancel      context.CancelFunc

	// P0-2: Rate limit state
	rateLimited    bool
	rateLimitUntil time.Time
	backoffAttempts int
	maxBackoff     time.Duration
}

func NewIRCClient(login, password, channel string) *IRCClient {
	ch := channel
	if !strings.HasPrefix(ch, "#") {
		ch = "#" + ch
	}

	return &IRCClient{
		login:    login,
		password: strings.TrimSpace(password),
		channel:  strings.ToLower(ch),
	}
}

func (c *IRCClient) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil && c.connected {
		return nil
	}

	log.Printf("[IRC %s] Connecting to Twitch IRC (%s)...", c.login, twitchAddr)

	conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 10 * time.Second}, "tcp", twitchAddr, &tls.Config{MinVersion: tls.VersionTLS12})
	if err != nil {
		return fmt.Errorf("dial twitch irc: %w", err)
	}

	c.conn = conn
	c.writer = bufio.NewWriter(conn)
	c.connected = true
	c.lastPing = time.Now()

	log.Printf("[IRC %s] Authenticating...", c.login)

	if err := c.sendRawLocked("PASS " + c.password); err != nil {
		c.closeLocked()
		return err
	}
	if err := c.sendRawLocked("NICK " + c.login); err != nil {
		c.closeLocked()
		return err
	}

	log.Printf("[IRC %s] Joining channel %s...", c.login, c.channel)

	if err := c.sendRawLocked("JOIN " + c.channel); err != nil {
		c.closeLocked()
		return err
	}

	log.Printf("[IRC %s] Successfully connected and joined %s", c.login, c.channel)

	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel
	go c.readLoop(ctx, conn)
	return nil
}

func (c *IRCClient) IsConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.connected
}

func (c *IRCClient) Reconnect() error {
	c.mu.Lock()
	if c.reconnecting {
		c.mu.Unlock()
		return fmt.Errorf("already reconnecting")
	}
	c.reconnecting = true
	c.mu.Unlock()

	c.Close()
	time.Sleep(2 * time.Second)

	c.mu.Lock()
	c.reconnecting = false
	c.mu.Unlock()

	return c.Connect()
}

func (c *IRCClient) SendMessage(text string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil || !c.connected {
		return fmt.Errorf("irc connection is not established")
	}

	msg := strings.TrimSpace(text)
	if msg == "" {
		return nil
	}

	log.Printf("[IRC %s] Sending message to %s: %s", c.login, c.channel, msg)

	err := c.sendRawLocked(fmt.Sprintf("PRIVMSG %s :%s", c.channel, msg))
	if err != nil {
		// P0-2: Check for rate limit error (420)
		if strings.Contains(err.Error(), "420") {
			c.handleRateLimitLocked()
			return fmt.Errorf("rate limited by Twitch (420)")
		}
	}
	return err
}

// P0-2: Internal rate limit handler (must be called with lock held)
func (c *IRCClient) handleRateLimitLocked() {
	c.rateLimited = true
	c.backoffAttempts++

	// Exponential backoff: 2s, 4s, 8s, 16s, 32s, max 5 minutes
	baseBackoff := time.Duration(1<<uint(c.backoffAttempts)) * time.Second
	if baseBackoff > 5*time.Minute {
		baseBackoff = 5 * time.Minute
	}

	// Add jitter (0-1 second)
	jitter := time.Duration(rand.Intn(1000)) * time.Millisecond
	c.rateLimitUntil = time.Now().Add(baseBackoff + jitter)

	log.Printf("[IRC %s] Rate limited (attempt %d), backing off until %v",
		c.login, c.backoffAttempts, c.rateLimitUntil)
}

func (c *IRCClient) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closeLocked()
}

// P0-2: Check if rate limited and sleep if needed. The sleep happens without
// holding c.mu, so Close/IsRateLimited/SendMessage are not blocked while the
// client waits out a rate limit.
func (c *IRCClient) WaitForRateLimit() {
	c.mu.Lock()
	if !c.rateLimited {
		c.mu.Unlock()
		return
	}
	until := c.rateLimitUntil
	c.mu.Unlock()

	if d := time.Until(until); d > 0 {
		log.Printf("[IRC %s] Rate limited, sleeping for %v", c.login, d)
		time.Sleep(d)
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	// Reset only if the limit we waited out is still the current one; if a new
	// backoff was scheduled while we slept, leave it in place.
	if !time.Now().Before(c.rateLimitUntil) {
		c.rateLimited = false
		c.backoffAttempts = 0
	}
}

// P0-2: Handle rate limit error (420)
func (c *IRCClient) HandleRateLimit() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.rateLimited = true
	c.backoffAttempts++

	// Exponential backoff: 2s, 4s, 8s, 16s, 32s, max 5 minutes
	baseBackoff := time.Duration(1<<uint(c.backoffAttempts)) * time.Second
	if baseBackoff > 5*time.Minute {
		baseBackoff = 5 * time.Minute
	}

	// Add jitter (0-1 second)
	jitter := time.Duration(rand.Intn(1000)) * time.Millisecond
	c.rateLimitUntil = time.Now().Add(baseBackoff + jitter)

	log.Printf("[IRC %s] Rate limited (attempt %d), backing off until %v",
		c.login, c.backoffAttempts, c.rateLimitUntil)
}

// P0-2: Check if currently rate limited
func (c *IRCClient) IsRateLimited() bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.rateLimited && time.Now().After(c.rateLimitUntil) {
		c.rateLimited = false
		c.backoffAttempts = 0
	}
	return c.rateLimited
}

// P0-2: Reset rate limit state (on successful reconnect)
func (c *IRCClient) ResetRateLimit() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.rateLimited = false
	c.rateLimitUntil = time.Time{}
	c.backoffAttempts = 0
}

func (c *IRCClient) readLoop(ctx context.Context, conn net.Conn) {
	scanner := bufio.NewScanner(conn)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				log.Printf("[IRC %s] Scanner error: %v", c.login, err)
			}
			break
		}

		line := scanner.Text()
		log.Printf("[IRC %s] Received: %s", c.login, line)

		if strings.HasPrefix(line, "PING ") {
			payload := strings.TrimPrefix(line, "PING ")
			log.Printf("[IRC %s] Responding to PING with PONG %s", c.login, payload)
			_ = c.sendRaw("PONG " + payload)
			c.mu.Lock()
			c.lastPing = time.Now()
			c.mu.Unlock()
		}
	}

	c.mu.Lock()
	if c.conn == conn {
		c.connected = false
		c.closeLocked()
		log.Printf("[IRC %s] Connection lost, will attempt reconnect on next message", c.login)
	}
	c.mu.Unlock()
}

func (c *IRCClient) sendRaw(line string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sendRawLocked(line)
}

func (c *IRCClient) sendRawLocked(line string) error {
	if c.writer == nil {
		return fmt.Errorf("writer is nil")
	}

	if _, err := c.writer.WriteString(line + "\r\n"); err != nil {
		return fmt.Errorf("write irc line: %w", err)
	}
	if err := c.writer.Flush(); err != nil {
		return fmt.Errorf("flush irc line: %w", err)
	}
	return nil
}

func (c *IRCClient) closeLocked() {
	if c.cancel != nil {
		c.cancel()
		c.cancel = nil
	}
	if c.conn != nil {
		_ = c.conn.Close()
	}
	c.conn = nil
	c.writer = nil
	c.connected = false
}
