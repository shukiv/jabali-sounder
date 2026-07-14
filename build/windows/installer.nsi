; NSIS installer for the Jabali Sounder desktop app (SND-95).
; Per-user install (no admin), Start-menu entry, optional desktop shortcut,
; Apps & Features registration, and an uninstaller that preserves user data.
;
; Build (on Linux or Windows) with makensis, passing version + output name and
; expecting jabali-sounder-desktop.exe + app.ico in the working directory:
;   makensis -DVERSION=0.5.11 -DOUTFILE=out/jabali-sounder-setup-0.5.11-amd64.exe build/windows/installer.nsi

Unicode true
!ifndef VERSION
  !define VERSION "0.0.0"
!endif
!ifndef OUTFILE
  !define OUTFILE "jabali-sounder-setup.exe"
!endif

!define APPNAME "Jabali Sounder"
!define EXENAME "jabali-sounder-desktop.exe"
!define PUBLISHER "Jabali Panel"
!define UNINSTKEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\JabaliSounder"

Name "${APPNAME}"
OutFile "${OUTFILE}"
; Per-user install: no administrator rights required.
RequestExecutionLevel user
InstallDir "$LOCALAPPDATA\Programs\JabaliSounder"
InstallDirRegKey HKCU "Software\JabaliSounder" "InstallDir"
SetCompressor /SOLID lzma
Icon "app.ico"
UninstallIcon "app.ico"

!include "MUI2.nsh"
!define MUI_ICON "app.ico"
!define MUI_UNICON "app.ico"
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_COMPONENTS
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES
!insertmacro MUI_LANGUAGE "English"

Section "Jabali Sounder (required)" SEC_APP
  SectionIn RO
  SetOutPath "$INSTDIR"
  ; Overwrites program files in place on upgrade/repair; user data in
  ; %APPDATA% is never touched here, so it is preserved across upgrades.
  File "${EXENAME}"
  File "app.ico"

  WriteRegStr HKCU "Software\JabaliSounder" "InstallDir" "$INSTDIR"

  CreateDirectory "$SMPROGRAMS\${APPNAME}"
  CreateShortcut "$SMPROGRAMS\${APPNAME}\${APPNAME}.lnk" "$INSTDIR\${EXENAME}" "" "$INSTDIR\app.ico"

  ; Apps & Features (per-user) registration.
  WriteRegStr   HKCU "${UNINSTKEY}" "DisplayName"     "${APPNAME}"
  WriteRegStr   HKCU "${UNINSTKEY}" "DisplayVersion"  "${VERSION}"
  WriteRegStr   HKCU "${UNINSTKEY}" "Publisher"       "${PUBLISHER}"
  WriteRegStr   HKCU "${UNINSTKEY}" "DisplayIcon"     "$INSTDIR\app.ico"
  WriteRegStr   HKCU "${UNINSTKEY}" "InstallLocation" "$INSTDIR"
  WriteRegStr   HKCU "${UNINSTKEY}" "UninstallString" "$INSTDIR\uninstall.exe"
  WriteRegStr   HKCU "${UNINSTKEY}" "QuietUninstallString" '"$INSTDIR\uninstall.exe" /S'
  WriteRegDWORD HKCU "${UNINSTKEY}" "NoModify" 1
  WriteRegDWORD HKCU "${UNINSTKEY}" "NoRepair" 1

  WriteUninstaller "$INSTDIR\uninstall.exe"
SectionEnd

Section /o "Desktop shortcut" SEC_DESKTOP
  CreateShortcut "$DESKTOP\${APPNAME}.lnk" "$INSTDIR\${EXENAME}" "" "$INSTDIR\app.ico"
SectionEnd

Section "Uninstall"
  ; Remove program files + registrations only. User data under %APPDATA% is
  ; intentionally preserved by default (matches the deb/rpm behaviour).
  Delete "$INSTDIR\${EXENAME}"
  Delete "$INSTDIR\app.ico"
  Delete "$INSTDIR\uninstall.exe"
  RMDir  "$INSTDIR"
  Delete "$SMPROGRAMS\${APPNAME}\${APPNAME}.lnk"
  RMDir  "$SMPROGRAMS\${APPNAME}"
  Delete "$DESKTOP\${APPNAME}.lnk"
  DeleteRegKey HKCU "${UNINSTKEY}"
  DeleteRegKey HKCU "Software\JabaliSounder"
SectionEnd
