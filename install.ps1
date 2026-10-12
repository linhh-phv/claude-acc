# Installs claude-acc on Windows into %USERPROFILE%\.local\bin — the folder
# where the Claude Code installer puts claude.exe, so it is usually on PATH.
#
# Usage (PowerShell):
#   irm https://raw.githubusercontent.com/linhh-phv/claude-acc/main/install.ps1 | iex
# or with a binary you were given:
#   .\install.ps1 -From .\claude-acc-windows-amd64.exe

param([string]$From = "")

$ErrorActionPreference = "Stop"
$repo   = "linhh-phv/claude-acc"
$binDir = Join-Path $env:USERPROFILE ".local\bin"
$dest   = Join-Path $binDir "claude-acc.exe"

if (-not (Get-Command claude -ErrorAction SilentlyContinue)) {
    Write-Warning "``claude`` not found in PATH yet - install Claude Code first: https://claude.com/claude-code"
}

New-Item -ItemType Directory -Force -Path $binDir | Out-Null
$tmp = "$dest.new"

if ($From -ne "") {
    Copy-Item -Force $From $tmp
} else {
    $arch = if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") { "arm64" } else { "amd64" }
    $url = "https://github.com/$repo/releases/latest/download/claude-acc-windows-$arch.exe"
    Invoke-WebRequest -UseBasicParsing -Uri $url -OutFile $tmp
}
Move-Item -Force $tmp $dest
# A downloaded or copied file carries the "came from the internet" mark;
# clear it so Windows does not block or warn on every run.
Unblock-File -Path $dest -ErrorAction SilentlyContinue
Write-Host "installed: $dest ($(& $dest version))" -ForegroundColor Green

$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if (($userPath -split ";") -notcontains $binDir) {
    [Environment]::SetEnvironmentVariable("Path", "$userPath;$binDir", "User")
    Write-Host "Added $binDir to your PATH - open a new terminal to use it." -ForegroundColor Yellow
}
Write-Host "Run 'claude-acc help' to get started."
