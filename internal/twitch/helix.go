package twitch

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/nicklaw5/helix/v2"
)

// P0-3: Login validation regex
var validLoginRegex = regexp.MustCompile(`^[a-z0-9_]{3,25}$`)

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
	if h == nil || h.client == nil {
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

// P0-3: Validate Twitch account
// Returns nil error if account is valid, or error with description
func (h *HelixClient) ValidateAccount(ctx context.Context, login, oauthToken string) error {
	// 1. Validate login format
	login = strings.TrimPrefix(strings.ToLower(login), "#")
	if !validLoginRegex.MatchString(login) {
		return fmt.Errorf("invalid Twitch login format: must be 3-25 characters (a-z, 0-9, _)")
	}

	// 2. Validate OAuth token by making a test API call
	if oauthToken != "" {
		// Create a temporary client with the user's token
		testClient, err := helix.NewClient(&helix.Options{
			ClientID:     h.clientID,
			ClientSecret: h.clientSecret,
			UserAccessToken: oauthToken,
		})
		if err != nil {
			return fmt.Errorf("failed to create test client: %w", err)
		}

		// Try to get users with the token to validate it
		resp, err := testClient.GetUsers(&helix.UsersParams{})
		if err != nil {
			return fmt.Errorf("OAuth token validation failed: %w", err)
		}
		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			return fmt.Errorf("invalid OAuth token: authentication failed (status %d)", resp.StatusCode)
		}
	}

	// 3. Check if account exists
	if err := h.ensureToken(ctx); err != nil {
		return fmt.Errorf("failed to ensure app token: %w", err)
	}

	usersResp, err := h.client.GetUsers(&helix.UsersParams{
		Logins: []string{login},
	})
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}
	if len(usersResp.Data.Users) == 0 {
		return fmt.Errorf("Twitch account '%s' does not exist", login)
	}

	log.Printf("[Helix] Account '%s' validated successfully (ID: %s)", login, usersResp.Data.Users[0].ID)
	return nil
}

// P0-3: Validate login format only
func ValidateLogin(login string) error {
	login = strings.TrimPrefix(strings.ToLower(login), "#")
	if !validLoginRegex.MatchString(login) {
		return fmt.Errorf("invalid Twitch login format: must be 3-25 characters (a-z, 0-9, _)")
	}
	return nil
}

func (h *HelixClient) GetChannelInfo(ctx context.Context, channelLogin string) (*StreamInfo, error) {
	if h == nil || h.client == nil {
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
