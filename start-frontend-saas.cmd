@echo off
setlocal
title SaaS Frontend - Vite 5173
cd /d "%~dp0web"

echo [SaaS] Starting frontend on http://localhost:5173 ...
echo [SaaS] Log file: %~dp0web\vite-dev.log
echo.

npm.cmd run dev -- --host 0.0.0.0 2>&1 | powershell -NoProfile -Command "$input | Tee-Object -FilePath '%~dp0web\vite-dev.log'"

echo.
echo [SaaS] Frontend stopped. Check web\vite-dev.log for details.
pause
