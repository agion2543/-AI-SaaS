@echo off
setlocal
cd /d "%~dp0"

echo [1/3] Check port 3306...
netstat -ano | findstr ":3306" | findstr "LISTENING" >nul
if errorlevel 1 (
  echo [2/3] Starting MySQL...
  start "mysql-3306" "C:\Program Files\MySQL\MySQL Server 8.4\bin\mysqld.exe" --standalone --console --basedir="C:\Program Files\MySQL\MySQL Server 8.4" --datadir="%~dp0mysql-data" --port=3306 --bind-address=127.0.0.1
  timeout /t 5 /nobreak >nul
) else (
  echo [2/3] MySQL is already running.
)

echo [3/3] Check port 8080...
netstat -ano | findstr ":8080" | findstr "LISTENING" >nul
if not errorlevel 1 (
  echo.
  echo Port 8080 is already in use.
  echo Close the old Go backend window or stop the old process, then run this script again.
  echo.
  pause
  exit /b 1
)

if exist "%~dp0saas-server.exe" (
  echo Starting compiled backend...
  start "saas-server" /b "%~dp0saas-server.exe"
) else (
  echo saas-server.exe not found, falling back to go run...
  set GOCACHE=%~dp0.gocache
  "C:\Program Files\Go\bin\go.exe" run ./cmd/server
  exit /b %errorlevel%
)

timeout /t 2 /nobreak >nul
echo Backend start command sent.
echo If you need logs, run saas-server.exe or go run ./cmd/server directly.
pause
