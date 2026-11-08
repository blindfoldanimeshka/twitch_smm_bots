package llm

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"twitch-smm-bot/internal/storage"
)

func defaultConfig() Config {
	return Config{
		Provider:        "openrouter",
		BaseURL:         "https://openrouter.ai/api/v1",
		FreeModel:       "openrouter/free",
		PaidModel:       "z-ai/glm-5.3-flash",
		PaidEnabled:     true,
		PaidDailyCapUSD: 1.0,
		Referer:         "http://localhost:3000",
		Title:           "Twitch SMM Bot Farm",
	}
}

func boolPtr(v bool) *bool        { return &v }
func floatPtr(v float64) *float64 { return &v }

func TestResolveConfig(t *testing.T) {
	tests := []struct {
		name       string
		env        EnvConfig
		db         DBConfig
		mutateWant func(*Config)
		wantSource string
	}{
		{
			name:       "all empty uses built-in defaults",
			wantSource: KeySourceNone,
		},
		{
			name:       "env key wins over database key",
			env:        EnvConfig{APIKey: "env-key"},
			db:         DBConfig{APIKey: "db-key"},
			mutateWant: func(c *Config) { c.APIKey = "env-key" },
			wantSource: KeySourceEnv,
		},
		{
			name:       "database key wins over legacy env key",
			env:        EnvConfig{LegacyAPIKey: "legacy-key"},
			db:         DBConfig{APIKey: "db-key"},
			mutateWant: func(c *Config) { c.APIKey = "db-key" },
			wantSource: KeySourceDatabase,
		},
		{
			name:       "legacy env key alone is reported as env",
			env:        EnvConfig{LegacyAPIKey: "legacy-key"},
			mutateWant: func(c *Config) { c.APIKey = "legacy-key" },
			wantSource: KeySourceEnv,
		},
		{
			name: "database provider url and models win over env",
			env: EnvConfig{
				Provider:  "env-provider",
				BaseURL:   "https://env.example/v1",
				FreeModel: "env/free",
				PaidModel: "env/paid",
			},
			db: DBConfig{
				Provider:  "db-provider",
				BaseURL:   "https://db.example/v1",
				FreeModel: "db/free",
				PaidModel: "db/paid",
			},
			mutateWant: func(c *Config) {
				c.Provider = "db-provider"
				c.BaseURL = "https://db.example/v1"
				c.FreeModel = "db/free"
				c.PaidModel = "db/paid"
			},
			wantSource: KeySourceNone,
		},
		{
			name: "empty database values fall back to env",
			env: EnvConfig{
				Provider:  "env-provider",
				BaseURL:   "https://env.example/v1",
				FreeModel: "env/free",
				PaidModel: "env/paid",
				Referer:   "https://env.example",
				Title:     "Env Title",
			},
			mutateWant: func(c *Config) {
				c.Provider = "env-provider"
				c.BaseURL = "https://env.example/v1"
				c.FreeModel = "env/free"
				c.PaidModel = "env/paid"
				c.Referer = "https://env.example"
				c.Title = "Env Title"
			},
			wantSource: KeySourceNone,
		},
		{
			name:       "empty database model falls back to default",
			db:         DBConfig{PaidModel: ""},
			wantSource: KeySourceNone,
		},
		{
			name:       "env paid enabled false overrides database true",
			env:        EnvConfig{PaidEnabled: "false"},
			db:         DBConfig{PaidEnabled: boolPtr(true)},
			mutateWant: func(c *Config) { c.PaidEnabled = false },
			wantSource: KeySourceNone,
		},
		{
			name:       "env paid enabled true overrides database false",
			env:        EnvConfig{PaidEnabled: "true"},
			db:         DBConfig{PaidEnabled: boolPtr(false)},
			mutateWant: func(c *Config) { c.PaidEnabled = true },
			wantSource: KeySourceNone,
		},
		{
			name:       "invalid env paid enabled falls back to database value",
			env:        EnvConfig{PaidEnabled: "maybe"},
			db:         DBConfig{PaidEnabled: boolPtr(false)},
			mutateWant: func(c *Config) { c.PaidEnabled = false },
			wantSource: KeySourceNone,
		},
		{
			name:       "env daily cap overrides database value",
			env:        EnvConfig{PaidDailyCapUSD: "0.25"},
			db:         DBConfig{PaidDailyCapUSD: floatPtr(5)},
			mutateWant: func(c *Config) { c.PaidDailyCapUSD = 0.25 },
			wantSource: KeySourceNone,
		},
		{
			name:       "negative env daily cap means unlimited",
			env:        EnvConfig{PaidDailyCapUSD: "-1"},
			db:         DBConfig{PaidDailyCapUSD: floatPtr(5)},
			mutateWant: func(c *Config) { c.PaidDailyCapUSD = -1 },
			wantSource: KeySourceNone,
		},
		{
			name:       "invalid env daily cap falls back to database value",
			env:        EnvConfig{PaidDailyCapUSD: "abc"},
			db:         DBConfig{PaidDailyCapUSD: floatPtr(2.5)},
			mutateWant: func(c *Config) { c.PaidDailyCapUSD = 2.5 },
			wantSource: KeySourceNone,
		},
		{
			name:       "empty env paid values use database pointers",
			db:         DBConfig{PaidEnabled: boolPtr(true), PaidDailyCapUSD: floatPtr(3.5)},
			mutateWant: func(c *Config) { c.PaidDailyCapUSD = 3.5 },
			wantSource: KeySourceNone,
		},
		{
			name:       "nil database pointers use defaults",
			db:         DBConfig{},
			wantSource: KeySourceNone,
		},
		{
			name:       "stored false and zero survive without env override",
			db:         DBConfig{PaidEnabled: boolPtr(false), PaidDailyCapUSD: floatPtr(0)},
			mutateWant: func(c *Config) { c.PaidEnabled = false; c.PaidDailyCapUSD = 0 },
			wantSource: KeySourceNone,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, source := ResolveConfig(tt.env, tt.db)

			want := defaultConfig()
			if tt.mutateWant != nil {
				tt.mutateWant(&want)
			}

			assert.Equal(t, want, cfg)
			assert.Equal(t, tt.wantSource, source)
			assert.Nil(t, cfg.HTTPClient)
		})
	}
}

func TestEnvFromLookupTreatsMissingAsEmpty(t *testing.T) {
	values := map[string]string{
		"OPENROUTER_API_KEY":     "env-key",
		"LLM_API_KEY":            "legacy-key",
		"LLM_PROVIDER":           "env-provider",
		"LLM_API_URL":            "https://env.example/v1",
		"LLM_MODEL":              "env/free",
		"LLM_PAID_MODEL":         "env/paid",
		"LLM_PAID_ENABLED":       "false",
		"LLM_PAID_DAILY_CAP_USD": "0.5",
		"OPENROUTER_REFERER":     "https://env.example",
		"OPENROUTER_TITLE":       "Env Title",
	}

	got := EnvFromLookup(func(key string) string {
		return values[key]
	})

	assert.Equal(t, EnvConfig{
		APIKey:          "env-key",
		LegacyAPIKey:    "legacy-key",
		Provider:        "env-provider",
		BaseURL:         "https://env.example/v1",
		FreeModel:       "env/free",
		PaidModel:       "env/paid",
		PaidEnabled:     "false",
		PaidDailyCapUSD: "0.5",
		Referer:         "https://env.example",
		Title:           "Env Title",
	}, got)

	assert.Equal(t, EnvConfig{}, EnvFromLookup(func(string) string { return "" }))
}

func TestDBConfigFromSettingsCopiesAllFields(t *testing.T) {
	settings := &storage.Settings{
		LLMProvider:        "openrouter",
		LLMApiURL:          "https://openrouter.ai/api/v1",
		LLMModel:           "openrouter/free",
		LLMPaidModel:       "z-ai/glm-5.3-flash",
		LLMApiKey:          "db-key",
		LLMPaidEnabled:     false,
		LLMPaidDailyCapUSD: 2.5,
	}

	got := DBConfigFromSettings(settings)
	assert.Equal(t, DBConfig{
		Provider:        "openrouter",
		BaseURL:         "https://openrouter.ai/api/v1",
		FreeModel:       "openrouter/free",
		PaidModel:       "z-ai/glm-5.3-flash",
		APIKey:          "db-key",
		PaidEnabled:     boolPtr(false),
		PaidDailyCapUSD: floatPtr(2.5),
	}, got)

	assert.Equal(t, DBConfig{}, DBConfigFromSettings(nil))
}
