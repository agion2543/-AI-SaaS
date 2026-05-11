@echo off
cd /d "%~dp0"
"C:\Program Files\MySQL\MySQL Server 8.4\bin\mysqld.exe" --standalone --console --basedir="C:\Program Files\MySQL\MySQL Server 8.4" --datadir="%~dp0mysql-data" --port=3306 --bind-address=127.0.0.1
