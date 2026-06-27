; Teaveloper Runner — Windows 설치 프로그램 (NSIS)
; 리눅스(포털/CI)에서 makensis 로 컴파일된다(크로스 빌드).
;   저장소 루트에서:  makensis installer/installer.nsi
;   사전 준비:        dist/teaveloper-runner.exe (먼저 빌드)  → ./build.sh
;
; 무설치(더블클릭) 방식과 별개로, 설치형도 제공한다. 관리자 권한 불필요(per-user).
; 설치 시 "Windows 시작 시 항상 실행(자동 시작)" 옵션 포함.

Unicode true
SetCompressor /SOLID lzma

!include "MUI2.nsh"

!define APPNAME    "Teaveloper Runner"
!define EXE        "teaveloper-runner.exe"
!define TASK       "TeaveloperRunner"     ; 앱 트레이 토글과 동일한 작업 이름
!define COMPANY    "TeaveloperHQ"
!define UNINST_KEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\${APPNAME}"

; 경로는 이 .nsi 파일(installer/) 기준 → 저장소 루트는 ..\
Name "${APPNAME}"
OutFile "..\dist\teaveloper-runner-setup.exe"
RequestExecutionLevel user
InstallDir "$LOCALAPPDATA\Programs\${APPNAME}"
InstallDirRegKey HKCU "Software\${APPNAME}" "InstallDir"

!define MUI_ICON   "..\assets\icon.ico"
!define MUI_UNICON "..\assets\icon.ico"
!define MUI_ABORTWARNING

; ── 설치 페이지 ──
!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_COMPONENTS          ; 여기서 "항상 실행" 옵션 토글
!insertmacro MUI_PAGE_INSTFILES
; 완료 페이지: 설치 폴더 열기(여기에 config.json·app 폴더를 넣게 안내)
!define MUI_FINISHPAGE_SHOWREADME ""
!define MUI_FINISHPAGE_SHOWREADME_TEXT "설치 폴더 열기 (config.json 과 app 폴더를 여기에 두세요)"
!define MUI_FINISHPAGE_SHOWREADME_FUNCTION OpenInstallDir
!insertmacro MUI_PAGE_FINISH

; ── 제거 페이지 ──
!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES

!insertmacro MUI_LANGUAGE "Korean"
!insertmacro MUI_LANGUAGE "English"

; ── 섹션 ──
Section "${APPNAME} (필수)" SecCore
  SectionIn RO
  SetOutPath "$INSTDIR"
  File "..\dist\${EXE}"
  CreateDirectory "$INSTDIR\app"

  ; 바로가기 (시작 메뉴 + 바탕화면)
  CreateDirectory "$SMPROGRAMS\${APPNAME}"
  CreateShortcut  "$SMPROGRAMS\${APPNAME}\${APPNAME}.lnk" "$INSTDIR\${EXE}" "" "$INSTDIR\${EXE}" 0
  CreateShortcut  "$DESKTOP\${APPNAME}.lnk"               "$INSTDIR\${EXE}" "" "$INSTDIR\${EXE}" 0

  ; 제거 정보
  WriteRegStr HKCU "Software\${APPNAME}" "InstallDir" "$INSTDIR"
  WriteUninstaller "$INSTDIR\uninstall.exe"
  WriteRegStr HKCU "${UNINST_KEY}" "DisplayName"     "${APPNAME}"
  WriteRegStr HKCU "${UNINST_KEY}" "DisplayIcon"     "$INSTDIR\${EXE}"
  WriteRegStr HKCU "${UNINST_KEY}" "Publisher"       "${COMPANY}"
  WriteRegStr HKCU "${UNINST_KEY}" "UninstallString" "$\"$INSTDIR\uninstall.exe$\""
  WriteRegStr HKCU "${UNINST_KEY}" "InstallLocation" "$INSTDIR"
SectionEnd

Section "Windows 시작 시 항상 실행 (자동 시작)" SecAutostart
  ; 관리자 권한 없이 로그온 트리거 등록. 앱 트레이 토글과 동일한 작업 이름이라
  ; 나중에 트레이에서 끄면 같은 작업이 해제된다.
  nsExec::ExecToLog 'schtasks /Create /F /SC ONLOGON /RL LIMITED /TN "${TASK}" /TR "$\"$INSTDIR\${EXE}$\""'
SectionEnd

; 컴포넌트 설명 (Components 페이지 우측)
!insertmacro MUI_FUNCTION_DESCRIPTION_BEGIN
  !insertmacro MUI_DESCRIPTION_TEXT ${SecCore}      "프로그램 본체(필수)."
  !insertmacro MUI_DESCRIPTION_TEXT ${SecAutostart} "컴퓨터를 켤 때 자동으로 실행되어 항상 연결을 유지합니다. (권장)"
!insertmacro MUI_FUNCTION_DESCRIPTION_END

Function OpenInstallDir
  ExecShell "open" "$INSTDIR"
FunctionEnd

Section "Uninstall"
  ; 자동 시작 해제
  nsExec::ExecToLog 'schtasks /Delete /F /TN "${TASK}"'

  ; 프로그램 파일만 제거 — 사용자 데이터(config.json, app/, teaveloper-data.db)는 보존
  Delete "$INSTDIR\${EXE}"
  Delete "$INSTDIR\uninstall.exe"
  Delete "$SMPROGRAMS\${APPNAME}\${APPNAME}.lnk"
  RMDir  "$SMPROGRAMS\${APPNAME}"
  Delete "$DESKTOP\${APPNAME}.lnk"

  DeleteRegKey HKCU "${UNINST_KEY}"
  DeleteRegKey HKCU "Software\${APPNAME}"

  ; 데이터가 없으면 폴더도 정리(있으면 남겨둠)
  RMDir "$INSTDIR\app"
  RMDir "$INSTDIR"
SectionEnd
