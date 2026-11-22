param(
  [string]$MysqlBin = "C:\Program Files\MySQL\MySQL Server 8.4\bin\mysqldump.exe",
  [string]$Database = "saas_billing",
  [string]$User = "root",
  [string]$HostName = "127.0.0.1",
  [string]$BackupDir = ".\backups",
  [string]$AgeBin = "age.exe",
  [string]$AgeRecipientsFile = $env:AGE_RECIPIENTS_FILE
)

$ErrorActionPreference = "Stop"

if (-not (Test-Path -LiteralPath $MysqlBin -PathType Leaf)) {
  throw "mysqldump not found: $MysqlBin"
}
$ageCommand = Get-Command -Name $AgeBin -CommandType Application -ErrorAction SilentlyContinue
if ($null -eq $ageCommand) {
  throw "age executable not found: $AgeBin"
}
$ageExecutable = $ageCommand.Source
if ([string]::IsNullOrWhiteSpace($AgeRecipientsFile) -or
    -not (Test-Path -LiteralPath $AgeRecipientsFile -PathType Leaf) -or
    (Get-Item -LiteralPath $AgeRecipientsFile).Length -eq 0) {
  throw "AgeRecipientsFile must name a non-empty age recipients file"
}

if (-not (Test-Path -LiteralPath $BackupDir -PathType Container)) {
  New-Item -ItemType Directory -Path $BackupDir | Out-Null
}

$timestamp = Get-Date -Format "yyyyMMdd-HHmmss"
$output = Join-Path $BackupDir "$Database-$timestamp.sql.age"
if (Test-Path -LiteralPath $output) {
  throw "Refusing to overwrite existing backup: $output"
}

# Age's -o overwrites an existing path. Give it a fresh, private temporary
# directory so it cannot target an existing backup or unrelated file.
$temporaryDirectory = Join-Path $BackupDir ".$Database-$timestamp-$([Guid]::NewGuid().ToString('N')).tmpdir"
if (Test-Path -LiteralPath $temporaryDirectory) {
  throw "Refusing to reuse existing temporary directory: $temporaryDirectory"
}
$temporarySql = Join-Path $temporaryDirectory "$Database-$timestamp.sql"
$temporaryEncrypted = Join-Path $temporaryDirectory "$Database-$timestamp.sql.age"
$previousMysqlPwd = [Environment]::GetEnvironmentVariable("MYSQL_PWD", "Process")
$previousDbPassword = [Environment]::GetEnvironmentVariable("DB_PASSWORD", "Process")
$databasePassword = $previousMysqlPwd
$tempDirectoryCreated = $false

try {
  if ($env:OS -eq "Windows_NT") {
    $currentSid = [System.Security.Principal.WindowsIdentity]::GetCurrent().User
    $directorySecurity = New-Object System.Security.AccessControl.DirectorySecurity
    $directorySecurity.SetAccessRuleProtection($true, $false)
    $directorySecurity.SetOwner($currentSid)
    $inheritance = [System.Security.AccessControl.InheritanceFlags]::ContainerInherit -bor [System.Security.AccessControl.InheritanceFlags]::ObjectInherit
    $accessRule = [System.Security.AccessControl.FileSystemAccessRule]::new(
      $currentSid,
      [System.Security.AccessControl.FileSystemRights]::FullControl,
      $inheritance,
      [System.Security.AccessControl.PropagationFlags]::None,
      [System.Security.AccessControl.AccessControlType]::Allow
    )
    $directorySecurity.AddAccessRule($accessRule)
    [System.IO.Directory]::CreateDirectory($temporaryDirectory, $directorySecurity) | Out-Null
    $tempDirectoryCreated = $true
  } else {
    [System.IO.Directory]::CreateDirectory($temporaryDirectory) | Out-Null
    $tempDirectoryCreated = $true
    $setUnixFileModeMethod = [System.IO.File].GetMethod(
      "SetUnixFileMode",
      [type[]]@([string], [System.IO.UnixFileMode])
    )
    if ($null -ne $setUnixFileModeMethod) {
      $privateMode = [System.IO.UnixFileMode]::UserRead -bor [System.IO.UnixFileMode]::UserWrite -bor [System.IO.UnixFileMode]::UserExecute
      [System.IO.File]::SetUnixFileMode($temporaryDirectory, $privateMode)
    }
  }

  try {
    if ([string]::IsNullOrEmpty($databasePassword)) {
      [Environment]::SetEnvironmentVariable("MYSQL_PWD", $null, "Process")
    } else {
      [Environment]::SetEnvironmentVariable("MYSQL_PWD", $databasePassword, "Process")
    }

    $dumpArgs = @(
      "-h$HostName",
      "-u$User",
      "--default-character-set=utf8mb4",
      "--single-transaction",
      "--routines",
      "--triggers",
      $Database
    )
    & $MysqlBin @dumpArgs | Out-File -FilePath $temporarySql -Encoding utf8
    $dumpExitCode = $LASTEXITCODE
    if ($dumpExitCode -ne 0) {
      throw "mysqldump failed with exit code $dumpExitCode"
    }
  } finally {
    # The age process does not need database credentials.
    [Environment]::SetEnvironmentVariable("MYSQL_PWD", $null, "Process")
  }

  if (-not (Test-Path -LiteralPath $temporarySql -PathType Leaf) -or (Get-Item -LiteralPath $temporarySql).Length -eq 0) {
    throw "Refusing to encrypt an empty database dump: $output"
  }

  $ageMysqlPwd = [Environment]::GetEnvironmentVariable("MYSQL_PWD", "Process")
  $ageDbPassword = [Environment]::GetEnvironmentVariable("DB_PASSWORD", "Process")
  try {
    [Environment]::SetEnvironmentVariable("MYSQL_PWD", $null, "Process")
    [Environment]::SetEnvironmentVariable("DB_PASSWORD", $null, "Process")
    & $ageExecutable -R $AgeRecipientsFile -o $temporaryEncrypted $temporarySql
    $ageExitCode = $LASTEXITCODE
  } finally {
    [Environment]::SetEnvironmentVariable("MYSQL_PWD", $ageMysqlPwd, "Process")
    [Environment]::SetEnvironmentVariable("DB_PASSWORD", $ageDbPassword, "Process")
  }
  if ($ageExitCode -ne 0) {
    throw "age encryption failed with exit code $ageExitCode"
  }
  if (-not (Test-Path -LiteralPath $temporaryEncrypted -PathType Leaf) -or (Get-Item -LiteralPath $temporaryEncrypted).Length -eq 0) {
    throw "Refusing to publish an empty encrypted backup: $output"
  }

  # Delete the plaintext staging file before publishing the encrypted artifact.
  Remove-Item -LiteralPath $temporarySql -Force
  [System.IO.File]::Move($temporaryEncrypted, $output)
} finally {
  [Environment]::SetEnvironmentVariable("MYSQL_PWD", $previousMysqlPwd, "Process")
  [Environment]::SetEnvironmentVariable("DB_PASSWORD", $previousDbPassword, "Process")
  if ($tempDirectoryCreated -and (Test-Path -LiteralPath $temporaryDirectory -PathType Container)) {
    Remove-Item -LiteralPath $temporaryDirectory -Recurse -Force
  }
}

Write-Host "Encrypted backup created: $output"
