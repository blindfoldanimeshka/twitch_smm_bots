package twitch

import (
	"context"
	"sync"
	"testing"
	"time"
)

// TestValidLoginRegex tests the login validation regex
func TestValidLoginRegex(t *testing.T) {
	tests := []struct {
		login    string
		expected bool
	}{
		{"testuser", true},
		{"ab", false},                      // too short
		{"abcdefghijklmnopqrstuvwxyz", false}, // too long (>25)
		{"user123", true},
		{"user_name", true},
		{"USERNAME", false},                 // uppercase
		{"user-name", false},                // hyphen
		{"user.name", false},                // dot
		{"", false},                          // empty
	{"user1234567890123456789012", false}, // 26 chars
	{"user12345678901234567890123", false}, // 27 chars
	}

	for _, tt := range tests {
		got := validLoginRegex.MatchString(tt.login)
		if got != tt.expected {
			t.Errorf("validLoginRegex.MatchString(%s) = %v, want %v", tt.login, got, tt.expected)
		}
	}
}

// TestValidateLogin tests the ValidateLogin helper
func TestValidateLogin(t *testing.T) {
	tests := []struct {
		login    string
		hasError bool
	}{
		{"validuser", false},
		{"", true},
		{"ab", true},
		{"#channel", false}, // should trim #
		{"user_name", false},
	}

	for _, tt := range tests {
		err := ValidateLogin(tt.login)
		if (err != nil) != tt.hasError {
			t.Errorf("ValidateLogin(%s) error = %v, want error = %v", tt.login, err, tt.hasError)
		}
	}
}

// TestNewHelixClient tests client creation
func TestNewHelixClient(t *testing.T) {
	client := NewHelixClient("test_client_id", "test_client_secret")
	if client == nil {
		t.Fatal("expected client to be created")
	}
	if client.clientID != "test_client_id" {
		t.Errorf("expected clientID 'test_client_id', got '%s'", client.clientID)
	}
	if client.clientSecret != "test_client_secret" {
		t.Errorf("expected clientSecret 'test_client_secret', got '%s'", client.clientSecret)
	}
}

// TestStreamInfoStructure tests the StreamInfo struct
func TestStreamInfoStructure(t *testing.T) {
	info := &StreamInfo{
		IsLive:       true,
		GameName:     "Just Chatting",
		GameID:       "509658",
		CategoryName: "Just Chatting",
		Title:        "Testing stream",
		ViewerCount:  42,
	}

	if !info.IsLive {
		t.Error("expected IsLive to be true")
	}
	if info.GameName != "Just Chatting" {
		t.Errorf("expected GameName 'Just Chatting', got '%s'", info.GameName)
	}
	if info.ViewerCount != 42 {
		t.Errorf("expected ViewerCount 42, got %d", info.ViewerCount)
	}
}

// TestHelixClientEnsureToken tests token management
func TestHelixClientEnsureToken(t *testing.T) {
	client := NewHelixClient("test_id", "test_secret")
	if client == nil {
		t.Fatal("expected client to be created")
	}

	ctx := context.Background()

	// First call should try to get token
	err := client.ensureToken(ctx)
	// May fail without valid credentials, but shouldn't panic
	if err != nil {
		t.Logf("ensureToken() returned error (expected without valid credentials): %v", err)
	}
}

// TestHelixClientGetStreamInfoNoClient tests nil client
func TestHelixClientGetStreamInfoNoClient(t *testing.T) {
	client := &HelixClient{}
	_, err := client.GetStreamInfo(context.Background(), "testchannel")
	if err == nil {
		t.Error("expected error for nil internal client")
	}
}

// TestHelixClientGetChannelInfoNoClient tests nil client
func TestHelixClientGetChannelInfoNoClient(t *testing.T) {
	client := &HelixClient{}
	_, err := client.GetChannelInfo(context.Background(), "testchannel")
	if err == nil {
		t.Error("expected error for nil internal client")
	}
}

// TestHelixClientTokenExpiry tests token expiry logic
func TestHelixClientTokenExpiry(t *testing.T) {
	client := NewHelixClient("test_id", "test_secret")

	// Set token as expired
	client.mu.Lock()
	client.accessToken = "old_token"
	client.tokenExpiry = time.Now().Add(-1 * time.Hour) // expired
	client.mu.Unlock()

	ctx := context.Background()

	// ensureToken should detect expiry
	err := client.ensureToken(ctx)
	// Will fail without valid credentials, but shouldn't panic
	if err != nil {
		t.Logf("ensureToken() returned error (expected): %v", err)
	}
}

// TestConcurrentHelixClientAccess tests concurrent access
func TestConcurrentHelixClientAccess(t *testing.T) {
	client := NewHelixClient("test_id", "test_secret")

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			ctx := context.Background()
			_ = client.ensureToken(ctx)
			// GetStreamInfo returns (*StreamInfo, error)
			info1, err1 := client.GetStreamInfo(ctx, "channel1")
			_ = info1
			_ = err1
			// GetChannelInfo returns (*StreamInfo, error)
			info2, err2 := client.GetChannelInfo(ctx, "channel2")
			_ = info2
			_ = err2
		}(i)
	}
	wg.Wait()

	// Should not panic or race
}
