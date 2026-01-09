@echo off
REM ========================================
REM Particle Firmware Flash Script (Windows)
REM ========================================
REM
REM Compiles and flashes firmware to a Particle device
REM
REM Usage: flash.bat [device-name] [platform]
REM   device-name: Name of the Particle device (default: garage-door-lights)
REM   platform:    Device platform: argon, boron, photon, p2, electron
REM
REM Examples:
REM   flash.bat                          # Flash to garage-door-lights (auto-detect)
REM   flash.bat my-device                # Flash to my-device (auto-detect platform)
REM   flash.bat my-device argon          # Flash to my-device as Argon
REM

setlocal enabledelayedexpansion

REM Configuration
set "DEFAULT_DEVICE=garage-door-lights"
set "FIRMWARE_FILE=candle-lights.ino"
set "OUTPUT_DIR=bin"

REM Parse arguments
set "DEVICE=%~1"
set "PLATFORM=%~2"
set "FLASH_MODE=ota"

REM Handle help flag
if "%~1"=="-h" goto :show_help
if "%~1"=="--help" goto :show_help
if "%~1"=="/?" goto :show_help

REM Handle special flags
if "%~1"=="-l" goto :list_devices
if "%~1"=="--list" goto :list_devices
if "%~1"=="-u" (
    set "FLASH_MODE=usb"
    set "DEVICE=%~2"
    set "PLATFORM=%~3"
)
if "%~1"=="--usb" (
    set "FLASH_MODE=usb"
    set "DEVICE=%~2"
    set "PLATFORM=%~3"
)
if "%~1"=="-c" (
    set "COMPILE_ONLY=1"
    set "DEVICE=%~2"
    set "PLATFORM=%~3"
)
if "%~1"=="--compile" (
    set "COMPILE_ONLY=1"
    set "DEVICE=%~2"
    set "PLATFORM=%~3"
)

REM Use default device if not specified
if "%DEVICE%"=="" set "DEVICE=%DEFAULT_DEVICE%"

echo ========================================
echo   Particle Firmware Flash Script
echo ========================================
echo.

REM Check if Particle CLI is installed
call :check_particle_cli
if errorlevel 1 goto :eof

REM Check if logged in
call :check_login
if errorlevel 1 goto :eof

echo [INFO] Target device: %DEVICE%

REM Check if firmware file exists
if not exist "%FIRMWARE_FILE%" (
    echo [ERROR] Firmware file not found: %FIRMWARE_FILE%
    exit /b 1
)

REM Detect or validate platform
if "%PLATFORM%"=="" (
    call :detect_platform "%DEVICE%"
    if errorlevel 1 (
        echo [ERROR] Could not auto-detect platform.
        echo.
        echo Please specify platform manually:
        echo   flash.bat %DEVICE% ^<platform^>
        echo.
        echo Supported platforms: argon, boron, photon, p2, electron
        exit /b 1
    )
    echo [INFO] Auto-detected platform: !DETECTED_PLATFORM!
    set "PLATFORM=!DETECTED_PLATFORM!"
) else (
    call :validate_platform "%PLATFORM%"
    if errorlevel 1 (
        echo [ERROR] Invalid platform: %PLATFORM%
        echo Supported platforms: argon, boron, photon, p2, electron
        exit /b 1
    )
    echo [INFO] Using specified platform: %PLATFORM%
)

REM Create output directory
if not exist "%OUTPUT_DIR%" mkdir "%OUTPUT_DIR%"

REM Compile firmware
set "OUTPUT_FILE=%OUTPUT_DIR%\%PLATFORM%_firmware.bin"
call :compile_firmware "%PLATFORM%" "%OUTPUT_FILE%"
if errorlevel 1 (
    echo [ERROR] Compilation failed!
    exit /b 1
)

REM Check if compile only
if defined COMPILE_ONLY (
    echo.
    echo ========================================
    echo [SUCCESS] Compilation complete!
    echo ========================================
    exit /b 0
)

REM Flash firmware
echo.
echo [INFO] Flash mode: %FLASH_MODE%

if "%FLASH_MODE%"=="usb" (
    call :flash_usb "%OUTPUT_FILE%"
) else (
    call :flash_ota "%DEVICE%" "%OUTPUT_FILE%"
)

if errorlevel 1 (
    echo [ERROR] Flash failed!
    exit /b 1
)

echo.
echo ========================================
echo [SUCCESS] Done!
echo ========================================
exit /b 0

REM ========================================
REM Functions
REM ========================================

:check_particle_cli
where particle >nul 2>&1
if errorlevel 1 (
    echo [ERROR] Particle CLI is not installed.
    echo Install it with: npm install -g particle-cli
    echo Or visit: https://docs.particle.io/getting-started/developer-tools/cli/
    exit /b 1
)
exit /b 0

:check_login
particle whoami >nul 2>&1
if errorlevel 1 (
    echo [WARNING] Not logged in to Particle Cloud.
    echo Please run: particle login
    exit /b 1
)
for /f "tokens=*" %%a in ('particle whoami 2^>nul') do set "WHOAMI=%%a"
echo [INFO] Logged in as: %WHOAMI%
exit /b 0

:detect_platform
set "TARGET_DEVICE=%~1"
set "DETECTED_PLATFORM="

REM Get device list and find our device
for /f "tokens=*" %%a in ('particle list 2^>nul ^| findstr /i "^%TARGET_DEVICE%"') do (
    set "DEVICE_LINE=%%a"
)

if not defined DEVICE_LINE (
    echo [WARNING] Could not find device '%TARGET_DEVICE%' in your account.
    echo [INFO] Available devices:
    particle list 2>nul
    exit /b 1
)

REM Try to extract platform from device line
echo !DEVICE_LINE! | findstr /i "(argon)" >nul && set "DETECTED_PLATFORM=argon"
echo !DEVICE_LINE! | findstr /i "(boron)" >nul && set "DETECTED_PLATFORM=boron"
echo !DEVICE_LINE! | findstr /i "(photon)" >nul && set "DETECTED_PLATFORM=photon"
echo !DEVICE_LINE! | findstr /i "(p2)" >nul && set "DETECTED_PLATFORM=p2"
echo !DEVICE_LINE! | findstr /i "(electron)" >nul && set "DETECTED_PLATFORM=electron"
echo !DEVICE_LINE! | findstr /i "(photon2)" >nul && set "DETECTED_PLATFORM=p2"

if not defined DETECTED_PLATFORM (
    exit /b 1
)
exit /b 0

:validate_platform
set "CHECK_PLATFORM=%~1"
if /i "%CHECK_PLATFORM%"=="argon" exit /b 0
if /i "%CHECK_PLATFORM%"=="boron" exit /b 0
if /i "%CHECK_PLATFORM%"=="photon" exit /b 0
if /i "%CHECK_PLATFORM%"=="p2" exit /b 0
if /i "%CHECK_PLATFORM%"=="electron" exit /b 0
if /i "%CHECK_PLATFORM%"=="photon2" exit /b 0
exit /b 1

:compile_firmware
set "COMP_PLATFORM=%~1"
set "COMP_OUTPUT=%~2"

echo [INFO] Compiling firmware for %COMP_PLATFORM%...

particle compile "%COMP_PLATFORM%" . --saveTo "%COMP_OUTPUT%"
if errorlevel 1 exit /b 1

echo [SUCCESS] Compiled successfully: %COMP_OUTPUT%
exit /b 0

:flash_ota
set "FLASH_DEVICE=%~1"
set "FLASH_FIRMWARE=%~2"

echo [INFO] Flashing firmware to %FLASH_DEVICE% via OTA...

particle flash "%FLASH_DEVICE%" "%FLASH_FIRMWARE%"
if errorlevel 1 exit /b 1

echo [SUCCESS] Firmware flashed successfully!
exit /b 0

:flash_usb
set "USB_FIRMWARE=%~1"

echo [WARNING] USB flashing requires device in DFU mode.
echo Put your device in DFU mode:
echo   1. Hold both RESET and MODE buttons
echo   2. Release RESET while holding MODE
echo   3. Wait for LED to blink yellow
echo   4. Release MODE
echo.
pause

echo [INFO] Flashing firmware via USB...

particle flash --usb "%USB_FIRMWARE%"
if errorlevel 1 exit /b 1

echo [SUCCESS] Firmware flashed successfully!
exit /b 0

:list_devices
call :check_particle_cli
if errorlevel 1 goto :eof
call :check_login
if errorlevel 1 goto :eof
echo [INFO] Available devices:
particle list
exit /b 0

:show_help
echo Particle Firmware Flash Script (Windows)
echo.
echo Usage: flash.bat [options] [device-name] [platform]
echo.
echo Arguments:
echo   device-name    Name of the Particle device (default: garage-door-lights)
echo   platform       Device platform: argon, boron, photon, p2, electron
echo                  If not specified, auto-detects from Particle Cloud
echo.
echo Options:
echo   -h, --help     Show this help message
echo   -u, --usb      Flash via USB (DFU mode) instead of OTA
echo   -l, --list     List available devices
echo   -c, --compile  Compile only (don't flash)
echo.
echo Examples:
echo   flash.bat                           # Flash to garage-door-lights (auto-detect)
echo   flash.bat my-device                 # Flash to my-device (auto-detect platform)
echo   flash.bat my-device argon           # Flash to my-device as Argon
echo   flash.bat -u my-device              # Flash via USB
echo   flash.bat -c "" photon              # Compile for Photon only
exit /b 0
