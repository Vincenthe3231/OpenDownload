[CmdletBinding()]
param(
    [ValidateSet('Desktop', 'CLI')]
    [string]$Target = 'Desktop'
)

$ErrorActionPreference = 'Stop'

if ($Target -eq 'Desktop') {
    & go build -o build\bin\opendownload-native-host.exe ./cmd/opendownload-native-host
    if ($LASTEXITCODE -ne 0) {
        exit $LASTEXITCODE
    }

    & wails build -nsis -installscope user
    exit $LASTEXITCODE
}

& go build -o build\bin\opendownload-cli.exe ./cmd/cli
exit $LASTEXITCODE
