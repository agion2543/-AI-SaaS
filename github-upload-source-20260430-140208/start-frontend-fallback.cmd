@echo off
cd /d "%~dp0web"
"C:\Program Files\nodejs\npm.cmd" run dev:fallback
