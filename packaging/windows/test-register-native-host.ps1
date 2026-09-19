[CmdletBinding()]
param(
  # Use this in restricted environments to validate the configuration and script parser
  # without creating even a test-only HKCU key.
  [switch]$SkipRegistry
)

$ErrorActionPreference = 'Stop'

$scriptRoot = $PSScriptRoot
$repositoryRoot = Split-Path -Parent (Split-Path -Parent $scriptRoot)
$registrationScript = Join-Path $scriptRoot 'register-native-host.ps1'
$configPath = Join-Path $scriptRoot 'browser-ids.json'
$firefoxManifestPath = Join-Path $repositoryRoot 'extensions\firefox\manifest.json'

function Assert-True {
  param(
    [bool]$Condition,
    [string]$Message
  )

  if (-not $Condition) {
    throw "Assertion failed: $Message"
  }
}

[void][scriptblock]::Create((Get-Content -LiteralPath $registrationScript -Raw))
$config = Get-Content -LiteralPath $configPath -Raw | ConvertFrom-Json -ErrorAction Stop
$firefoxManifest = Get-Content -LiteralPath $firefoxManifestPath -Raw | ConvertFrom-Json -ErrorAction Stop
Assert-True ($config.firefox.extensionId -eq $firefoxManifest.applications.gecko.id) 'browser-ids.json must match extensions/firefox/manifest.json.'

if ($SkipRegistry) {
  Write-Output 'Configuration and PowerShell parser checks passed. Registry integration was skipped.'
  return
}

$testId = [Guid]::NewGuid().ToString('N')
$testInstallRoot = Join-Path ([System.IO.Path]::GetTempPath()) "OpenDownload Native Host-$testId"
$testRegistryRoot = "HKCU:\Software\OpenDownload\Tests\$testId\NativeMessagingHosts"
$testRegistryParent = "HKCU:\Software\OpenDownload\Tests\$testId"
$hostName = 'com.opendownload.capture'
$testRegistrationKey = Join-Path $testRegistryRoot $hostName
$testManifestPath = Join-Path $testInstallRoot 'native-manifests\firefox.json'

try {
  New-Item -ItemType Directory -Path $testInstallRoot -Force | Out-Null
  [System.IO.File]::WriteAllBytes((Join-Path $testInstallRoot 'opendownload-native-host.exe'), [byte[]]@(0))

  try {
    & $registrationScript -Action Install -InstallRoot $testInstallRoot -RegistryRoot $testRegistryRoot | Out-Null
  } catch {
    throw "Install action should succeed against the test registry root: $($_.Exception.Message)"
  }

  $status = (& $registrationScript -Action Inspect -InstallRoot $testInstallRoot -RegistryRoot $testRegistryRoot | ConvertFrom-Json)
  Assert-True $status.healthy 'Inspect should report a healthy test installation.'
  Assert-True $status.hostExecutableFound 'Inspect should find the test native host.'
  Assert-True $status.manifestExists 'Inspect should find the Firefox manifest.'
  Assert-True $status.manifestPathMatchesInstall 'Manifest path should point at the test install root.'
  Assert-True $status.manifestExtensionIdMatches 'Manifest extension ID should match browser-ids.json.'
  Assert-True $status.mozillaRegistryPointsToExpectedManifest 'Test registry should point at the generated manifest.'
  Assert-True (-not (Test-Path -LiteralPath (Join-Path $testInstallRoot 'native-manifests\chrome.json'))) 'Chrome manifest must not be created.'
  Assert-True (-not (Test-Path -LiteralPath (Join-Path $testInstallRoot 'native-manifests\edge.json'))) 'Edge manifest must not be created.'
  Assert-True (@(Get-ChildItem -LiteralPath (Split-Path -Parent $testManifestPath) -Filter '*.tmp' -File).Count -eq 0) 'Atomic manifest write must not leave temporary files.'

  Set-ItemProperty -LiteralPath $testRegistrationKey -Name '(default)' -Value (Join-Path $testInstallRoot 'foreign.json')
  & $registrationScript -Action Uninstall -InstallRoot $testInstallRoot -RegistryRoot $testRegistryRoot
  Assert-True (Test-Path -LiteralPath $testRegistrationKey) 'Uninstall must preserve a registration owned by another installation.'
  Assert-True (Test-Path -LiteralPath $testManifestPath) 'Uninstall must preserve a manifest when registration ownership is lost.'

  Set-ItemProperty -LiteralPath $testRegistrationKey -Name '(default)' -Value $testManifestPath
  & $registrationScript -Action Uninstall -InstallRoot $testInstallRoot -RegistryRoot $testRegistryRoot
  Assert-True (-not (Test-Path -LiteralPath $testRegistrationKey)) 'Uninstall should remove its own registration.'
  Assert-True (-not (Test-Path -LiteralPath $testManifestPath)) 'Uninstall should remove its own manifest.'

  Write-Output 'Native-host registration tests passed.'
} finally {
  Remove-Item -LiteralPath $testRegistryParent -Recurse -Force -ErrorAction SilentlyContinue
  Remove-Item -LiteralPath $testInstallRoot -Recurse -Force -ErrorAction SilentlyContinue
}
