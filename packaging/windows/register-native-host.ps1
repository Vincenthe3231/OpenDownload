[CmdletBinding()]
param(
  [ValidateSet('Install', 'Inspect', 'Uninstall')]
  [string]$Action = 'Install',

  [string]$InstallRoot = (Join-Path $env:LOCALAPPDATA 'Programs\OpenDownload'),

  [string]$ConfigPath,

  # Tests may use a non-browser HKCU key. Production must keep the default.
  [string]$RegistryRoot = 'HKCU:\Software\Mozilla\NativeMessagingHosts'
)

$ErrorActionPreference = 'Stop'

$ScriptDirectory = $PSScriptRoot
if ([string]::IsNullOrWhiteSpace($ScriptDirectory)) {
  $ScriptDirectory = Split-Path -Parent ([string]$MyInvocation.MyCommand.Path)
}
if ([string]::IsNullOrWhiteSpace($ConfigPath)) {
  if ([string]::IsNullOrWhiteSpace($ScriptDirectory)) {
    throw 'Could not resolve the native-host configuration directory.'
  }
  $ConfigPath = Join-Path -Path $ScriptDirectory -ChildPath 'browser-ids.json'
}
$ConfigPath = [System.IO.Path]::GetFullPath($ConfigPath)

$HostName = 'com.opendownload.capture'
$InstallRoot = [System.IO.Path]::GetFullPath($InstallRoot)
$ManifestRoot = Join-Path $InstallRoot 'native-manifests'
$ManifestPath = Join-Path $ManifestRoot 'firefox.json'
$HostPath = Join-Path $InstallRoot 'opendownload-native-host.exe'
$RegistrationKey = Join-Path $RegistryRoot $HostName

function Get-FirefoxExtensionId {
  if (-not (Test-Path -LiteralPath $ConfigPath -PathType Leaf)) {
    throw "Browser ID configuration was not found: $ConfigPath"
  }

  try {
    $config = Get-Content -LiteralPath $ConfigPath -Raw | ConvertFrom-Json -ErrorAction Stop
  } catch {
    throw "Browser ID configuration is invalid: $ConfigPath"
  }

  $extensionId = [string]$config.firefox.extensionId
  if ([string]::IsNullOrWhiteSpace($extensionId)) {
    throw "Browser ID configuration does not define firefox.extensionId: $ConfigPath"
  }

  return $extensionId
}

function Test-PathEquals {
  param(
    [AllowNull()]
    [string]$Left,

    [AllowNull()]
    [string]$Right
  )

  if ([string]::IsNullOrWhiteSpace($Left) -or [string]::IsNullOrWhiteSpace($Right)) {
    return $false
  }

  try {
    $leftPath = [System.IO.Path]::GetFullPath($Left).TrimEnd([System.IO.Path]::DirectorySeparatorChar, [System.IO.Path]::AltDirectorySeparatorChar)
    $rightPath = [System.IO.Path]::GetFullPath($Right).TrimEnd([System.IO.Path]::DirectorySeparatorChar, [System.IO.Path]::AltDirectorySeparatorChar)
  } catch {
    return $false
  }

  return [string]::Equals($leftPath, $rightPath, [System.StringComparison]::OrdinalIgnoreCase)
}

function Get-RegistryDefaultValue {
  param([string]$Path)

  try {
    return [string](Get-ItemPropertyValue -LiteralPath $Path -Name '(default)' -ErrorAction Stop)
  } catch {
    return $null
  }
}

function New-HostManifest {
  param([string]$FirefoxExtensionId)

  $manifest = [ordered]@{
    name = $HostName
    description = 'OpenDownload browser capture host'
    path = $HostPath
    type = 'stdio'
    allowed_extensions = @($FirefoxExtensionId)
  }

  return $manifest | ConvertTo-Json -Depth 4
}

function Write-AtomicTextFile {
  param(
    [string]$Path,
    [string]$Content
  )

  $directory = Split-Path -Parent $Path
  New-Item -ItemType Directory -Path $directory -Force | Out-Null

  $fileName = [System.IO.Path]::GetFileName($Path)
  $nonce = [Guid]::NewGuid().ToString('N')
  $temporaryPath = Join-Path $directory ('.' + $fileName + '.' + $nonce + '.tmp')
  $backupPath = Join-Path $directory ('.' + $fileName + '.' + $nonce + '.bak')

  try {
    [System.IO.File]::WriteAllText($temporaryPath, $Content, (New-Object System.Text.UTF8Encoding($false)))

    if ([System.IO.File]::Exists($Path)) {
      [System.IO.File]::Replace($temporaryPath, $Path, $backupPath, $true)
    } else {
      [System.IO.File]::Move($temporaryPath, $Path)
    }
  } finally {
    if ([System.IO.File]::Exists($temporaryPath)) {
      [System.IO.File]::Delete($temporaryPath)
    }
    if ([System.IO.File]::Exists($backupPath)) {
      [System.IO.File]::Delete($backupPath)
    }
  }
}

function Get-Manifest {
  if (-not (Test-Path -LiteralPath $ManifestPath -PathType Leaf)) {
    return $null
  }

  try {
    return Get-Content -LiteralPath $ManifestPath -Raw | ConvertFrom-Json -ErrorAction Stop
  } catch {
    return $null
  }
}

function Test-ManifestOwnership {
  param([string]$FirefoxExtensionId)

  $manifest = Get-Manifest
  if ($null -eq $manifest) {
    return $false
  }

  $allowedExtensions = @($manifest.allowed_extensions | ForEach-Object { [string]$_ })
  return (
    $manifest.name -eq $HostName -and
    $manifest.type -eq 'stdio' -and
    (Test-PathEquals ([string]$manifest.path) $HostPath) -and
    $allowedExtensions.Count -eq 1 -and
    $allowedExtensions[0] -eq $FirefoxExtensionId
  )
}

function New-InspectionFailure {
  param([string]$Code)

  switch ($Code) {
    'NATIVE_HOST_EXECUTABLE_MISSING' {
      return [ordered]@{ code = $Code; userMessage = 'Browser capture host files are missing. Repair browser capture.'; retryable = $true }
    }
    'NATIVE_HOST_MANIFEST_MISSING' {
      return [ordered]@{ code = $Code; userMessage = 'Browser capture needs repair.'; retryable = $true }
    }
    'NATIVE_HOST_MANIFEST_INVALID' {
      return [ordered]@{ code = $Code; userMessage = 'Browser capture configuration is invalid. Repair browser capture.'; retryable = $true }
    }
    default {
      return [ordered]@{ code = 'NATIVE_HOST_NOT_REGISTERED'; userMessage = 'Firefox browser capture is not registered. Repair browser capture.'; retryable = $true }
    }
  }
}

function Get-HostStatus {
  $firefoxExtensionId = Get-FirefoxExtensionId
  $hostExecutableFound = Test-Path -LiteralPath $HostPath -PathType Leaf
  $manifestExists = Test-Path -LiteralPath $ManifestPath -PathType Leaf
  $manifest = Get-Manifest
  $manifestPathMatchesInstall = $false
  $manifestExtensionIdMatches = $false

  if ($null -ne $manifest) {
    $manifestPathMatchesInstall = Test-PathEquals ([string]$manifest.path) $HostPath
    $allowedExtensions = @($manifest.allowed_extensions | ForEach-Object { [string]$_ })
    $manifestExtensionIdMatches = $allowedExtensions.Count -eq 1 -and $allowedExtensions[0] -eq $firefoxExtensionId
  }

  $mozillaRegistryPointsToExpectedManifest = Test-PathEquals (Get-RegistryDefaultValue $RegistrationKey) $ManifestPath
  $healthy = $hostExecutableFound -and $manifestExists -and $manifestPathMatchesInstall -and $manifestExtensionIdMatches -and $mozillaRegistryPointsToExpectedManifest

  $failure = $null
  if (-not $healthy) {
    if (-not $hostExecutableFound) {
      $failure = New-InspectionFailure 'NATIVE_HOST_EXECUTABLE_MISSING'
    } elseif (-not $manifestExists) {
      $failure = New-InspectionFailure 'NATIVE_HOST_MANIFEST_MISSING'
    } elseif (-not $manifestPathMatchesInstall -or -not $manifestExtensionIdMatches) {
      $failure = New-InspectionFailure 'NATIVE_HOST_MANIFEST_INVALID'
    } else {
      $failure = New-InspectionFailure 'NATIVE_HOST_NOT_REGISTERED'
    }
  }

  return [ordered]@{
    hostExecutableFound = $hostExecutableFound
    manifestExists = $manifestExists
    manifestPathMatchesInstall = $manifestPathMatchesInstall
    manifestExtensionIdMatches = $manifestExtensionIdMatches
    mozillaRegistryPointsToExpectedManifest = $mozillaRegistryPointsToExpectedManifest
    healthy = $healthy
    failure = $failure
  }
}

function Install-Host {
  if (-not (Test-Path -LiteralPath $HostPath -PathType Leaf)) {
    throw "Native host executable not found: $HostPath"
  }

  $firefoxExtensionId = Get-FirefoxExtensionId
  $manifestExisted = Test-Path -LiteralPath $ManifestPath -PathType Leaf
  $originalManifest = if ($manifestExisted) { Get-Content -LiteralPath $ManifestPath -Raw } else { $null }
  $registrationExisted = Test-Path -LiteralPath $RegistrationKey
  $registrationCreated = $false
  $manifestUpdated = $false

  if ($registrationExisted) {
    $registeredManifestPath = Get-RegistryDefaultValue $RegistrationKey
    if (-not (Test-PathEquals $registeredManifestPath $ManifestPath)) {
      throw 'Firefox native-host registration is owned by another installation.'
    }
  }

  try {
    Write-AtomicTextFile -Path $ManifestPath -Content (New-HostManifest $firefoxExtensionId)
    $manifestUpdated = $true

    if (-not $registrationExisted) {
      New-Item -Path $RegistrationKey -Force | Out-Null
      $registrationCreated = $true
    }

    Set-ItemProperty -LiteralPath $RegistrationKey -Name '(default)' -Value $ManifestPath
  } catch {
    if ($registrationCreated -and (Test-Path -LiteralPath $RegistrationKey)) {
      Remove-Item -LiteralPath $RegistrationKey -Force -ErrorAction SilentlyContinue
    }

    if ($manifestUpdated) {
      if ($manifestExisted) {
        Write-AtomicTextFile -Path $ManifestPath -Content $originalManifest
      } elseif (Test-Path -LiteralPath $ManifestPath) {
        Remove-Item -LiteralPath $ManifestPath -Force -ErrorAction SilentlyContinue
      }
    }

    throw
  }
}

function Uninstall-Host {
  $firefoxExtensionId = Get-FirefoxExtensionId
  $registeredManifestPath = Get-RegistryDefaultValue $RegistrationKey
  $registrationOwned = Test-PathEquals $registeredManifestPath $ManifestPath

  if (-not $registrationOwned) {
    return
  }

  $manifestOwned = Test-ManifestOwnership $firefoxExtensionId
  Remove-Item -LiteralPath $RegistrationKey -Force -ErrorAction SilentlyContinue

  if ($manifestOwned -and (Test-Path -LiteralPath $ManifestPath)) {
    Remove-Item -LiteralPath $ManifestPath -Force -ErrorAction SilentlyContinue
    Remove-Item -LiteralPath $ManifestRoot -Force -ErrorAction SilentlyContinue
  }
}

switch ($Action) {
  'Install' {
    Install-Host
    break
  }
  'Uninstall' {
    Uninstall-Host
    break
  }
  'Inspect' {
    try {
      Get-HostStatus | ConvertTo-Json -Depth 5 -Compress
    } catch {
      [ordered]@{
        hostExecutableFound = $false
        manifestExists = $false
        manifestPathMatchesInstall = $false
        manifestExtensionIdMatches = $false
        mozillaRegistryPointsToExpectedManifest = $false
        healthy = $false
        failure = (New-InspectionFailure 'NATIVE_HOST_MANIFEST_INVALID')
      } | ConvertTo-Json -Depth 5 -Compress
    }
    break
  }
}
