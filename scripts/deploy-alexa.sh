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

# Configure ASK CLI credentials using jq for proper JSON escaping
echo "Configuring ASK CLI..."
mkdir -p ~/.ask

# Use jq to safely create the JSON config with proper escaping
jq -n \
  --arg token "$ALEXA_LWA_TOKEN" \
  --arg vendor "$ALEXA_VENDOR_ID" \
  '{
    "profiles": {
      "default": {
        "aws_profile": null,
        "token": {
          "access_token": $token,
          "refresh_token": $token,
          "token_type": "bearer",
          "expires_in": 3600,
          "expires_at": "2099-01-01T00:00:00.000Z"
        },
        "vendor_id": $vendor
      }
    }
  }' > ~/.ask/cli_config

# Install ASK CLI if not present
if ! command -v ask &> /dev/null; then
    echo "Installing ASK CLI..."
    npm install -g ask-cli
fi

# Deploy skill
if [ "$CREATE_NEW" = true ]; then
    echo "Creating new Alexa skill..."

    # Create skill package directory structure
    mkdir -p skill-package
    cp skill-updated.json skill-package/skill.json

    # Deploy new skill
    ask smapi create-skill-for-vendor \
        --manifest file:skill-updated.json \
        --vendor-id "$ALEXA_VENDOR_ID" \
        > skill-creation-response.json

    SKILL_ID=$(cat skill-creation-response.json | jq -r '.skillId')
    echo "Created skill with ID: $SKILL_ID"
    echo "ALEXA_SKILL_ID=$SKILL_ID" >> $GITHUB_ENV

    # Wait for skill creation to complete
    echo "Waiting for skill creation to complete..."
    sleep 10

else
    echo "Updating existing Alexa skill: $ALEXA_SKILL_ID"

    # Update existing skill
    ask smapi update-skill-manifest \
        --skill-id "$ALEXA_SKILL_ID" \
        --manifest file:skill-updated.json \
        --stage development

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

# Enable skill for testing
echo "Enabling skill for testing..."
ask smapi set-skill-enablement \
    --skill-id "$SKILL_ID" \
    --stage development \
    --enablement-status "ENABLED" \
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
