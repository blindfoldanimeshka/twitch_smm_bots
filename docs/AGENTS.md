# TwitchSMM Project Instructions

## Project Overview
TwitchSMM is a Twitch bot management system with a Go backend and React frontend.

**Before changing code:** read `docs/CODE_GRAPH.md` (human/agent map) and
`docs/code-graph.json` (machine-readable nodes/edges). They name the responsible
files, key symbols, data flows, invariants, and common commands.

## Tech Stack
- **Backend**: Go 1.26+, PostgreSQL, Redis, Docker
- **Frontend**: React 19, TypeScript, Vite
- **LLM Integration**: OpenRouter API (free-first with paid fallback)
- **STT**: whisper.cpp sidecar with a local GGML model

## Project Structure
```
twitchSmm/
├── frontend/          # React frontend: light UI, four sections (Dashboard, Stream, Bots, Settings)
├── internal/          # Go backend code
│   ├── api/          # HTTP handlers
│   ├── llm/          # OpenRouter client and config resolution
│   ├── audio/        # stream capture and whisper.cpp transcription
│   ├── twitch/       # public GQL client, read-only IRC monitor, stream monitor
│   └── storage/      # Database layer
├── models/           # local whisper.cpp models (gitignored)
├── db/               # Database migrations
└── docker-compose.yml
```

### Frontend layout
- `src/App.tsx` — app shell (Sidebar + Topbar), four pages, WebSocketProvider, ErrorBoundary
- `src/pages/` — `Dashboard`, `Stream`, `Bots`, `Settings` (the only sections; no routing library)
- `src/components/layout/` — `Sidebar`, `Topbar`, `PageHeader`
- `src/components/ui/` — design primitives (`Card`, `Button`, `Input`, `Modal`, `Badge`, `Alert`, ...)
- `src/api/client.ts` — axios client, API types, `X-API-Key`
- `src/hooks/useWebSocket.ts` — single WS connection, reconnect, log store
- Legacy visual components (Dock, PixelSnow, TargetCursor, GooeyNav, FloatingPanel, StatusBar, ...) were removed; do not reintroduce them.

## Development Guidelines

### Code Style
- Use clear, descriptive variable names
- Keep functions focused and small
- Add comments only when necessary (explain WHY, not WHAT)
- Follow Go and TypeScript best practices

### Git Workflow
- Work on `main`
- Write clear commit messages

### Testing
- Test backend changes with `go test ./...`
- Test frontend changes in browser
- Verify Docker builds before committing

## Key Features
1. Multi-bot management (table, add/edit modal, per-bot logs)
2. LLM-powered responses via OpenRouter (free-first, paid fallback)
3. Stream monitoring: public Twitch GraphQL for status/viewers + anonymous read-only IRC monitor for active viewers and the chat feed. The public GQL endpoint is unofficial and undocumented — it may change or get blocked; the UI then shows "no data". Plan a later migration to Helix with a Client ID.
4. Presence simulation: random session/cooldown ranges with greeting and farewell messages. It only connects to IRC and sends chat messages; it never inflates `viewer_count` and is not viewbotting.
5. Settings management
6. Four-section light UI: Dashboard, Stream, Bots, Settings

Channel rotation was removed from UI and runtime; `rotation_enabled`/`rotation_interval` stay in the DB for compatibility but are unused.

## Important Notes
- `OPENROUTER_API_KEY` is optional at startup; without it the API reports LLM unavailable
- Local STT needs `models/ggml-small-q5_1.bin` before `docker compose --profile audio up -d`
- PostgreSQL runs in Docker container
- Frontend dev server: http://localhost:5173
- Backend API: http://localhost:8080

## Common Commands
```bash
# Start services
docker-compose up -d

# Run backend
go run .

# Run frontend
cd frontend && npm run dev

# Run tests
go test ./...
```
