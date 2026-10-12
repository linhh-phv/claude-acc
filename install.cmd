@echo off
rem Double-click to install claude-acc (runs install.ps1 next to this file).
if not exist "%~dp0install.ps1" (
  echo Please extract the zip first ^(right-click - Extract All^), then double-click install.cmd in the extracted folder.
  echo Hay giai nen file zip truoc ^(chuot phai - Extract All^), roi bam dup install.cmd trong thu muc vua giai nen.
  echo.
  pause
  exit /b 1
)
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0install.ps1" %*
echo.
pause
