Unicode true
!include "MUI2.nsh"
!include "LogicLib.nsh"
!include "StrFunc.nsh"
!include "TextFunc.nsh"
${StrStr}
${UnStrStr}
Name "光宇无限画布"
OutFile "${OUTPUT}"
RequestExecutionLevel user
SetCompressor /SOLID lzma
InstallDir "$LOCALAPPDATA\Programs\光宇无限画布"
Var StageDir
Var BackupDir
Var CleanDir
VIProductVersion "${VERSION}.0"
VIAddVersionKey "ProductName" "光宇无限画布"
VIAddVersionKey "FileVersion" "${VERSION}"
VIAddVersionKey "FileDescription" "光宇无限画布 Setup"
!define MUI_ABORTWARNING
!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_COMPONENTS
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH
!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES
!insertmacro MUI_LANGUAGE "English"
!insertmacro MUI_LANGUAGE "SimpChinese"

!macro RequireClosed PREFIX
Function ${PREFIX}RequireClosed
  nsExec::ExecToStack '"$SYSDIR\tasklist.exe" /FI "IMAGENAME eq 光宇无限画布.exe" /NH'
  Pop $0
  Pop $1
!if "${PREFIX}" == "un."
  ${UnStrStr} $2 $1 "光宇无限画布.exe"
!else
  ${StrStr} $2 $1 "光宇无限画布.exe"
!endif
  ${If} $2 == ""
!if "${PREFIX}" == "un."
    ${UnStrStr} $2 $1 "光宇无限画布.exe"
!else
    ${StrStr} $2 $1 "光宇无限画布.exe"
!endif
  ${EndIf}
  ${If} $0 != 0
  ${OrIf} $2 != ""
    MessageBox MB_OK|MB_ICONEXCLAMATION "Please close 光宇无限画布 and its CLI before installing or uninstalling." /SD IDOK
    SetErrorLevel 2
    Abort
  ${EndIf}
FunctionEnd
!macroend
!insertmacro RequireClosed ""
!insertmacro RequireClosed "un."

# Clean only shipped root files and runtime directories. Unknown user files remain.
!macro RemovePayload PREFIX
Function ${PREFIX}RemovePayload
  FileOpen $0 "$CleanDir\.installed-root-files.txt" r
  ${If} $0 != ""
    loop_${PREFIX}:
      ClearErrors
      FileRead $0 $1
      IfErrors done_${PREFIX}
      ${TrimNewLines} $1 $1
!if "${PREFIX}" == "un."
      ${UnStrStr} $2 $1 "\"
      ${UnStrStr} $3 $1 "/"
      ${UnStrStr} $4 $1 ":"
      ${UnStrStr} $5 $1 ".."
!else
      ${StrStr} $2 $1 "\"
      ${StrStr} $3 $1 "/"
      ${StrStr} $4 $1 ":"
      ${StrStr} $5 $1 ".."
!endif
      ${If} $1 != ""
      ${AndIf} $2 == ""
      ${AndIf} $3 == ""
      ${AndIf} $4 == ""
      ${AndIf} $5 == ""
        Delete "$CleanDir\$1"
      ${EndIf}
      Goto loop_${PREFIX}
    done_${PREFIX}:
    FileClose $0
  ${EndIf}
  Delete "$CleanDir\.installed-root-files.txt"
  Delete "$CleanDir\Uninstall.exe"
  RMDir /r "$CleanDir\agent-host"
  RMDir /r "$CleanDir\plugin-packages"
  RMDir /r "$CleanDir\cli"
  RMDir /r "$CleanDir\media-runtime"
  RMDir "$CleanDir"
FunctionEnd
!macroend
!insertmacro RemovePayload ""
!insertmacro RemovePayload "un."

Function .onInit
  SetShellVarContext current
  # A fixed app-owned directory prevents /D from targeting the user's project data.
  StrCpy $INSTDIR "$LOCALAPPDATA\Programs\光宇无限画布"
  Call RequireClosed
FunctionEnd

Function un.onInit
  SetShellVarContext current
  StrCpy $INSTDIR "$LOCALAPPDATA\Programs\光宇无限画布"
  Call un.RequireClosed
FunctionEnd

Section "光宇无限画布 (required)" SEC_APP
  SectionIn RO
  ClearErrors
  CreateDirectory "$LOCALAPPDATA\Programs"
  GetTempFileName $StageDir "$LOCALAPPDATA\Programs"
  IfErrors prepare_failed
  Delete "$StageDir"
  IfErrors prepare_failed
  GetTempFileName $BackupDir "$LOCALAPPDATA\Programs"
  IfErrors prepare_failed
  Delete "$BackupDir"
  IfErrors prepare_failed
  Goto stage_ready
  prepare_failed:
  MessageBox MB_OK|MB_ICONSTOP "Cannot prepare the installation. Check available disk space and try again. Your existing installation is unchanged." /SD IDOK
  SetErrorLevel 1
  Abort
  stage_ready:
  ClearErrors
  SetOutPath "$StageDir"
  IfErrors prepare_failed
  File /r "${PAYLOAD}\*"
  File /oname=.installed-root-files.txt "${ROOTFILES}"
  WriteUninstaller "$StageDir\Uninstall.exe"
  ${If} ${Errors}
    SetOutPath "$LOCALAPPDATA\Programs"
    RMDir /r "$StageDir"
    MessageBox MB_OK|MB_ICONSTOP "Installation failed. Please run the installer again. Your projects have been preserved." /SD IDOK
    SetErrorLevel 1
    Abort
  ${EndIf}
  # Directory renames stay on the same volume. No old payload is removed until
  # all new files have been extracted successfully.
  SetOutPath "$LOCALAPPDATA\Programs"
  ClearErrors
  IfFileExists "$INSTDIR\*.*" 0 install_new
  Rename "$INSTDIR" "$BackupDir"
  IfErrors install_failed
  install_new:
  Rename "$StageDir" "$INSTDIR"
  IfErrors install_restore
  StrCpy $CleanDir "$BackupDir"
  Call RemovePayload
  Goto install_registered
  install_restore:
  Rename "$BackupDir" "$INSTDIR"
  install_failed:
  RMDir /r "$StageDir"
  MessageBox MB_OK|MB_ICONSTOP "Installation could not replace the application. Close programs using 光宇无限画布 and try again. The previous application is kept at $INSTDIR or $BackupDir." /SD IDOK
  SetErrorLevel 1
  Abort
  install_registered:
  CreateDirectory "$SMPROGRAMS\光宇无限画布"
  CreateShortcut "$SMPROGRAMS\光宇无限画布\光宇无限画布.lnk" "$INSTDIR\光宇无限画布.exe"
  CreateShortcut "$SMPROGRAMS\光宇无限画布\Uninstall.lnk" "$INSTDIR\Uninstall.exe"
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\光宇无限画布" "DisplayName" "光宇无限画布"
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\光宇无限画布" "DisplayVersion" "${VERSION}"
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\光宇无限画布" "UninstallString" '$\"$INSTDIR\Uninstall.exe$\"'
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\光宇无限画布" "QuietUninstallString" '$\"$INSTDIR\Uninstall.exe$\" /S'
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\光宇无限画布" "InstallLocation" "$INSTDIR"
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\光宇无限画布" "DisplayIcon" "$INSTDIR\光宇无限画布.exe"
  WriteRegDWORD HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\光宇无限画布" "NoModify" 1
  WriteRegDWORD HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\光宇无限画布" "NoRepair" 1
SectionEnd

Section /o "Desktop shortcut" SEC_DESKTOP
  CreateShortcut "$DESKTOP\光宇无限画布.lnk" "$INSTDIR\光宇无限画布.exe"
SectionEnd

Section "Uninstall"
  SetOutPath "$LOCALAPPDATA\Programs"
  StrCpy $CleanDir "$INSTDIR"
  Call un.RemovePayload
  Delete "$DESKTOP\光宇无限画布.lnk"
  Delete "$SMPROGRAMS\光宇无限画布\光宇无限画布.lnk"
  Delete "$SMPROGRAMS\光宇无限画布\Uninstall.lnk"
  RMDir "$SMPROGRAMS\光宇无限画布"
  DeleteRegKey HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\光宇无限画布"
SectionEnd
