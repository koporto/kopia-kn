# Knockout Networks — silent enrollment for RMM / technician scripts.
# Requires kopia.exe on PATH or next to this script.
param(
    [Parameter(Mandatory = $true)]
    [string]$SiteId,

    [Parameter(Mandatory = $true)]
    [string]$Passcode,

    [ValidateSet("workstation", "workstation-full", "server", "server-system")]
    [string]$Profile = "workstation",

    [string]$Destination = "",

    [string[]]$Path = @()
)

$ErrorActionPreference = "Stop"

$kopia = Join-Path $PSScriptRoot "kopia.exe"
if (-not (Test-Path $kopia)) {
    $cmd = Get-Command kopia.exe -ErrorAction SilentlyContinue
    if ($cmd) {
        $kopia = $cmd.Source
    }
    else {
        throw "kopia.exe was not found. Place it next to enroll.ps1 or add it to PATH."
    }
}

$args = @(
    "knockout", "setup",
    "--site-id", $SiteId,
    "--passcode", $Passcode,
    "--profile", $Profile
)

if ($Destination) {
    $args += @("--destination", $Destination)
}

foreach ($p in $Path) {
    $args += @("--path", $p)
}

& $kopia @args
if ($LASTEXITCODE -ne 0) {
    throw "Knockout Backup enrollment failed with exit code $LASTEXITCODE"
}

Write-Host "Enrolled site $SiteId with profile $Profile. Nightly backups run while Knockout Backup is running."
