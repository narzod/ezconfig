@echo off
rem setup.cmd - set up the signage player on Windows.
rem ezconfig asks for the settings; this script does the Windows parts,
rem in the same style. Keep ezconfig.exe, config.ini and schema.ini next
rem to this file.
setlocal
set "here=%~dp0"
set "startup=%APPDATA%\Microsoft\Windows\Start Menu\Programs\Startup"
set "launcher=%startup%\Signage player.cmd"

rem The settings. ezconfig exits 0 when saved (or nothing changed),
rem 1 when saved but a value still breaks the schema, 6 when the person
rem quit or stopped, and 2 to 5 when something is wrong with the files.
"%here%ezconfig.exe" -c "%here%config.ini" -s "%here%schema.ini" ask
if errorlevel 6 goto stopped
if errorlevel 2 goto failed
if errorlevel 1 echo some settings still break the schema; run ezconfig check

rem The Windows parts.
echo.
echo [windows] Windows: How the player starts
echo.
set "now=n"
if exist "%launcher%" set "now=y"
:ask_startup
echo Start the player when this user signs in? (y or n)
set "answer="
set /p "answer=windows.startup [%now%]> "
if not defined answer set "answer=%now%"
if /i "%answer%"=="yes" set "answer=y"
if /i "%answer%"=="no" set "answer=n"
if /i not "%answer%"=="y" if /i not "%answer%"=="n" (
  echo type y or n
  goto ask_startup
)

echo.
echo ----------------------------------------
if /i "%answer%"=="y" (
  > "%launcher%" echo @start "" "%here%player.exe"
  echo the player starts at sign-in
) else (
  if exist "%launcher%" del "%launcher%"
  echo the player doesn't start at sign-in
)
echo setup finished
pause
exit /b 0

:stopped
echo setup stopped; nothing else was changed
pause
exit /b 6

:failed
echo setup could not finish (see the message above)
pause
exit /b 2
