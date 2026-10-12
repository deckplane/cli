# Installs the deckplane CLI on Windows:
#
#   irm https://raw.githubusercontent.com/deckplane/cli/main/install.ps1 | iex
#
# Optional environment variables:
#   DECKPLANE_VERSION       release to install, e.g. v0.1.0 (default: latest)
#   DECKPLANE_INSTALL_DIR   where to put deckplane.exe (default: %LOCALAPPDATA%\Programs\deckplane)
#   DECKPLANE_RELEASES_URL  release download base (default: GitHub releases)
#
# Everything runs inside one script block, so a partially downloaded script does nothing.

& {
    $ErrorActionPreference = 'Stop'
    $ProgressPreference = 'SilentlyContinue'  # makes Invoke-WebRequest much faster on Windows PowerShell 5.1
    [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12

    $Repo = 'deckplane/cli'

    function Step($msg) { Write-Host "-> $msg" -ForegroundColor Cyan }
    function Ok($msg) { Write-Host "OK $msg" -ForegroundColor Green }
    function Warn($msg) { Write-Host "!  $msg" -ForegroundColor Yellow }
    function Fail($msg, $hint) {
        Write-Host "`nX  $msg" -ForegroundColor Red
        if ($hint) { Write-Host "   $hint" }
        throw $msg
    }

    Write-Host "`nDeckplane CLI installer`n" -ForegroundColor White

    switch ([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture) {
        'X64' { $Arch = 'amd64' }
        'Arm64' { $Arch = 'arm64' }
        default { Fail "Unsupported processor: $_" 'Deckplane supports 64-bit Intel/AMD and ARM processors.' }
    }

    $Version = $env:DECKPLANE_VERSION
    if (-not $Version) {
        try {
            $Version = (Invoke-RestMethod -UseBasicParsing "https://api.github.com/repos/$Repo/releases/latest").tag_name
        } catch {
            Fail 'Could not find the latest version on GitHub' "Check the internet connection, or set DECKPLANE_VERSION (see https://github.com/$Repo/releases)."
        }
    }

    $Base = if ($env:DECKPLANE_RELEASES_URL) { $env:DECKPLANE_RELEASES_URL } else { "https://github.com/$Repo/releases" }
    $Base = "$Base/download/$Version"
    $Archive = "deckplane_windows_$Arch.zip"
    $Tmp = Join-Path ([IO.Path]::GetTempPath()) ("deckplane-" + [Guid]::NewGuid())
    New-Item -ItemType Directory -Path $Tmp | Out-Null

    try {
        Step "Downloading deckplane $Version for windows/$Arch"
        try {
            Invoke-WebRequest -UseBasicParsing "$Base/$Archive" -OutFile "$Tmp\$Archive"
            Invoke-WebRequest -UseBasicParsing "$Base/checksums.txt" -OutFile "$Tmp\checksums.txt"
        } catch {
            Fail "Download failed: $Base/$Archive" "Check the version name and the internet connection."
        }

        $line = Get-Content "$Tmp\checksums.txt" | Where-Object { $_ -match "\s$([regex]::Escape($Archive))$" } | Select-Object -First 1
        if (-not $line) { Fail "$Archive is not listed in checksums.txt" }
        $expected = ($line -split '\s+')[0]
        $actual = (Get-FileHash -Algorithm SHA256 "$Tmp\$Archive").Hash
        if ($actual -ne $expected) { Fail 'The download is corrupted (checksum mismatch)' 'Please run the installer again.' }
        Ok 'Download verified'

        Expand-Archive -Path "$Tmp\$Archive" -DestinationPath "$Tmp\out" -Force

        $Dir = if ($env:DECKPLANE_INSTALL_DIR) { $env:DECKPLANE_INSTALL_DIR } else { Join-Path $env:LOCALAPPDATA 'Programs\deckplane' }
        Step "Installing to $Dir"
        New-Item -ItemType Directory -Force -Path $Dir | Out-Null
        Copy-Item "$Tmp\out\deckplane.exe" (Join-Path $Dir 'deckplane.exe') -Force
        Ok ("Installed " + (& (Join-Path $Dir 'deckplane.exe') version))
    } finally {
        Remove-Item -Recurse -Force $Tmp -ErrorAction SilentlyContinue
    }

    $UserPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    if (($UserPath -split ';') -notcontains $Dir) {
        [Environment]::SetEnvironmentVariable('Path', (@($Dir) + ($UserPath -split ';' | Where-Object { $_ })) -join ';', 'User')
        $env:Path = "$Dir;$env:Path"
        Ok 'Added deckplane to your PATH (open a new terminal to use it everywhere)'
    }

    if (Get-Command docker -ErrorAction SilentlyContinue) {
        Ok 'Docker is installed'
    } else {
        Warn 'Docker is not installed. Deckplane runs on Docker.'
        Write-Host '   Install Docker Desktop: https://docs.docker.com/desktop/setup/install/windows-install/'
        Write-Host '   or run:  winget install Docker.DockerDesktop'
    }

    Write-Host "`nAll set! Install Deckplane Core with:`n" -ForegroundColor Green
    Write-Host '  deckplane core install' -ForegroundColor White
    Write-Host "`nNot sure this machine is ready? Run:  deckplane doctor`n"
}
