@echo off
setlocal DisableDelayedExpansion
call "%~dp0Launch.cmd" keygen %*
set "diagnosticExit=%errorlevel%"
echo Exit code: %diagnosticExit%
pause
exit /b %diagnosticExit%
