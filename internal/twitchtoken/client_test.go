package twitchtoken

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testAccessToken  = "ACCESS_SECRET_9f2b4c"
	testRefreshToken = "REFRESH_SECRET_4c7d8e"
)

// capture records what the test server received so assertions run on the test
// goroutine (require must not be called from a handler).
type capture struct {
	path        string
	method      string
	contentType string
	body        map[string]any
}

// newTestServer starts an httptest server with h and records the request it
// receives.
func newTestServer(t *testing.T, h http.HandlerFunc) (*httptest.Server, *capture) {
	t.Helper()

	got := &capture{}
	record := func(w http.ResponseWriter, r *http.Request) {
		got.path = r.URL.Path
		got.method = r.Method
		got.contentType = r.Header.Get("Content-Type")
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "read body", http.StatusBadRequest)
			return
		}
		_ = json.Unmarshal(raw, &got.body)
		h(w, r)
	}

	srv := httptest.NewServer(http.HandlerFunc(record))
	t.Cleanup(srv.Close)

	return srv, got
}

// newTestClient starts an httptest server with h and returns a client for it
// plus the request the server received.
func newTestClient(t *testing.T, h http.HandlerFunc) (*Client, *capture) {
	t.Helper()

	srv, got := newTestServer(t, h)
	return NewClient(srv.URL, srv.Client()), got
}

func TestValidate(t *testing.T) {
	c, got := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"success":true,"username":"exampleuser","user_id":"123456789",`+
			`"client_id":"ttgsharedclient","expires_in":3600,"scopes":["chat:read","chat:edit"]}`)
	})

	v, err := c.Validate(context.Background(), testAccessToken)
	require.NoError(t, err)
	require.NotNil(t, v)

	assert.Equal(t, http.MethodPost, got.method)
	assert.Equal(t, "/api/v2/tokens/validate", got.path)
	assert.Equal(t, "application/json", got.contentType)
	assert.Equal(t, map[string]any{"access_token": testAccessToken}, got.body, "only the access token is sent")

	assert.Equal(t, "exampleuser", v.Username)
	assert.Equal(t, "123456789", v.UserID)
	assert.Equal(t, "ttgsharedclient", v.ClientID)
	assert.Equal(t, 3600, v.ExpiresIn)
	assert.Equal(t, []string{"chat:read", "chat:edit"}, v.Scopes)
}

func TestRefresh(t *testing.T) {
	c, got := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"success":true,"access_token":"`+testAccessToken+`",`+
			`"refresh_token":"`+testRefreshToken+`","client_id":"ttgsharedclient","expires_in":3600,`+
			`"scopes":["chat:read","chat:edit"]}`)
	})

	tok, err := c.Refresh(context.Background(), testRefreshToken)
	require.NoError(t, err)
	require.NotNil(t, tok)

	assert.Equal(t, http.MethodPost, got.method)
	assert.Equal(t, "/api/v2/tokens/refresh", got.path)
	assert.Equal(t, map[string]any{"refresh_token": testRefreshToken}, got.body,
		"client_id/client_secret are omitted so the shared app is used")

	assert.Equal(t, testAccessToken, tok.AccessToken)
	assert.Equal(t, testRefreshToken, tok.RefreshToken)
	assert.Equal(t, "ttgsharedclient", tok.ClientID)
	assert.Equal(t, 3600, tok.ExpiresIn)
	assert.Equal(t, []string{"chat:read", "chat:edit"}, tok.Scopes)
}

func TestRefreshNullScopes(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"success":true,"access_token":"a","refresh_token":"r","client_id":"c","expires_in":3600,"scopes":null}`)
	})

	tok, err := c.Refresh(context.Background(), testRefreshToken)
	require.NoError(t, err)
	assert.Nil(t, tok.Scopes)
}

// A 200 with success:false must not be treated as a valid result: storing the
// zero value would wipe a working token pair.
func TestValidateUnsuccessfulResponse(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"success":false,"message":"invalid request"}`)
	})

	v, err := c.Validate(context.Background(), testAccessToken)
	require.Error(t, err)
	assert.Nil(t, v, "no result may be stored")
	assert.Contains(t, err.Error(), "unsuccessful")
	assert.NotContains(t, err.Error(), testAccessToken)
	assert.NotContains(t, err.Error(), "invalid request")
}

func TestRefreshUnsuccessfulResponse(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"success":false,"message":"invalid request"}`)
	})

	tok, err := c.Refresh(context.Background(), testRefreshToken)
	require.Error(t, err)
	assert.Nil(t, tok, "no result may be stored")
	assert.Contains(t, err.Error(), "unsuccessful")
	assert.NotContains(t, err.Error(), testRefreshToken)
	assert.NotContains(t, err.Error(), "invalid request")
}

func TestValidateEmptyUsername(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"success":true,"username":"","user_id":"123456789","client_id":"c","expires_in":3600,"scopes":[]}`)
	})

	v, err := c.Validate(context.Background(), testAccessToken)
	require.Error(t, err)
	assert.Nil(t, v)
	assert.NotContains(t, err.Error(), testAccessToken)
}

func TestRefreshIncompleteTokenPair(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"empty access token", `{"success":true,"access_token":"","refresh_token":"` + testRefreshToken + `","client_id":"c","expires_in":3600,"scopes":[]}`},
		{"empty refresh token", `{"success":true,"access_token":"` + testAccessToken + `","refresh_token":"","client_id":"c","expires_in":3600,"scopes":[]}`},
		{"missing refresh token", `{"success":true,"access_token":"` + testAccessToken + `","client_id":"c","expires_in":3600,"scopes":[]}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
				_, _ = io.WriteString(w, tc.body)
			})

			tok, err := c.Refresh(context.Background(), testRefreshToken)
			require.Error(t, err)
			assert.Nil(t, tok)
			assert.NotContains(t, err.Error(), testRefreshToken)
			assert.NotContains(t, err.Error(), testAccessToken)
		})
	}
}

func TestSuccessFieldDecoded(t *testing.T) {
	vc, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"success":true,"username":"exampleuser","user_id":"1","client_id":"c","expires_in":3600,"scopes":[]}`)
	})
	v, err := vc.Validate(context.Background(), testAccessToken)
	require.NoError(t, err)
	assert.True(t, v.Success)

	tc, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"success":true,"access_token":"a","refresh_token":"r","client_id":"c","expires_in":3600,"scopes":[]}`)
	})
	tok, err := tc.Refresh(context.Background(), testRefreshToken)
	require.NoError(t, err)
	assert.True(t, tok.Success)
}

func TestTrailingSlashBaseURL(t *testing.T) {
	srv, got := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"success":true,"username":"u","user_id":"1","client_id":"c","expires_in":1,"scopes":[]}`)
	})

	c := NewClient(srv.URL+"/", srv.Client())
	_, err := c.Validate(context.Background(), testAccessToken)
	require.NoError(t, err)
	assert.Equal(t, "/api/v2/tokens/validate", got.path)
}

func TestValidateNon200ErrorOmitsToken(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"success":false,"message":"invalid token"}`)
	})

	v, err := c.Validate(context.Background(), testAccessToken)
	require.Error(t, err)
	assert.Nil(t, v)
	assert.NotContains(t, err.Error(), testAccessToken)
	assert.Contains(t, err.Error(), "401")
}

func TestRefreshNon200ErrorOmitsToken(t *testing.T) {
	// The upstream body echoes the token back: it must not reach the error.
	c, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = io.WriteString(w, `{"success":false,"message":"upstream detail for `+testRefreshToken+`"}`)
	})

	tok, err := c.Refresh(context.Background(), testRefreshToken)
	require.Error(t, err)
	assert.Nil(t, tok)
	assert.NotContains(t, err.Error(), testRefreshToken)
	assert.NotContains(t, err.Error(), "upstream detail")
	assert.Contains(t, err.Error(), "502")
}

func TestMalformedJSONResponse(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `<html>gateway</html>`)
	})

	_, err := c.Validate(context.Background(), testAccessToken)
	require.Error(t, err)
	assert.NotContains(t, err.Error(), testAccessToken)
}

func TestCancelledContext(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{}`)
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := c.Validate(ctx, testAccessToken)
	require.Error(t, err)
	assert.NotContains(t, err.Error(), testAccessToken)
}

// StatusCode is the contract callers use instead of parsing error text: an HTTP
// answer is reported with its status, and anything that never got one is false.
func TestStatusCodeFromAPIError(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = io.WriteString(w, `{"success":false,"message":"slow down `+testAccessToken+`"}`)
	})

	_, err := c.Validate(context.Background(), testAccessToken)
	require.Error(t, err)

	status, ok := StatusCode(err)
	assert.True(t, ok, "a non-2xx answer carries a provider status")
	assert.Equal(t, http.StatusTooManyRequests, status)

	// The same error stays token-free.
	assert.NotContains(t, err.Error(), testAccessToken)
}

func TestStatusCodeWithoutProviderAnswer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	url, httpClient := srv.URL, srv.Client()
	srv.Close() // every request from here on fails in the transport

	_, err := NewClient(url, httpClient).Validate(context.Background(), testAccessToken)
	require.Error(t, err)

	status, ok := StatusCode(err)
	assert.False(t, ok, "a transport failure carries no provider status")
	assert.Equal(t, 0, status)

	// Errors from outside the package carry nothing either.
	_, ok = StatusCode(errors.New("unrelated"))
	assert.False(t, ok)
}

func TestStatusCodeOnUnsuccessfulBody(t *testing.T) {
	// HTTP 200 with success:false is rejected by the client, but it is not an
	// HTTP error: callers must not read a token verdict out of it.
	c, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"success":false,"message":"invalid request"}`)
	})

	_, err := c.Refresh(context.Background(), testRefreshToken)
	require.Error(t, err)

	_, ok := StatusCode(err)
	assert.False(t, ok, "an HTTP 200 body saying no is not an HTTP status")
}

func TestNewClientDefaults(t *testing.T) {
	c := NewClient("", nil)

	assert.Equal(t, DefaultBaseURL, c.baseURL)
	require.NotNil(t, c.httpClient)
	assert.Equal(t, 10*time.Second, c.httpClient.Timeout)

	// A caller-provided client is used as-is.
	provided := &http.Client{Timeout: time.Minute}
	assert.Same(t, provided, NewClient("https://example.test/", provided).httpClient)
	assert.Equal(t, "https://example.test", NewClient("https://example.test/", nil).baseURL)
}
