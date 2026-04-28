package twitch

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/nicklaw5/helix/v2"
)

type HelixClient struct {
	client      *helix.Client
	clientID    string
	clientSecret string
	mu          sync.RWMutex
	accessToken string
	tokenExpiry time.Time
}

type StreamInfo struct {
	IsLive       bool
	GameName     string
	GameID       string
	CategoryName string
	Title        string
	ViewerCount  int
}

func NewHelixClient(clientID, clientSecret string) *HelixClient {
	client, err := helix.NewClient(&helix.Options{
		ClientID:     clientID,
		ClientSecret: clientSecret,
	})
	if err != nil {
		log.Printf("[Helix] Failed to create client: %v", err)
		return nil
	}

	return &HelixClient{
		client:       client,
		clientID:     clientID,
		clientSecret: clientSecret,
	}
}

func (h *HelixClient) ensureToken(ctx context.Context) error {
	h.mu.RLock()
	if h.accessToken != "" && time.Now().Before(h.tokenExpiry) {
		h.mu.RUnlock()
		return nil
	}
	h.mu.RUnlock()

	h.mu.Lock()
	defer h.mu.Unlock()

	// Double-check after acquiring write lock
	if h.accessToken != "" && time.Now().Before(h.tokenExpiry) {
		return nil
	}

	resp, err := h.client.RequestAppAccessToken([]string{})
	if err != nil {
		return fmt.Errorf("request app access token: %w", err)
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf("token request failed with status %d", resp.StatusCode)
	}

	h.accessToken = resp.Data.AccessToken
	h.tokenExpiry = time.Now().Add(time.Duration(resp.Data.ExpiresIn) * time.Second)
	h.client.SetAppAccessToken(h.accessToken)

	log.Printf("[Helix] Access token obtained, expires in %d seconds", resp.Data.ExpiresIn)
	return nil
}

func (h *HelixClient) GetStreamInfo(ctx context.Context, channelLogin string) (*StreamInfo, error) {
	if h == nil {
		return nil, fmt.Errorf("helix client is nil")
	}

	if err := h.ensureToken(ctx); err != nil {
		return nil, fmt.Errorf("ensure token: %w", err)
	}

	channelLogin = strings.TrimPrefix(strings.ToLower(channelLogin), "#")

	// Get user ID first
	usersResp, err := h.client.GetUsers(&helix.UsersParams{
		Logins: []string{channelLogin},
	})
	if err != nil {
		return nil, fmt.Errorf("get users: %w", err)
	}

	if len(usersResp.Data.Users) == 0 {
		return nil, fmt.Errorf("user not found: %s", channelLogin)
	}

	userID := usersResp.Data.Users[0].ID

	// Get stream info
	streamsResp, err := h.client.GetStreams(&helix.StreamsParams{
		UserIDs: []string{userID},
	})
	if err != nil {
		return nil, fmt.Errorf("get streams: %w", err)
	}

	info := &StreamInfo{
		IsLive: false,
	}

	if len(streamsResp.Data.Streams) > 0 {
		stream := streamsResp.Data.Streams[0]
		info.IsLive = true
		info.GameName = stream.GameName
		info.GameID = stream.GameID
		info.CategoryName = stream.GameName // Category and Game are the same in Twitch API
		info.Title = stream.Title
		info.ViewerCount = stream.ViewerCount
	}

	return info, nil
}

func (h *HelixClient) GetChannelInfo(ctx context.Context, channelLogin string) (*StreamInfo, error) {
	if h == nil {
		return nil, fmt.Errorf("helix client is nil")
	}

	if err := h.ensureToken(ctx); err != nil {
		return nil, fmt.Errorf("ensure token: %w", err)
	}

	channelLogin = strings.TrimPrefix(strings.ToLower(channelLogin), "#")

	// Get user ID
	usersResp, err := h.client.GetUsers(&helix.UsersParams{
		Logins: []string{channelLogin},
	})
	if err != nil {
		return nil, fmt.Errorf("get users: %w", err)
	}

	if len(usersResp.Data.Users) == 0 {
		return nil, fmt.Errorf("user not found: %s", channelLogin)
	}

	userID := usersResp.Data.Users[0].ID

	// Get channel information
	channelResp, err := h.client.GetChannelInformation(&helix.GetChannelInformationParams{
		BroadcasterIDs: []string{userID},
	})
	if err != nil {
		return nil, fmt.Errorf("get channel information: %w", err)
	}

	info := &StreamInfo{
		IsLive: false,
	}

	if len(channelResp.Data.Channels) > 0 {
		channel := channelResp.Data.Channels[0]
		info.GameName = channel.GameName
		info.GameID = channel.GameID
		info.CategoryName = channel.GameName
		info.Title = channel.Title
	}

	return info, nil
}
