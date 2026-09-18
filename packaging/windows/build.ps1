$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot | Split-Path -Parent
$output = Join-Path $root 'build\bin\opendownload-native-host.exe'

New-Item -ItemType Directory -Path (Split-Path $output) -Force | Out-Null
go build -trimpath -ldflags='-s -w' -o $output (Join-Path $root 'cmd\opendownload-native-host')
if ($LASTEXITCODE -ne 0) { throw 'Native host build failed.' }
wails build -nsis
if ($LASTEXITCODE -ne 0) { throw 'Wails NSIS build failed.' }
