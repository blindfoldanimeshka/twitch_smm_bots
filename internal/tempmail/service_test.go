package tempmail

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"twitch-smm-bot/internal/guerrilla"
)

// testSID is the session token the fake provider hands out. It must only ever
// appear inside the stored session payload, never in a returned value, a
// serialized response or an error string.
const testSID = "s3cr3t-sid-token"

// defaultHTMLBody is a fetch_email body exercising script/style removal, tag
// stripping, entity decoding and whitespace collapsing.
const defaultHTMLBody = `<html><head><style>.x{display:none}</style><script>document.write("evil")</script></head>` +
	`<body><p>Hello&nbsp;<b>World</b> &amp; welcome<br>to the &lt;inbox&gt;</p></body></html>`

// fakeCache is an in-memory stand-in for the Redis session store. It mirrors
// the production contract: a missing key is (nil, nil).
type fakeCacheEntry struct {
	data []byte
	ttl  time.Duration
}

type fakeCache struct {
	entries map[string]fakeCacheEntry
	setErr  error
	getErr  error
	delErr  error
}

func newFakeCache() *fakeCache {
	return &fakeCache{entries: map[string]fakeCacheEntry{}}
}

func (f *fakeCache) SetTempMailSession(_ context.Context, id string, data []byte, ttl time.Duration) error {
	if f.setErr != nil {
		return f.setErr
	}
	f.entries[id] = fakeCacheEntry{data: data, ttl: ttl}
	return nil
}

func (f *fakeCache) GetTempMailSession(_ context.Context, id string) ([]byte, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	entry, ok := f.entries[id]
	if !ok {
		return nil, nil
	}
	return entry.data, nil
}

func (f *fakeCache) DeleteTempMailSession(_ context.Context, id string) error {
	if f.delErr != nil {
		return f.delErr
	}
	delete(f.entries, id)
	return nil
}

// fakeGuerrilla mimics the Guerrilla AJAX endpoint and records every call,
// including the sid_token each call carried.
type fakeGuerrilla struct {
	server *httptest.Server
	ops    []string
	sids   map[string]string
	failOn string
	body   string
}

func newFakeGuerrilla(t *testing.T) *fakeGuerrilla {
	t.Helper()
	f := &fakeGuerrilla{sids: map[string]string{}, body: defaultHTMLBody}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		op := r.FormValue("f")
		f.ops = append(f.ops, op)
		f.sids[op] = r.FormValue("sid_token")

		if f.failOn != "" && op == f.failOn {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		switch op {
		case "get_email_address":
			fmt.Fprintf(w, `{"email_addr":"guest@guerrillamail.com","sid_token":"%s"}`, testSID)
		case "set_email_user":
			fmt.Fprintf(w, `{"email_addr":"%s@guerrillamail.com","sid_token":"%s"}`, r.FormValue("email_user"), testSID)
		case "get_email_list":
			w.Write([]byte(`{"list":[{"mail_id":101,"mail_from":"sender@example.com",` +
				`"mail_subject":"Weekly digest","mail_excerpt":"top story","mail_timestamp":1700000000}],"sid_token":"rotated"}`))
		case "fetch_email":
			out, _ := json.Marshal(map[string]string{
				"mail_id":        "101",
				"mail_from":      "sender@example.com",
				"mail_subject":   "Weekly digest",
				"mail_body":      f.body,
				"mail_timestamp": "1700000000",
			})
			w.Write(out)
		case "del_email":
			w.Write([]byte(`{"success":true}`))
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	t.Cleanup(f.server.Close)
	return f
}

func newService(t *testing.T, cache *fakeCache, provider *fakeGuerrilla) *Service {
	t.Helper()
	return New(cache, guerrilla.NewClient(provider.server.URL, nil))
}

func marshalNoErr(t *testing.T, v interface{}) []byte {
	t.Helper()
	raw, err := json.Marshal(v)
	require.NoError(t, err)
	return raw
}

// assertPublicInbox checks the sid never surfaces in a value the caller can
// serialize.
func assertPublicInbox(t *testing.T, inbox *Inbox) {
	t.Helper()
	raw := marshalNoErr(t, inbox)
	assert.NotContains(t, string(raw), testSID)
	assert.NotContains(t, strings.ToLower(string(raw)), `"sid`)
}

func TestCreateStoresSessionWithOneHourTTLAndHidesSID(t *testing.T) {
	cache, provider := newFakeCache(), newFakeGuerrilla(t)
	svc := newService(t, cache, provider)

	before := time.Now().UTC()
	inbox, err := svc.Create(context.Background(), "johnny.test")
	require.NoError(t, err)

	require.NotNil(t, inbox)
	assert.Equal(t, "johnny.test@guerrillamail.com", inbox.Email)
	assert.NotEqual(t, testSID, inbox.ID)
	assert.Regexp(t, regexp.MustCompile(`^[0-9a-f]{32}$`), inbox.ID)
	assert.WithinDuration(t, before.Add(time.Hour), inbox.ExpiresAt, time.Minute)
	assert.Empty(t, inbox.Messages)
	assertPublicInbox(t, inbox)

	entry, ok := cache.entries[inbox.ID]
	require.True(t, ok, "the session must be stored under the inbox id")
	assert.Equal(t, time.Hour, entry.ttl)
	assert.Contains(t, string(entry.data), testSID, "only the backend copy carries the sid")

	var stored map[string]interface{}
	require.NoError(t, json.Unmarshal(entry.data, &stored))
	assert.Equal(t, testSID, stored["sid"])
	assert.Equal(t, "johnny.test@guerrillamail.com", stored["email"])
	assert.NotEmpty(t, stored["created_at"])
}

func TestCreateWithoutAliasKeepsProviderDefaultAddress(t *testing.T) {
	cache, provider := newFakeCache(), newFakeGuerrilla(t)
	svc := newService(t, cache, provider)

	inbox, err := svc.Create(context.Background(), "")
	require.NoError(t, err)

	assert.Equal(t, "guest@guerrillamail.com", inbox.Email)
	assert.Equal(t, []string{"get_email_address"}, provider.ops, "no rename may be attempted")
	assertPublicInbox(t, inbox)
}

func TestCreateGeneratesUniqueIDs(t *testing.T) {
	cache, provider := newFakeCache(), newFakeGuerrilla(t)
	svc := newService(t, cache, provider)

	first, err := svc.Create(context.Background(), "")
	require.NoError(t, err)
	second, err := svc.Create(context.Background(), "")
	require.NoError(t, err)

	assert.NotEqual(t, first.ID, second.ID)
	assert.Len(t, cache.entries, 2)
}

func TestCreateRejectsInvalidAlias(t *testing.T) {
	cases := []string{"ab", "abcd", "a b", "bad!char", "user@host", "_lead", "trail_", ".dot", strings.Repeat("a", 65)}

	for _, alias := range cases {
		t.Run(alias, func(t *testing.T) {
			cache, provider := newFakeCache(), newFakeGuerrilla(t)
			svc := newService(t, cache, provider)

			inbox, err := svc.Create(context.Background(), alias)

			assert.Nil(t, inbox)
			assert.ErrorIs(t, err, ErrInvalidAlias)
			assert.Empty(t, cache.entries, "a rejected alias must not store a session")
			assert.Empty(t, provider.ops, "a rejected alias must not reach the provider")
		})
	}
}

func TestCreateAcceptsCommonAliases(t *testing.T) {
	for _, alias := range []string{"abcde", "johnny.test", "a-b-c", "Z9_Z9"} {
		cache, provider := newFakeCache(), newFakeGuerrilla(t)
		svc := newService(t, cache, provider)

		_, err := svc.Create(context.Background(), alias)
		assert.NoError(t, err, "alias %q must be accepted", alias)
	}
}

func TestCreateProviderFailureWrapsErrProviderWithoutSID(t *testing.T) {
	t.Run("address", func(t *testing.T) {
		cache, provider := newFakeCache(), newFakeGuerrilla(t)
		provider.failOn = "get_email_address"
		svc := newService(t, cache, provider)

		inbox, err := svc.Create(context.Background(), "")

		assert.Nil(t, inbox)
		assert.ErrorIs(t, err, ErrProvider)
		assert.NotContains(t, err.Error(), testSID)
		assert.Empty(t, cache.entries)
	})

	t.Run("rename", func(t *testing.T) {
		cache, provider := newFakeCache(), newFakeGuerrilla(t)
		provider.failOn = "set_email_user"
		svc := newService(t, cache, provider)

		inbox, err := svc.Create(context.Background(), "johnny")

		assert.Nil(t, inbox)
		assert.ErrorIs(t, err, ErrProvider)
		assert.NotContains(t, err.Error(), testSID, "the sid must never enter an error")
		assert.Empty(t, cache.entries)
	})
}

func TestCreateSessionStoreFailureIsNotProviderError(t *testing.T) {
	cache, provider := newFakeCache(), newFakeGuerrilla(t)
	cache.setErr = errors.New("redis down")
	svc := newService(t, cache, provider)

	inbox, err := svc.Create(context.Background(), "")

	assert.Nil(t, inbox)
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrProvider)
	assert.NotErrorIs(t, err, ErrInboxNotFound)
}

func TestMessagesUnknownOrCorruptInboxIsNotFound(t *testing.T) {
	t.Run("missing", func(t *testing.T) {
		cache, provider := newFakeCache(), newFakeGuerrilla(t)
		svc := newService(t, cache, provider)

		inbox, err := svc.Messages(context.Background(), "00000000000000000000000000000000")

		assert.Nil(t, inbox)
		assert.ErrorIs(t, err, ErrInboxNotFound)
		assert.Empty(t, provider.ops, "a missing session must not reach the provider")
	})

	t.Run("expired", func(t *testing.T) {
		cache, provider := newFakeCache(), newFakeGuerrilla(t)
		svc := newService(t, cache, provider)

		created, err := svc.Create(context.Background(), "")
		require.NoError(t, err)
		delete(cache.entries, created.ID) // Redis TTL fired

		inbox, err := svc.Messages(context.Background(), created.ID)
		assert.Nil(t, inbox)
		assert.ErrorIs(t, err, ErrInboxNotFound)
		assert.Equal(t, []string{"get_email_address"}, provider.ops)
	})

	t.Run("corrupt", func(t *testing.T) {
		cache, provider := newFakeCache(), newFakeGuerrilla(t)
		cache.entries["deadbeef"] = fakeCacheEntry{data: []byte("not json"), ttl: time.Hour}
		svc := newService(t, cache, provider)

		inbox, err := svc.Messages(context.Background(), "deadbeef")
		assert.Nil(t, inbox)
		assert.ErrorIs(t, err, ErrInboxNotFound)
		assert.Empty(t, provider.ops)
	})
}

func TestMessagesReturnsInboxWithoutSID(t *testing.T) {
	cache, provider := newFakeCache(), newFakeGuerrilla(t)
	svc := newService(t, cache, provider)

	created, err := svc.Create(context.Background(), "michael")
	require.NoError(t, err)

	inbox, err := svc.Messages(context.Background(), created.ID)
	require.NoError(t, err)

	assert.Equal(t, created.ID, inbox.ID)
	assert.Equal(t, "michael@guerrillamail.com", inbox.Email)
	assert.True(t, created.ExpiresAt.Equal(inbox.ExpiresAt), "expiry must come from the stored created_at")

	require.Len(t, inbox.Messages, 1)
	msg := inbox.Messages[0]
	assert.Equal(t, "101", msg.ID)
	assert.Equal(t, "sender@example.com", msg.From)
	assert.Equal(t, "Weekly digest", msg.Subject)
	assert.Equal(t, "top story", msg.Excerpt)
	assert.Empty(t, msg.Body, "the list must not carry bodies")
	assert.Equal(t, time.Unix(1700000000, 0), msg.ReceivedAt)

	assert.Equal(t, testSID, provider.sids["get_email_list"], "the stored sid must be used against the provider")
	assertPublicInbox(t, inbox)
}

func TestMessagesProviderFailure(t *testing.T) {
	cache, provider := newFakeCache(), newFakeGuerrilla(t)
	svc := newService(t, cache, provider)

	created, err := svc.Create(context.Background(), "")
	require.NoError(t, err)
	provider.failOn = "get_email_list"

	inbox, err := svc.Messages(context.Background(), created.ID)
	assert.Nil(t, inbox)
	assert.ErrorIs(t, err, ErrProvider)
	assert.NotContains(t, err.Error(), testSID)
}

func TestMessagesCacheFailureIsNotSentinel(t *testing.T) {
	cache, provider := newFakeCache(), newFakeGuerrilla(t)
	svc := newService(t, cache, provider)

	created, err := svc.Create(context.Background(), "")
	require.NoError(t, err)
	cache.getErr = errors.New("redis down")

	inbox, err := svc.Messages(context.Background(), created.ID)
	assert.Nil(t, inbox)
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrInboxNotFound)
	assert.NotErrorIs(t, err, ErrProvider)
}

func TestMessageStripsHTMLAndDecodesEntities(t *testing.T) {
	cache, provider := newFakeCache(), newFakeGuerrilla(t)
	svc := newService(t, cache, provider)

	created, err := svc.Create(context.Background(), "")
	require.NoError(t, err)

	msg, err := svc.Message(context.Background(), created.ID, "101")
	require.NoError(t, err)

	assert.Equal(t, "101", msg.ID)
	assert.Equal(t, "sender@example.com", msg.From)
	assert.Equal(t, "Weekly digest", msg.Subject)
	assert.Equal(t, "Hello World & welcome to the <inbox>", msg.Body)
	assert.NotContains(t, msg.Body, "display:none")
	assert.NotContains(t, msg.Body, "document.write")
	assert.NotContains(t, msg.Body, "<b>")
	assert.NotContains(t, msg.Body, "nbsp")
	assert.NotContains(t, msg.Body, "&amp;")
	assert.Equal(t, testSID, provider.sids["fetch_email"])

	raw := marshalNoErr(t, msg)
	assert.NotContains(t, string(raw), testSID)
	assert.NotContains(t, strings.ToLower(string(raw)), `"sid`)
}

func TestMessageCapsBodySize(t *testing.T) {
	cache, provider := newFakeCache(), newFakeGuerrilla(t)
	provider.body = "<p>" + strings.Repeat("x", 400*1024) + "</p>"
	svc := newService(t, cache, provider)

	created, err := svc.Create(context.Background(), "")
	require.NoError(t, err)

	msg, err := svc.Message(context.Background(), created.ID, "101")
	require.NoError(t, err)

	assert.LessOrEqual(t, len(msg.Body), maxBodyBytes)
	assert.NotEmpty(t, msg.Body)
}

func TestMessageProviderFailure(t *testing.T) {
	cache, provider := newFakeCache(), newFakeGuerrilla(t)
	svc := newService(t, cache, provider)

	created, err := svc.Create(context.Background(), "")
	require.NoError(t, err)
	provider.failOn = "fetch_email"

	msg, err := svc.Message(context.Background(), created.ID, "101")
	assert.Nil(t, msg)
	assert.ErrorIs(t, err, ErrProvider)
	assert.NotContains(t, err.Error(), testSID)
}

func TestMessageUnknownInboxIsNotFound(t *testing.T) {
	cache, provider := newFakeCache(), newFakeGuerrilla(t)
	svc := newService(t, cache, provider)

	msg, err := svc.Message(context.Background(), "ffffffffffffffffffffffffffffffff", "101")
	assert.Nil(t, msg)
	assert.ErrorIs(t, err, ErrInboxNotFound)
	assert.Empty(t, provider.ops)
}

func TestDeleteRemovesSession(t *testing.T) {
	cache, provider := newFakeCache(), newFakeGuerrilla(t)
	svc := newService(t, cache, provider)

	created, err := svc.Create(context.Background(), "")
	require.NoError(t, err)

	require.NoError(t, svc.Delete(context.Background(), created.ID))
	assert.NotContains(t, cache.entries, created.ID)

	err = svc.Delete(context.Background(), created.ID)
	assert.ErrorIs(t, err, ErrInboxNotFound)
}

func TestDeleteCacheFailureIsNotSentinel(t *testing.T) {
	cache, provider := newFakeCache(), newFakeGuerrilla(t)
	svc := newService(t, cache, provider)

	created, err := svc.Create(context.Background(), "")
	require.NoError(t, err)
	cache.delErr = errors.New("redis down")

	err = svc.Delete(context.Background(), created.ID)
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrInboxNotFound)
	assert.NotErrorIs(t, err, ErrProvider)
}

func TestPlainTextCollapse(t *testing.T) {
	assert.Equal(t, "", plainText(""))
	assert.Equal(t, "a b c", plainText("  a\t\n b\u00a0 c  "))
	assert.Equal(t, "Tom & Jerry", plainText("Tom &amp; Jerry"))
	assert.Equal(t, "made in <b>bold</b> way", plainText(`made in &lt;b&gt;bold&lt;/b&gt; way`))
}
