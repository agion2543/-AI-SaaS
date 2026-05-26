$ErrorActionPreference = "Stop"

$syncRoot = "C:\tmp\ai-saas-github-sync-20260511144604"
$git = "E:\Git-2.54.0-64-bit\Git\cmd\git.exe"

& $git -c safe.directory=$syncRoot -C $syncRoot push origin main
