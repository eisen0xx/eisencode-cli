# eisencode installer for Windows (PowerShell)
# usage: irm https://eisencode.waivy.dev/install.ps1 | iex

$ErrorActionPreference = "Stop"
$repo    = "eisen0xx/eisencode-cli"
$version = if ($env:EISENCODE_VERSION) { $env:EISENCODE_VERSION } else { "latest" }
$arch    = if ([Environment]::Is64BitOperatingSystem) { "amd64" } else { "amd64" }
$bin     = "eisencode-windows-$arch.exe"

if ($version -eq "latest") {
  $url = "https://github.com/$repo/releases/latest/download/$bin"
} else {
  $url = "https://github.com/$repo/releases/download/$version/$bin"
}

$dest = Join-Path $env:LOCALAPPDATA "eisencode"
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
