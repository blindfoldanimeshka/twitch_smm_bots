# TwitchSMM Project Instructions

## Project Overview
TwitchSMM is a Twitch bot management system with a Go backend and React frontend.

## Tech Stack
- **Backend**: Go 1.23+, PostgreSQL, Docker
- **Frontend**: React 18, TypeScript, Vite
- **LLM Integration**: LM Studio API for local AI responses

## Project Structure
```
twitchSmm/
├── frontend/          # React frontend
├── internal/          # Go backend code
│   ├── api/          # HTTP handlers
│   ├── bot/          # Twitch bot logic
│   ├── llm/          # LLM integration
│   └── storage/      # Database layer
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
- Work on `dev` branch for development
- Merge to `main` only for releases
- Write clear commit messages

### Testing
- Test backend changes with `go test ./...`
- Test frontend changes in browser
- Verify Docker builds before committing

## Key Features
1. Multi-bot management
2. LLM-powered responses via LM Studio
3. Real-time chat monitoring
4. Settings management
5. Statistics dashboard

## Important Notes
- LM Studio must be running on localhost:1234 for LLM features
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
