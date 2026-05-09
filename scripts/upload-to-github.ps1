param(
  [string]$Repo = "agion2543/-AI-SaaS",
  [string]$Branch = "main"
)

$ErrorActionPreference = "Stop"
$root = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$tempDir = Join-Path $root ".github-upload-tmp"

if (Test-Path $tempDir) {
  Remove-Item -LiteralPath $tempDir -Recurse -Force
}
New-Item -ItemType Directory -Path $tempDir | Out-Null

function Should-SkipFile {
  param([System.IO.FileInfo]$File)

  $relative = $File.FullName.Substring($root.Length + 1).Replace("\", "/")
  $skipPrefixes = @(
    ".git/",
    ".gocache/",
    ".github-upload-tmp/",
    "mysql-data/",
    "mysql-run/",
    "web/node_modules/",
    "web/dist/"
  )
  foreach ($prefix in $skipPrefixes) {
    if ($relative.StartsWith($prefix, [StringComparison]::OrdinalIgnoreCase)) {
      return $true
    }
  }

  if ($File.Name -eq ".env") { return $true }
  if ($File.Name -like "github-upload-source-*.zip") { return $true }
  if ($File.Name -like "*.exe") { return $true }
  if ($File.Name -like "*.exe~") { return $true }
  if ($File.Name -like "*.log") { return $true }
  if ($File.Name -like "*.err.log") { return $true }
  return $false
}

function Invoke-GhJson {
  param(
    [string[]]$Arguments,
    [object]$Body = $null
  )

  $inputFile = $null
  if ($null -ne $Body) {
    $inputFile = Join-Path $tempDir ("input-" + [Guid]::NewGuid().ToString("N") + ".json")
    $json = $Body | ConvertTo-Json -Depth 100
    $utf8NoBom = New-Object System.Text.UTF8Encoding($false)
    [System.IO.File]::WriteAllText($inputFile, $json, $utf8NoBom)
    $Arguments = @($Arguments + @("--input", $inputFile))
  }

  $output = & gh @Arguments
  if ($LASTEXITCODE -ne 0) {
    throw "gh api failed: gh $($Arguments -join ' ')"
  }
  if ([string]::IsNullOrWhiteSpace($output)) {
    return $null
  }
  return $output | ConvertFrom-Json
}

Write-Host "Preparing upload list..."
$files = Get-ChildItem -LiteralPath $root -Recurse -File | Where-Object { -not (Should-SkipFile $_) }
Write-Host "Uploading $($files.Count) files to $Repo..."

$tree = @()
$index = 0
foreach ($file in $files) {
  $index++
  $path = $file.FullName.Substring($root.Length + 1).Replace("\", "/")
  Write-Host ("[{0}/{1}] {2}" -f $index, $files.Count, $path)

  $bytes = [System.IO.File]::ReadAllBytes($file.FullName)
  $base64 = [Convert]::ToBase64String($bytes)
  $blob = Invoke-GhJson -Arguments @("api", "-X", "POST", "/repos/$Repo/git/blobs") -Body @{
    content = $base64
    encoding = "base64"
  }

  $tree += @{
    path = $path
    mode = "100644"
    type = "blob"
    sha = $blob.sha
  }
}

$ref = Invoke-GhJson -Arguments @("api", "/repos/$Repo/git/ref/heads/$Branch")
$parentSha = $ref.object.sha
$parentCommit = Invoke-GhJson -Arguments @("api", "/repos/$Repo/git/commits/$parentSha")

$newTree = Invoke-GhJson -Arguments @("api", "-X", "POST", "/repos/$Repo/git/trees") -Body @{
  tree = $tree
}

$commit = Invoke-GhJson -Arguments @("api", "-X", "POST", "/repos/$Repo/git/commits") -Body @{
  message = "chore: upload structured SaaS project source"
  tree = $newTree.sha
  parents = @($parentSha)
}

Invoke-GhJson -Arguments @("api", "-X", "PATCH", "/repos/$Repo/git/refs/heads/$Branch") -Body @{
  sha = $commit.sha
  force = $false
} | Out-Null

Write-Host "Upload completed."
Write-Host "Commit: $($commit.sha)"
