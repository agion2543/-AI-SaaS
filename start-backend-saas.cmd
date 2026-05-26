@echo off
setlocal
title SaaS Backend - Gin 8080
cd /d "%~dp0"

echo [SaaS] Starting backend on http://localhost:8080 ...
echo [SaaS] Log file: %~dp0backend-start.log
echo.

set APP_ENV=development

if exist "%~dp0saas-server.exe" (
  "%~dp0saas-server.exe" 2>&1 | powershell -NoProfile -Command "$input | Tee-Object -FilePath '%~dp0backend-start.log'"
) else (
  set GOCACHE=%~dp0.gocache
  "C:\Program Files\Go\bin\go.exe" run ./cmd/server 2>&1 | powershell -NoProfile -Command "$input | Tee-Object -FilePath '%~dp0backend-start.log'"
)

echo.
echo [SaaS] Backend stopped. Check backend-start.log for details.
pause
