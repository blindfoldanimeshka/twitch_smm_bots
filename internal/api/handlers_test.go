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
	"twitch-smm-bot/internal/llm"
	"twitch-smm-bot/internal/storage"
	"twitch-smm-bot/internal/twitch"
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

func (m *MockBotManager) StartBotWithOptions(ctx context.Context, id string, disableLLM bool) error {
	args := m.Called(ctx, id, disableLLM)
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

// MockSettingsManager is a mock implementation of SettingsManager.
type MockSettingsManager struct {
	mock.Mock
}

func (m *MockSettingsManager) ApplySettings(ctx context.Context, settings *storage.Settings) error {
	args := m.Called(ctx, settings)
	return args.Error(0)
}

func (m *MockSettingsManager) GetAudioContext() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockSettingsManager) TestLLMGeneration(ctx context.Context, personality string) (string, error) {
	args := m.Called(ctx, personality)
	return args.String(0), args.Error(1)
}

// fixedEnv returns an env lookup backed by the given map.
func fixedEnv(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

// FakeStreamStatusProvider is a stub StreamStatusProvider that records the
// arguments every Snapshot call receives.
type FakeStreamStatusProvider struct {
	channels []string
	status   twitch.StreamStatus

	gotChannel string
	gotExclude map[string]struct{}
}

func (f *FakeStreamStatusProvider) Channels() []string { return f.channels }

func (f *FakeStreamStatusProvider) Snapshot(ctx context.Context, channel string, excludeLogins map[string]struct{}) twitch.StreamStatus {
	f.gotChannel = channel
	f.gotExclude = excludeLogins
	return f.status
}

func TestHealthCheck(t *testing.T) {
	mockManager := new(MockBotManager)
	mockStore := new(MockStore)
	handler := NewHandler(mockManager, mockStore, nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	handler.HealthCheck(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "ok", response["status"])
}

func TestListBots(t *testing.T) {
	t.Run("returns empty list when no bots", func(t *testing.T) {
		mockManager := new(MockBotManager)
		mockStore := new(MockStore)
		handler := NewHandler(mockManager, mockStore, nil, nil, nil, nil)

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
		handler := NewHandler(mockManager, mockStore, nil, nil, nil, nil)

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
		handler := NewHandler(mockManager, mockStore, nil, nil, nil, nil)

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
			LLMProvider:      "openrouter",
			LLMApiURL:        "https://openrouter.ai/api/v1",
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
		assert.Equal(t, "openrouter", settings.LLMProvider)
		assert.Equal(t, "test-client-id", settings.TwitchClientID)
	})

	t.Run("handles error when settings not found", func(t *testing.T) {
		mockManager := new(MockBotManager)
		mockStore := new(MockStore)
		handler := NewHandler(mockManager, mockStore, nil, nil, nil, nil)

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
		handler := NewHandler(mockManager, mockStore, nil, nil, nil, nil)

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
			LLMProvider: "openrouter",
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
		handler := NewHandler(mockManager, mockStore, nil, nil, nil, nil)

		mockStore.On("CreateSettingsLog", mock.Anything, mock.Anything).Return(nil)

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
		handler := NewHandler(mockManager, mockStore, nil, nil, nil, nil)

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
		handler := NewHandler(mockManager, mockStore, nil, nil, nil, nil)

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
		handler := NewHandler(mockManager, mockStore, nil, nil, nil, nil)

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

func TestGetSettingsMasksAPIKeyAndReportsSource(t *testing.T) {
	mockManager := new(MockBotManager)
	mockStore := new(MockStore)
	handler := NewHandler(mockManager, mockStore, nil, nil, nil, nil)
	handler.envLookup = fixedEnv(nil)

	stored := &storage.Settings{
		ID:        "default",
		LLMApiKey: "sk-db",
	}
	mockStore.On("GetSettings", mock.Anything).Return(stored, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	w := httptest.NewRecorder()
	handler.GetSettings(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "", body["llm_api_key"])
	assert.Equal(t, true, body["llm_api_key_set"])
	assert.Equal(t, "database", body["llm_api_key_source"])
}

func TestGetSettingsReportsEnvKey(t *testing.T) {
	mockManager := new(MockBotManager)
	mockStore := new(MockStore)
	handler := NewHandler(mockManager, mockStore, nil, nil, nil, nil)
	handler.envLookup = fixedEnv(map[string]string{
		"OPENROUTER_API_KEY": "sk-env",
	})

	stored := &storage.Settings{
		ID:        "default",
		LLMApiKey: "sk-db",
	}
	mockStore.On("GetSettings", mock.Anything).Return(stored, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	w := httptest.NewRecorder()
	handler.GetSettings(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "env", body["llm_api_key_source"])
	assert.Equal(t, true, body["llm_api_key_set"])
	assert.NotContains(t, w.Body.String(), "sk-env")
	assert.NotContains(t, w.Body.String(), "sk-db")
}

func TestGetSettingsReportsPaidEnvOverride(t *testing.T) {
	mockManager := new(MockBotManager)
	mockStore := new(MockStore)
	handler := NewHandler(mockManager, mockStore, nil, nil, nil, nil)
	handler.envLookup = fixedEnv(map[string]string{
		"LLM_PAID_ENABLED":       "true",
		"LLM_PAID_DAILY_CAP_USD": "0.25",
	})

	stored := &storage.Settings{
		ID:                 "default",
		LLMPaidEnabled:     false,
		LLMPaidDailyCapUSD: 0.5,
	}
	mockStore.On("GetSettings", mock.Anything).Return(stored, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	w := httptest.NewRecorder()
	handler.GetSettings(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var body storage.Settings
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, true, body.LLMPaidEnabledEffective)
	assert.Equal(t, 0.25, body.LLMPaidDailyCapUSDEffective)
}

func TestUpdateSettingsPreservesStoredKeyWhenEmpty(t *testing.T) {
	mockManager := new(MockBotManager)
	mockStore := new(MockStore)
	mockSettings := new(MockSettingsManager)
	handler := NewHandler(mockManager, mockStore, nil, nil, nil, nil)
	handler.settingsManager = mockSettings
	handler.envLookup = fixedEnv(nil)

	oldSettings := &storage.Settings{
		ID:        "default",
		LLMApiKey: "sk-db",
	}
	mockStore.On("GetSettings", mock.Anything).Return(oldSettings, nil)

	var storeReceived *storage.Settings
	mockStore.On("UpdateSettings", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) { storeReceived = args.Get(1).(*storage.Settings) }).
		Return(nil)
	mockStore.On("CreateSettingsLog", mock.Anything, mock.Anything).Return(nil)

	mockSettings.On("ApplySettings", mock.Anything, mock.MatchedBy(func(s *storage.Settings) bool {
		return s.LLMApiKey == "sk-db"
	})).Return(nil)

	body, _ := json.Marshal(map[string]interface{}{"llm_api_key": ""})
	req := httptest.NewRequest(http.MethodPut, "/api/settings", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.UpdateSettings(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	require.NotNil(t, storeReceived)
	assert.Equal(t, "sk-db", storeReceived.LLMApiKey)
	mockSettings.AssertExpectations(t)
}

func TestUpdateSettingsUsesIncomingKeyWhenProvided(t *testing.T) {
	mockManager := new(MockBotManager)
	mockStore := new(MockStore)
	mockSettings := new(MockSettingsManager)
	handler := NewHandler(mockManager, mockStore, nil, nil, nil, nil)
	handler.settingsManager = mockSettings
	handler.envLookup = fixedEnv(nil)

	oldSettings := &storage.Settings{
		ID:        "default",
		LLMApiKey: "sk-db",
	}
	mockStore.On("GetSettings", mock.Anything).Return(oldSettings, nil)

	var storeReceived *storage.Settings
	mockStore.On("UpdateSettings", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) { storeReceived = args.Get(1).(*storage.Settings) }).
		Return(nil)
	mockStore.On("CreateSettingsLog", mock.Anything, mock.Anything).Return(nil)

	mockSettings.On("ApplySettings", mock.Anything, mock.MatchedBy(func(s *storage.Settings) bool {
		return s.LLMApiKey == "sk-new"
	})).Return(nil)

	body, _ := json.Marshal(map[string]interface{}{"llm_api_key": "sk-new"})
	req := httptest.NewRequest(http.MethodPut, "/api/settings", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.UpdateSettings(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	require.NotNil(t, storeReceived)
	assert.Equal(t, "sk-new", storeReceived.LLMApiKey)
	mockSettings.AssertExpectations(t)
}

func TestGetLLMStatusIncludesGenerationFields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/key" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"usage_daily":0.1,"free_model_daily_requests":{"limit":50,"remaining":50,"used":0}}}`))
			return
		}
		t.Errorf("unexpected path %s", r.URL.Path)
	}))
	defer srv.Close()

	client := llm.NewClient(llm.Config{
		BaseURL:         srv.URL,
		APIKey:          "sk-test",
		FreeModel:       "openrouter/free",
		PaidModel:       "z-ai/glm-5.3-flash",
		PaidEnabled:     true,
		PaidDailyCapUSD: 1.0,
	})

	mockManager := new(MockBotManager)
	mockStore := new(MockStore)
	handler := NewHandler(mockManager, mockStore, nil, client, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/llm/status", nil)
	w := httptest.NewRecorder()
	handler.GetLLMStatus(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))

	for _, key := range []string{"available", "generation_available", "model", "free_model", "url", "paid_spent_today_usd", "free_requests_remaining", "inflight_paid_requests"} {
		_, ok := body[key]
		assert.True(t, ok, "missing key %q", key)
	}
	assert.Equal(t, true, body["generation_available"])
}

func TestCalculateSettingsChangesIncludesPaidFields(t *testing.T) {
	handler := &Handler{}
	old := &storage.Settings{
		LLMPaidModel:       "old-paid",
		LLMPaidEnabled:     true,
		LLMPaidDailyCapUSD: 1.0,
	}
	new := &storage.Settings{
		LLMPaidModel:       "new-paid",
		LLMPaidEnabled:     false,
		LLMPaidDailyCapUSD: 0.5,
	}

	changes := handler.calculateSettingsChanges(old, new)

	assert.Contains(t, changes, "llm_paid_model")
	assert.Contains(t, changes, "llm_paid_enabled")
	assert.Contains(t, changes, "llm_paid_daily_cap_usd")
}

func TestCalculateSettingsChangesMasksAPIKey(t *testing.T) {
	handler := &Handler{}
	old := &storage.Settings{LLMApiKey: "sk-db"}
	new := &storage.Settings{LLMApiKey: "sk-new"}

	changes := handler.calculateSettingsChanges(old, new)

	entry, ok := changes["llm_api_key"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "***", entry["old"])
	assert.Equal(t, "***", entry["new"])

	same := handler.calculateSettingsChanges(old, old)
	assert.NotContains(t, same, "llm_api_key")
}

func TestUpdateSettingsFailsWhenOldSettingsUnavailable(t *testing.T) {
	mockManager := new(MockBotManager)
	mockStore := new(MockStore)
	mockSettings := new(MockSettingsManager)
	handler := NewHandler(mockManager, mockStore, nil, nil, nil, nil)
	handler.settingsManager = mockSettings
	handler.envLookup = fixedEnv(nil)

	mockStore.On("GetSettings", mock.Anything).Return(nil, assert.AnError)
	mockStore.On("CreateSettingsLog", mock.Anything, mock.Anything).Return(nil)

	body, _ := json.Marshal(map[string]interface{}{"llm_api_key": ""})
	req := httptest.NewRequest(http.MethodPut, "/api/settings", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.UpdateSettings(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	mockStore.AssertNotCalled(t, "UpdateSettings", mock.Anything, mock.Anything)
	mockSettings.AssertNotCalled(t, "ApplySettings", mock.Anything, mock.Anything)
}

func TestCalculateSettingsChangesHandlesNilOld(t *testing.T) {
	handler := &Handler{}
	new := &storage.Settings{
		Channels:  []string{"ch"},
		LLMApiKey: "k",
	}

	changes := handler.calculateSettingsChanges(nil, new)

	assert.Contains(t, changes, "channels")
	assert.Contains(t, changes, "llm_api_key")
}

func TestGetStreamStatusNoChannel(t *testing.T) {
	provider := &FakeStreamStatusProvider{status: twitch.StreamStatus{Configured: false}}
	mockManager := new(MockBotManager)
	mockManager.On("ListBots").Return([]BotInfo{})
	handler := NewHandler(mockManager, new(MockStore), nil, nil, nil, provider)

	req := httptest.NewRequest(http.MethodGet, "/api/twitch/stream", nil)
	w := httptest.NewRecorder()

	handler.GetStreamStatus(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, false, body["configured"])
}

func TestGetStreamStatusNilMonitor(t *testing.T) {
	handler := NewHandler(new(MockBotManager), new(MockStore), nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/twitch/stream", nil)
	w := httptest.NewRecorder()

	handler.GetStreamStatus(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, false, body["configured"])
}

func TestGetStreamStatusLive(t *testing.T) {
	provider := &FakeStreamStatusProvider{status: twitch.StreamStatus{
		Configured:     true,
		Channel:        "a",
		Live:           true,
		Title:          "hello",
		Game:           "Just Chatting",
		UptimeSeconds:  120.5,
		ViewersTotal:   100,
		ActiveViewers:  2,
		PassiveViewers: 98,
		Chatters:       []twitch.Chatter{{Login: "alice"}},
		Messages:       []twitch.ChatMessage{{Login: "alice", Text: "hi"}},
	}}
	mockManager := new(MockBotManager)
	mockManager.On("ListBots").Return([]BotInfo{})
	handler := NewHandler(mockManager, new(MockStore), nil, nil, nil, provider)

	req := httptest.NewRequest(http.MethodGet, "/api/twitch/stream", nil)
	w := httptest.NewRecorder()

	handler.GetStreamStatus(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	for _, key := range []string{"uptime_seconds", "viewers_total", "active_viewers", "passive_viewers", "messages"} {
		_, ok := body[key]
		assert.True(t, ok, "missing key %q", key)
	}
	assert.Equal(t, 120.5, body["uptime_seconds"])
	assert.Equal(t, 100.0, body["viewers_total"])
	assert.Equal(t, 2.0, body["active_viewers"])
	assert.Equal(t, 98.0, body["passive_viewers"])
}

func TestGetStreamStatusRejectsUnconfiguredChannel(t *testing.T) {
	provider := &FakeStreamStatusProvider{channels: []string{"a"}}
	handler := NewHandler(new(MockBotManager), new(MockStore), nil, nil, nil, provider)

	req := httptest.NewRequest(http.MethodGet, "/api/twitch/stream?channel=b", nil)
	w := httptest.NewRecorder()

	handler.GetStreamStatus(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "channel is not configured")
}

func TestGetStreamStatusExcludesBots(t *testing.T) {
	provider := &FakeStreamStatusProvider{channels: []string{"a"}}
	mockManager := new(MockBotManager)
	mockManager.On("ListBots").Return([]BotInfo{
		{Config: storage.BotConfig{ID: "b1", TwitchLogin: "bot1"}, Running: true},
	})
	handler := NewHandler(mockManager, new(MockStore), nil, nil, nil, provider)

	req := httptest.NewRequest(http.MethodGet, "/api/twitch/stream?channel=a", nil)
	w := httptest.NewRecorder()

	handler.GetStreamStatus(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "a", provider.gotChannel)
	assert.Contains(t, provider.gotExclude, "bot1", "bot logins must be passed as the exclusion set")
}
