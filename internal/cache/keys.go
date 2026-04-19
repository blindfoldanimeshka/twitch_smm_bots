package cache

import "fmt"

const (
	GlobalRateLimitKey = "global:ratelimit"
)

func BotRateLimitKey(botID string) string {
	return fmt.Sprintf("bot:ratelimit:%s", botID)
}

func BotStatusKey(botID string) string {
	return fmt.Sprintf("bot:status:%s", botID)
}

func BotMessagesKey(botID string) string {
	return fmt.Sprintf("bot:messages:%s", botID)
}
