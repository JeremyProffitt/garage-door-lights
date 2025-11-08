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

    # Create skill using SMAPI REST API
    SMAPI_RESPONSE=$(curl -X POST \
        -H "Authorization: Bearer $ALEXA_LWA_TOKEN" \
        -H "Content-Type: application/json" \
        -d @skill-updated.json \
        "https://api.amazonalexa.com/v1/skills" \
        2>/dev/null)

    SKILL_ID=$(echo "$SMAPI_RESPONSE" | jq -r '.skillId // empty')

    if [ -z "$SKILL_ID" ]; then
        echo "Error: Failed to create skill"
        echo "Response: $SMAPI_RESPONSE"
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

    # Update existing skill using SMAPI REST API
    curl -X PUT \
        -H "Authorization: Bearer $ALEXA_LWA_TOKEN" \
        -H "Content-Type: application/json" \
        -d @skill-updated.json \
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
    -H "Authorization: Bearer $ALEXA_LWA_TOKEN" \
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
