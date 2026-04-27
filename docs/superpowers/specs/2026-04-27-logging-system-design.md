---
name: TwitchSMM Comprehensive Logging System
description: Hybrid logging architecture with PostgreSQL persistence, Redis caching, and real-time WebSocket updates for bot activity and settings changes
type: design
date: 2026-04-27
---

# TwitchSMM Comprehensive Logging System - Design Specification

## Overview

This document specifies a comprehensive logging system for the TwitchSMM bot management platform. The system provides real-time visibility into bot activity and settings changes through a hybrid architecture combining PostgreSQL persistence, Redis caching, and WebSocket real-time updates.

## Goals

1. **Real-time Monitoring** - Instant visibility into bot activity (messages sent, errors, lifecycle events)
2. **Settings Audit Trail** - Track all configuration changes with detailed before/after comparisons
3. **Performance** - Fast log retrieval without impacting bot operations
4. **Reliability** - Graceful degradation when components fail
5. **Storage Efficiency** - 24-hour retention with automatic cleanup

## Requirements Summary

### Functional Requirements

- Display bot activity logs under each bot card (expandable panel)
- Display unified settings change log at bottom of Settings page
- Real-time log updates via WebSocket
- 24-hour log retention in PostgreSQL
- 1-hour fast cache in Redis
- Frontend in-memory cache (100 logs per bot, 50 settings logs)

### Non-Functional Requirements

- Log writes must not block bot message sending
- API responses < 500ms (Redis cache hit)
- Graceful fallback: Redis → PostgreSQL → frontend cache
- Automatic cleanup of logs older than 24 hours
- Support for multiple concurrent bots without performance degradation

## Architecture

### Storage Layers

```
┌─────────────────────────────────────────────────────────────┐
│                        Frontend                              │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  In-Memory Cache (Zustand Store)                     │  │
│  │  - Bot logs: 100 entries per bot                     │  │
│  │  - Settings logs: 50 entries                         │  │
│  │  - Real-time updates via WebSocket                   │  │
│  └──────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
                            ▲
                            │ WebSocket + REST API
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                        Backend (Go)                          │
│  ┌──────────────────┐         ┌──────────────────────────┐ │
│  │  Redis Cache     │         │  PostgreSQL              │ │
│  │  - 1 hour TTL    │◄────────┤  - 24 hour retention     │ │
│  │  - Fast reads    │ Fallback│  - Persistent storage    │ │
│  │  - Auto-trim     │         │  - Indexed queries       │ │
│  └──────────────────┘         └──────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
```

### Data Flow

**Bot Activity Logging:**
```
Bot sends message
    ↓
logic.go: logBotEvent()
    ↓
├─→ PostgreSQL (async, 2s timeout)
├─→ Redis cache (async, 500ms timeout)
└─→ WebSocket broadcast (immediate)
    ↓
Frontend receives WebSocket message
    ↓
Zustand store updates
    ↓
BotLogPanel re-renders
```

**Settings Change Logging:**
```
User saves settings
    ↓
handlers.go: UpdateSettings()
    ↓
Calculate changes (old vs new)
    ↓
├─→ PostgreSQL (async, 2s timeout)
├─→ Redis cache (async, 500ms timeout)
└─→ WebSocket broadcast (immediate)
    ↓
Frontend receives WebSocket message
    ↓
Zustand store updates
    ↓
SettingsLogPanel re-renders
```

**Log Retrieval (API Request):**
```
Frontend: GET /api/bots/{id}/logs
    ↓
Backend: Try Redis first
    ├─→ Cache hit? Return immediately
    └─→ Cache miss? Query PostgreSQL
        ├─→ Success? Return logs
        └─→ Error? Return empty array with X-Log-Status header
```

## Database Schema

### PostgreSQL Tables

**bot_logs table:**
```sql
CREATE TABLE IF NOT EXISTS bot_logs (
    id BIGSERIAL PRIMARY KEY,
    bot_id TEXT NOT NULL,
    log_type TEXT NOT NULL, -- 'lifecycle', 'message', 'error', 'rate_limit', 'llm'
    level TEXT NOT NULL, -- 'info', 'success', 'warning', 'error'
    message TEXT NOT NULL,
    context JSONB, -- Additional structured data
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    INDEX idx_bot_logs_bot_id_created (bot_id, created_at DESC),
    INDEX idx_bot_logs_created (created_at DESC)
);
```

**settings_logs table:**
```sql
CREATE TABLE IF NOT EXISTS settings_logs (
    id BIGSERIAL PRIMARY KEY,
    level TEXT NOT NULL, -- 'info', 'success', 'error'
    message TEXT NOT NULL,
    changes JSONB, -- {"channels": {"old": ["x"], "new": ["x", "y"]}}
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    INDEX idx_settings_logs_created (created_at DESC)
);
```

### Redis Cache Structure

**Bot logs:**
- Key: `bot_logs:{bot_id}`
- Type: List (LPUSH for newest first)
- TTL: 3600 seconds (1 hour)
- Max entries: 100 (LTRIM after each insert)

**Settings logs:**
- Key: `settings_logs`
- Type: List (LPUSH for newest first)
- TTL: 3600 seconds (1 hour)
- Max entries: 50 (LTRIM after each insert)

## Backend Implementation

### New Files

**internal/storage/logs.go:**
```go
type BotLog struct {
    ID        int64
    BotID     string
    LogType   string
    Level     string
    Message   string
    Context   map[string]interface{}
    CreatedAt time.Time
}

type SettingsLog struct {
    ID        int64
    Level     string
    Message   string
    Changes   map[string]interface{}
    CreatedAt time.Time
}

// Methods:
- CreateBotLog(ctx, log BotLog) error
- GetBotLogs(ctx, botID string, limit int) ([]BotLog, error)
- CreateSettingsLog(ctx, log SettingsLog) error
- GetSettingsLogs(ctx, limit int) ([]SettingsLog, error)
- CleanupOldLogs(ctx) error
```

**internal/cache/logs.go:**
```go
// Methods:
- AddBotLog(ctx, botID string, log BotLog) error
- GetBotLogs(ctx, botID string, limit int) ([]BotLog, error)
- AddSettingsLog(ctx, log SettingsLog) error
- GetSettingsLogs(ctx, limit int) ([]SettingsLog, error)
```

### Modified Files

**logic.go:**
- Add `logBotEvent()` helper method to botManager
- Integrate logging calls in `runBotLoop()`:
  - Bot started/stopped (lifecycle)
  - IRC reconnection (error/success)
  - Rate limit hit (warning)
  - LLM generation failure (error)
  - Message sent successfully (success)
- Add `runLogCleanupJob()` background goroutine in main()

**internal/api/handlers.go:**
- Add `GetBotLogs(w, r)` endpoint: GET /api/bots/{id}/logs
- Add `GetSettingsLogs(w, r)` endpoint: GET /api/settings/logs
- Modify `UpdateSettings()` to:
  - Calculate changes (old vs new)
  - Format detailed message
  - Call `logSettingsChange()`
- Add helper methods:
  - `calculateSettingsChanges(old, new)`
  - `formatSettingsChangeMessage(changes)`
  - `logSettingsChange(level, message, changes)`

**internal/api/websocket.go:**
- Extend `BroadcastBotMessage()` to include log context
- Add `BroadcastBotLog()` method
- Add `BroadcastSettingsLog()` method

**internal/api/server.go:**
- Register new routes:
  - `GET /api/bots/{id}/logs`
  - `GET /api/settings/logs`

## Frontend Implementation

### New Files

**frontend/src/stores/logStore.ts:**
```typescript
interface LogStore {
  botLogs: Map<string, LogEntry[]>
  settingsLogs: SettingsLogEntry[]
  
  addBotLog: (botId: string, log: LogEntry) => void
  setBotLogs: (botId: string, logs: LogEntry[]) => void
  clearBotLogs: (botId: string) => void
  
  addSettingsLog: (log: SettingsLogEntry) => void
  setSettingsLogs: (logs: SettingsLogEntry[]) => void
  clearSettingsLogs: () => void
}
```

**frontend/src/components/BotLogPanel.tsx:**
- Expandable log panel component
- Fetches initial logs from API on mount
- Displays logs with color-coded levels
- Auto-scrollable with max height 300px
- Clear button to reset logs
- Relative timestamps ("2m ago")

**frontend/src/components/SettingsLogPanel.tsx:**
- Unified settings log panel
- Collapsible by default
- Shows detailed change information
- Displays old → new value comparisons
- Max height 250px with scroll

### Modified Files

**frontend/src/api/client.ts:**
- Add `getBotLogs(botId, limit)` method
- Add `getSettingsLogs(limit)` method

**frontend/src/hooks/useWebSocket.ts:**
- Import `useLogStore`
- Handle `bot_message_sent` messages → add to bot logs
- Handle `settings_log` messages → add to settings logs

**frontend/src/pages/Bots.tsx:**
- Add state: `expandedLogs: Set<string>`
- Add "📋 ЛОГИ" button to each bot card
- Render `<BotLogPanel>` when expanded

**frontend/src/pages/Settings.tsx:**
- Add `<SettingsLogPanel>` at bottom (before toast notifications)
- Collapsed by default

## Log Types and Levels

### Bot Log Types

| Type | Description | Example |
|------|-------------|---------|
| `lifecycle` | Bot start/stop events | "Bot started", "Bot stopped" |
| `message` | Successfully sent messages | "привет стример!" |
| `error` | Send failures, IRC errors | "Failed to send message: connection lost" |
| `rate_limit` | Rate limit hits | "Global rate limit reached" |
| `llm` | LLM generation failures | "LLM generation failed: timeout" |

### Log Levels

| Level | Color | Icon | Usage |
|-------|-------|------|-------|
| `info` | Blue/Cyan | ℹ | Informational events |
| `success` | Green | ✓ | Successful operations |
| `warning` | Yellow | ⚠ | Non-critical issues |
| `error` | Red | ✗ | Failures requiring attention |

### Settings Log Levels

| Level | Usage |
|-------|-------|
| `success` | Settings saved successfully |
| `error` | Validation errors, save failures |
| `warning` | Saved but runtime apply failed |

## Error Handling

### Backend Error Scenarios

**PostgreSQL unavailable:**
- Log writes fail silently (don't crash bots)
- API reads fall back to Redis
- If both fail, return empty array with `X-Log-Status: unavailable` header

**Redis unavailable:**
- Log writes skip Redis (PostgreSQL only)
- API reads query PostgreSQL directly
- Slightly slower but functional

**WebSocket disconnected:**
- Logs still written to PostgreSQL/Redis
- Frontend shows "Real-time updates paused" indicator
- Auto-reconnect after 5 seconds
- On reconnect, frontend doesn't fetch missed logs (acceptable trade-off)

### Frontend Error Scenarios

**API request fails:**
- Show "Logs unavailable" message
- Display cached logs if any exist in memory
- Don't block UI

**WebSocket drops:**
- Show connection status indicator
- Continue displaying cached logs
- Auto-reconnect in background

**Bot deleted:**
- Logs remain in database for 24-hour retention
- Frontend clears logs from memory immediately
- Cleanup job removes from PostgreSQL after 24 hours

## Performance Considerations

### Write Performance

**Async logging:**
- All log writes happen in goroutines
- Don't block bot message sending loop
- Timeouts prevent hanging:
  - PostgreSQL: 2 seconds
  - Redis: 500 milliseconds

**Batch operations:**
- Redis uses pipelining (LPUSH + LTRIM + EXPIRE in one roundtrip)
- PostgreSQL uses prepared statements

### Read Performance

**Caching strategy:**
- Redis cache hit: ~5-10ms response time
- PostgreSQL query: ~50-100ms response time
- Frontend cache: instant (no network)

**Database indexing:**
- `(bot_id, created_at DESC)` - Fast bot log queries
- `(created_at DESC)` - Fast cleanup queries

### Memory Management

**Backend:**
- No in-memory log storage (stateless)
- Redis auto-evicts with TTL
- PostgreSQL cleanup job runs hourly

**Frontend:**
- Max 100 logs per bot in memory (~50KB per bot)
- Max 50 settings logs (~25KB)
- Auto-trim on new entries (FIFO)
- Total memory: ~500KB for 10 bots

## Cleanup and Retention

### PostgreSQL Cleanup

**Schedule:** Every 1 hour

**Logic:**
```sql
DELETE FROM bot_logs WHERE created_at < NOW() - INTERVAL '24 hours';
DELETE FROM settings_logs WHERE created_at < NOW() - INTERVAL '24 hours';
```

**Implementation:**
- Background goroutine in `main()`
- 30-second timeout per cleanup
- Logs number of rows deleted

### Redis TTL

**Automatic expiration:**
- Each key has 1-hour TTL
- Redis handles expiration automatically
- No manual cleanup needed

**Why 1 hour?**
- Balances memory usage vs cache hit rate
- Most users check logs within 1 hour
- Older logs fetched from PostgreSQL

### Frontend Cache

**Automatic trimming:**
- On each new log entry, check length
- If > limit, remove oldest entries
- Happens in Zustand store action

## UI/UX Design

### Bot Log Panel

**Location:** Below each bot card (expandable)

**Interaction:**
- Click "📋 ЛОГИ" button to toggle
- Auto-scroll to newest log
- Manual scroll for history
- "CLEAR" button to reset

**Visual design:**
- Color-coded by level
- Monospace font for messages
- Relative timestamps
- Alternating row backgrounds

### Settings Log Panel

**Location:** Bottom of Settings page

**Interaction:**
- Collapsed by default
- Click header to expand/collapse
- Shows last save timestamp in header
- "CLEAR" button when expanded

**Visual design:**
- Prominent border (accent color)
- Detailed change display
- Old → New value comparison
- Grouped by timestamp

## Testing Strategy

### Backend Tests

**Unit tests:**
- `storage/logs_test.go` - CRUD operations
- `cache/logs_test.go` - Redis operations
- Test cleanup logic
- Test fallback scenarios

**Integration tests:**
- End-to-end log flow (write → read)
- Redis failure fallback
- PostgreSQL failure handling
- WebSocket broadcast

### Frontend Tests

**Component tests:**
- BotLogPanel rendering
- SettingsLogPanel rendering
- Log store actions
- WebSocket message handling

**E2E tests:**
- Create bot → verify logs appear
- Change settings → verify log entry
- WebSocket disconnect → verify reconnect

## Deployment Considerations

### Database Migration

**Steps:**
1. Run schema migration (create tables + indexes)
2. Verify indexes created successfully
3. Test cleanup function
4. No data migration needed (new feature)

**Rollback:**
- Drop tables if needed
- No impact on existing functionality

### Redis Configuration

**Requirements:**
- Redis already in stack (no new dependency)
- Ensure sufficient memory for log cache
- Estimate: ~10MB for 100 bots with full cache

### Environment Variables

No new environment variables needed. Uses existing:
- `DATABASE_URL` - PostgreSQL connection
- `REDIS_URL` - Redis connection

### Monitoring

**Metrics to track:**
- Log write latency (PostgreSQL, Redis)
- Cache hit rate (Redis vs PostgreSQL)
- Cleanup job execution time
- WebSocket message rate

## Future Enhancements

**Out of scope for MVP, consider later:**

1. **Log filtering** - Filter by level, type, date range
2. **Log search** - Full-text search across messages
3. **Log export** - Download logs as CSV/JSON
4. **Pagination** - Load more logs on demand
5. **Log aggregation** - Statistics dashboard (errors per hour, etc.)
6. **Retention configuration** - User-configurable retention period
7. **Log alerts** - Notify on error threshold

## Success Metrics

**MVP is successful if:**

1. ✓ Bot logs appear in real-time under each bot card
2. ✓ Settings changes are logged with detailed information
3. ✓ Logs persist for 24 hours
4. ✓ API response time < 500ms (Redis cache hit)
5. ✓ No performance impact on bot message sending
6. ✓ Graceful degradation when Redis/PostgreSQL unavailable
7. ✓ Automatic cleanup prevents database bloat

## Implementation Phases

### Phase 1: Backend Foundation (Priority 1)
- Create database schema
- Implement storage layer (logs.go)
- Implement cache layer (logs.go)
- Add API endpoints
- Add cleanup job

### Phase 2: Bot Logging Integration (Priority 1)
- Modify logic.go to log events
- Extend WebSocket messages
- Test bot lifecycle logging

### Phase 3: Settings Logging (Priority 2)
- Modify UpdateSettings handler
- Implement change detection
- Test settings change logging

### Phase 4: Frontend Components (Priority 1)
- Create log store (Zustand)
- Build BotLogPanel component
- Build SettingsLogPanel component
- Integrate WebSocket handling

### Phase 5: UI Integration (Priority 2)
- Integrate BotLogPanel into Bots page
- Integrate SettingsLogPanel into Settings page
- Polish UI/UX

### Phase 6: Testing & Polish (Priority 3)
- Write tests
- Performance testing
- Error scenario testing
- Documentation

## Why This Design?

**Hybrid architecture chosen because:**
- Real-time updates critical for monitoring
- PostgreSQL provides reliable persistence
- Redis cache reduces database load
- Frontend cache enables instant UI updates
- Graceful degradation ensures reliability

**Trade-offs accepted:**
- Missed logs during WebSocket disconnect (acceptable for monitoring use case)
- 24-hour retention only (sufficient for debugging)
- No pagination in MVP (100 logs per bot is enough)
- No log search in MVP (can scroll through 100 entries)

**How to apply:**
This design balances real-time monitoring needs with performance and reliability. The three-tier caching strategy (frontend → Redis → PostgreSQL) ensures fast reads while maintaining data durability. Async writes prevent logging from impacting bot performance. The 24-hour retention with automatic cleanup keeps storage costs low while providing sufficient debugging history.

---

**Document Status:** Ready for implementation
**Last Updated:** 2026-04-27
**Author:** Claude (Kiro AI Assistant)
