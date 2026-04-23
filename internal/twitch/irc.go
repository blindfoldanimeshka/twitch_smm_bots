package twitch

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"
)

const twitchAddr = "irc.chat.twitch.tv:6697"

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

	conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 10 * time.Second}, "tcp", twitchAddr, &tls.Config{MinVersion: tls.VersionTLS12})
	if err != nil {
		return fmt.Errorf("dial twitch irc: %w", err)
	}

	c.conn = conn
	c.writer = bufio.NewWriter(conn)
	c.connected = true
	c.lastPing = time.Now()

	if err := c.sendRawLocked("PASS " + c.password); err != nil {
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

	return c.sendRawLocked(fmt.Sprintf("PRIVMSG %s :%s", c.channel, msg))
}

func (c *IRCClient) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closeLocked()
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
			break
		}

		line := scanner.Text()
		if strings.HasPrefix(line, "PING ") {
			payload := strings.TrimPrefix(line, "PING ")
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
