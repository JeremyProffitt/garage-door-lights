#!/bin/bash

# Script to deploy Alexa Smart Home Skill
# This script uses the ASK CLI to deploy the skill configuration

set -e

echo "======================================"
echo "Alexa Skill Deployment"
echo "======================================"

# Check required environment variables
if [ -z "$ALEXA_VENDOR_ID" ]; then
    echo "Error: ALEXA_VENDOR_ID not set"
    exit 1
fi

if [ -z "$ALEXA_LWA_TOKEN" ]; then
    echo "Error: ALEXA_LWA_TOKEN not set"
    exit 1
fi

# Parse LWA token JSON to extract tokens
ACCESS_TOKEN=$(echo "$ALEXA_LWA_TOKEN" | jq -r '.access_token // empty')
REFRESH_TOKEN=$(echo "$ALEXA_LWA_TOKEN" | jq -r '.refresh_token // empty')
EXPIRES_AT=$(echo "$ALEXA_LWA_TOKEN" | jq -r '.expires_at // empty')

if [ -z "$ACCESS_TOKEN" ]; then
    # If parsing failed, assume it's already a plain token
    ACCESS_TOKEN="$ALEXA_LWA_TOKEN"
    echo "Using token as-is (not JSON)"
else
    echo "LWA token parsed successfully"

    # Check if token is expired or will expire soon
    if [ -n "$EXPIRES_AT" ]; then
        CURRENT_TIME=$(date -u +%s)
        EXPIRY_TIME=$(date -d "$EXPIRES_AT" +%s 2>/dev/null || date -j -f "%Y-%m-%dT%H:%M:%S" "${EXPIRES_AT%.*}" +%s 2>/dev/null)

        if [ -n "$EXPIRY_TIME" ] && [ "$CURRENT_TIME" -ge "$EXPIRY_TIME" ]; then
            echo "Access token expired, refreshing..."

            if [ -z "$REFRESH_TOKEN" ]; then
                echo "Error: No refresh token available"
                exit 1
            fi

            # Refresh the access token using LWA refresh endpoint
            if [ -z "$ALEXA_CLIENT_ID" ] || [ -z "$ALEXA_CLIENT_SECRET" ]; then
                echo "Error: ALEXA_CLIENT_ID and ALEXA_CLIENT_SECRET required for token refresh"
                exit 1
            fi

            REFRESH_RESPONSE=$(curl -s -X POST https://api.amazon.com/auth/o2/token \
                -H "Content-Type: application/x-www-form-urlencoded" \
                -d "grant_type=refresh_token&refresh_token=$REFRESH_TOKEN&client_id=$ALEXA_CLIENT_ID&client_secret=$ALEXA_CLIENT_SECRET")

            NEW_ACCESS_TOKEN=$(echo "$REFRESH_RESPONSE" | jq -r '.access_token // empty')

            if [ -n "$NEW_ACCESS_TOKEN" ]; then
                ACCESS_TOKEN="$NEW_ACCESS_TOKEN"
                echo "Access token refreshed successfully"
            else
                echo "Warning: Token refresh failed, using expired token"
                echo "Response: $REFRESH_RESPONSE"
            fi
        else
            echo "Access token is valid"
        fi
    fi
fi

if [ -z "$ALEXA_SKILL_ID" ]; then
    echo "Warning: ALEXA_SKILL_ID not set - will create new skill"
    CREATE_NEW=true
else
    CREATE_NEW=false
fi

# Get Lambda ARN from CloudFormation
STACK_NAME=${STACK_NAME:-"candle-lights-prod"}
AWS_REGION=${AWS_REGION:-"us-east-1"}

echo "Getting Lambda ARN from CloudFormation stack: $STACK_NAME"
LAMBDA_ARN=$(aws cloudformation describe-stacks \
    --stack-name "$STACK_NAME" \
    --region "$AWS_REGION" \
    --query "Stacks[0].Outputs[?OutputKey=='AlexaSkillEndpoint'].OutputValue" \
    --output text)

if [ -z "$LAMBDA_ARN" ]; then
    echo "Error: Could not get Lambda ARN from CloudFormation"
    exit 1
fi

echo "Lambda ARN: $LAMBDA_ARN"

# Get AWS Account ID from Lambda ARN
AWS_ACCOUNT_ID=$(echo $LAMBDA_ARN | cut -d':' -f5)
echo "AWS Account ID: $AWS_ACCOUNT_ID"

# Update skill.json with Lambda ARN
echo "Updating skill.json with Lambda ARN..."
cd alexa-skill

# Create temporary skill.json with updated Lambda ARN
cat skill.json | \
    sed "s|arn:aws:lambda:us-east-1:ACCOUNT_ID:function:FUNCTION_NAME|$LAMBDA_ARN|g" | \
    sed "s|ACCOUNT_ID|$AWS_ACCOUNT_ID|g" \
    > skill-updated.json

# Deploy skill using SMAPI REST API directly
if [ "$CREATE_NEW" = true ]; then
    echo "Creating new Alexa skill..."

    # Read the skill manifest JSON
    SKILL_MANIFEST=$(cat skill-updated.json)

    # Create skill using SMAPI REST API
    SMAPI_RESPONSE=$(curl -X POST \
        -H "Authorization: Bearer $ACCESS_TOKEN" \
        -H "Content-Type: application/json" \
        -d "$SKILL_MANIFEST" \
        "https://api.amazonalexa.com/v1/skills")

    echo "API Response: $SMAPI_RESPONSE"

    SKILL_ID=$(echo "$SMAPI_RESPONSE" | jq -r '.skillId // empty')

    if [ -z "$SKILL_ID" ]; then
        echo "Error: Failed to create skill"
        exit 1
    fi

    echo "Created skill with ID: $SKILL_ID"

    if [ -n "$GITHUB_ENV" ]; then
        echo "ALEXA_SKILL_ID=$SKILL_ID" >> $GITHUB_ENV
    fi

    # Wait for skill creation to complete
    echo "Waiting for skill creation to complete..."
    sleep 10

else
    echo "Updating existing Alexa skill: $ALEXA_SKILL_ID"

    # Read the skill manifest JSON
    SKILL_MANIFEST=$(cat skill-updated.json)

    # Update existing skill using SMAPI REST API
    curl -X PUT \
        -H "Authorization: Bearer $ACCESS_TOKEN" \
        -H "Content-Type: application/json" \
        -d "$SKILL_MANIFEST" \
        "https://api.amazonalexa.com/v1/skills/$ALEXA_SKILL_ID/stages/development/manifest"

    SKILL_ID=$ALEXA_SKILL_ID
fi

# Add Lambda trigger permission for Alexa
echo "Adding Lambda trigger permission for Alexa..."
aws lambda add-permission \
    --function-name "$LAMBDA_ARN" \
    --statement-id "AlexaSkill-$SKILL_ID" \
    --action "lambda:InvokeFunction" \
    --principal alexa-appkit.amazon.com \
    --event-source-token "$SKILL_ID" \
    --region "$AWS_REGION" \
    || echo "Permission may already exist"

# Enable skill for testing using SMAPI REST API
echo "Enabling skill for testing..."
curl -X PUT \
    -H "Authorization: Bearer $ACCESS_TOKEN" \
    -H "Content-Type: application/json" \
    "https://api.amazonalexa.com/v1/skills/$SKILL_ID/stages/development/enablement" \
    -d '{"stage": "development"}' \
    || echo "Skill may already be enabled"

echo ""
echo "======================================"
echo "Alexa Skill Deployment Complete!"
echo "======================================"
echo "Skill ID: $SKILL_ID"
echo "Lambda ARN: $LAMBDA_ARN"
echo ""
echo "Next steps:"
echo "1. Configure account linking in Alexa Developer Console"
echo "2. Test with 'Alexa, discover devices'"
echo "3. Submit for certification when ready"
echo ""
echo "Alexa Developer Console: https://developer.amazon.com/alexa/console/ask/build/$SKILL_ID"
