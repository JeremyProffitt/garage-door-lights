@echo off
setlocal enabledelayedexpansion

REM Extract Alexa credentials from ASK CLI config and set as GitHub secrets
REM Usage: scripts\set-alexa-secrets.bat

set "CONFIG_FILE=%USERPROFILE%\.ask\cli_config"

if not exist "%CONFIG_FILE%" (
    echo Error: ASK CLI config not found at %CONFIG_FILE%
    echo Run 'ask configure' first to set up the ASK CLI
    exit /b 1
)

echo Reading credentials from %CONFIG_FILE%...
echo.

REM Use PowerShell to do everything - avoids batch file special character issues
powershell -ExecutionPolicy Bypass -Command ^
  "$config = Get-Content '%CONFIG_FILE%' | ConvertFrom-Json; ^
   $clientId = $config.profiles.default.lwa_client_id; ^
   $clientSecret = $config.profiles.default.lwa_client_secret; ^
   $refreshToken = $config.profiles.default.token.refresh_token; ^
   Write-Host 'Found credentials:'; ^
   Write-Host ('  client_id:     ' + $clientId.Substring(0,30) + '...'); ^
   Write-Host ('  client_secret: ' + $clientSecret.Substring(0,30) + '...'); ^
   Write-Host ('  refresh_token: ' + $refreshToken.Substring(0,30) + '...'); ^
   Write-Host ''; ^
   $confirm = Read-Host 'Set these as GitHub secrets? (y/n)'; ^
   if ($confirm -ne 'y') { Write-Host 'Aborted.'; exit 0 }; ^
   Write-Host ''; ^
   Write-Host 'Setting GitHub secrets...'; ^
   gh secret set ALEXA_CLIENT_ID --body $clientId; ^
   Write-Host '  [OK] ALEXA_CLIENT_ID set'; ^
   gh secret set ALEXA_SECRET_KEY --body $clientSecret; ^
   Write-Host '  [OK] ALEXA_SECRET_KEY set'; ^
   gh secret set ALEXA_LWA_TOKEN --body $refreshToken; ^
   Write-Host '  [OK] ALEXA_LWA_TOKEN set'; ^
   Write-Host ''; ^
   Write-Host 'Done! All secrets have been updated.'; ^
   Write-Host ''; ^
   Write-Host 'You can now re-run the Alexa deployment workflow:'; ^
   Write-Host '  gh workflow run deploy-alexa.yml'"

endlocal
