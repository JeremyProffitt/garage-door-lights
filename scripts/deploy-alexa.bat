@echo off
REM Deploy Alexa integration to AWS

setlocal enabledelayedexpansion

REM Configuration
if "%STACK_NAME%"=="" set STACK_NAME=garage-lights
if "%AWS_REGION%"=="" set AWS_REGION=us-east-1
if "%ALEXA_CLIENT_ID%"=="" set ALEXA_CLIENT_ID=garage-lights-alexa

echo === Deploying Alexa Integration ===
echo Stack Name: %STACK_NAME%
echo Region: %AWS_REGION%

REM Check if Alexa Skill ID is provided
if "%ALEXA_SKILL_ID%"=="" (
    echo.
    echo WARNING: ALEXA_SKILL_ID not set. The Alexa Smart Home skill will not be functional.
    echo Set ALEXA_SKILL_ID environment variable after creating the skill in Amazon Developer Console.
    echo.
)

REM Navigate to project root
cd /d "%~dp0\.."

REM Copy shared files to all functions
echo Syncing shared files to all functions...
for /d %%d in (backend\functions\*) do (
    if exist "%%d\shared" (
        copy /Y backend\shared\*.go "%%d\shared\" >nul 2>&1
        echo   Updated: %%d\shared
    )
)

REM Build with SAM
echo.
echo Building with SAM...
call sam build --parallel
if errorlevel 1 (
    echo SAM build failed!
    exit /b 1
)

REM Deploy with SAM
echo.
echo Deploying with SAM...
call sam deploy ^
    --stack-name %STACK_NAME% ^
    --region %AWS_REGION% ^
    --capabilities CAPABILITY_IAM ^
    --parameter-overrides AlexaSkillId=%ALEXA_SKILL_ID% AlexaClientId=%ALEXA_CLIENT_ID% AlexaClientSecret=%ALEXA_CLIENT_SECRET% ^
    --no-confirm-changeset

if errorlevel 1 (
    echo SAM deploy failed!
    exit /b 1
)

echo.
echo === Deployment Complete ===
echo.
echo Run the following to see Alexa configuration values:
echo   aws cloudformation describe-stacks --stack-name %STACK_NAME% --region %AWS_REGION% --query "Stacks[0].Outputs"
echo.
echo Next Steps:
echo 1. Create an Alexa Smart Home Skill in the Amazon Developer Console
echo 2. Configure Account Linking with the OAuth URLs from the outputs
echo 3. Set the Lambda ARN as the skill endpoint
echo 4. Re-deploy with: set ALEXA_SKILL_ID=amzn1.ask.skill.xxx ^& scripts\deploy-alexa.bat
