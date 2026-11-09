package twitch

import (
	"sync"
	"testing"
	"time"
)

// TestNewIRCClient tests client creation
func TestNewIRCClient(t *testing.T) {
	client := NewIRCClient("testlogin", "oauth:test123", "testchannel")
	if client == nil {
		t.Fatal("expected client to be created")
	}
	if client.login != "testlogin" {
		t.Errorf("expected login 'testlogin', got '%s'", client.login)
	}
	if client.password != "oauth:test123" {
		t.Errorf("expected password 'oauth:test123', got '%s'", client.password)
	}
	if client.channel != "#testchannel" {
		t.Errorf("expected channel '#testchannel', got '%s'", client.channel)
	}
}

// TestNewIRCClientWithHashPrefix tests that # is added if not present
func TestNewIRCClientWithHashPrefix(t *testing.T) {
	client := NewIRCClient("testlogin", "pass", "#already_hashed")
	if client.channel != "#already_hashed" {
		t.Errorf("expected channel '#already_hashed', got '%s'", client.channel)
	}
}

// TestIsConnected tests initial state
func TestIsConnected(t *testing.T) {
	client := NewIRCClient("testlogin", "pass", "testchannel")
	if client.IsConnected() {
		t.Error("expected IsConnected() to be false initially")
	}
}

// TestRateLimitInitialState tests that rate limit is not active initially
func TestRateLimitInitialState(t *testing.T) {
	client := NewIRCClient("testlogin", "pass", "testchannel")
	if client.IsRateLimited() {
		t.Error("expected IsRateLimited() to be false initially")
	}
}

// TestHandleRateLimit tests rate limit handling
func TestHandleRateLimit(t *testing.T) {
	client := NewIRCClient("testlogin", "pass", "testchannel")

	// Simulate rate limit
	client.HandleRateLimit()

	if !client.IsRateLimited() {
		t.Error("expected IsRateLimited() to be true after HandleRateLimit()")
	}

	// Wait for rate limit to expire
	client.mu.Lock()
	waitTime := time.Until(client.rateLimitUntil)
	client.mu.Unlock()

	if waitTime > 0 {
		time.Sleep(waitTime + 100*time.Millisecond)
	}

	// After waiting, IsRateLimited should return false
	if client.IsRateLimited() {
		t.Error("expected IsRateLimited() to be false after waiting")
	}
}

// TestResetRateLimit tests resetting rate limit state
func TestResetRateLimit(t *testing.T) {
	client := NewIRCClient("testlogin", "pass", "testchannel")

	// Set rate limit state
	client.mu.Lock()
	client.rateLimited = true
	client.backoffAttempts = 3
	client.rateLimitUntil = time.Now().Add(5 * time.Minute)
	client.mu.Unlock()

	// Reset
	client.ResetRateLimit()

	// Verify reset
	client.mu.Lock()
	if client.rateLimited {
		t.Error("expected rateLimited to be false after ResetRateLimit()")
	}
	if client.backoffAttempts != 0 {
		t.Errorf("expected backoffAttempts to be 0, got %d", client.backoffAttempts)
	}
	client.mu.Unlock()
}

// TestWaitForRateLimit tests waiting for rate limit to expire
func TestWaitForRateLimit(t *testing.T) {
	client := NewIRCClient("testlogin", "pass", "testchannel")

	// Set a short rate limit
	client.mu.Lock()
	client.rateLimited = true
	client.rateLimitUntil = time.Now().Add(100 * time.Millisecond)
	client.mu.Unlock()

	// Wait should return quickly
	start := time.Now()
	client.WaitForRateLimit()
	elapsed := time.Since(start)

	if elapsed > 500*time.Millisecond {
		t.Errorf("WaitForRateLimit() took too long: %v", elapsed)
	}

	// After waiting, should not be rate limited
	if client.IsRateLimited() {
		t.Error("expected IsRateLimited() to be false after WaitForRateLimit()")
	}
}

// TestWaitForRateLimitDoesNotHoldLock verifies that the rate-limit sleep does
// not keep c.mu locked: while the client is sleeping, other goroutines (Close,
// IsRateLimited, SendMessage) must be able to take the lock.
func TestWaitForRateLimitDoesNotHoldLock(t *testing.T) {
	client := NewIRCClient("testlogin", "pass", "testchannel")

	client.mu.Lock()
	client.rateLimited = true
	client.rateLimitUntil = time.Now().Add(time.Second)
	client.mu.Unlock()

	done := make(chan struct{})
	go func() {
		client.WaitForRateLimit()
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)

	start := time.Now()
	client.mu.Lock()
	client.mu.Unlock()
	if elapsed := time.Since(start); elapsed > 200*time.Millisecond {
		t.Fatalf("mutex held during rate-limit sleep: lock acquisition took %v", elapsed)
	}

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("WaitForRateLimit did not return")
	}
}

// TestConcurrentRateLimit tests concurrent access to rate limit methods
func TestConcurrentRateLimit(t *testing.T) {
	client := NewIRCClient("testlogin", "pass", "testchannel")

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client.HandleRateLimit()
			_ = client.IsRateLimited()
			client.WaitForRateLimit()
			client.ResetRateLimit()
		}()
	}
	wg.Wait()

	// Should not panic or race
	client.ResetRateLimit()
}
