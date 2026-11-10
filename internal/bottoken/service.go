// Package bottoken manages imported Twitch tokens for bots: validating and
// persisting them on import, rotating them just before they expire, and
// reporting their status.
//
// Invariants:
//   - the access token lives in bots.password as "oauth:<token>" (IRC PASS);
//   - the refresh token lives only as tokencrypt ciphertext (never plaintext);
//   - no token value ever reaches a log line, an API response or an error
//     string — errors expose sentinel values only.
package bottoken

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"twitch-smm-bot/internal/storage"
	"twitch-smm-bot/internal/tokencrypt"
	"twitch-smm-bot/internal/twitchtoken"
)

// Sentinel errors for callers (the API maps them to HTTP statuses). They carry
// no token material; wrap them with %w and add only non-sensitive context.
var (
	// ErrInvalidToken: the access token is empty or Twitch rejected it.
	ErrInvalidToken = errors.New("bottoken: invalid access token")
	// ErrLoginMismatch: the token belongs to a different Twitch account than
	// the bot login.
	ErrLoginMismatch = errors.New("bottoken: token account does not match bot login")
	// ErrNoRefreshToken: no refresh token is stored (or the stored one is
	// undecryptable), so the token cannot be rotated.
	ErrNoRefreshToken = errors.New("bottoken: no refresh token stored")
	// ErrTokenExpired: the stored refresh token was rejected by Twitch, so the
	// token pair is dead and must be imported again.
	ErrTokenExpired = errors.New("bottoken: token expired and cannot be refreshed")
	// ErrEncryptionUnavailable: TOKEN_ENCRYPTION_KEY is not configured, so
	// refresh tokens cannot be stored or read.
	ErrEncryptionUnavailable = errors.New("bottoken: token encryption is not configured")
	// ErrUpstream: the token provider could not be reached or failed. Handlers
	// map it to 502; storage failures stay distinct (500).
	ErrUpstream = errors.New("bottoken: token provider request failed")
)

// refreshLead is how long before expiry EnsureFresh rotates the token pair, so
// a bot never connects with a token that dies mid-session.
const refreshLead = 5 * time.Minute

// storeRetryDelays are the waits between attempts to persist a rotated pair.
// The provider exchange already consumed the old refresh token, so a transient
// store failure must not lose the new pair; the exchange itself is never
// retried. Together with the first attempt this gives 3 store attempts.
var storeRetryDelays = []time.Duration{100 * time.Millisecond, 300 * time.Millisecond}

// oauthPrefix is what bots.password expects in front of the access token.
const oauthPrefix = "oauth:"

// TokenStore is the storage subset the service needs. *storage.Store is the
// production implementation.
type TokenStore interface {
	SetBotToken(ctx context.Context, id, accessToken, refreshTokenEnc string, expiresAt time.Time, clientID string, scopes []string) error
	GetBot(ctx context.Context, id string) (*storage.BotConfig, error)
}

// Status is the token-facing view of a bot: metadata only, never a token.
type Status struct {
	Valid      bool       `json:"valid"`
	Login      string     `json:"login"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	HasRefresh bool       `json:"has_refresh"`
	ClientID   string     `json:"client_id,omitempty"`
	Scopes     []string   `json:"scopes,omitempty"`
}

// Service imports and refreshes bot tokens. It is safe for concurrent use.
type Service struct {
	store  TokenStore
	cipher *tokencrypt.Cipher
	client *twitchtoken.Client
}

// New builds the service. cipher may be nil: refresh tokens are then rejected
// on import and rotation is disabled, while access-only imports still work.
func New(store TokenStore, cipher *tokencrypt.Cipher, client *twitchtoken.Client) *Service {
	return &Service{store: store, cipher: cipher, client: client}
}

// StatusOf derives the token status from a stored bot configuration. It is the
// single source of truth for "do we have a live, refreshable token here?"; the
// Service.Status method and the API DTO mapping both go through it.
func StatusOf(cfg storage.BotConfig) Status {
	st := Status{
		Login:      cfg.TwitchLogin,
		HasRefresh: cfg.TokenRefreshEnc != "",
		ClientID:   cfg.TokenClientID,
		Scopes:     cfg.TokenScopes,
	}
	if cfg.TokenExpiresAt != nil && !cfg.TokenExpiresAt.IsZero() {
		expiresAt := *cfg.TokenExpiresAt
		st.ExpiresAt = &expiresAt
		st.Valid = expiresAt.After(time.Now())
	}
	return st
}

// Status reports the token status of cfg. Provided on the service so callers
// holding only a *Service share one implementation.
func (s *Service) Status(cfg storage.BotConfig) Status {
	return StatusOf(cfg)
}

// Import validates an access token with the generator, checks that it belongs
// to login, and persists the pair. refreshToken may be empty (access-only
// import); when it is present a cipher is mandatory, because storing it in
// plaintext is never acceptable.
func (s *Service) Import(ctx context.Context, botID, login, accessToken, refreshToken string) (*Status, error) {
	if s == nil {
		return nil, ErrEncryptionUnavailable
	}

	access := stripOAuthPrefix(accessToken)
	if access == "" {
		return nil, fmt.Errorf("bottoken: import: %w", ErrInvalidToken)
	}
	refresh := strings.TrimSpace(refreshToken)
	if s.client == nil {
		return nil, fmt.Errorf("bottoken: import: %w: token provider is not configured", ErrUpstream)
	}

	validation, err := s.client.Validate(ctx, access)
	if err != nil {
		return nil, wrapProviderError("validate", ErrInvalidToken, err)
	}
	if !strings.EqualFold(strings.TrimSpace(validation.Username), strings.TrimSpace(login)) {
		return nil, fmt.Errorf("bottoken: import: %w", ErrLoginMismatch)
	}

	refreshEnc := ""
	if refresh != "" {
		if s.cipher == nil {
			return nil, fmt.Errorf("bottoken: import: %w: cannot store a refresh token without TOKEN_ENCRYPTION_KEY", ErrEncryptionUnavailable)
		}
		enc, err := s.cipher.Encrypt(refresh)
		if err != nil {
			return nil, fmt.Errorf("bottoken: import: encrypt refresh token: %w", err)
		}
		refreshEnc = enc
	}

	expiresAt := expiresAfterNow(validation.ExpiresIn)
	if err := s.store.SetBotToken(ctx, botID, oauthPrefix+access, refreshEnc, expiresAt, validation.ClientID, validation.Scopes); err != nil {
		return nil, fmt.Errorf("bottoken: import: store token: %w", err)
	}

	stored := storage.BotConfig{
		ID:              botID,
		TwitchLogin:     login,
		TokenRefreshEnc: refreshEnc,
		TokenClientID:   validation.ClientID,
		TokenScopes:     validation.Scopes,
	}
	if !expiresAt.IsZero() {
		stored.TokenExpiresAt = &expiresAt
	}
	status := StatusOf(stored)
	return &status, nil
}

// EnsureFresh rotates the token pair when cfg expires within refreshLead. It is
// a no-op (returns cfg unchanged) when the service is nil, when no expiry is
// recorded (a legacy password or an unknown provider expiry) or when the token
// still has plenty of life. Any other outcome refreshes through the provider
// and persists the new pair before the caller is allowed to connect.
func (s *Service) EnsureFresh(ctx context.Context, cfg storage.BotConfig) (storage.BotConfig, error) {
	if s == nil || cfg.TokenExpiresAt == nil || cfg.TokenExpiresAt.IsZero() {
		return cfg, nil
	}
	if cfg.TokenExpiresAt.After(time.Now().Add(refreshLead)) {
		return cfg, nil
	}
	return s.refresh(ctx, cfg)
}

// RefreshNow forces a rotation for botID regardless of the recorded expiry
// (the manual "refresh now" action).
func (s *Service) RefreshNow(ctx context.Context, botID string) (*Status, error) {
	if s == nil {
		return nil, ErrEncryptionUnavailable
	}

	cfg, err := s.store.GetBot(ctx, botID)
	if err != nil {
		return nil, fmt.Errorf("bottoken: refresh: load bot: %w", err)
	}

	updated, err := s.refresh(ctx, *cfg)
	if err != nil {
		return nil, err
	}

	status := StatusOf(updated)
	return &status, nil
}

// refresh performs the rotation: decrypt the stored refresh token, exchange it,
// persist the new pair, then — and only then — report success through the
// returned config. On any failure the input cfg is returned untouched so a
// caller can never start a bot with a password that is not in the database.
func (s *Service) refresh(ctx context.Context, cfg storage.BotConfig) (storage.BotConfig, error) {
	if cfg.TokenRefreshEnc == "" {
		return cfg, fmt.Errorf("bottoken: bot %s: %w", cfg.ID, ErrNoRefreshToken)
	}
	if s.cipher == nil {
		return cfg, fmt.Errorf("bottoken: bot %s: %w: cannot read the stored refresh token without TOKEN_ENCRYPTION_KEY", cfg.ID, ErrEncryptionUnavailable)
	}
	if s.client == nil {
		return cfg, fmt.Errorf("bottoken: bot %s: %w: token provider is not configured", cfg.ID, ErrUpstream)
	}

	refresh, err := s.cipher.Decrypt(cfg.TokenRefreshEnc)
	if err != nil {
		return cfg, fmt.Errorf("bottoken: bot %s: %w: stored refresh token could not be decrypted", cfg.ID, ErrNoRefreshToken)
	}

	tokens, err := s.client.Refresh(ctx, refresh)
	if err != nil {
		return cfg, wrapProviderError("refresh", ErrTokenExpired, err)
	}

	access := stripOAuthPrefix(tokens.AccessToken)
	if access == "" {
		return cfg, fmt.Errorf("bottoken: bot %s: %w: provider returned no access token", cfg.ID, ErrUpstream)
	}

	// Twitch rotates the refresh token on every call; an empty answer means the
	// generator kept the old one, so preserve its ciphertext.
	refreshEnc := cfg.TokenRefreshEnc
	if plain := strings.TrimSpace(tokens.RefreshToken); plain != "" {
		enc, err := s.cipher.Encrypt(plain)
		if err != nil {
			return cfg, fmt.Errorf("bottoken: bot %s: encrypt refresh token: %w", cfg.ID, err)
		}
		refreshEnc = enc
	}

	expiresAt := expiresAfterNow(tokens.ExpiresIn)
	clientID := tokens.ClientID
	if clientID == "" {
		clientID = cfg.TokenClientID
	}
	scopes := tokens.Scopes
	if scopes == nil {
		scopes = cfg.TokenScopes
	}

	password := oauthPrefix + access
	// The exchange above consumed the old refresh token: if we dropped the new
	// pair on a single transient write failure, the stored ciphertext would be
	// dead forever. Retry the write (never the provider call) before giving up.
	if err := s.storeRotated(ctx, cfg.ID, password, refreshEnc, expiresAt, clientID, scopes); err != nil {
		return cfg, fmt.Errorf("bottoken: bot %s: store refreshed token: %w", cfg.ID, err)
	}

	updated := cfg
	updated.Password = password
	updated.TokenRefreshEnc = refreshEnc
	updated.TokenClientID = clientID
	updated.TokenScopes = scopes
	if expiresAt.IsZero() {
		updated.TokenExpiresAt = nil
	} else {
		expiry := expiresAt
		updated.TokenExpiresAt = &expiry
	}
	return updated, nil
}

// storeRotated persists the freshly rotated pair, retrying transient failures
// with the storeRetryDelays backoff. It stops early when ctx is done (the
// remaining attempts could not succeed) and reports the last store error.
// Callers must not wrap token material into that error: SetBotToken failures
// are storage errors, not verdicts on the token.
func (s *Service) storeRotated(ctx context.Context, id, password, refreshEnc string, expiresAt time.Time, clientID string, scopes []string) error {
	var err error
	for attempt := 0; attempt <= len(storeRetryDelays); attempt++ {
		if attempt > 0 && !waitBackoff(ctx, storeRetryDelays[attempt-1]) {
			break
		}
		err = s.store.SetBotToken(ctx, id, password, refreshEnc, expiresAt, clientID, scopes)
		if err == nil {
			return nil
		}
	}
	return err
}

// waitBackoff sleeps for d and reports whether it completed before ctx ended.
func waitBackoff(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

// expiresAfterNow converts a provider expires_in (seconds) into an absolute
// deadline. A non-positive value means the provider did not tell us when the
// token dies: store the zero time, which EnsureFresh treats as "never refresh
// automatically" rather than "already expired".
func expiresAfterNow(expiresIn int) time.Time {
	if expiresIn <= 0 {
		return time.Time{}
	}
	return time.Now().Add(time.Duration(expiresIn) * time.Second)
}

// stripOAuthPrefix removes the "oauth:" prefix the token cannot be validated
// with, and re-added on store.
func stripOAuthPrefix(token string) string {
	t := strings.TrimSpace(token)
	if len(t) >= len(oauthPrefix) && strings.EqualFold(t[:len(oauthPrefix)], oauthPrefix) {
		t = strings.TrimSpace(t[len(oauthPrefix):])
	}
	return t
}

// wrapProviderError classifies a twitchtoken failure. A 4xx answer that
// judges the credential (bad signature, unknown or unauthorised token) is the
// rejection sentinel — the API maps it to 400 so the user can re-import.
// Capacity answers (429, 408), 5xx and plain transport/decode failures say
// nothing about the token, so they are ErrUpstream (502).
func wrapProviderError(op string, rejection error, err error) error {
	if status, ok := twitchtoken.StatusCode(err); ok && isTokenRejection(status) {
		return fmt.Errorf("bottoken: %s rejected the token (status %d): %w", op, status, rejection)
	}
	return fmt.Errorf("bottoken: %s request failed: %w: %w", op, ErrUpstream, err)
}

// isTokenRejection reports whether an HTTP status is the provider's verdict on
// the credential itself. It is an allowlist, not "every 4xx": a wrong rejection
// tells the user to throw away a good token, while a wrong 502 only asks them to
// retry. Rate limiting (429), request timeouts (408) and unexpected codes such
// as 404/405/422 therefore stay upstream.
func isTokenRejection(status int) bool {
	switch status {
	case http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden:
		return true
	default:
		return false
	}
}
