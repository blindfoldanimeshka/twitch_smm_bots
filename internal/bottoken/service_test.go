package bottoken

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"twitch-smm-bot/internal/storage"
	"twitch-smm-bot/internal/tokencrypt"
	"twitch-smm-bot/internal/twitchtoken"
)

// testCipher is a deterministic 32-byte AES key, never a real secret.
func testCipher(t *testing.T) *tokencrypt.Cipher {
	t.Helper()
	key := base64.StdEncoding.EncodeToString(bytes32(0x11))
	cipher, err := tokencrypt.NewCipher(key)
	require.NoError(t, err)
	return cipher
}

func bytes32(b byte) []byte {
	out := make([]byte, 32)
	for i := range out {
		out[i] = b
	}
	return out
}

type setCall struct {
	id       string
	access   string
	enc      string
	expiry   time.Time
	clientID string
	scopes   []string
}

// fakeStore records SetBotToken calls and serves GetBot from a map.
// setFailures makes the first N writes fail (transient store outage) and
// setErr fails every remaining write; setAttempts counts all write calls.
type fakeStore struct {
	bots        map[string]*storage.BotConfig
	setCalls    []setCall
	setErr      error
	setFailures int
	setAttempts int
}

func newFakeStore() *fakeStore {
	return &fakeStore{bots: map[string]*storage.BotConfig{}}
}

func (f *fakeStore) GetBot(_ context.Context, id string) (*storage.BotConfig, error) {
	b, ok := f.bots[id]
	if !ok {
		return nil, errors.New("get bot: no rows")
	}
	return b, nil
}

func (f *fakeStore) SetBotToken(_ context.Context, id, accessToken, refreshTokenEnc string, expiresAt time.Time, clientID string, scopes []string) error {
	f.setAttempts++
	if f.setFailures > 0 {
		f.setFailures--
		return errors.New("db down")
	}
	if f.setErr != nil {
		return f.setErr
	}
	f.setCalls = append(f.setCalls, setCall{id: id, access: accessToken, enc: refreshTokenEnc, expiry: expiresAt, clientID: clientID, scopes: scopes})
	if b, ok := f.bots[id]; ok {
		b.Password = accessToken
		b.TokenRefreshEnc = refreshTokenEnc
		b.TokenExpiresAt = &expiresAt
		b.TokenClientID = clientID
		b.TokenScopes = scopes
	}
	return nil
}

// fakeTTG is an httptest stand-in for the Twitch Token Generator API.
type fakeTTG struct {
	server       *httptest.Server
	client       *twitchtoken.Client
	validateUser string
	validateCode int
	refreshOK    bool
	refreshCode  int
	validateHits int
	refreshHits  int
}

func newFakeTTG(t *testing.T) *fakeTTG {
	t.Helper()
	f := &fakeTTG{validateUser: "cooluser", validateCode: 200, refreshOK: true, refreshCode: 200}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/tokens/validate":
			f.validateHits++
			if f.validateCode != 200 {
				w.WriteHeader(f.validateCode)
				return
			}
			_, _ = w.Write([]byte(`{"success":true,"username":"` + f.validateUser + `","user_id":"42","client_id":"cid","expires_in":10000,"scopes":["chat:read","chat:edit"]}`))
		case "/api/v2/tokens/refresh":
			f.refreshHits++
			if f.refreshCode != 200 || !f.refreshOK {
				w.WriteHeader(f.refreshCode)
				return
			}
			_, _ = w.Write([]byte(`{"success":true,"access_token":"newaccess","refresh_token":"newrefresh","client_id":"cid","expires_in":5000,"scopes":["chat:read","chat:edit"]}`))
		default:
			t.Errorf("unexpected TTG path %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(f.server.Close)
	f.client = twitchtoken.NewClient(f.server.URL, nil)
	return f
}

func TestImportValidatesAndStores(t *testing.T) {
	ttg := newFakeTTG(t)
	store := newFakeStore()
	svc := New(store, testCipher(t), ttg.client)

	status, err := svc.Import(context.Background(), "bot-1", "CoolUser", "oauth:abc", "refreshsecret")
	require.NoError(t, err)

	require.Len(t, store.setCalls, 1)
	call := store.setCalls[0]
	assert.Equal(t, "bot-1", call.id)
	assert.Equal(t, "oauth:abc", call.access, "access must be stored with the oauth: prefix")
	assert.NotEqual(t, "refreshsecret", call.enc, "refresh token must be stored encrypted")
	assert.True(t, call.expiry.After(time.Now()), "expiry must be in the future")
	assert.Equal(t, "cid", call.clientID)
	assert.Equal(t, []string{"chat:read", "chat:edit"}, call.scopes)

	assert.True(t, status.Valid)
	assert.True(t, status.HasRefresh)
	assert.Equal(t, "CoolUser", status.Login)
	assert.Equal(t, 1, ttg.validateHits)
}

func TestImportRefreshIsDecryptable(t *testing.T) {
	ttg := newFakeTTG(t)
	store := newFakeStore()
	cipher := testCipher(t)
	svc := New(store, cipher, ttg.client)

	_, err := svc.Import(context.Background(), "bot-1", "cooluser", "abc", "refreshsecret")
	require.NoError(t, err)
	require.Len(t, store.setCalls, 1)

	plain, err := cipher.Decrypt(store.setCalls[0].enc)
	require.NoError(t, err)
	assert.Equal(t, "refreshsecret", plain)
}

func TestImportLoginMismatch(t *testing.T) {
	ttg := newFakeTTG(t)
	ttg.validateUser = "someoneelse"
	store := newFakeStore()
	svc := New(store, testCipher(t), ttg.client)

	_, err := svc.Import(context.Background(), "bot-1", "cooluser", "abc", "r")
	assert.ErrorIs(t, err, ErrLoginMismatch)
	assert.Empty(t, store.setCalls, "mismatch must not persist a token")
}

func TestImportInvalidTokenMapsToSentinel(t *testing.T) {
	ttg := newFakeTTG(t)
	ttg.validateCode = http.StatusUnauthorized
	store := newFakeStore()
	svc := New(store, testCipher(t), ttg.client)

	_, err := svc.Import(context.Background(), "bot-1", "cooluser", "bad", "")
	assert.ErrorIs(t, err, ErrInvalidToken)
	assert.NotContains(t, strings.ToLower(err.Error()), "bad", "error must not echo the token")
	assert.Empty(t, store.setCalls)
}

func TestImportUpstreamFailureNotInvalid(t *testing.T) {
	ttg := newFakeTTG(t)
	ttg.validateCode = http.StatusInternalServerError
	store := newFakeStore()
	svc := New(store, testCipher(t), ttg.client)

	_, err := svc.Import(context.Background(), "bot-1", "cooluser", "abc", "")
	assert.ErrorIs(t, err, ErrUpstream)
	assert.NotErrorIs(t, err, ErrInvalidToken)
}

func TestImportProviderBadRequestIsInvalidToken(t *testing.T) {
	ttg := newFakeTTG(t)
	ttg.validateCode = http.StatusBadRequest
	store := newFakeStore()
	svc := New(store, testCipher(t), ttg.client)

	_, err := svc.Import(context.Background(), "bot-1", "cooluser", "abc", "")
	assert.ErrorIs(t, err, ErrInvalidToken)
	assert.Empty(t, store.setCalls)
}

// Rate limiting is a capacity problem, not a verdict about the token: it must
// stay ErrUpstream so the API never tells the user their token is invalid.
func TestImportProviderRateLimitIsUpstream(t *testing.T) {
	ttg := newFakeTTG(t)
	ttg.validateCode = http.StatusTooManyRequests
	store := newFakeStore()
	svc := New(store, testCipher(t), ttg.client)

	_, err := svc.Import(context.Background(), "bot-1", "cooluser", "abc", "")
	assert.ErrorIs(t, err, ErrUpstream)
	assert.NotErrorIs(t, err, ErrInvalidToken)
}

func TestRefreshProviderRateLimitIsUpstream(t *testing.T) {
	ttg := newFakeTTG(t)
	ttg.refreshCode = http.StatusTooManyRequests
	store := newFakeStore()
	cipher := testCipher(t)
	svc := New(store, cipher, ttg.client)

	enc, err := cipher.Encrypt("oldrefresh")
	require.NoError(t, err)
	store.bots["bot-1"] = &storage.BotConfig{ID: "bot-1", TokenRefreshEnc: enc}

	_, err = svc.RefreshNow(context.Background(), "bot-1")
	assert.ErrorIs(t, err, ErrUpstream)
	assert.NotErrorIs(t, err, ErrTokenExpired)
}

// An HTTP 200 whose body says success:false carries no HTTP status, so it is
// reported as "the provider gave no usable answer" (ErrUpstream) rather than a
// token verdict.
func TestImportUnsuccessfulBodyIsUpstream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"success":false,"message":"provider detail"}`))
	}))
	t.Cleanup(srv.Close)

	store := newFakeStore()
	svc := New(store, testCipher(t), twitchtoken.NewClient(srv.URL, nil))

	_, err := svc.Import(context.Background(), "bot-1", "cooluser", "abc", "")
	assert.ErrorIs(t, err, ErrUpstream)
	assert.NotContains(t, err.Error(), "provider detail")
	assert.NotContains(t, err.Error(), "abc")
}

func TestImportEmptyAccessTokenIsInvalid(t *testing.T) {
	ttg := newFakeTTG(t)
	store := newFakeStore()
	svc := New(store, testCipher(t), ttg.client)

	_, err := svc.Import(context.Background(), "bot-1", "cooluser", "   ", "")
	assert.ErrorIs(t, err, ErrInvalidToken)
	assert.Equal(t, 0, ttg.validateHits, "empty token must not hit the network")
}

func TestImportNoCipherRejectsRefresh(t *testing.T) {
	ttg := newFakeTTG(t)
	store := newFakeStore()
	svc := New(store, nil, ttg.client)

	_, err := svc.Import(context.Background(), "bot-1", "cooluser", "abc", "refreshsecret")
	assert.ErrorIs(t, err, ErrEncryptionUnavailable)
	assert.Empty(t, store.setCalls, "cannot persist refresh without a cipher")
}

func TestImportAccessOnlyWithoutCipherSucceeds(t *testing.T) {
	ttg := newFakeTTG(t)
	store := newFakeStore()
	svc := New(store, nil, ttg.client)

	status, err := svc.Import(context.Background(), "bot-1", "cooluser", "abc", "")
	require.NoError(t, err)
	require.Len(t, store.setCalls, 1)
	assert.Equal(t, "", store.setCalls[0].enc, "no refresh stored")
	assert.False(t, status.HasRefresh)
	assert.True(t, status.Valid)
}

func TestImportStoreFailureReturnsError(t *testing.T) {
	ttg := newFakeTTG(t)
	store := newFakeStore()
	store.setErr = errors.New("db down")
	svc := New(store, testCipher(t), ttg.client)

	_, err := svc.Import(context.Background(), "bot-1", "cooluser", "abc", "r")
	require.Error(t, err)
}

func TestEnsureFreshValidTokenIsNoop(t *testing.T) {
	ttg := newFakeTTG(t)
	store := newFakeStore()
	svc := New(store, testCipher(t), ttg.client)

	exp := time.Now().Add(10 * time.Minute)
	cfg := storage.BotConfig{ID: "bot-1", TokenExpiresAt: &exp}

	out, err := svc.EnsureFresh(context.Background(), cfg)
	require.NoError(t, err)
	assert.Equal(t, exp, *out.TokenExpiresAt)
	assert.Equal(t, 0, ttg.refreshHits, "a still-valid token must not trigger refresh")
	assert.Equal(t, 0, ttg.validateHits)
	assert.Empty(t, store.setCalls)
}

func TestEnsureFreshNilExpiryIsNoop(t *testing.T) {
	ttg := newFakeTTG(t)
	store := newFakeStore()
	svc := New(store, testCipher(t), ttg.client)

	out, err := svc.EnsureFresh(context.Background(), storage.BotConfig{ID: "bot-1"})
	require.NoError(t, err)
	assert.Nil(t, out.TokenExpiresAt)
	assert.Equal(t, 0, ttg.refreshHits)
}

func TestEnsureFreshRefreshesExpiringToken(t *testing.T) {
	ttg := newFakeTTG(t)
	store := newFakeStore()
	cipher := testCipher(t)
	svc := New(store, cipher, ttg.client)

	enc, err := cipher.Encrypt("oldrefresh")
	require.NoError(t, err)

	exp := time.Now().Add(1 * time.Minute)
	cfg := storage.BotConfig{
		ID:              "bot-1",
		TwitchLogin:     "cooluser",
		Password:        "oauth:oldaccess",
		TokenRefreshEnc: enc,
		TokenExpiresAt:  &exp,
	}

	out, err := svc.EnsureFresh(context.Background(), cfg)
	require.NoError(t, err)

	assert.Equal(t, "oauth:newaccess", out.Password, "new access token must be stored with oauth: prefix")
	require.NotNil(t, out.TokenExpiresAt)
	assert.True(t, out.TokenExpiresAt.After(time.Now()), "new expiry must be in the future")

	require.Len(t, store.setCalls, 1)
	assert.Equal(t, "oauth:newaccess", store.setCalls[0].access)
	plain, err := cipher.Decrypt(store.setCalls[0].enc)
	require.NoError(t, err)
	assert.Equal(t, "newrefresh", plain, "new refresh token stored encrypted")
	assert.Equal(t, 1, ttg.refreshHits)
}

func TestEnsureFreshNoRefreshToken(t *testing.T) {
	ttg := newFakeTTG(t)
	store := newFakeStore()
	svc := New(store, testCipher(t), ttg.client)

	exp := time.Now().Add(1 * time.Minute)
	cfg := storage.BotConfig{ID: "bot-1", TokenExpiresAt: &exp} // empty refresh

	_, err := svc.EnsureFresh(context.Background(), cfg)
	assert.ErrorIs(t, err, ErrNoRefreshToken)
	assert.Empty(t, store.setCalls)
}

func TestEnsureFreshNoCipher(t *testing.T) {
	ttg := newFakeTTG(t)
	store := newFakeStore()
	svc := New(store, nil, ttg.client)

	exp := time.Now().Add(1 * time.Minute)
	cfg := storage.BotConfig{ID: "bot-1", TokenExpiresAt: &exp, TokenRefreshEnc: "whatever"}

	_, err := svc.EnsureFresh(context.Background(), cfg)
	assert.ErrorIs(t, err, ErrEncryptionUnavailable)
	assert.Equal(t, 0, ttg.refreshHits)
}

func TestEnsureFreshStoreFailureAfterRefresh(t *testing.T) {
	ttg := newFakeTTG(t)
	store := newFakeStore()
	cipher := testCipher(t)
	svc := New(store, cipher, ttg.client)

	enc, err := cipher.Encrypt("oldrefresh")
	require.NoError(t, err)
	exp := time.Now().Add(1 * time.Minute)
	cfg := storage.BotConfig{ID: "bot-1", TokenRefreshEnc: enc, TokenExpiresAt: &exp, Password: "oauth:old"}

	// The TTG refresh succeeds, but persisting the rotated pair fails every time.
	store.setErr = errors.New("db down")

	out, err := svc.EnsureFresh(context.Background(), cfg)
	require.Error(t, err, "a store failure after refresh must be reported")
	assert.Equal(t, "oauth:old", out.Password, "failed refresh must not report a rotated password")
	assert.Equal(t, 1, ttg.refreshHits)
	assert.Equal(t, 3, store.setAttempts, "the store write is retried up to 3 times")
}

// The provider exchange consumed the old refresh token, so a single transient
// store failure must not lose the rotated pair: the write is retried (and only
// the write) until it succeeds, and the caller gets a config matching exactly
// what ended up in the store.
func TestEnsureFreshStoreRetriesTransientFailure(t *testing.T) {
	ttg := newFakeTTG(t)
	store := newFakeStore()
	store.setFailures = 2 // the first two writes fail, the third persists the pair
	cipher := testCipher(t)
	svc := New(store, cipher, ttg.client)

	enc, err := cipher.Encrypt("oldrefresh")
	require.NoError(t, err)
	exp := time.Now().Add(1 * time.Minute)
	cfg := storage.BotConfig{ID: "bot-1", TokenRefreshEnc: enc, TokenExpiresAt: &exp, Password: "oauth:old"}

	out, err := svc.EnsureFresh(context.Background(), cfg)
	require.NoError(t, err, "a store outage that clears within the retries must keep the pair")

	assert.Equal(t, 1, ttg.refreshHits, "the provider exchange must not be retried")
	assert.Equal(t, 3, store.setAttempts, "the write must be attempted up to 3 times")

	require.Len(t, store.setCalls, 1)
	stored := store.setCalls[0]
	require.NotNil(t, out.TokenExpiresAt)
	assert.Equal(t, out.Password, stored.access, "returned access must be the stored access")
	assert.True(t, stored.expiry.Equal(*out.TokenExpiresAt), "returned expiry must be the stored expiry")
	assert.Equal(t, out.TokenClientID, stored.clientID)
	assert.Equal(t, out.TokenScopes, stored.scopes)

	outRefresh, err := cipher.Decrypt(out.TokenRefreshEnc)
	require.NoError(t, err)
	storedRefresh, err := cipher.Decrypt(stored.enc)
	require.NoError(t, err)
	assert.Equal(t, "newrefresh", outRefresh)
	assert.Equal(t, outRefresh, storedRefresh, "returned refresh must be the stored refresh")
}

func TestRefreshNowRequiresStoredRefresh(t *testing.T) {
	ttg := newFakeTTG(t)
	store := newFakeStore()
	svc := New(store, testCipher(t), ttg.client)

	store.bots["bot-1"] = &storage.BotConfig{ID: "bot-1", TwitchLogin: "cooluser"}

	_, err := svc.RefreshNow(context.Background(), "bot-1")
	assert.ErrorIs(t, err, ErrNoRefreshToken)
}

func TestRefreshNowRotates(t *testing.T) {
	ttg := newFakeTTG(t)
	store := newFakeStore()
	cipher := testCipher(t)
	svc := New(store, cipher, ttg.client)

	enc, err := cipher.Encrypt("oldrefresh")
	require.NoError(t, err)
	store.bots["bot-1"] = &storage.BotConfig{ID: "bot-1", TwitchLogin: "cooluser", TokenRefreshEnc: enc}

	status, err := svc.RefreshNow(context.Background(), "bot-1")
	require.NoError(t, err)
	require.Len(t, store.setCalls, 1)
	assert.Equal(t, "oauth:newaccess", store.setCalls[0].access)
	assert.True(t, status.Valid)
	assert.True(t, status.HasRefresh)
	assert.Equal(t, "cooluser", status.Login)
}

func TestRefreshRejectedByProviderIsExpired(t *testing.T) {
	ttg := newFakeTTG(t)
	ttg.refreshCode = http.StatusBadRequest
	store := newFakeStore()
	cipher := testCipher(t)
	svc := New(store, cipher, ttg.client)

	enc, err := cipher.Encrypt("oldrefresh")
	require.NoError(t, err)
	store.bots["bot-1"] = &storage.BotConfig{ID: "bot-1", TokenRefreshEnc: enc}

	_, err = svc.RefreshNow(context.Background(), "bot-1")
	assert.ErrorIs(t, err, ErrTokenExpired)
}

func TestRefreshUpstreamFailure(t *testing.T) {
	ttg := newFakeTTG(t)
	ttg.refreshCode = http.StatusServiceUnavailable
	store := newFakeStore()
	cipher := testCipher(t)
	svc := New(store, cipher, ttg.client)

	enc, err := cipher.Encrypt("oldrefresh")
	require.NoError(t, err)
	store.bots["bot-1"] = &storage.BotConfig{ID: "bot-1", TokenRefreshEnc: enc}

	_, err = svc.RefreshNow(context.Background(), "bot-1")
	assert.ErrorIs(t, err, ErrUpstream)
	assert.NotErrorIs(t, err, ErrTokenExpired)
}

func TestNilServiceEnsureFreshIsNoop(t *testing.T) {
	var svc *Service

	exp := time.Now().Add(-time.Minute) // expired: would normally force a refresh
	cfg := storage.BotConfig{ID: "bot-1", Password: "oauth:legacy", TokenExpiresAt: &exp, TokenRefreshEnc: "enc"}

	out, err := svc.EnsureFresh(context.Background(), cfg)
	require.NoError(t, err, "a nil service must never block a start")
	assert.Equal(t, cfg, out, "cfg must come back untouched")
}

func TestStatusDerivedFromConfig(t *testing.T) {
	svc := New(newFakeStore(), testCipher(t), twitchtoken.NewClient("http://unused", nil))

	future := time.Now().Add(time.Hour)
	past := time.Now().Add(-time.Hour)

	live := svc.Status(storage.BotConfig{ID: "b", TwitchLogin: "cooluser", TokenExpiresAt: &future, TokenRefreshEnc: "enc", TokenClientID: "cid", TokenScopes: []string{"chat:read"}})
	assert.True(t, live.Valid)
	assert.True(t, live.HasRefresh)
	assert.Equal(t, "cooluser", live.Login)
	assert.Equal(t, "cid", live.ClientID)
	assert.Equal(t, []string{"chat:read"}, live.Scopes)

	dead := svc.Status(storage.BotConfig{ID: "b", TokenExpiresAt: &past})
	assert.False(t, dead.Valid)

	legacy := svc.Status(storage.BotConfig{ID: "b"})
	assert.False(t, legacy.Valid, "a bot without expiry metadata is not reported valid")
}

func TestStatusOmitsTokenMaterial(t *testing.T) {
	future := time.Now().Add(time.Hour)
	st := StatusOf(storage.BotConfig{ID: "b", TwitchLogin: "cooluser", TokenExpiresAt: &future, TokenRefreshEnc: "ciphertext"})

	serialized := strings.ToLower(marshalString(st))
	assert.NotContains(t, serialized, "ciphertext")
	assert.NotContains(t, serialized, "token")
}

// marshalString encodes v for substring assertions in tests.
func marshalString(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(data)
}
