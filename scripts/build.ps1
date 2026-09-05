[CmdletBinding()]
param(
    [ValidateSet('Desktop', 'CLI')]
    [string]$Target = 'Desktop'
)

$ErrorActionPreference = 'Stop'

if ($Target -eq 'Desktop') {
    & wails build
    exit $LASTEXITCODE
}

& go build -o build\bin\opendownload-cli.exe ./cmd/cli
exit $LASTEXITCODE
