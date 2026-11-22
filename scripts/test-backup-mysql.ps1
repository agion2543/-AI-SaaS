$ErrorActionPreference = "Stop"

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$backupScript = Join-Path $scriptDir "backup-mysql.ps1"
$testRoot = Join-Path ([System.IO.Path]::GetTempPath()) ("cue-backup-test-" + [Guid]::NewGuid().ToString("N"))
$mockBin = Join-Path $testRoot "mock-bin"
$syntheticPassword = "synthetic-only-password-do-not-use"
$syntheticDbPassword = "synthetic-inherited-DB_PASSWORD-do-not-use"
$script:backupTestTimestamp = "20261120-120000"
$originalMysqlPwd = [Environment]::GetEnvironmentVariable("MYSQL_PWD", "Process")
$originalDbPassword = [Environment]::GetEnvironmentVariable("DB_PASSWORD", "Process")
$script:recipientsFile = Join-Path $testRoot "synthetic-recipients.txt"

# Shadow Get-Date while the backup script runs so its destination is deterministic.
function Get-Date([string]$Format) {
  if ($Format -eq "yyyyMMdd-HHmmss") { return "20261120-120000" }
  throw "Unexpected Get-Date format in backup test: $Format"
}

function Assert-True([bool]$Condition, [string]$Message) {
  if (-not $Condition) { throw "FAIL: $Message" }
}

function Assert-NoTemporaryFiles([string]$Directory) {
  $leftovers = @(Get-ChildItem -LiteralPath $Directory -Force | Where-Object { $_.Name -like ".*.tmpdir" })
  Assert-True ($leftovers.Count -eq 0) "temporary directory was left in $Directory"
}

function Invoke-MockedBackup([string]$DumpMode, [string]$AgeMode, [string]$Directory, [string]$ArgsFile, [string]$AgeArgsFile, [string]$PasswordFile, [string]$Password) {
  $env:MOCK_DUMP_MODE = $DumpMode
  $env:MOCK_AGE_MODE = $AgeMode
  $env:MOCK_ARGS_FILE = $ArgsFile
  $env:MOCK_AGE_ARGS_FILE = $AgeArgsFile
  $env:MOCK_PASSWORD_FILE = $PasswordFile
  $env:AGE_RECIPIENTS_FILE = $script:recipientsFile
  [Environment]::SetEnvironmentVariable("DB_PASSWORD", $script:syntheticDbPassword, "Process")
  [Environment]::SetEnvironmentVariable("MYSQL_PWD", $Password, "Process")
  try {
    & $backupScript `
      -MysqlBin $script:mockDump `
      -AgeBin $script:mockAge `
      -AgeRecipientsFile $script:recipientsFile `
      -Database "synthetic_test_db" `
      -User "synthetic_test_user" `
      -HostName "127.0.0.1" `
      -BackupDir $Directory
  } finally {
    Remove-Item Env:MOCK_DUMP_MODE, Env:MOCK_AGE_MODE, Env:MOCK_ARGS_FILE, Env:MOCK_AGE_ARGS_FILE, Env:MOCK_PASSWORD_FILE, Env:AGE_RECIPIENTS_FILE -ErrorAction SilentlyContinue
    Assert-True ([Environment]::GetEnvironmentVariable("MYSQL_PWD", "Process") -eq $Password) "MYSQL_PWD was not restored after the backup"
    Assert-True ([Environment]::GetEnvironmentVariable("DB_PASSWORD", "Process") -eq $script:syntheticDbPassword) "DB_PASSWORD was not restored after the backup"
  }
}

try {
  New-Item -ItemType Directory -Path $mockBin -Force | Out-Null
  Set-Content -LiteralPath $script:recipientsFile -Value "age1syntheticrecipientnotarealkey" -Encoding ASCII

  if ($env:OS -eq "Windows_NT") {
    $script:mockDump = Join-Path $mockBin "mysqldump.cmd"
    @'
@echo off
if defined MOCK_ARGS_FILE echo %*>"%MOCK_ARGS_FILE%"
if defined MOCK_PASSWORD_FILE echo %MYSQL_PWD%>"%MOCK_PASSWORD_FILE%"
if "%MOCK_DUMP_MODE%"=="empty" exit /b 0
if "%MOCK_DUMP_MODE%"=="partial-failure" (
  echo -- partial synthetic output
  exit /b 42
)
echo -- synthetic isolated backup fixture
echo CREATE TABLE `backup_probe` (`id` int NOT NULL);
echo INSERT INTO `backup_probe` VALUES (1);
exit /b 0
'@ | Set-Content -LiteralPath $script:mockDump -Encoding ASCII

    $script:mockAge = Join-Path $mockBin "age.cmd"
    @'
@echo off
if not "%~1"=="-R" exit /b 65
if not "%~2"=="%AGE_RECIPIENTS_FILE%" exit /b 66
if not "%~3"=="-o" exit /b 67
set "AGE_OUT=%~4"
set "AGE_IN=%~5"
if defined MOCK_AGE_ARGS_FILE echo %*>"%MOCK_AGE_ARGS_FILE%"
if defined MOCK_AGE_ARGS_FILE if defined MYSQL_PWD echo MYSQL_PWD=SET>>"%MOCK_AGE_ARGS_FILE%"
if defined MOCK_AGE_ARGS_FILE if not defined MYSQL_PWD echo MYSQL_PWD=UNSET>>"%MOCK_AGE_ARGS_FILE%"
if defined MOCK_AGE_ARGS_FILE if defined DB_PASSWORD echo DB_PASSWORD=SET>>"%MOCK_AGE_ARGS_FILE%"
if defined MOCK_AGE_ARGS_FILE if not defined DB_PASSWORD echo DB_PASSWORD=UNSET>>"%MOCK_AGE_ARGS_FILE%"
if defined MYSQL_PWD exit /b 69
if defined DB_PASSWORD exit /b 70
if not exist "%AGE_IN%" exit /b 68
if "%MOCK_AGE_MODE%"=="empty" exit /b 0
if "%MOCK_AGE_MODE%"=="partial-failure" (
  echo partial synthetic encrypted output>"%AGE_OUT%"
  exit /b 43
)
echo SYNTHETIC-AGE-MOCK-NOT-REAL-ENCRYPTION>"%AGE_OUT%"
exit /b 0
'@ | Set-Content -LiteralPath $script:mockAge -Encoding ASCII
  } else {
    $script:mockDump = Join-Path $mockBin "mysqldump"
    @'
#!/bin/sh
set -eu
if [ -n "${MOCK_ARGS_FILE-}" ]; then printf '%s\n' "$@" > "$MOCK_ARGS_FILE"; fi
if [ -n "${MOCK_PASSWORD_FILE-}" ]; then printf '%s' "${MYSQL_PWD-}" > "$MOCK_PASSWORD_FILE"; fi
case "${MOCK_DUMP_MODE-}" in
  empty) exit 0 ;;
  partial-failure) echo '-- partial synthetic output'; exit 42 ;;
  success)
    echo '-- synthetic isolated backup fixture'
    echo 'CREATE TABLE `backup_probe` (`id` int NOT NULL);'
    echo 'INSERT INTO `backup_probe` VALUES (1);'
    exit 0 ;;
  *) echo 'unknown mock mode' >&2; exit 90 ;;
esac
'@ | Set-Content -LiteralPath $script:mockDump -Encoding utf8
    $mode = [System.IO.UnixFileMode]::UserRead -bor [System.IO.UnixFileMode]::UserWrite -bor [System.IO.UnixFileMode]::UserExecute
    [System.IO.File]::SetUnixFileMode($script:mockDump, $mode)

    $script:mockAge = Join-Path $mockBin "age"
    @'
#!/bin/sh
set -eu
[ "$#" -eq 5 ] && [ "$1" = '-R' ] && [ "$2" = "$AGE_RECIPIENTS_FILE" ] && [ "$3" = '-o' ] || exit 65
out="$4"
input="$5"
[ -s "$input" ] || exit 66
if [ -n "${MOCK_AGE_ARGS_FILE-}" ]; then
  printf '%s\n' "$@" > "$MOCK_AGE_ARGS_FILE"
  printf 'MYSQL_PWD=%s\n' "${MYSQL_PWD-UNSET}" >> "$MOCK_AGE_ARGS_FILE"
  printf 'DB_PASSWORD=%s\n' "${DB_PASSWORD-UNSET}" >> "$MOCK_AGE_ARGS_FILE"
fi
[ -z "${MYSQL_PWD+x}" ] || { echo 'age mock received MYSQL_PWD' >&2; exit 69; }
[ -z "${DB_PASSWORD+x}" ] || { echo 'age mock received DB_PASSWORD' >&2; exit 70; }
case "${MOCK_AGE_MODE-}" in
  empty) exit 0 ;;
  partial-failure) printf '%s\n' 'partial synthetic encrypted output' > "$out"; exit 43 ;;
  success) printf '%s\n' 'SYNTHETIC-AGE-MOCK-NOT-REAL-ENCRYPTION' > "$out"; exit 0 ;;
  *) echo 'unknown age mock mode' >&2; exit 91 ;;
esac
'@ | Set-Content -LiteralPath $script:mockAge -Encoding utf8
    [System.IO.File]::SetUnixFileMode($script:mockAge, $mode)
  }

  # Success: the age mock publishes a marker, not the synthetic SQL plaintext.
  $successDir = Join-Path $testRoot "success path with spaces"
  New-Item -ItemType Directory -Path $successDir -Force | Out-Null
  $argsFile = Join-Path $testRoot "mysqldump-args.txt"
  $ageArgsFile = Join-Path $testRoot "age-args.txt"
  $passwordFile = Join-Path $testRoot "mock-password.txt"
  Invoke-MockedBackup "success" "success" $successDir $argsFile $ageArgsFile $passwordFile $syntheticPassword
  $successFile = Join-Path $successDir "synthetic_test_db-$script:backupTestTimestamp.sql.age"
  Assert-True (Test-Path -LiteralPath $successFile) "successful encrypted dump was not published"
  Assert-True ((Get-Item -LiteralPath $successFile).Length -gt 0) "successful encrypted dump was empty"
  Assert-True ((Get-Content -LiteralPath $successFile -Raw) -match "SYNTHETIC-AGE-MOCK-NOT-REAL-ENCRYPTION") "published file did not contain the mock marker"
  Assert-True (-not ((Get-Content -LiteralPath $successFile -Raw).Contains("CREATE TABLE"))) "plaintext SQL appeared in the published artifact"
  Assert-True ((Get-Content -LiteralPath $passwordFile -Raw).Trim() -eq $syntheticPassword) "mysqldump mock did not receive the synthetic password"
  Assert-True (-not ((Get-Content -LiteralPath $argsFile -Raw).Contains($syntheticPassword))) "password appeared in mysqldump arguments"
  $ageArgs = Get-Content -LiteralPath $ageArgsFile -Raw
  Assert-True (-not $ageArgs.Contains($syntheticPassword)) "password appeared in age arguments"
  Assert-True (-not $ageArgs.Contains($syntheticDbPassword)) "DB_PASSWORD appeared in age arguments"
  Assert-True ($ageArgs -match "MYSQL_PWD=UNSET") "age process inherited MYSQL_PWD"
  Assert-True ($ageArgs -match "DB_PASSWORD=UNSET") "age process inherited DB_PASSWORD"
  Assert-NoTemporaryFiles $successDir

  # Empty dump, failed dump, failed encryption and empty encryption all fail closed.
  foreach ($case in @(
    @{ Name = "empty-dump"; DumpMode = "empty"; AgeMode = "success" },
    @{ Name = "partial-dump"; DumpMode = "partial-failure"; AgeMode = "success" },
    @{ Name = "age-failure"; DumpMode = "success"; AgeMode = "partial-failure" },
    @{ Name = "age-empty"; DumpMode = "success"; AgeMode = "empty" }
  )) {
    $failureDir = Join-Path $testRoot $case.Name
    New-Item -ItemType Directory -Path $failureDir -Force | Out-Null
    $failed = $false
    try {
      Invoke-MockedBackup $case.DumpMode $case.AgeMode $failureDir $argsFile $ageArgsFile $passwordFile $syntheticPassword
    } catch {
      $failed = $true
    }
    Assert-True $failed "$($case.Name) unexpectedly succeeded"
    Assert-True (@(Get-ChildItem -LiteralPath $failureDir -Force).Count -eq 0) "$($case.Name) left a published or temporary file"
  }

  # A pre-existing destination must remain unchanged.
  $collisionDir = Join-Path $testRoot "collision"
  New-Item -ItemType Directory -Path $collisionDir -Force | Out-Null
  $collisionFile = Join-Path $collisionDir "synthetic_test_db-$script:backupTestTimestamp.sql.age"
  Set-Content -LiteralPath $collisionFile -Value "preserve existing synthetic encrypted file" -NoNewline
  $failed = $false
  try {
    Invoke-MockedBackup "success" "success" $collisionDir $argsFile $ageArgsFile $passwordFile $syntheticPassword
  } catch {
    $failed = $true
  }
  Assert-True $failed "existing destination was unexpectedly overwritten"
  Assert-True ((Get-Content -LiteralPath $collisionFile -Raw) -eq "preserve existing synthetic encrypted file") "existing destination changed"
  Assert-NoTemporaryFiles $collisionDir

  Write-Output "PASS: synthetic PowerShell mysqldump/age flow, password isolation, failure cleanup and no-overwrite checks"
} finally {
  [Environment]::SetEnvironmentVariable("MYSQL_PWD", $originalMysqlPwd, "Process")
  [Environment]::SetEnvironmentVariable("DB_PASSWORD", $originalDbPassword, "Process")
  if (Test-Path -LiteralPath $testRoot) {
    Remove-Item -LiteralPath $testRoot -Recurse -Force
  }
}
