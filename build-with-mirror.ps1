# Build TwitchSMM with Yandex Mirror (for RU/CIS regions)
# Run this script if docker compose build fails with DeadlineExceeded

Write-Host "Building TwitchSMM with Yandex Mirror..." -ForegroundColor Green
Write-Host "This uses mirror.yandex.ru to bypass Docker Hub restrictions" -ForegroundColor Yellow

# Set environment variable for this session
$env:BASE_IMAGE_PREFIX = "mirror.yandex.ru/mirrors/docker/library/"

# Navigate to project directory
Set-Location "C:\Users\blindfold\Desktop\2026\twitchSmm"

# Stop existing containers
Write-Host "Stopping existing containers..." -ForegroundColor Cyan
docker compose down

# Build with mirror prefix
Write-Host "Building with Yandex mirror..." -ForegroundColor Cyan
docker compose build --no-cache

# If build succeeds, start services
if ($LASTEXITCODE -eq 0) {
    Write-Host "Build successful! Starting services..." -ForegroundColor Green
    docker compose up -d
    
    Write-Host "Waiting for services to start..." -ForegroundColor Cyan
    Start-Sleep -Seconds 30
    
    Write-Host "Checking service status..." -ForegroundColor Cyan
    docker compose ps
    
    Write-Host "`nServices available at:" -ForegroundColor Green
    Write-Host "  Frontend: http://localhost:3000" -ForegroundColor White
    Write-Host "  Backend API: http://localhost:8080" -ForegroundColor White
    Write-Host "  STT Health: http://localhost:9000/health" -ForegroundColor White
} else {
    Write-Host "Build failed. Trying alternative approach..." -ForegroundColor Red
    
    # Alternative: manually pull images first
    Write-Host "Manually pulling images via Yandex mirror..." -ForegroundColor Yellow
    
    docker pull mirror.yandex.ru/mirrors/docker/library/ubuntu:22.04
    docker tag mirror.yandex.ru/mirrors/docker/library/ubuntu:22.04 ubuntu:22.04
    
    docker pull mirror.yandex.ru/mirrors/docker/library/node:20-alpine
    docker tag mirror.yandex.ru/mirrors/docker/library/node:20-alpine node:20-alpine
    
    docker pull mirror.yandex.ru/mirrors/docker/library/nginx:alpine
    docker tag mirror.yandex.ru/mirrors/docker/library/nginx:alpine nginx:alpine
    
    docker pull mirror.yandex.ru/mirrors/docker/library/python:3.12-slim
    docker tag mirror.yandex.ru/mirrors/docker/library/python:3.12-slim python:3.12-slim
    
    docker pull mirror.yandex.ru/mirrors/docker/library/postgres:16
    docker tag mirror.yandex.ru/mirrors/docker/library/postgres:16 postgres:16
    
    docker pull mirror.yandex.ru/mirrors/docker/library/redis:7-alpine
    docker tag mirror.yandex.ru/mirrors/docker/library/redis:7-alpine redis:7-alpine
    
    docker pull mirror.yandex.ru/mirrors/docker/library/dpage/pgadmin4:8
    docker tag mirror.yandex.ru/mirrors/docker/library/dpage/pgadmin4:8 dpage/pgadmin4:8
    
    Write-Host "Images pulled. Retrying build..." -ForegroundColor Green
    docker compose up -d --build
}
