# eisencode installer for Windows (PowerShell)
# usage: irm https://eisencode.waivy.dev/install.ps1 | iex
#
# Installs the standalone eisencode CLI. It does not need or modify opencode.

$ErrorActionPreference = "Stop"
$repo    = "eisen0xx/eisencode-cli"
$version = if ($env:EISENCODE_VERSION) { $env:EISENCODE_VERSION } else { "latest" }
$arch    = if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") { "arm64" } else { "amd64" }
$bin     = "eisencode-windows-$arch.exe"

if ($version -eq "latest") {
  $url = "https://github.com/$repo/releases/latest/download/$bin"
} else {
  $url = "https://github.com/$repo/releases/download/$version/$bin"
}

if ($env:EISENCODE_BIN_DIR) {
  $dest = $env:EISENCODE_BIN_DIR
} else {
  $dest = Join-Path $env:LOCALAPPDATA "eisencode"
}
New-Item -ItemType Directory -Force -Path $dest | Out-Null

Write-Host "eisencode - installing $bin" -ForegroundColor Cyan
$out = Join-Path $dest "eisencode.exe"
Invoke-WebRequest -Uri $url -OutFile $out -UseBasicParsing

$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($userPath -notlike "*$dest*") {
  [Environment]::SetEnvironmentVariable("Path", "$userPath;$dest", "User")
  Write-Host "added $dest to your user PATH (restart the terminal)" -ForegroundColor Yellow
}

& $out version
Write-Host ""
Write-Host "next:" -ForegroundColor Cyan
Write-Host "  eisencode login            # prompts for your key"
Write-Host "  eisencode run `"hello`""
Write-Host "  eisencode chat             # isolated sessions, /help inside"
Write-Host ""
Write-Host "shell completion:" -ForegroundColor Cyan
Write-Host "  eisencode completion powershell | Out-File -Append -Encoding utf8 `$PROFILE"
Write-Host ""
Write-Host "optional: hand a session to opencode (only if installed):" -ForegroundColor Cyan
Write-Host "  eisencode sessions export <id> --import"
