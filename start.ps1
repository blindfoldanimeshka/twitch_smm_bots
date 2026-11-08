$ErrorActionPreference = 'Stop'
Set-Location -LiteralPath $PSScriptRoot

function Write-Step([string]$Message) {
    Write-Host "==> $Message" -ForegroundColor Cyan
}

function Fail([string]$Message) {
    Write-Host ""
    Write-Host "ERROR: $Message" -ForegroundColor Red
    exit 1
}

if (-not (Get-Command docker -ErrorAction SilentlyContinue)) {
    Fail "Docker is not installed. Install Docker Desktop first: https://www.docker.com/products/docker-desktop/"
}

Write-Step "Checking Docker engine"
docker info *> $null
if ($LASTEXITCODE -ne 0) {
    Write-Step "Docker engine is not running, starting Docker Desktop"
    $candidates = @(
        (Join-Path $env:ProgramFiles 'Docker\Docker\Docker Desktop.exe'),
        (Join-Path $env:LOCALAPPDATA 'Programs\DockerDesktop\Docker Desktop.exe')
    )
    $dockerDesktop = $candidates | Where-Object { Test-Path -LiteralPath $_ } | Select-Object -First 1
    if (-not $dockerDesktop) {
        Fail "Docker engine is not running and Docker Desktop was not found. Start Docker manually and re-run this script."
    }

    Start-Process -FilePath $dockerDesktop | Out-Null
    $deadline = (Get-Date).AddMinutes(5)
    $ready = $false
    do {
        Start-Sleep -Seconds 3
        docker info *> $null
        $ready = $LASTEXITCODE -eq 0
    } while (-not $ready -and (Get-Date) -lt $deadline)

    if (-not $ready) {
        Fail "Docker engine did not become ready within 5 minutes."
    }
}

Write-Step "Building and starting containers"
Write-Host "    First run downloads images and dependencies and can take 5-10 minutes." -ForegroundColor DarkGray
docker compose up -d --build --remove-orphans --wait
if ($LASTEXITCODE -ne 0) {
    Write-Host ""
    docker compose ps
    Write-Host ""
    docker compose logs --tail=80
    Fail "docker compose up failed. See the log above."
}

Write-Step "Checking API"
$apiKey = 'secret-api-key-12345'
$envFile = Join-Path $PSScriptRoot '.env'
if (Test-Path -LiteralPath $envFile) {
    $line = Get-Content -LiteralPath $envFile | Where-Object { $_ -match '^\s*API_KEY\s*=' } | Select-Object -First 1
    if ($line) {
        $apiKey = ($line -split '=', 2)[1].Trim()
    }
}

try {
    $response = Invoke-RestMethod -Uri 'http://localhost:8080/api/health' -Headers @{ 'X-API-Key' = $apiKey } -TimeoutSec 10
    Write-Host "    API health: $($response.status)" -ForegroundColor DarkGray
} catch {
    Write-Host "    API health check failed: $($_.Exception.Message)" -ForegroundColor Yellow
}

Write-Step "Ready"
Write-Host "    UI:      http://localhost:3000"
Write-Host "    API:     http://localhost:8080/api/health"
Write-Host "    API key: $apiKey"
Write-Host ""
Write-Host "    Stop:    stop.cmd" -ForegroundColor DarkGray
Write-Host "    Logs:    docker compose logs -f backend" -ForegroundColor DarkGray
Write-Host "    Audio:   docker compose --profile audio up -d --build" -ForegroundColor DarkGray
Write-Host ""

Start-Process 'http://localhost:3000'
