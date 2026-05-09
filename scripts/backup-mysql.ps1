param(
  [string]$MysqlBin = "C:\Program Files\MySQL\MySQL Server 8.4\bin\mysqldump.exe",
  [string]$Database = "saas_billing",
  [string]$User = "root",
  [string]$Password = "",
  [string]$HostName = "127.0.0.1",
  [string]$BackupDir = ".\backups"
)

$ErrorActionPreference = "Stop"

if (-not (Test-Path $MysqlBin)) {
  throw "mysqldump not found: $MysqlBin"
}

if (-not (Test-Path $BackupDir)) {
  New-Item -ItemType Directory -Path $BackupDir | Out-Null
}

$timestamp = Get-Date -Format "yyyyMMdd-HHmmss"
$output = Join-Path $BackupDir "$Database-$timestamp.sql"

$args = @(
  "-h$HostName",
  "-u$User",
  "--default-character-set=utf8mb4",
  "--single-transaction",
  "--routines",
  "--triggers",
  $Database
)

if ($Password -ne "") {
  $args = @("-p$Password") + $args
}

& $MysqlBin @args | Out-File -FilePath $output -Encoding utf8
Write-Host "Backup created: $output"
