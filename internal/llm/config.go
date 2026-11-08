package llm

import (
	"net/http"
	"strconv"

	"twitch-smm-bot/internal/storage"
)

// Default configuration values used when neither the database nor the
// environment provides one.
const (
	ProviderOpenRouter  = "openrouter"
	DefaultBaseURL      = "https://openrouter.ai/api/v1"
	DefaultFreeModel    = "openrouter/free"
	DefaultPaidModel    = "z-ai/glm-5.3-flash"
	DefaultPaidEnabled  = true
	DefaultPaidDailyCap = 1.0
	DefaultReferer      = "http://localhost:3000"
	DefaultTitle        = "Twitch SMM Bot Farm"
)

// API key sources reported by ResolveConfig.
const (
	KeySourceEnv      = "env"
	KeySourceDatabase = "database"
	KeySourceNone     = "none"
)

// Config is the fully resolved LLM configuration.
// HTTPClient is left nil here and supplied by NewClient.
type Config struct {
	Provider        string
	BaseURL         string
	APIKey          string
	FreeModel       string
	PaidModel       string
	PaidEnabled     bool
	PaidDailyCapUSD float64
	Referer         string
	Title           string
	HTTPClient      *http.Client
}

// EnvConfig holds the raw environment values relevant to the LLM config.
type EnvConfig struct {
	APIKey          string
	LegacyAPIKey    string
	Provider        string
	BaseURL         string
	FreeModel       string
	PaidModel       string
	PaidEnabled     string
	PaidDailyCapUSD string
	Referer         string
	Title           string
}

// DBConfig holds the LLM-related values stored in the settings table.
// PaidEnabled and PaidDailyCapUSD are pointers so that a missing row (nil) is
// distinguishable from a stored false/0.
type DBConfig struct {
	Provider        string
	BaseURL         string
	FreeModel       string
	PaidModel       string
	APIKey          string
	PaidEnabled     *bool
	PaidDailyCapUSD *float64
}

// EnvFromLookup reads the LLM environment variables through lookup.
// Missing keys are treated as empty strings.
func EnvFromLookup(lookup func(string) string) EnvConfig {
	return EnvConfig{
		APIKey:          lookup("OPENROUTER_API_KEY"),
		LegacyAPIKey:    lookup("LLM_API_KEY"),
		Provider:        lookup("LLM_PROVIDER"),
		BaseURL:         lookup("LLM_API_URL"),
		FreeModel:       lookup("LLM_MODEL"),
		PaidModel:       lookup("LLM_PAID_MODEL"),
		PaidEnabled:     lookup("LLM_PAID_ENABLED"),
		PaidDailyCapUSD: lookup("LLM_PAID_DAILY_CAP_USD"),
		Referer:         lookup("OPENROUTER_REFERER"),
		Title:           lookup("OPENROUTER_TITLE"),
	}
}

// DBConfigFromSettings maps stored settings to a DBConfig. It returns a zero
// DBConfig when settings is nil.
func DBConfigFromSettings(s *storage.Settings) DBConfig {
	if s == nil {
		return DBConfig{}
	}

	return DBConfig{
		Provider:        s.LLMProvider,
		BaseURL:         s.LLMApiURL,
		FreeModel:       s.LLMModel,
		PaidModel:       s.LLMPaidModel,
		APIKey:          s.LLMApiKey,
		PaidEnabled:     &s.LLMPaidEnabled,
		PaidDailyCapUSD: &s.LLMPaidDailyCapUSD,
	}
}

// ResolveConfig merges environment and database values into a Config and
// reports where the API key came from (env, database or none).
// It is pure: it never reads the process environment.
func ResolveConfig(env EnvConfig, db DBConfig) (Config, string) {
	cfg := Config{
		Provider:        ProviderOpenRouter,
		BaseURL:         DefaultBaseURL,
		FreeModel:       DefaultFreeModel,
		PaidModel:       DefaultPaidModel,
		PaidEnabled:     DefaultPaidEnabled,
		PaidDailyCapUSD: DefaultPaidDailyCap,
		Referer:         DefaultReferer,
		Title:           DefaultTitle,
	}

	source := KeySourceNone
	switch {
	case env.APIKey != "":
		cfg.APIKey = env.APIKey
		source = KeySourceEnv
	case db.APIKey != "":
		cfg.APIKey = db.APIKey
		source = KeySourceDatabase
	case env.LegacyAPIKey != "":
		cfg.APIKey = env.LegacyAPIKey
		source = KeySourceEnv
	}

	cfg.Provider = firstNonEmpty(db.Provider, env.Provider, cfg.Provider)
	cfg.BaseURL = firstNonEmpty(db.BaseURL, env.BaseURL, cfg.BaseURL)
	cfg.FreeModel = firstNonEmpty(db.FreeModel, env.FreeModel, cfg.FreeModel)
	cfg.PaidModel = firstNonEmpty(db.PaidModel, env.PaidModel, cfg.PaidModel)

	if v, ok := parseEnvBool(env.PaidEnabled); ok {
		cfg.PaidEnabled = v
	} else if db.PaidEnabled != nil {
		cfg.PaidEnabled = *db.PaidEnabled
	}

	if v, ok := parseEnvFloat(env.PaidDailyCapUSD); ok {
		cfg.PaidDailyCapUSD = v
	} else if db.PaidDailyCapUSD != nil {
		cfg.PaidDailyCapUSD = *db.PaidDailyCapUSD
	}

	cfg.Referer = firstNonEmpty(env.Referer, cfg.Referer)
	cfg.Title = firstNonEmpty(env.Title, cfg.Title)

	return cfg, source
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// parseEnvBool parses a non-empty env value. Empty or invalid values are
// reported as not set so callers can fall back to the database.
func parseEnvBool(raw string) (bool, bool) {
	if raw == "" {
		return false, false
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		return false, false
	}
	return v, true
}

// parseEnvFloat parses a non-empty env value. Empty or invalid values are
// reported as not set so callers can fall back to the database.
func parseEnvFloat(raw string) (float64, bool) {
	if raw == "" {
		return 0, false
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}
