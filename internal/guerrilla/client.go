// Package guerrilla is a minimal client for the public Guerrilla Mail AJAX
// API (https://api.guerrillamail.com/ajax.php). It covers inbox creation,
// listing, fetching and deleting messages.
//
// sid_token is a bearer secret for the temporary mailbox: the client never
// logs it and strips it from every error message, including the request URLs
// quoted by transport errors.
package guerrilla

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// DefaultBaseURL is the public host of the Guerrilla Mail AJAX API.
const DefaultBaseURL = "https://api.guerrillamail.com"

// endpointPath is the single AJAX endpoint every operation is routed through,
// selected by the "f" parameter.
const endpointPath = "/ajax.php"

// defaultTimeout bounds a single API request when the caller does not supply
// a client.
const defaultTimeout = 10 * time.Second

// maxResponseBytes bounds the response body we are willing to read.
const maxResponseBytes = 1 << 20

// maxBodyBytes caps a single message body (mail_body) that we keep in memory.
const maxBodyBytes = 256 << 10

// maxDrainBytes bounds how much of an error response body is drained so the
// connection can be reused. The content is never surfaced.
const maxDrainBytes = 4 << 10

// Inbox is a temporary mailbox: the current address and the session token
// required to operate on it.
type Inbox struct {
	Email string
	SID   string
}

// Message is one email in an inbox. ReceivedAt is zero when the provider did
// not supply a usable mail_timestamp.
type Message struct {
	ID         string
	From       string
	Subject    string
	Excerpt    string
	Body       string
	ReceivedAt time.Time
}

// Client is a Guerrilla Mail AJAX API client. It is safe for concurrent use.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient returns a client for baseURL (DefaultBaseURL when empty). When
// httpClient is nil a client with a 10 s timeout is used. A trailing slash in
// baseURL is ignored.
func NewClient(baseURL string, httpClient *http.Client) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultTimeout}
	}

	return &Client{
		baseURL:    strings.TrimSuffix(baseURL, "/"),
		httpClient: httpClient,
	}
}

// Create claims a temporary inbox via get_email_address. When alias is not
// empty a second call (set_email_user) renames the address; the address and
// session token from that call are returned. An empty result is an error so
// the caller never stores a zero Inbox.
func (c *Client) Create(ctx context.Context, alias string) (*Inbox, error) {
	var first inboxDTO
	if err := c.get(ctx, "get_email_address", nil, &first); err != nil {
		return nil, err
	}
	inbox := first.toInbox()
	if alias == "" {
		if inbox.Email == "" || inbox.SID == "" {
			return nil, errors.New("guerrilla: get_email_address returned no address or session")
		}
		return inbox, nil
	}

	form := url.Values{"email_user": {alias}, "sid_token": {inbox.SID}}
	var second inboxDTO
	if err := c.postForm(ctx, "set_email_user", form, &second); err != nil {
		return nil, err
	}
	inbox = second.toInbox()
	if inbox.Email == "" || inbox.SID == "" {
		return nil, errors.New("guerrilla: set_email_user returned no address or session")
	}
	return inbox, nil
}

// List returns the messages currently in the inbox addressed by sid.
func (c *Client) List(ctx context.Context, sid string) ([]Message, error) {
	q := url.Values{"sid_token": {sid}, "offset": {"0"}}

	var out listDTO
	if err := c.get(ctx, "get_email_list", q, &out); err != nil {
		return nil, err
	}

	msgs := make([]Message, 0, len(out.List))
	for _, m := range out.List {
		msgs = append(msgs, m.toMessage())
	}
	return msgs, nil
}

// Fetch retrieves one message, body included. mailID is the provider's
// mail_id as reported by List.
func (c *Client) Fetch(ctx context.Context, sid, mailID string) (*Message, error) {
	q := url.Values{"sid_token": {sid}, "email_id": {mailID}}

	var out messageDTO
	if err := c.get(ctx, "fetch_email", q, &out); err != nil {
		return nil, err
	}
	msg := out.toMessage()
	return &msg, nil
}

// Delete removes one message from the inbox. An answer explicitly saying
// otherwise is reported as an error.
func (c *Client) Delete(ctx context.Context, sid, mailID string) error {
	q := url.Values{"sid_token": {sid}, "email_ids[]": {mailID}}

	var out deleteDTO
	if err := c.get(ctx, "del_email", q, &out); err != nil {
		return err
	}
	switch strings.ToLower(string(out.Success)) {
	case "false", "0":
		return errors.New("guerrilla: del_email was rejected")
	}
	return nil
}

// apiError is a non-2xx HTTP response. It reports only the operation and the
// status code: the request URL carries the sid_token and the response body
// can echo it back, so neither is included.
type apiError struct {
	op     string
	status int
}

func (e *apiError) Error() string {
	return fmt.Sprintf("guerrilla: %s failed with status %d", e.op, e.status)
}

// inboxDTO mirrors the get_email_address / set_email_user response.
type inboxDTO struct {
	EmailAddr string     `json:"email_addr"`
	SIDToken  flexString `json:"sid_token"`
}

func (d inboxDTO) toInbox() *Inbox {
	return &Inbox{Email: d.EmailAddr, SID: string(d.SIDToken)}
}

// listDTO mirrors the get_email_list response. The rotated sid_token in it is
// ignored: the caller owns the session it passed in.
type listDTO struct {
	List     []messageDTO `json:"list"`
	SIDToken flexString   `json:"sid_token"`
}

// messageDTO mirrors one entry of get_email_list and the whole fetch_email
// response. The API mixes numeric and string scalars, so every field decodes
// through flexString.
type messageDTO struct {
	MailID        flexString `json:"mail_id"`
	MailFrom      flexString `json:"mail_from"`
	MailSubject   flexString `json:"mail_subject"`
	MailExcerpt   flexString `json:"mail_excerpt"`
	MailBody      flexString `json:"mail_body"`
	MailTimestamp flexString `json:"mail_timestamp"`
}

func (d messageDTO) toMessage() Message {
	return Message{
		ID:         string(d.MailID),
		From:       string(d.MailFrom),
		Subject:    string(d.MailSubject),
		Excerpt:    string(d.MailExcerpt),
		Body:       capBody(string(d.MailBody)),
		ReceivedAt: parseUnixSeconds(d.MailTimestamp),
	}
}

// deleteDTO mirrors the del_email response: a success flag.
type deleteDTO struct {
	Success flexString `json:"success"`
}

// flexString is a JSON scalar (string, number or bool) decoded as a string,
// because the API reports ids and timestamps inconsistently as quoted or bare
// values. A missing or null field becomes the empty string.
type flexString string

func (s *flexString) UnmarshalJSON(data []byte) error {
	switch string(data) {
	case "null":
		*s = ""
		return nil
	case "true", "false":
		*s = flexString(data)
		return nil
	}

	if len(data) > 0 && data[0] == '"' {
		var str string
		if err := json.Unmarshal(data, &str); err != nil {
			return err
		}
		*s = flexString(str)
		return nil
	}

	var num json.Number
	if err := json.Unmarshal(data, &num); err != nil {
		return err
	}
	*s = flexString(num)
	return nil
}

// parseUnixSeconds turns mail_timestamp (unix seconds, quoted or bare) into a
// time. 0 or an unparseable value becomes the zero time.
func parseUnixSeconds(raw flexString) time.Time {
	secs, err := strconv.ParseInt(string(raw), 10, 64)
	if err != nil || secs <= 0 {
		return time.Time{}
	}
	return time.Unix(secs, 0)
}

// capBody bounds the message body we keep. The cut is on a byte boundary, as
// the cap is about memory, not presentation.
func capBody(body string) string {
	if len(body) <= maxBodyBytes {
		return body
	}
	return body[:maxBodyBytes]
}

// redact removes every occurrence of the session token (raw or percent
// encoded) from text that is about to become part of an error message.
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

// get issues a GET to the AJAX endpoint with operation f and query q, and
// decodes the JSON answer into out.
func (c *Client) get(ctx context.Context, op string, q url.Values, out any) error {
	if q == nil {
		q = url.Values{}
	}
	q.Set("f", op)

	req, err := c.newRequest(ctx, http.MethodGet, c.baseURL+endpointPath+"?"+q.Encode(), nil)
	if err != nil {
		return fmt.Errorf("guerrilla: build %s request: %s", op, redact(err.Error(), q.Get("sid_token")))
	}
	return c.send(req, op, q.Get("sid_token"), out)
}

// postForm issues a POST selecting operation f in the query and carrying form
// as an urlencoded body, and decodes the JSON answer into out.
func (c *Client) postForm(ctx context.Context, op string, form url.Values, out any) error {
	endpoint := c.baseURL + endpointPath + "?" + url.Values{"f": {op}}.Encode()

	req, err := c.newRequest(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("guerrilla: build %s request: %s", op, redact(err.Error(), form.Get("sid_token")))
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return c.send(req, op, form.Get("sid_token"), out)
}

func (c *Client) newRequest(ctx context.Context, method, endpoint string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	return req, nil
}

// send executes req and decodes a bounded 2xx JSON answer into out. Every
// failure message is operation-scoped and scrubbed of the sid_token.
func (c *Client) send(req *http.Request, op, sid string, out any) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		// url.Error quotes the full request URL, sid_token included: keep
		// only the transport cause.
		cause := err
		var urlErr *url.Error
		if errors.As(err, &urlErr) && urlErr.Err != nil {
			cause = urlErr.Err
		}
		return fmt.Errorf("guerrilla: %s request: %s", op, redact(cause.Error(), sid))
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxDrainBytes))
		return &apiError{op: op, status: resp.StatusCode}
	}

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return fmt.Errorf("guerrilla: read %s response: %s", op, redact(err.Error(), sid))
	}
	if err := json.Unmarshal(raw, out); err != nil {
		// Decode errors can quote response content (which may echo the sid);
		// the text is scrubbed and the error itself is not wrapped.
		return fmt.Errorf("guerrilla: decode %s response: %s", op, redact(err.Error(), sid))
	}
	return nil
}
