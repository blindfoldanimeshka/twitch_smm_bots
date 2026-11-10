// Package tempmail provides short-lived anonymous inboxes on top of the
// Guerrilla Mail API. Each inbox is a Redis session holding the provider's
// sid_token: the token stays server-side, callers only ever see the random
// inbox id, and sessions expire one hour after creation.
package tempmail

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode"

	"twitch-smm-bot/internal/guerrilla"
)

const (
	// sessionTTL bounds an inbox's lifetime, matching the provider's own
	// one-hour window.
	sessionTTL = time.Hour

	// maxBodyBytes caps the plain-text body we hand out.
	maxBodyBytes = 256 << 10
)

// Sentinel errors. Handler layers map these to HTTP statuses; none of them
// ever carries provider material such as the sid_token.
var (
	// ErrInboxNotFound is returned for an unknown, expired or corrupt session.
	ErrInboxNotFound = errors.New("tempmail: inbox not found")

	// ErrInvalidAlias is returned when a requested alias cannot be claimed.
	ErrInvalidAlias = errors.New("tempmail: invalid alias")

	// ErrProvider marks a failure of the upstream Guerrilla call.
	ErrProvider = errors.New("tempmail: provider request failed")
)

// aliasPattern mirrors what Guerrilla accepts as an email local part: 5 to 64
// characters, alphanumerics plus dot, underscore and hyphen, anchored by an
// alphanumeric on both ends.
var aliasPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{3,62}[a-zA-Z0-9]$`)

// Cache stores serialized inbox sessions under an opaque id. A missing key
// must be reported as (nil, nil).
type Cache interface {
	SetTempMailSession(ctx context.Context, id string, data []byte, ttl time.Duration) error
	GetTempMailSession(ctx context.Context, id string) ([]byte, error)
	DeleteTempMailSession(ctx context.Context, id string) error
}

// Inbox is the public view of a temporary mailbox. It deliberately has no
// session token field: everything the client needs is reachable through ID.
type Inbox struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	ExpiresAt time.Time `json:"expires_at"`
	Messages  []Message `json:"messages,omitempty"`
}

// Message is one email in an inbox. Body is only filled by Service.Message,
// which converts the provider's HTML to plain text.
type Message struct {
	ID         string    `json:"id"`
	From       string    `json:"from"`
	Subject    string    `json:"subject"`
	Excerpt    string    `json:"excerpt"`
	Body       string    `json:"body,omitempty"`
	ReceivedAt time.Time `json:"received_at"`
}

// session is the Redis payload. It is the only type in this package that
// carries the sid_token, and it never leaves the package.
type session struct {
	SID       string    `json:"sid"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

// Service binds a Guerrilla client to a session cache. Safe for concurrent
// use; both dependencies are.
type Service struct {
	cache  Cache
	client *guerrilla.Client
}

// New returns a service storing Guerrilla sessions in cache.
func New(cache Cache, client *guerrilla.Client) *Service {
	return &Service{cache: cache, client: client}
}

// Create claims a Guerrilla inbox (renaming it to alias when one is given),
// stores the session under a fresh random id with a one-hour TTL and returns
// the public inbox view.
func (s *Service) Create(ctx context.Context, alias string) (*Inbox, error) {
	alias = strings.TrimSpace(alias)
	if alias != "" && !aliasPattern.MatchString(alias) {
		return nil, fmt.Errorf("%w: %q", ErrInvalidAlias, alias)
	}

	created, err := s.client.Create(ctx, alias)
	if err != nil {
		return nil, providerErr("create inbox", "", err)
	}

	id, err := newSessionID()
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	data, err := json.Marshal(session{SID: created.SID, Email: created.Email, CreatedAt: now})
	if err != nil {
		return nil, fmt.Errorf("tempmail: encode session: %w", err)
	}
	if err := s.cache.SetTempMailSession(ctx, id, data, sessionTTL); err != nil {
		return nil, fmt.Errorf("tempmail: store session: %w", err)
	}

	return &Inbox{ID: id, Email: created.Email, ExpiresAt: now.Add(sessionTTL)}, nil
}

// Messages lists the current mail of a stored inbox without bodies.
func (s *Service) Messages(ctx context.Context, id string) (*Inbox, error) {
	sess, err := s.loadSession(ctx, id)
	if err != nil {
		return nil, err
	}

	list, err := s.client.List(ctx, sess.SID)
	if err != nil {
		return nil, providerErr("list messages", sess.SID, err)
	}

	inbox := &Inbox{ID: id, Email: sess.Email, ExpiresAt: sess.CreatedAt.Add(sessionTTL)}
	for _, m := range list {
		inbox.Messages = append(inbox.Messages, Message{
			ID:         m.ID,
			From:       m.From,
			Subject:    m.Subject,
			Excerpt:    m.Excerpt,
			ReceivedAt: m.ReceivedAt,
		})
	}
	return inbox, nil
}

// Message fetches one mail of a stored inbox with its body converted to plain
// text.
func (s *Service) Message(ctx context.Context, id, mailID string) (*Message, error) {
	sess, err := s.loadSession(ctx, id)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(mailID) == "" {
		return nil, ErrInboxNotFound
	}

	fetched, err := s.client.Fetch(ctx, sess.SID, mailID)
	if err != nil {
		return nil, providerErr("fetch message", sess.SID, err)
	}

	return &Message{
		ID:         fetched.ID,
		From:       fetched.From,
		Subject:    fetched.Subject,
		Excerpt:    fetched.Excerpt,
		Body:       plainText(fetched.Body),
		ReceivedAt: fetched.ReceivedAt,
	}, nil
}

// Delete drops the stored session, which orphans the provider side of the
// inbox; the Guerrilla session itself dies with its own one-hour window.
func (s *Service) Delete(ctx context.Context, id string) error {
	if _, err := s.loadSession(ctx, id); err != nil {
		return err
	}
	if err := s.cache.DeleteTempMailSession(ctx, id); err != nil {
		return fmt.Errorf("tempmail: delete session: %w", err)
	}
	return nil
}

// loadSession reads a session blob from the cache. A missing, expired or
// unusable payload is indistinguishable to callers: ErrInboxNotFound.
func (s *Service) loadSession(ctx context.Context, id string) (*session, error) {
	raw, err := s.cache.GetTempMailSession(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("tempmail: read session: %w", err)
	}
	if len(raw) == 0 {
		return nil, ErrInboxNotFound
	}

	var sess session
	if err := json.Unmarshal(raw, &sess); err != nil || sess.SID == "" {
		return nil, ErrInboxNotFound
	}
	return &sess, nil
}

// newSessionID returns a random 32-character hex id. It has no relationship
// to the provider's sid_token.
func newSessionID() (string, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", fmt.Errorf("tempmail: generate session id: %w", err)
	}
	return hex.EncodeToString(buf[:]), nil
}

// providerErr wraps an upstream failure in ErrProvider. The Guerrilla client
// already scrubs its own errors, but the sid is removed once more here so no
// future provider change can leak it into an error string.
func providerErr(op, sid string, err error) error {
	return fmt.Errorf("tempmail: %s: %w: %s", op, ErrProvider, redact(err.Error(), sid))
}

// redact removes every occurrence of the session token (raw or percent
// encoded) from text.
func redact(text, sid string) string {
	if sid == "" {
		return text
	}
	text = strings.ReplaceAll(text, sid, "[redacted]")
	if encoded := url.QueryEscape(sid); encoded != sid {
		text = strings.ReplaceAll(text, encoded, "[redacted]")
	}
	return text
}

// scriptStylePattern matches whole script/style elements, attributes included.
// A non-greedy close keeps sibling blocks intact.
var scriptStylePattern = regexp.MustCompile(`(?is)<(?:script|style)\b[^>]*>.*?</(?:script|style)\s*>`)

// tagPattern matches any remaining tag or tag fragment.
var tagPattern = regexp.MustCompile(`(?is)<[^>]*>`)

// plainText converts an email HTML body to capped plain text: script and
// style blocks are dropped, every other tag is removed, entities are decoded
// afterwards (so a decoded "<" stays text), whitespace runs are collapsed and
// the result is capped at 256 KiB on a rune boundary.
func plainText(body string) string {
	if body == "" {
		return ""
	}

	text := scriptStylePattern.ReplaceAllString(body, " ")
	text = tagPattern.ReplaceAllString(text, " ")
	text = html.UnescapeString(text)

	fields := strings.FieldsFunc(text, func(r rune) bool {
		// unicode.IsSpace already covers NBSP (U+00A0); BOM is its own case.
		return unicode.IsSpace(r) || r == '\ufeff'
	})
	text = strings.Join(fields, " ")

	if len(text) > maxBodyBytes {
		// Cap on a rune boundary: a continuation byte never starts a rune.
		cut := maxBodyBytes
		for cut > 0 && text[cut]&0xc0 == 0x80 {
			cut--
		}
		text = text[:cut]
	}
	return text
}
