# Installs claude-acc on Windows into %USERPROFILE%\.local\bin - the folder
# where the Claude Code installer puts claude.exe, so it is usually on PATH.
#
# Usage, in PowerShell:
#   irm https://raw.githubusercontent.com/linhh-phv/claude-acc/main/install.ps1 | iex
# or from a release zip: double-click install.cmd (it runs this file, which
# picks up the claude-acc-windows-*.exe next to it), or
#   .\install.ps1 -From .\claude-acc-windows-amd64.exe

param([string]$From = "")

# Run in a scope of our own: through `irm ... | iex` this script executes in the
# caller's PowerShell session, which must not be left with our variables or
# with ErrorActionPreference changed.
& {
    param([string]$From, [string]$Here)
    $ErrorActionPreference = "Stop"

    $repo   = "linhh-phv/claude-acc"
    $binDir = Join-Path $env:USERPROFILE ".local\bin"
    $dest   = Join-Path $binDir "claude-acc.exe"

    if (-not (Get-Command claude -ErrorAction SilentlyContinue)) {
        Write-Warning "``claude`` not found in PATH yet - install Claude Code first: https://claude.com/claude-code"
    }

    New-Item -ItemType Directory -Force -Path $binDir | Out-Null
    $tmp = "$dest.new"

    $arch = if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") { "arm64" } else { "amd64" }
    # A binary next to this script (the zip) wins over downloading one.
    if ($From -eq "" -and $Here) {
        foreach ($name in @("claude-acc-windows-$arch.exe", "claude-acc-windows-amd64.exe", "claude-acc.exe")) {
            $candidate = Join-Path $Here $name
            if (Test-Path $candidate) { $From = $candidate; break }
        }
    }

    if ($From -ne "") {
        Copy-Item -Force $From $tmp
    } else {
        $url = "https://github.com/$repo/releases/latest/download/claude-acc-windows-$arch.exe"
        Invoke-WebRequest -UseBasicParsing -Uri $url -OutFile $tmp
    }
    Move-Item -Force $tmp $dest
    # A downloaded or copied file carries the "came from the internet" mark;
    # clear it so Windows does not block or warn on every run.
    Unblock-File -Path $dest -ErrorAction SilentlyContinue
    Write-Host "installed: $dest ($(& $dest version))" -ForegroundColor Green

    # Add the folder to the user PATH, keeping its %VARIABLES% unexpanded:
    # [Environment]::SetEnvironmentVariable would rewrite it as plain text.
    $key = [Microsoft.Win32.Registry]::CurrentUser.OpenSubKey("Environment", $true)
    $raw = $key.GetValue("Path", "", [Microsoft.Win32.RegistryValueOptions]::DoNotExpandEnvironmentNames)
    $entries = @($raw -split ";" | Where-Object { $_ -ne "" })
    $present = $entries | Where-Object { [Environment]::ExpandEnvironmentVariables($_).TrimEnd("\") -ieq $binDir.TrimEnd("\") }
    if (-not $present) {
        $key.SetValue("Path", (($entries + $binDir) -join ";"), [Microsoft.Win32.RegistryValueKind]::ExpandString)
        # Tell running programs (Explorer, new terminals) that the environment changed.
        [Environment]::SetEnvironmentVariable("CLAUDE_ACC_PATH_REFRESH", "1", "User")
        [Environment]::SetEnvironmentVariable("CLAUDE_ACC_PATH_REFRESH", $null, "User")
        Write-Host "Added $binDir to your PATH - open a new terminal to use it." -ForegroundColor Yellow
    }
    $key.Close()
    Write-Host "Run 'claude-acc help' to get started."
} $From $PSScriptRoot
