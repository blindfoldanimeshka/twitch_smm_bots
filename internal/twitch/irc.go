package twitch

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

const twitchAddr = "irc.chat.twitch.tv:6697"

type IRCClient struct {
	login   string
	token   string
	channel string

	mu     sync.Mutex
	conn   net.Conn
	writer *bufio.Writer
}

func NewIRCClient(login, token, channel string) *IRCClient {
	ch := channel
	if !strings.HasPrefix(ch, "#") {
		ch = "#" + ch
	}

	tok := strings.TrimSpace(token)
	if tok != "" && !strings.HasPrefix(strings.ToLower(tok), "oauth:") {
		tok = "oauth:" + tok
	}

	return &IRCClient{login: login, token: tok, channel: strings.ToLower(ch)}
}

func (c *IRCClient) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		return nil
	}

	conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 10 * time.Second}, "tcp", twitchAddr, &tls.Config{MinVersion: tls.VersionTLS12})
	if err != nil {
		return fmt.Errorf("dial twitch irc: %w", err)
	}

	c.conn = conn
	c.writer = bufio.NewWriter(conn)

	if err := c.sendRawLocked("PASS " + c.token); err != nil {
		c.closeLocked()
		return err
	}
	if err := c.sendRawLocked("NICK " + c.login); err != nil {
		c.closeLocked()
		return err
	}
	if err := c.sendRawLocked("JOIN " + c.channel); err != nil {
		c.closeLocked()
		return err
	}

	go c.readLoop(conn)
	return nil
}

func (c *IRCClient) SendMessage(text string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return fmt.Errorf("irc connection is not established")
	}

	msg := strings.TrimSpace(text)
	if msg == "" {
		return nil
	}

	return c.sendRawLocked(fmt.Sprintf("PRIVMSG %s :%s", c.channel, msg))
}

func (c *IRCClient) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closeLocked()
}

func (c *IRCClient) readLoop(conn net.Conn) {
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "PING ") {
			payload := strings.TrimPrefix(line, "PING ")
			_ = c.sendRaw("PONG " + payload)
		}
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == conn {
		c.closeLocked()
	}
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
	if c.conn != nil {
		_ = c.conn.Close()
	}
	c.conn = nil
	c.writer = nil
}
