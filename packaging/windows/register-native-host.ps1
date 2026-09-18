param(
  [ValidateSet('Install', 'Uninstall')]
  [string]$Action = 'Install',
  [string]$InstallRoot = (Join-Path $env:LOCALAPPDATA 'OpenDownload')
)

$ErrorActionPreference = 'Stop'
$HostName = 'com.opendownload.capture'
$ManifestRoot = Join-Path $InstallRoot 'native-manifests'
$HostPath = Join-Path $InstallRoot 'opendownload-native-host.exe'
$ChromiumExtensionId = 'ofhdpijhmbjoegmabdglpoemginnljlh'
$FirefoxExtensionId = 'opendownload-capture@local'

$Registrations = @(
  @{ Key = 'HKCU:\Software\Google\Chrome\NativeMessagingHosts\' + $HostName; Manifest = 'chrome.json' },
  @{ Key = 'HKCU:\Software\Microsoft\Edge\NativeMessagingHosts\' + $HostName; Manifest = 'edge.json' },
  @{ Key = 'HKCU:\Software\Mozilla\NativeMessagingHosts\' + $HostName; Manifest = 'firefox.json' }
)

function New-HostManifest([string]$Browser) {
  $manifest = @{
    name = $HostName
    description = 'OpenDownload passwordless browser capture host'
    path = $HostPath
    type = 'stdio'
  }
  if ($Browser -eq 'firefox') {
    $manifest.allowed_extensions = @($FirefoxExtensionId)
  } else {
    $manifest.allowed_origins = @('chrome-extension://' + $ChromiumExtensionId + '/')
  }
  return $manifest | ConvertTo-Json -Depth 4
}

function Write-Manifest([string]$Path, [string]$Content) {
  [IO.File]::WriteAllText($Path, $Content, (New-Object Text.UTF8Encoding($false)))
}

function Install-Host {
  if (-not (Test-Path -LiteralPath $HostPath)) {
    throw "Native host executable not found: $HostPath"
  }
  New-Item -ItemType Directory -Path $ManifestRoot -Force | Out-Null
  Write-Manifest (Join-Path $ManifestRoot 'chrome.json') (New-HostManifest 'chrome')
  Write-Manifest (Join-Path $ManifestRoot 'edge.json') (New-HostManifest 'edge')
  Write-Manifest (Join-Path $ManifestRoot 'firefox.json') (New-HostManifest 'firefox')
  foreach ($registration in $Registrations) {
    New-Item -Path (Split-Path $registration.Key) -Force | Out-Null
    New-Item -Path $registration.Key -Force | Out-Null
    Set-ItemProperty -LiteralPath $registration.Key -Name '(default)' -Value (Join-Path $ManifestRoot $registration.Manifest)
  }
}

function Uninstall-Host {
  foreach ($registration in $Registrations) {
    Remove-Item -LiteralPath $registration.Key -Recurse -Force -ErrorAction SilentlyContinue
  }
  Remove-Item -LiteralPath $ManifestRoot -Recurse -Force -ErrorAction SilentlyContinue
}

if ($Action -eq 'Install') { Install-Host } else { Uninstall-Host }
