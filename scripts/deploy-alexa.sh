#!/bin/bash

# Script to deploy Alexa Smart Home Skill
# Uses ASK CLI for automated deployment

set -e

echo "======================================"
echo "Alexa Skill Deployment"
echo "======================================"

# Check required environment variables
if [ -z "$ALEXA_LWA_TOKEN" ]; then
    echo "Error: ALEXA_LWA_TOKEN not set"
    echo "This should contain the refresh_token from ASK CLI configuration"
    exit 1
fi

# Install ASK CLI if not present
if ! command -v ask &> /dev/null; then
    echo "Installing ASK CLI..."
    npm install -g ask-cli
fi

echo "✓ ASK CLI installed"

# Configure ASK CLI with LWA token
echo "Configuring ASK CLI with LWA token..."
mkdir -p ~/.ask

# Extract refresh token from ALEXA_LWA_TOKEN (supports both JSON and plain string)
REFRESH_TOKEN=""
VENDOR_ID_FROM_JSON=""

if echo "$ALEXA_LWA_TOKEN" | jq -e . >/dev/null 2>&1; then
    # Token is JSON, extract refresh_token
    echo "Detected JSON format token, extracting refresh_token..."
    REFRESH_TOKEN=$(echo "$ALEXA_LWA_TOKEN" | jq -r '.refresh_token')

    if [ -z "$REFRESH_TOKEN" ] || [ "$REFRESH_TOKEN" = "null" ]; then
        echo "Error: Failed to extract refresh_token from JSON"
        exit 1
    fi

    # Try to extract vendor_id if present
    VENDOR_ID_FROM_JSON=$(echo "$ALEXA_LWA_TOKEN" | jq -r '.vendor_id // empty')
else
    # Token is plain string
    REFRESH_TOKEN="$ALEXA_LWA_TOKEN"
fi

# Determine vendor ID (priority: env var > JSON > auto-fetch)
VENDOR_ID=""
if [ -n "$ALEXA_VENDOR_ID" ]; then
    VENDOR_ID="$ALEXA_VENDOR_ID"
    echo "✓ Using vendor ID from ALEXA_VENDOR_ID: $VENDOR_ID"
elif [ -n "$VENDOR_ID_FROM_JSON" ]; then
    VENDOR_ID="$VENDOR_ID_FROM_JSON"
    echo "✓ Using vendor ID from JSON token: $VENDOR_ID"
fi

# Create ASK CLI config
if [ -n "$VENDOR_ID" ]; then
    jq -n \
      --arg refresh_token "$REFRESH_TOKEN" \
      --arg vendor_id "$VENDOR_ID" \
      '{
        profiles: {
          default: {
            aws_profile: "default",
            token: {
              access_token: "",
              refresh_token: $refresh_token,
              token_type: "bearer",
              expires_in: 3600,
              expires_at: "1970-01-01T00:00:00.000Z"
            },
            vendor_id: $vendor_id
          }
        }
      }' > ~/.ask/cli_config
    echo "✓ ASK CLI configured with refresh token and vendor ID"
else
    # Create config without vendor_id and try to fetch it
    jq -n \
      --arg refresh_token "$REFRESH_TOKEN" \
      '{
        profiles: {
          default: {
            aws_profile: "default",
            token: {
              access_token: "",
              refresh_token: $refresh_token,
              token_type: "bearer",
              expires_in: 3600,
              expires_at: "1970-01-01T00:00:00.000Z"
            }
          }
        }
      }' > ~/.ask/cli_config

    echo "Attempting to fetch vendor ID..."
    set +e
    VENDOR_ID_RESPONSE=$(ask smapi list-vendors 2>&1)
    SMAPI_EXIT_CODE=$?
    set -e

    if [ $SMAPI_EXIT_CODE -eq 0 ]; then
        VENDOR_ID=$(echo "$VENDOR_ID_RESPONSE" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)

        if [ -n "$VENDOR_ID" ]; then
            echo "✓ Vendor ID retrieved: $VENDOR_ID"
            jq --arg vendor_id "$VENDOR_ID" '.profiles.default.vendor_id = $vendor_id' ~/.ask/cli_config > ~/.ask/cli_config.tmp && mv ~/.ask/cli_config.tmp ~/.ask/cli_config
            echo "✓ ASK CLI configured"
        else
            echo "Warning: Could not parse vendor ID, ASK CLI will attempt to fetch during deployment"
        fi
    else
        echo "Error: Could not fetch vendor ID"
        echo "Please set ALEXA_VENDOR_ID as a GitHub secret"
        exit 1
    fi
fi

# Get Lambda ARN from CloudFormation
STACK_NAME=${STACK_NAME:-"candle-lights-prod"}
AWS_REGION=${AWS_REGION:-"us-east-2"}

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

# Backup original and update
cp skill.json skill.json.bak
sed -i.tmp "s|ACCOUNT_ID|${AWS_ACCOUNT_ID}|g" skill.json
sed -i.tmp "s|arn:aws:lambda:[^:]*:${AWS_ACCOUNT_ID}:function:[^\"]*|${LAMBDA_ARN}|g" skill.json
rm -f skill.json.tmp

echo "✓ skill.json updated"

# Setup skill package structure for ASK CLI v2
echo "Setting up skill package structure..."
mkdir -p skill-package/interactionModels/custom

# Copy skill manifest
cp skill.json skill-package/skill.json

# Copy interaction model if it exists
if [ -f interactionModel.json ]; then
    cp interactionModel.json skill-package/interactionModels/custom/en-US.json
fi

# Create ask-resources.json
cat > ask-resources.json << 'EOF'
{
  "askcliResourcesVersion": "2020-03-31",
  "profiles": {
    "default": {
      "skillMetadata": {
        "src": "./skill-package"
      }
    }
  }
}
EOF

echo "✓ Skill package structure created"

# Deploy skill using ASK CLI
echo "Deploying skill with ASK CLI..."
set +e
ask deploy > /tmp/ask-deploy.log 2>&1
DEPLOY_RESULT=$?
set -e

if [ $DEPLOY_RESULT -eq 0 ]; then
    # Extract skill ID
    SKILL_ID=""
    if [ -f .ask/ask-states.json ]; then
        SKILL_ID=$(grep -o '"skillId":"[^"]*"' .ask/ask-states.json | cut -d'"' -f4 | head -1)
    fi

    echo "✓ Skill deployment complete!"

    if [ -n "$SKILL_ID" ]; then
        echo "Skill ID: $SKILL_ID"

        # Add Lambda trigger permission
        echo "Adding Lambda trigger permission..."
        FUNCTION_NAME=$(echo "$LAMBDA_ARN" | cut -d':' -f7)
        aws lambda add-permission \
            --function-name "$FUNCTION_NAME" \
            --statement-id "AlexaSkill-$SKILL_ID" \
            --action "lambda:InvokeFunction" \
            --principal "alexa-appkit.amazon.com" \
            --event-source-token "$SKILL_ID" \
            --region "$AWS_REGION" \
            2>&1 | grep -v "ResourceConflictException" || true

        echo "✓ Lambda permissions configured"
    fi

    # Restore backup
    mv skill.json.bak skill.json

    cd ..

    echo ""
    echo "======================================"
    echo "Alexa Skill Deployment Complete!"
    echo "======================================"
    echo "Lambda ARN: $LAMBDA_ARN"
    if [ -n "$SKILL_ID" ]; then
        echo "Skill ID: $SKILL_ID"
        echo ""
        echo "Next steps:"
        echo "1. Go to https://developer.amazon.com/alexa/console/ask"
        echo "2. Find your skill: Candle Lights Controller"
        echo "3. Enable testing in Development"
        echo "4. Test with: 'Alexa, discover devices'"
    fi
    echo ""
else
    echo "Error: Skill deployment failed"
    echo ""
    echo "Deployment log:"
    cat /tmp/ask-deploy.log
    echo ""

    # Restore backup
    mv skill.json.bak skill.json

    cd ..
    exit 1
fi
