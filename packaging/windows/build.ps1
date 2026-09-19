$ErrorActionPreference = 'Stop'
$repositoryRoot = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$buildScript = Join-Path $repositoryRoot 'scripts\build.ps1'

& $buildScript -Target Desktop
exit $LASTEXITCODE
