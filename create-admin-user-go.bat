@echo off
REM Create or update admin user in DynamoDB using Go
REM Username: Jeremy
REM Password: Ninja4President

echo Building Go script...
go build -o create-admin-user.exe create-admin-user.go

if %ERRORLEVEL% NEQ 0 (
    echo.
    echo Build failed! Make sure you have Go installed and dependencies downloaded.
    echo Run 'go mod tidy' to download dependencies.
    exit /b %ERRORLEVEL%
)

echo.
echo Running admin user creation script...
echo.
create-admin-user.exe

if %ERRORLEVEL% EQU 0 (
    echo.
    echo Script completed successfully!
) else (
    echo.
    echo Script failed with error code %ERRORLEVEL%
    exit /b %ERRORLEVEL%
)

REM Cleanup
del create-admin-user.exe
