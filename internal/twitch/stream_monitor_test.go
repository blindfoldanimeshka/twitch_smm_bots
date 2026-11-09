package twitch

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeStreamProvider is a stub PublicStreamClient for StreamMonitor tests.
type fakeStreamProvider struct {
	info *PublicStreamInfo
	err  error
}

func (f *fakeStreamProvider) GetStream(ctx context.Context, channel string) (*PublicStreamInfo, error) {
	return f.info, f.err
}

// fakeChatProvider is a stub ChatMonitor for StreamMonitor tests.
type fakeChatProvider struct {
	chatters   []Chatter
	messages   []ChatMessage
	connected  bool
	closeCount int
}

func (f *fakeChatProvider) ActiveChatters(window time.Duration) []Chatter { return f.chatters }

func (f *fakeChatProvider) RecentMessages(limit int) []ChatMessage { return f.messages }

func (f *fakeChatProvider) Connected() bool { return f.connected }

func (f *fakeChatProvider) Close() { f.closeCount++ }

// newFakeMonitor builds a monitor around fakes and records every chat provider
// it creates, keyed by normalized channel.
func newFakeMonitor(public *fakeStreamProvider, chat *fakeChatProvider) (*StreamMonitor, map[string]*fakeChatProvider) {
	chats := make(map[string]*fakeChatProvider)
	mon := newStreamMonitor(public, func(ch string) chatProvider {
		fc := chat
		if fc == nil {
			fc = &fakeChatProvider{}
		}
		chats[ch] = fc
		return fc
	})
	return mon, chats
}

func TestStreamMonitorNoChannels(t *testing.T) {
	mon, _ := newFakeMonitor(&fakeStreamProvider{info: &PublicStreamInfo{Live: true, ViewerCount: 10}}, nil)

	mon.SetChannels(nil)

	status := mon.Snapshot(context.Background(), "", nil)
	assert.False(t, status.Configured)
	assert.Empty(t, status.Error)
	assert.False(t, status.Live)
	assert.Equal(t, 0, status.ViewersTotal)
}

func TestStreamMonitorLive(t *testing.T) {
	now := time.Now()
	started := now.Add(-2 * time.Minute)
	public := &fakeStreamProvider{info: &PublicStreamInfo{
		Live:        true,
		Title:       "hello",
		GameName:    "Just Chatting",
		ViewerCount: 100,
		StartedAt:   started,
	}}
	chat := &fakeChatProvider{
		connected: true,
		chatters: []Chatter{
			{Login: "alice", LastMessageAt: now},
			{Login: "bob", LastMessageAt: now.Add(-time.Minute)},
			{Login: "bot1", LastMessageAt: now.Add(-30 * time.Second)},
		},
		messages: []ChatMessage{{Login: "alice", Text: "hi"}},
	}
	mon, _ := newFakeMonitor(public, chat)
	mon.SetChannels([]string{"a"})

	status := mon.Snapshot(context.Background(), "", map[string]struct{}{"bot1": {}})

	require.True(t, status.Configured)
	assert.Equal(t, "a", status.Channel)
	assert.True(t, status.Live)
	assert.Equal(t, "hello", status.Title)
	assert.Equal(t, "Just Chatting", status.Game)
	require.NotNil(t, status.StartedAt, "live status must carry started_at")
	assert.Equal(t, started.UTC(), *status.StartedAt)
	assert.GreaterOrEqual(t, status.UptimeSeconds, int64(120))
	assert.Equal(t, 100, status.ViewersTotal)
	assert.Equal(t, 2, status.ActiveViewers)
	assert.Equal(t, 98, status.PassiveViewers)
	assert.Equal(t, 100, status.ActiveViewers+status.PassiveViewers)
	assert.Equal(t, 300, status.ActiveWindowSeconds)
	assert.True(t, status.ChatConnected)

	// Bot chatters are excluded; the rest are sorted newest first.
	require.Len(t, status.Chatters, 2)
	assert.Equal(t, "alice", status.Chatters[0].Login)
	assert.Equal(t, "bob", status.Chatters[1].Login)
}

func TestStreamMonitorOffline(t *testing.T) {
	public := &fakeStreamProvider{info: &PublicStreamInfo{Live: false}}
	mon, _ := newFakeMonitor(public, nil)
	mon.SetChannels([]string{"a"})

	status := mon.Snapshot(context.Background(), "", nil)

	require.True(t, status.Configured)
	assert.False(t, status.Live)
	assert.Nil(t, status.StartedAt, "offline status must not carry started_at")
	assert.Equal(t, int64(0), status.UptimeSeconds)
	assert.Equal(t, 0, status.ViewersTotal)
	assert.Equal(t, 0, status.ActiveViewers)
	assert.Equal(t, 0, status.PassiveViewers)
	assert.Empty(t, status.Error)
}

func TestStreamMonitorProviderError(t *testing.T) {
	public := &fakeStreamProvider{err: errors.New("boom: raw response body")}
	mon, _ := newFakeMonitor(public, nil)
	mon.SetChannels([]string{"a"})

	status := mon.Snapshot(context.Background(), "", nil)

	assert.True(t, status.Configured)
	assert.NotEmpty(t, status.Error)
	assert.False(t, status.Live)
	assert.Equal(t, 0, status.ViewersTotal)
	assert.NotContains(t, status.Error, "raw response body", "error must be a short safe string")
}

// TestStreamMonitorNilInfo is the regression test for a provider returning
// (nil, nil): Snapshot must report a safe error string instead of panicking
// on the nil dereference.
func TestStreamMonitorNilInfo(t *testing.T) {
	public := &fakeStreamProvider{} // GetStream returns (nil, nil)
	mon, _ := newFakeMonitor(public, nil)
	mon.SetChannels([]string{"a"})

	var status StreamStatus
	assert.NotPanics(t, func() {
		status = mon.Snapshot(context.Background(), "", nil)
	})

	assert.True(t, status.Configured)
	assert.NotEmpty(t, status.Error, "nil info must surface a safe error string")
	assert.False(t, status.Live)
}

func TestStreamMonitorProviderErrorChannelNotFound(t *testing.T) {
	public := &fakeStreamProvider{err: errors.New("channel not found: a")}
	mon, _ := newFakeMonitor(public, nil)
	mon.SetChannels([]string{"a"})

	status := mon.Snapshot(context.Background(), "", nil)

	assert.Equal(t, "channel not found", status.Error)
	assert.False(t, status.Live)
}

func TestStreamMonitorSetChannelsNormalizesAndCaps(t *testing.T) {
	public := &fakeStreamProvider{}
	mon, chats := newFakeMonitor(public, nil)

	// "#A" and "https://twitch.tv/b" normalize; duplicates collapse.
	mon.SetChannels([]string{"#A", "https://twitch.tv/b", "a", "A", "  b  "})
	assert.Equal(t, []string{"a", "b"}, mon.Channels())
	assert.Len(t, chats, 2)
	assert.Contains(t, chats, "a")
	assert.Contains(t, chats, "b")

	// Six unique channels cap at five, and removed channels stop their monitors.
	mon.SetChannels([]string{"c1", "c2", "c3", "c4", "c5", "c6"})
	channels := mon.Channels()
	assert.Len(t, channels, 5)
	assert.NotContains(t, channels, "a")
	assert.NotContains(t, channels, "b")
	assert.Contains(t, chats, "c1")
	assert.GreaterOrEqual(t, chats["a"].closeCount, 1, "removed monitor must be closed")
	assert.GreaterOrEqual(t, chats["b"].closeCount, 1, "removed monitor must be closed")
}

// TestStreamMonitorCapClosesCappedOutMonitor is the regression test for the
// leaked chat monitor: a channel that falls out of the top five because newer
// channels arrived must be closed exactly like any other removed channel.
func TestStreamMonitorCapClosesCappedOutMonitor(t *testing.T) {
	public := &fakeStreamProvider{}
	mon, chats := newFakeMonitor(public, nil)
	mon.SetChannels([]string{"a", "b", "c", "d", "e"})

	// Settings change keeps "a" alive in the input but capped out of the set.
	mon.SetChannels([]string{"x1", "x2", "x3", "x4", "x5", "a"})

	channels := mon.Channels()
	assert.Len(t, channels, 5)
	assert.NotContains(t, channels, "a")
	assert.GreaterOrEqual(t, chats["a"].closeCount, 1, "capped-out channel's chat monitor must be closed")

	// The factory map records every monitor ever created; the leak invariant
	// is that exactly the current five are still open and nothing else is.
	var live []string
	for ch, fc := range chats {
		if fc.closeCount == 0 {
			live = append(live, ch)
		}
	}
	assert.ElementsMatch(t, []string{"x1", "x2", "x3", "x4", "x5"}, live)
}

func TestStreamMonitorMessagesMarkBots(t *testing.T) {
	public := &fakeStreamProvider{info: &PublicStreamInfo{Live: true, ViewerCount: 5}}
	chat := &fakeChatProvider{
		messages: []ChatMessage{
			{Login: "alice", Text: "hi"},
			{Login: "bot1", Text: "beep"},
		},
	}
	mon, _ := newFakeMonitor(public, chat)
	mon.SetChannels([]string{"a"})

	status := mon.Snapshot(context.Background(), "", map[string]struct{}{"bot1": {}})

	require.Len(t, status.Messages, 2)
	assert.False(t, status.Messages[0].IsBot)
	assert.Equal(t, "alice", status.Messages[0].Login)
	assert.True(t, status.Messages[1].IsBot)
	assert.Equal(t, "bot1", status.Messages[1].Login)
}

func TestStreamMonitorExplicitChannel(t *testing.T) {
	public := &fakeStreamProvider{info: &PublicStreamInfo{Live: false}}
	mon, _ := newFakeMonitor(public, nil)
	mon.SetChannels([]string{"a", "b"})

	status := mon.Snapshot(context.Background(), "b", nil)

	assert.Equal(t, "b", status.Channel)
}

// TestStreamStatusJSONOmitsZeroStartedAt is the regression test for the
// omitempty-on-struct bug: time.Time is a struct, so a zero value was always
// serialized as "0001-01-01T00:00:00Z". Only a *time.Time nil pointer omits it.
func TestStreamStatusJSONOmitsZeroStartedAt(t *testing.T) {
	payload, err := json.Marshal(StreamStatus{Configured: false})
	require.NoError(t, err)
	assert.NotContains(t, string(payload), "started_at", "unconfigured status must not carry started_at")

	started := time.Now().Add(-2 * time.Minute).UTC()
	payload, err = json.Marshal(StreamStatus{
		Configured:    true,
		Live:          true,
		StartedAt:     &started,
		UptimeSeconds: int64(time.Since(started).Seconds()),
	})
	require.NoError(t, err)
	assert.Contains(t, string(payload), `"started_at":`, "live status must carry started_at")
	assert.Contains(t, string(payload), `"uptime_seconds":120`, "uptime_seconds must be an integer JSON number")
}

// TestStreamMonitorFutureStartedAtClampsUptime guards against negative uptime
// from clock skew between Twitch and this host.
func TestStreamMonitorFutureStartedAtClampsUptime(t *testing.T) {
	public := &fakeStreamProvider{info: &PublicStreamInfo{
		Live:        true,
		ViewerCount: 5,
		StartedAt:   time.Now().Add(time.Minute), // starts in the future
	}}
	mon, _ := newFakeMonitor(public, nil)
	mon.SetChannels([]string{"a"})

	status := mon.Snapshot(context.Background(), "", nil)

	assert.True(t, status.Live)
	assert.Equal(t, int64(0), status.UptimeSeconds)
}

func TestStreamMonitorCloseStopsChats(t *testing.T) {
	public := &fakeStreamProvider{}
	mon, chats := newFakeMonitor(public, nil)
	mon.SetChannels([]string{"a", "b"})

	mon.Close()

	assert.Empty(t, mon.Channels())
	for ch, fc := range chats {
		assert.GreaterOrEqual(t, fc.closeCount, 1, "monitor %s must be closed", ch)
	}
}
