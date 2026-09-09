@echo off
setlocal DisableDelayedExpansion
call "%~dp0Launch.cmd" run --config config.yaml
set "diagnosticExit=%errorlevel%"
echo Exit code: %diagnosticExit%
pause
exit /b %diagnosticExit%
