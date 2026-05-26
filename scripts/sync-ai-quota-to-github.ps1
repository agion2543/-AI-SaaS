$ErrorActionPreference = "Stop"

$repo = "C:\Users\Administrator\Documents\Codex\2026-04-27\go-web-gin-health"
$syncRoot = "C:\tmp\ai-saas-github-sync-20260511144604"
$git = "E:\Git-2.54.0-64-bit\Git\cmd\git.exe"

$files = @(
  "internal\model\ai_usage.go",
  "internal\model\merchant.go",
  "internal\bootstrap\app.go",
  "internal\service\merchant_service.go",
  "internal\controller\merchant_controller.go",
  "internal\router\router.go",
  "web\src\api\modules.js",
  "web\src\views\merchants\MerchantAIView.vue",
  "sql\migrations\20260511_add_merchant_ai_usage_logs.sql"
)

foreach ($file in $files) {
  $src = Join-Path $repo $file
  $dst = Join-Path $syncRoot $file
  $dir = Split-Path $dst -Parent
  New-Item -ItemType Directory -Path $dir -Force | Out-Null
  if (Test-Path $dst) {
    attrib -R $dst
  }
  Copy-Item -LiteralPath $src -Destination $dst -Force
}

& $git -c safe.directory=$syncRoot -C $syncRoot status --short
