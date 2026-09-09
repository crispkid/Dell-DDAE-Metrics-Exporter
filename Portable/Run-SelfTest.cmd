@echo off
setlocal DisableDelayedExpansion
call "%~dp0Launch.cmd" self-test --output results
set "diagnosticExit=%errorlevel%"
echo Exit code: %diagnosticExit%
pause
exit /b %diagnosticExit%
