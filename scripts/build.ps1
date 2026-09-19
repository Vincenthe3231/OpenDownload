[CmdletBinding()]
param(
    [ValidateSet('Desktop', 'CLI')]
    [string]$Target = 'Desktop'
)

$ErrorActionPreference = 'Stop'
$repositoryRoot = Split-Path -Parent $PSScriptRoot

function Assert-NSISAvailable {
    $nsis = Get-Command -Name 'makensis.exe' -ErrorAction SilentlyContinue
    if ($null -eq $nsis) {
        $nsis = Get-Command -Name 'makensis' -ErrorAction SilentlyContinue
    }
    if ($null -eq $nsis) {
        throw 'NSIS is required to build the desktop installer. Install NSIS and ensure makensis.exe is on PATH.'
    }
}

if ($Target -eq 'Desktop') {
    Assert-NSISAvailable

    $nativeHostOutput = Join-Path $repositoryRoot 'build\bin\opendownload-native-host.exe'
    $buildStartedAt = Get-Date
    Push-Location $repositoryRoot
    try {
        & go build -o $nativeHostOutput ./cmd/opendownload-native-host
        if ($LASTEXITCODE -ne 0) {
            exit $LASTEXITCODE
        }
        if (-not (Test-Path -LiteralPath $nativeHostOutput -PathType Leaf)) {
            throw "Native host build did not produce: $nativeHostOutput"
        }
        $nativeHost = Get-Item -LiteralPath $nativeHostOutput
        if ($nativeHost.Length -le 0 -or $nativeHost.LastWriteTime -lt $buildStartedAt) {
            throw "Native host build output is stale or empty: $nativeHostOutput"
        }

        & wails build -nsis -installscope user
        exit $LASTEXITCODE
    } finally {
        Pop-Location
    }
}

Push-Location $repositoryRoot
try {
    & go build -o build\bin\opendownload-cli.exe ./cmd/cli
    exit $LASTEXITCODE
} finally {
    Pop-Location
}
