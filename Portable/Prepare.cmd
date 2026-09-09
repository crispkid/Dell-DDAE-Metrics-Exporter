@echo off
setlocal DisableDelayedExpansion
call "%~dp0Launch.cmd" prepare --root .
set "diagnosticExit=%errorlevel%"
echo Exit code: %diagnosticExit%
pause
exit /b %diagnosticExit%
