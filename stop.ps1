$ErrorActionPreference = 'Stop'
Set-Location -LiteralPath $PSScriptRoot

Write-Host "==> Stopping containers" -ForegroundColor Cyan
docker compose down --remove-orphans
if ($LASTEXITCODE -ne 0) {
    Write-Host "ERROR: docker compose down failed." -ForegroundColor Red
    exit 1
}

Write-Host "==> Stopped" -ForegroundColor Green
Write-Host "    Data is kept in Docker volumes. To delete it too: docker compose down -v" -ForegroundColor DarkGray
