!macro customInstall
  SetOutPath "$INSTDIR"
  File /oname=opendownload-native-host.exe "${__FILEDIR__}\..\..\..\build\bin\opendownload-native-host.exe"
  File /oname=register-native-host.ps1 "${__FILEDIR__}\..\..\..\packaging\windows\register-native-host.ps1"
  File /oname=browser-ids.json "${__FILEDIR__}\..\..\..\packaging\windows\browser-ids.json"
  DetailPrint "Registering the Firefox browser capture host"
  nsExec::ExecToLog '"$SYSDIR\WindowsPowerShell\v1.0\powershell.exe" -NoProfile -ExecutionPolicy Bypass -File "$INSTDIR\register-native-host.ps1" -Action Install -InstallRoot "$INSTDIR" -ConfigPath "$INSTDIR\browser-ids.json"'
  Pop $0
  StrCmp $0 "0" customInstallDone
  DetailPrint "Browser capture host registration failed with exit code $0."
  Abort "OpenDownload could not register its Firefox browser capture host. See installer details for the error code."
  customInstallDone:
  DetailPrint "Firefox browser capture host registered."
!macroend

!macro customUnInstall
  IfFileExists "$INSTDIR\register-native-host.ps1" 0 customUninstallDone
  DetailPrint "Removing the Firefox browser capture host registration"
  nsExec::ExecToLog '"$SYSDIR\WindowsPowerShell\v1.0\powershell.exe" -NoProfile -ExecutionPolicy Bypass -File "$INSTDIR\register-native-host.ps1" -Action Uninstall -InstallRoot "$INSTDIR" -ConfigPath "$INSTDIR\browser-ids.json"'
  Pop $0
  StrCmp $0 "0" customUninstallDone
  DetailPrint "Browser capture host unregistration returned exit code $0."
  customUninstallDone:
!macroend
