@echo off
setlocal DisableDelayedExpansion
call "%~dp0Launch.cmd" decrypt %*
set "diagnosticExit=%errorlevel%"
echo Exit code: %diagnosticExit%
pause
exit /b %diagnosticExit%
