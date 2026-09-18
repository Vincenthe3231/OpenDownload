!macro customInstall
  SetOutPath "$INSTDIR"
  File /oname=opendownload-native-host.exe "${__FILEDIR__}\..\..\..\build\bin\opendownload-native-host.exe"
  File /oname=register-native-host.ps1 "${__FILEDIR__}\..\..\..\packaging\windows\register-native-host.ps1"
  nsExec::ExecToLog '"$SYSDIR\WindowsPowerShell\v1.0\powershell.exe" -NoProfile -ExecutionPolicy Bypass -File "$INSTDIR\register-native-host.ps1" -Action Install -InstallRoot "$INSTDIR"'
  Pop $0
  StrCmp $0 "0" customInstallDone
  Abort "OpenDownload could not register its browser capture host."
  customInstallDone:
!macroend

!macro customUnInstall
  nsExec::ExecToLog '"$SYSDIR\WindowsPowerShell\v1.0\powershell.exe" -NoProfile -ExecutionPolicy Bypass -File "$INSTDIR\register-native-host.ps1" -Action Uninstall -InstallRoot "$INSTDIR"'
  Pop $0
!macroend
