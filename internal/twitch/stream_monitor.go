package twitch

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	// snapshotTimeout bounds every public GQL call made by Snapshot.
	snapshotTimeout = 5 * time.Second

	// activeViewerWindow is how long a chatter counts as an active viewer.
	activeViewerWindow = 300 * time.Second

	// Response caps for the stream status payload.
	chatterLimit = 20
	messageLimit = 50

	// maxMonitoredChannels is the upper bound of channels tracked at once.
	maxMonitoredChannels = 5
)

// StreamStatus is the public state of one monitored channel, as served by
// GET /api/twitch/stream. All numbers come from real observations; nothing
// is extrapolated when data is unavailable.
type StreamStatus struct {
	Configured          bool          `json:"configured"`
	Channel             string        `json:"channel"`
	Live                bool          `json:"live"`
	Title               string        `json:"title"`
	Game                string        `json:"game"`
	StartedAt           *time.Time    `json:"started_at,omitempty"`
	UptimeSeconds       int64         `json:"uptime_seconds"`
	ViewersTotal        int           `json:"viewers_total"`
	ActiveViewers       int           `json:"active_viewers"`
	PassiveViewers      int           `json:"passive_viewers"`
	ActiveWindowSeconds int           `json:"active_window_seconds"`
	ChatConnected       bool          `json:"chat_connected"`
	Chatters            []Chatter     `json:"chatters"`
	Messages            []ChatMessage `json:"messages"`
	UpdatedAt           time.Time     `json:"updated_at"`
	Error               string        `json:"error,omitempty"`
}

// streamProvider is the read surface StreamMonitor needs from PublicStreamClient.
type streamProvider interface {
	GetStream(ctx context.Context, channel string) (*PublicStreamInfo, error)
}

// chatProvider is the read surface StreamMonitor needs from ChatMonitor.
type chatProvider interface {
	ActiveChatters(window time.Duration) []Chatter
	RecentMessages(limit int) []ChatMessage
	Connected() bool
	Close()
}

// StreamMonitor watches the configured channels with one shared public stream
// client and one anonymous chat monitor per channel.
type StreamMonitor struct {
	mu       sync.Mutex
	public   streamProvider
	newChat  func(channel string) chatProvider
	channels []string
	chats    map[string]chatProvider
}

// newStreamMonitor wires the monitor to its providers; tests inject fakes here.
func newStreamMonitor(public streamProvider, newChat func(channel string) chatProvider) *StreamMonitor {
	return &StreamMonitor{
		public:   public,
		newChat:  newChat,
		channels: []string{},
		chats:    make(map[string]chatProvider),
	}
}

// NewStreamMonitor creates a monitor backed by the real public GraphQL client
// and anonymous IRC chat monitors. One instance is enough for the whole app.
func NewStreamMonitor() *StreamMonitor {
	return newStreamMonitor(NewPublicStreamClient(), func(ch string) chatProvider {
		m := NewChatMonitor()
		m.SetChannel(ch)
		return m
	})
}

// SetChannels replaces the monitored set: input is normalized, deduped and
// capped at maxMonitoredChannels. Removed channels stop their chat monitors,
// new ones start them; unchanged channels keep their state.
func (m *StreamMonitor) SetChannels(channels []string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	seen := make(map[string]struct{}, len(channels))
	next := make([]string, 0, len(channels))
	for _, raw := range channels {
		login := NormalizeChannel(raw)
		if login == "" {
			continue
		}
		if _, dup := seen[login]; dup {
			continue
		}
		if len(next) >= maxMonitoredChannels {
			break
		}
		// Mark the channel as kept only once it survives the cap: the
		// cleanup below closes every chat monitor absent from seen, so a
		// channel capped out of the set must not be marked here.
		seen[login] = struct{}{}
		next = append(next, login)
	}

	for ch, chat := range m.chats {
		if _, keep := seen[ch]; !keep {
			chat.Close()
			delete(m.chats, ch)
		}
	}
	for _, ch := range next {
		if _, ok := m.chats[ch]; !ok {
			m.chats[ch] = m.newChat(ch)
		}
	}
	m.channels = next
}

// Channels returns the current normalized monitored logins.
func (m *StreamMonitor) Channels() []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	out := make([]string, len(m.channels))
	copy(out, m.channels)
	return out
}

// Snapshot observes the requested channel (or the first configured one) and
// returns its live state plus chat activity. It never fails: problems are
// reported as a short safe Error string on a 200 payload.
func (m *StreamMonitor) Snapshot(ctx context.Context, channel string, excludeLogins map[string]struct{}) StreamStatus {
	m.mu.Lock()
	channels := m.channels
	public := m.public
	var chat chatProvider
	target := NormalizeChannel(channel)
	for _, ch := range channels {
		if ch == target && target != "" {
			chat = m.chats[ch]
		}
	}
	if chat == nil && len(channels) > 0 {
		target = channels[0]
		chat = m.chats[target]
	}
	m.mu.Unlock()

	status := StreamStatus{
		UpdatedAt: time.Now().UTC(),
		Chatters:  []Chatter{},
		Messages:  []ChatMessage{},
	}
	if len(channels) == 0 {
		return status
	}

	status.Configured = true
	status.Channel = target
	status.ActiveWindowSeconds = int(activeViewerWindow / time.Second)

	exclude := make(map[string]struct{}, len(excludeLogins))
	for login := range excludeLogins {
		login = strings.ToLower(strings.TrimSpace(login))
		if login != "" {
			exclude[login] = struct{}{}
		}
	}

	if chat != nil {
		status.ChatConnected = chat.Connected()
		status.Chatters = topChatters(chat.ActiveChatters(activeViewerWindow), exclude)
		status.ActiveViewers = len(status.Chatters) // counted before the 20-item cap below
		if len(status.Chatters) > chatterLimit {
			status.Chatters = status.Chatters[:chatterLimit]
		}
		status.Messages = markedMessages(chat.RecentMessages(messageLimit), exclude)
	}

	if public == nil {
		status.Error = "twitch unavailable"
		return status
	}

	infoCtx, cancel := context.WithTimeout(ctx, snapshotTimeout)
	defer cancel()
	info, err := public.GetStream(infoCtx, target)
	if err != nil {
		if strings.Contains(err.Error(), "channel not found") {
			status.Error = "channel not found"
		} else {
			status.Error = "twitch unavailable"
		}
		return status
	}
	if !info.Live {
		return status
	}

	status.Live = true
	status.Title = info.Title
	status.Game = info.GameName
	if !info.StartedAt.IsZero() {
		started := info.StartedAt.UTC()
		status.StartedAt = &started
		// Clamp to zero: a clock skew with Twitch must not yield negative uptime.
		status.UptimeSeconds = max(0, int64(time.Since(info.StartedAt).Seconds()))
	}
	status.ViewersTotal = info.ViewerCount
	status.PassiveViewers = max(0, status.ViewersTotal-status.ActiveViewers)
	return status
}

// topChatters drops excluded logins and sorts the rest newest first. The count
// before capping is the real active-viewer number.
func topChatters(chatters []Chatter, exclude map[string]struct{}) []Chatter {
	filtered := make([]Chatter, 0, len(chatters))
	for _, c := range chatters {
		if _, skip := exclude[strings.ToLower(c.Login)]; skip {
			continue
		}
		filtered = append(filtered, c)
	}
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].LastMessageAt.After(filtered[j].LastMessageAt)
	})
	return filtered
}

// markedMessages copies the messages and flags bot logins instead of dropping
// them, so the UI can label them.
func markedMessages(messages []ChatMessage, exclude map[string]struct{}) []ChatMessage {
	out := make([]ChatMessage, 0, len(messages))
	for _, msg := range messages {
		if _, isBot := exclude[strings.ToLower(msg.Login)]; isBot {
			msg.IsBot = true
		}
		out = append(out, msg)
	}
	return out
}

// Close stops every chat monitor; the stream client needs no cleanup.
func (m *StreamMonitor) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, chat := range m.chats {
		chat.Close()
	}
	m.chats = make(map[string]chatProvider)
	m.channels = []string{}
}
