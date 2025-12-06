# Extract Alexa credentials from ASK CLI config and set as GitHub secrets
# Usage: powershell -ExecutionPolicy Bypass -File scripts\set-alexa-secrets.ps1
# Usage (auto-confirm): powershell -ExecutionPolicy Bypass -File scripts\set-alexa-secrets.ps1 -y

param(
    [switch]$y
)

$configFile = "$env:USERPROFILE\.ask\cli_config"

if (-not (Test-Path $configFile)) {
    Write-Host "Error: ASK CLI config not found at $configFile" -ForegroundColor Red
    Write-Host "Run 'ask configure' first to set up the ASK CLI"
    exit 1
}

Write-Host "Reading credentials from $configFile..."
Write-Host ""

$config = Get-Content $configFile | ConvertFrom-Json

$clientId = $config.profiles.default.lwa_client_id
$clientSecret = $config.profiles.default.lwa_client_secret
$refreshToken = $config.profiles.default.token.refresh_token

if (-not $clientId) {
    Write-Host "Error: Could not find lwa_client_id in config" -ForegroundColor Red
    exit 1
}

if (-not $clientSecret) {
    Write-Host "Error: Could not find lwa_client_secret in config" -ForegroundColor Red
    exit 1
}

if (-not $refreshToken) {
    Write-Host "Error: Could not find refresh_token in config" -ForegroundColor Red
    exit 1
}

Write-Host "Found credentials:"
Write-Host "  client_id:     $($clientId.Substring(0,30))..."
Write-Host "  client_secret: $($clientSecret.Substring(0,30))..."
Write-Host "  refresh_token: $($refreshToken.Substring(0,30))..."
Write-Host ""

if (-not $y) {
    $confirm = Read-Host "Set these as GitHub secrets? (y/n)"
    if ($confirm -ne 'y') {
        Write-Host "Aborted."
        exit 0
    }
}

Write-Host ""
Write-Host "Setting GitHub secrets..."

gh secret set ALEXA_CLIENT_ID --body $clientId
Write-Host "  [OK] ALEXA_CLIENT_ID set" -ForegroundColor Green

gh secret set ALEXA_SECRET_KEY --body $clientSecret
Write-Host "  [OK] ALEXA_SECRET_KEY set" -ForegroundColor Green

gh secret set ALEXA_LWA_TOKEN --body $refreshToken
Write-Host "  [OK] ALEXA_LWA_TOKEN set" -ForegroundColor Green

Write-Host ""
Write-Host "Done! All secrets have been updated." -ForegroundColor Green
Write-Host ""
Write-Host "You can now re-run the Alexa deployment workflow:"
Write-Host "  gh workflow run deploy-alexa.yml"
