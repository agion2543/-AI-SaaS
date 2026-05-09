@echo off
set GOCACHE=%~dp0.gocache
cd /d "%~dp0"
"C:\Program Files\Go\bin\go.exe" run ./cmd/server
