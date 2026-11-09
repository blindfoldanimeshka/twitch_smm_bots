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
├── frontend/          # React frontend
├── internal/          # Go backend code
│   ├── api/          # HTTP handlers
│   ├── llm/          # OpenRouter client and config resolution
│   ├── audio/        # stream capture and whisper.cpp transcription
│   └── storage/      # Database layer
├── models/           # local whisper.cpp models (gitignored)
├── db/               # Database migrations
└── docker-compose.yml
```

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
1. Multi-bot management
2. LLM-powered responses via OpenRouter (free-first, paid fallback)
3. Real-time chat monitoring
4. Settings management
5. Statistics dashboard

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
