package twitch

import (
	"context"
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
	assert.GreaterOrEqual(t, status.UptimeSeconds, 120.0)
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
