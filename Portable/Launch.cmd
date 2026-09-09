@echo off
setlocal DisableDelayedExpansion
cd /d "%~dp0"
set "nativeArch=%PROCESSOR_ARCHITECTURE%"
if defined PROCESSOR_ARCHITEW6432 set "nativeArch=%PROCESSOR_ARCHITEW6432%"
set "targetArch="
if /i "%nativeArch%"=="AMD64" set "targetArch=windows-amd64"
if /i "%nativeArch%"=="ARM64" set "targetArch=windows-arm64"
if not defined targetArch (
  echo Unsupported architecture. Windows 11 x64 or ARM64 is required.
  exit /b 2
)
if not exist "bin\%targetArch%\ddae-diagnose.exe" (
  echo Diagnostic executable missing. Extract the complete Portable folder.
  exit /b 2
)
if /i "%~1"=="exporter" goto exporter
"bin\%targetArch%\ddae-diagnose.exe" %*
exit /b %errorlevel%
:exporter
"bin\%targetArch%\ddae-diagnose.exe" verify-bundle --root .
if errorlevel 1 exit /b 2
rem Only this child process is changed. Keep the separate example resources-only
rem even when the operator's normal shell has exporter environment overrides.
set "DDAE_RESOURCE_MONITORING_ENABLED=true"
set "DDAE_ALERT_MONITORING_ENABLED=false"
set "DDAE_SERVICEABILITY_LOG_MONITORING_ENABLED=false"
set "EXPORTER_LISTEN_ADDRESS=127.0.0.1:9469"
"bin\%targetArch%\ddae-exporter.exe" --config exporter.yaml
exit /b %errorlevel%
