@echo off
setlocal
title SaaS One Click Launcher
cd /d "%~dp0"

echo ========================================
echo  SaaS One Click Launcher
echo ========================================
echo.
echo This launcher will start:
echo   MySQL   : 127.0.0.1:3306
echo   Backend : http://localhost:8080
echo   Web     : http://localhost:5173
echo.

echo [0/4] Cleaning old local backend/frontend ports ...
powershell -NoProfile -ExecutionPolicy Bypass -Command ^
  "$ports=@(8080,5173); foreach($port in $ports){ $pids=(Get-NetTCPConnection -LocalPort $port -ErrorAction SilentlyContinue | Select-Object -ExpandProperty OwningProcess -Unique); foreach($processId in $pids){ if($processId -and $processId -ne $PID){ try{ Stop-Process -Id $processId -Force -ErrorAction Stop; Write-Host ('Stopped old local process on port ' + $port + ': ' + $processId) } catch{} } } }"

echo.
echo [1/4] Checking MySQL ...
powershell -NoProfile -ExecutionPolicy Bypass -Command ^
  "if(Test-NetConnection 127.0.0.1 -Port 3306 -InformationLevel Quiet){ Write-Host 'MySQL already listening on 3306.' } else { Start-Process -FilePath '%~dp0start-mysql-saas.cmd' -WorkingDirectory '%~dp0' -WindowStyle Normal; Write-Host 'MySQL window opened.' }"
timeout /t 8 /nobreak >nul

echo.
echo [2/4] Starting backend ...
start "SaaS Backend 8080" "%~dp0start-backend-saas.cmd"
timeout /t 8 /nobreak >nul

echo.
echo [3/4] Starting frontend ...
start "SaaS Frontend 5173" "%~dp0start-frontend-saas.cmd"
timeout /t 8 /nobreak >nul

echo.
echo [4/4] Health check ...
powershell -NoProfile -ExecutionPolicy Bypass -Command ^
  "$backend=$false; $frontend=$false; try{ $r=Invoke-RestMethod -Uri 'http://localhost:8080/health' -TimeoutSec 5; $backend=($r.code -eq 0) }catch{}; try{ $res=Invoke-WebRequest -Uri 'http://localhost:5173' -UseBasicParsing -TimeoutSec 5; $frontend=($res.StatusCode -eq 200) }catch{}; Write-Host ('Backend 8080: ' + $(if($backend){'OK'}else{'FAILED'})); Write-Host ('Frontend 5173: ' + $(if($frontend){'OK'}else{'FAILED'})); if(-not $backend){ Write-Host 'Check backend-start.log or the Backend window.' -ForegroundColor Yellow }; if(-not $frontend){ Write-Host 'Check web\vite-dev.log or the Frontend window.' -ForegroundColor Yellow }"

echo.
echo ========================================
echo  Preview URLs
echo ========================================
echo  Admin    : http://localhost:5173/admin/login
echo  Merchant : http://localhost:5173/merchant/login
echo  Customer : http://localhost:5173/customer/store/5
echo  Backend  : http://localhost:8080/health
echo.
echo Keep the Backend and Frontend windows open while previewing.
echo If any health check failed, send me backend-start.log or the visible error.
echo.
pause
