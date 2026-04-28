package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"twitch-smm-bot/internal/storage"
)

// MockBotManager is a mock implementation of BotManager
type MockBotManager struct {
	mock.Mock
}

func (m *MockBotManager) ListBots() []BotInfo {
	args := m.Called()
	return args.Get(0).([]BotInfo)
}

func (m *MockBotManager) GetBot(id string) (*BotInfo, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*BotInfo), args.Error(1)
}

func (m *MockBotManager) CreateBot(ctx context.Context, cfg storage.BotConfig) error {
	args := m.Called(ctx, cfg)
	return args.Error(0)
}

func (m *MockBotManager) UpdateBot(ctx context.Context, id string, updates map[string]interface{}) error {
	args := m.Called(ctx, id, updates)
	return args.Error(0)
}

func (m *MockBotManager) DeleteBot(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockBotManager) StartBot(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockBotManager) StopBot(id string) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *MockBotManager) StartAll(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockBotManager) StopAll() error {
	args := m.Called()
	return args.Error(0)
}

// MockStore is a mock implementation of storage.Store
type MockStore struct {
	mock.Mock
}

func (m *MockStore) Close() {}

func (m *MockStore) EnsureSchema(ctx context.Context) error {
	return nil
}

func (m *MockStore) Ping(ctx context.Context) error {
	return nil
}

func (m *MockStore) ListBots(ctx context.Context) ([]storage.BotConfig, error) {
	args := m.Called(ctx)
	return args.Get(0).([]storage.BotConfig), args.Error(1)
}

func (m *MockStore) GetBot(ctx context.Context, botID string) (*storage.BotConfig, error) {
	args := m.Called(ctx, botID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*storage.BotConfig), args.Error(1)
}

func (m *MockStore) CreateBot(ctx context.Context, cfg storage.BotConfig) error {
	args := m.Called(ctx, cfg)
	return args.Error(0)
}

func (m *MockStore) UpdateBot(ctx context.Context, botID string, updates map[string]interface{}) error {
	args := m.Called(ctx, botID, updates)
	return args.Error(0)
}

func (m *MockStore) DeleteBot(ctx context.Context, botID string) error {
	args := m.Called(ctx, botID)
	return args.Error(0)
}

func (m *MockStore) SetBotEnabled(ctx context.Context, botID string, enabled bool) error {
	args := m.Called(ctx, botID, enabled)
	return args.Error(0)
}

func (m *MockStore) UpdateBotChannel(ctx context.Context, botID string, channel string) error {
	args := m.Called(ctx, botID, channel)
	return args.Error(0)
}

func (m *MockStore) GetSettings(ctx context.Context) (*storage.Settings, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*storage.Settings), args.Error(1)
}

func (m *MockStore) UpdateSettings(ctx context.Context, settings *storage.Settings) error {
	args := m.Called(ctx, settings)
	return args.Error(0)
}

func (m *MockStore) EnsureSettingsTable(ctx context.Context) error {
	return nil
}

func (m *MockStore) CreateBotLog(ctx context.Context, log storage.BotLog) error {
	args := m.Called(ctx, log)
	return args.Error(0)
}

func (m *MockStore) GetBotLogs(ctx context.Context, botID string, limit int) ([]storage.BotLog, error) {
	args := m.Called(ctx, botID, limit)
	return args.Get(0).([]storage.BotLog), args.Error(1)
}

func (m *MockStore) CreateSettingsLog(ctx context.Context, log storage.SettingsLog) error {
	args := m.Called(ctx, log)
	return args.Error(0)
}

func (m *MockStore) GetSettingsLogs(ctx context.Context, limit int) ([]storage.SettingsLog, error) {
	args := m.Called(ctx, limit)
	return args.Get(0).([]storage.SettingsLog), args.Error(1)
}

func (m *MockStore) CleanupOldLogs(ctx context.Context) error {
	return nil
}

func (m *MockStore) ListProxies(ctx context.Context) ([]storage.Proxy, error) {
	args := m.Called(ctx)
	return args.Get(0).([]storage.Proxy), args.Error(1)
}

func (m *MockStore) GetProxy(ctx context.Context, id string) (*storage.Proxy, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*storage.Proxy), args.Error(1)
}

func (m *MockStore) CreateProxy(ctx context.Context, proxy storage.Proxy) error {
	args := m.Called(ctx, proxy)
	return args.Error(0)
}

func (m *MockStore) UpdateProxyFailCount(ctx context.Context, id string, failCount int) error {
	return nil
}

func (m *MockStore) UpdateProxyLastUsed(ctx context.Context, id string, lastUsed time.Time) error {
	return nil
}

func (m *MockStore) SetProxyEnabled(ctx context.Context, id string, enabled bool) error {
	return nil
}

func (m *MockStore) DeleteProxy(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func TestHealthCheck(t *testing.T) {
	mockManager := new(MockBotManager)
	mockStore := new(MockStore)
	handler := NewHandler(mockManager, mockStore, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	handler.HealthCheck(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "ok", response["status"])
}

func TestListBots(t *testing.T) {
	t.Run("returns empty list when no bots", func(t *testing.T) {
		mockManager := new(MockBotManager)
		mockStore := new(MockStore)
		handler := NewHandler(mockManager, mockStore, nil, nil, nil)

		mockManager.On("ListBots").Return([]BotInfo{})

		req := httptest.NewRequest(http.MethodGet, "/api/bots", nil)
		w := httptest.NewRecorder()

		handler.ListBots(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var bots []BotDTO
		err := json.Unmarshal(w.Body.Bytes(), &bots)
		require.NoError(t, err)
		assert.Empty(t, bots)
	})

	t.Run("returns list of bots", func(t *testing.T) {
		mockManager := new(MockBotManager)
		mockStore := new(MockStore)
		handler := NewHandler(mockManager, mockStore, nil, nil, nil)

		mockBots := []BotInfo{
			{
				Config: storage.BotConfig{
					ID:              "bot1",
					TwitchLogin:     "testbot1",
					Name:            "Test Bot 1",
					Personality:     "нейтрал",
					MinDelaySec:     45,
					MaxDelaySec:     180,
					RateLimitPerMin: 10,
					Enabled:         true,
				},
				Running: true,
			},
			{
				Config: storage.BotConfig{
					ID:              "bot2",
					TwitchLogin:     "testbot2",
					Name:            "Test Bot 2",
					Personality:     "агрессив",
					MinDelaySec:     30,
					MaxDelaySec:     120,
					RateLimitPerMin: 15,
					Enabled:         false,
				},
				Running: false,
			},
		}

		mockManager.On("ListBots").Return(mockBots)

		req := httptest.NewRequest(http.MethodGet, "/api/bots", nil)
		w := httptest.NewRecorder()

		handler.ListBots(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var bots []BotDTO
		err := json.Unmarshal(w.Body.Bytes(), &bots)
		require.NoError(t, err)
		assert.Len(t, bots, 2)
		assert.Equal(t, "bot1", bots[0].ID)
		assert.Equal(t, "running", bots[0].Status)
		assert.Equal(t, "bot2", bots[1].ID)
		assert.Equal(t, "stopped", bots[1].Status)
	})
}

func TestGetSettings(t *testing.T) {
	t.Run("successfully retrieves settings", func(t *testing.T) {
		mockManager := new(MockBotManager)
		mockStore := new(MockStore)
		handler := NewHandler(mockManager, mockStore, nil, nil, nil)

		expectedSettings := &storage.Settings{
			ID:               "default",
			Channels:         []string{"https://www.twitch.tv/test"},
			DebugMode:        false,
			DebugChannel:     "",
			PositivePrompt:   "test prompt",
			NegativePrompt:   "",
			MaxTokens:        60,
			AudioEnabled:     true,
			RotationEnabled:  false,
			RotationInterval: 60,
			LLMProvider:      "lmstudio",
			LLMApiURL:        "http://localhost:1234/v1",
			LLMModel:         "test-model",
			LLMApiKey:        "",
			TwitchClientID:     "test-client-id",
			TwitchClientSecret: "test-secret",
			UpdatedAt:        time.Now(),
		}

		mockStore.On("GetSettings", mock.Anything).Return(expectedSettings, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
		w := httptest.NewRecorder()

		handler.GetSettings(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var settings storage.Settings
		err := json.Unmarshal(w.Body.Bytes(), &settings)
		require.NoError(t, err)
		assert.Equal(t, "default", settings.ID)
		assert.Equal(t, "lmstudio", settings.LLMProvider)
		assert.Equal(t, "test-client-id", settings.TwitchClientID)
	})

	t.Run("handles error when settings not found", func(t *testing.T) {
		mockManager := new(MockBotManager)
		mockStore := new(MockStore)
		handler := NewHandler(mockManager, mockStore, nil, nil, nil)

		mockStore.On("GetSettings", mock.Anything).Return(nil, assert.AnError)

		req := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
		w := httptest.NewRecorder()

		handler.GetSettings(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestUpdateSettings(t *testing.T) {
	t.Run("successfully updates settings", func(t *testing.T) {
		mockManager := new(MockBotManager)
		mockStore := new(MockStore)
		handler := NewHandler(mockManager, mockStore, nil, nil, nil)

		newSettings := storage.Settings{
			ID:               "default",
			Channels:         []string{"https://www.twitch.tv/updated"},
			DebugMode:        true,
			DebugChannel:     "https://www.twitch.tv/debug",
			PositivePrompt:   "updated prompt",
			NegativePrompt:   "",
			MaxTokens:        100,
			AudioEnabled:     false,
			RotationEnabled:  true,
			RotationInterval: 120,
			LLMProvider:      "openai",
			LLMApiURL:        "https://api.openai.com/v1",
			LLMModel:         "gpt-4",
			LLMApiKey:        "sk-test",
			TwitchClientID:     "new-client-id",
			TwitchClientSecret: "new-secret",
		}

		oldSettings := &storage.Settings{
			ID:          "default",
			Channels:    []string{"https://www.twitch.tv/old"},
			LLMProvider: "lmstudio",
		}

		mockStore.On("GetSettings", mock.Anything).Return(oldSettings, nil)
		mockStore.On("UpdateSettings", mock.Anything, mock.Anything).Return(nil)
		mockStore.On("CreateSettingsLog", mock.Anything, mock.Anything).Return(nil)

		body, _ := json.Marshal(newSettings)
		req := httptest.NewRequest(http.MethodPut, "/api/settings", bytes.NewReader(body))
		w := httptest.NewRecorder()

		handler.UpdateSettings(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("handles invalid JSON", func(t *testing.T) {
		mockManager := new(MockBotManager)
		mockStore := new(MockStore)
		handler := NewHandler(mockManager, mockStore, nil, nil, nil)

		req := httptest.NewRequest(http.MethodPut, "/api/settings", bytes.NewReader([]byte("invalid json")))
		w := httptest.NewRecorder()

		handler.UpdateSettings(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestStartBot(t *testing.T) {
	t.Run("successfully starts bot", func(t *testing.T) {
		mockManager := new(MockBotManager)
		mockStore := new(MockStore)
		handler := NewHandler(mockManager, mockStore, nil, nil, nil)

		mockManager.On("StartBot", mock.Anything, "bot1").Return(nil)

		req := httptest.NewRequest(http.MethodPost, "/api/bots/bot1/start", nil)
		req = mux.SetURLVars(req, map[string]string{"id": "bot1"})
		w := httptest.NewRecorder()

		handler.StartBot(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]string
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "started", response["status"])
	})

	t.Run("handles error when bot fails to start", func(t *testing.T) {
		mockManager := new(MockBotManager)
		mockStore := new(MockStore)
		handler := NewHandler(mockManager, mockStore, nil, nil, nil)

		mockManager.On("StartBot", mock.Anything, "bot2").Return(assert.AnError)

		req := httptest.NewRequest(http.MethodPost, "/api/bots/bot2/start", nil)
		req = mux.SetURLVars(req, map[string]string{"id": "bot2"})
		w := httptest.NewRecorder()

		handler.StartBot(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestStopBot(t *testing.T) {
	t.Run("successfully stops bot", func(t *testing.T) {
		mockManager := new(MockBotManager)
		mockStore := new(MockStore)
		handler := NewHandler(mockManager, mockStore, nil, nil, nil)

		mockManager.On("StopBot", "bot1").Return(nil)

		req := httptest.NewRequest(http.MethodPost, "/api/bots/bot1/stop", nil)
		req = mux.SetURLVars(req, map[string]string{"id": "bot1"})
		w := httptest.NewRecorder()

		handler.StopBot(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]string
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "stopped", response["status"])
	})
}
