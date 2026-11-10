package guerrilla

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testSID = "SIDSECRET_5a3c91f2"

// captured records one request the test server received. Assertions run on the
// test goroutine: require must not be called from a handler.
type captured struct {
	method string
	path   string
	query  url.Values
	form   url.Values
}

// newTestServer starts an httptest server with h and returns a client for it
// plus the requests it received, in order.
func newTestServer(t *testing.T, h http.HandlerFunc) (*Client, *[]captured) {
	t.Helper()

	var got []captured
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		got = append(got, captured{method: r.Method, path: r.URL.Path, query: r.URL.Query(), form: r.Form})
		h(w, r)
	}))
	t.Cleanup(srv.Close)

	return NewClient(srv.URL, srv.Client()), &got
}

func writeJSON(w http.ResponseWriter, body string) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = io.WriteString(w, body)
}

func TestCreateWithoutAlias(t *testing.T) {
	c, got := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, `{"email_addr":"block@guerrillamailblock.com","email_timestamp":1794334872,`+
			`"alias":"old@guerrillamail.com","sid_token":"`+testSID+`"}`)
	})

	inbox, err := c.Create(context.Background(), "")
	require.NoError(t, err)
	require.NotNil(t, inbox)

	assert.Equal(t, "block@guerrillamailblock.com", inbox.Email)
	assert.Equal(t, testSID, inbox.SID)

	require.Len(t, *got, 1, "no alias means no set_email_user call")
	assert.Equal(t, http.MethodGet, (*got)[0].method)
	assert.Equal(t, "/ajax.php", (*got)[0].path)
	assert.Equal(t, "get_email_address", (*got)[0].query.Get("f"))
}

func TestCreateWithAliasSendsSetEmailUser(t *testing.T) {
	c, got := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("f") {
		case "get_email_address":
			writeJSON(w, `{"email_addr":"block@guerrillamailblock.com","sid_token":"`+testSID+`"}`)
		case "set_email_user":
			writeJSON(w, `{"email_addr":"wanted@guerrillamail.com","sid_token":"`+testSID+`-new"}`)
		default:
			w.WriteHeader(http.StatusNotFound) // an unexpected operation must fail the call
		}
	})

	inbox, err := c.Create(context.Background(), "wanted")
	require.NoError(t, err)

	assert.Equal(t, "wanted@guerrillamail.com", inbox.Email)
	assert.Equal(t, testSID+"-new", inbox.SID, "the sid from set_email_user wins")

	require.Len(t, *got, 2)
	assert.Equal(t, http.MethodGet, (*got)[0].method)
	assert.Equal(t, "get_email_address", (*got)[0].query.Get("f"))

	assert.Equal(t, http.MethodPost, (*got)[1].method)
	assert.Equal(t, "set_email_user", (*got)[1].query.Get("f"))
	assert.Equal(t, "wanted", (*got)[1].form.Get("email_user"))
	assert.Equal(t, testSID, (*got)[1].form.Get("sid_token"), "set_email_user must reuse the sid from the first call")
}

func TestCreateEmptyResultIsError(t *testing.T) {
	c, _ := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, `{}`)
	})

	inbox, err := c.Create(context.Background(), "")
	require.Error(t, err)
	assert.Nil(t, inbox, "the caller must never store a zero inbox")
}

func TestListParsesMessages(t *testing.T) {
	c, got := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, `{"list":[`+
			`{"mail_id":"11","mail_from":"a@example.com","mail_subject":"Hello","mail_excerpt":"First...","mail_timestamp":"1794334872"},`+
			`{"mail_id":12,"mail_from":"b@example.com","mail_subject":"Second","mail_excerpt":"","mail_timestamp":0}`+
			`],"sid_token":"`+testSID+`"}`)
	})

	msgs, err := c.List(context.Background(), testSID)
	require.NoError(t, err)
	require.Len(t, msgs, 2)

	assert.Equal(t, http.MethodGet, (*got)[0].method)
	assert.Equal(t, "get_email_list", (*got)[0].query.Get("f"))
	assert.Equal(t, "0", (*got)[0].query.Get("offset"))
	assert.Equal(t, testSID, (*got)[0].query.Get("sid_token"))

	assert.Equal(t, "11", msgs[0].ID)
	assert.Equal(t, "a@example.com", msgs[0].From)
	assert.Equal(t, "Hello", msgs[0].Subject)
	assert.Equal(t, "First...", msgs[0].Excerpt)
	assert.Equal(t, time.Unix(1794334872, 0), msgs[0].ReceivedAt, "mail_timestamp is unix seconds")

	assert.Equal(t, "12", msgs[1].ID, "a numeric mail_id decodes into the string ID")
	assert.True(t, msgs[1].ReceivedAt.IsZero(), "0/invalid timestamp becomes zero time")
}

func TestListWithoutMessages(t *testing.T) {
	c, _ := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, `{"list":[],"sid_token":"`+testSID+`"}`)
	})

	msgs, err := c.List(context.Background(), testSID)
	require.NoError(t, err)
	assert.Empty(t, msgs)
}

func TestFetchParsesBody(t *testing.T) {
	long := strings.Repeat("x", maxBodyBytes+1024)
	c, got := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, `{"mail_id":7,"mail_from":"c@example.com","mail_subject":"Body",`+
			`"mail_timestamp":1794334872,"mail_body":"`+long+`"}`)
	})

	m, err := c.Fetch(context.Background(), testSID, "7")
	require.NoError(t, err)
	require.NotNil(t, m)

	assert.Equal(t, "fetch_email", (*got)[0].query.Get("f"))
	assert.Equal(t, "7", (*got)[0].query.Get("email_id"))
	assert.Equal(t, testSID, (*got)[0].query.Get("sid_token"))

	assert.Equal(t, "7", m.ID)
	assert.Equal(t, "c@example.com", m.From)
	assert.Equal(t, "Body", m.Subject)
	assert.Equal(t, time.Unix(1794334872, 0), m.ReceivedAt)
	assert.Len(t, m.Body, maxBodyBytes, "the body is capped at 256 KiB")
}

func TestDeleteSendsMailIDAndTreatsSuccessAsOK(t *testing.T) {
	c, got := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, `{"success":true}`)
	})

	require.NoError(t, c.Delete(context.Background(), testSID, "42"))

	assert.Equal(t, "del_email", (*got)[0].query.Get("f"))
	assert.Equal(t, []string{"42"}, (*got)[0].query["email_ids[]"])
	assert.Equal(t, testSID, (*got)[0].query.Get("sid_token"))
}

func TestDeleteRejectedResponseIsError(t *testing.T) {
	c, _ := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, `{"success":false}`)
	})

	err := c.Delete(context.Background(), testSID, "42")
	require.Error(t, err)
	assert.NotContains(t, err.Error(), testSID)
}

// The error for a non-2xx answer carries only the operation and the status:
// neither the sid we sent nor the upstream body may leak into it.
func TestNon2xxErrorOmitsSIDAndBody(t *testing.T) {
	c, _ := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, `{"error":"boom `+testSID+` echoed back"}`)
	})

	msgs, err := c.List(context.Background(), testSID)
	require.Error(t, err)
	assert.Nil(t, msgs)
	assert.Contains(t, err.Error(), "500")
	assert.Contains(t, err.Error(), "get_email_list")
	assert.NotContains(t, err.Error(), testSID)
	assert.NotContains(t, err.Error(), "boom")
	assert.NotContains(t, err.Error(), "sid_token")
}

// The sid lives in the request URL, and transport errors quote that URL, so
// the url.Error wrapper must be unwrapped (or scrubbed) before formatting.
func TestTransportErrorOmitsSID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	baseURL, httpClient := srv.URL, srv.Client()
	srv.Close() // every request from here on fails in the transport

	_, err := NewClient(baseURL, httpClient).List(context.Background(), testSID)
	require.Error(t, err)
	assert.NotContains(t, err.Error(), testSID)
	assert.NotContains(t, err.Error(), "sid_token")
}

func TestMalformedJSONDoesNotLeakBody(t *testing.T) {
	c, _ := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `<html>`+testSID+`</html>`)
	})

	_, err := c.Fetch(context.Background(), testSID, "1")
	require.Error(t, err)
	assert.NotContains(t, err.Error(), testSID)
}

// maxResponseBytes is the read bound: a larger body must fail, not be streamed
// into memory.
func TestOversizedResponseFails(t *testing.T) {
	c, _ := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"email_addr":"`+strings.Repeat("e", maxResponseBytes)+`@x.com"}`)
	})

	_, err := c.Create(context.Background(), "")
	require.Error(t, err)
}

func TestCancelledContext(t *testing.T) {
	c, _ := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, `{}`)
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := c.List(ctx, testSID)
	require.Error(t, err)
	assert.NotContains(t, err.Error(), testSID)
}

func TestNewClientDefaults(t *testing.T) {
	c := NewClient("", nil)

	assert.Equal(t, DefaultBaseURL, c.baseURL)
	require.NotNil(t, c.httpClient)
	assert.Equal(t, 10*time.Second, c.httpClient.Timeout)

	// A caller-provided client is used as-is; a trailing slash is ignored.
	provided := &http.Client{Timeout: time.Minute}
	assert.Same(t, provided, NewClient("https://example.test/", provided).httpClient)
	assert.Equal(t, "https://example.test", NewClient("https://example.test/", nil).baseURL)
}
