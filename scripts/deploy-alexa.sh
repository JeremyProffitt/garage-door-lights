#!/bin/bash
# Deploy Alexa integration to AWS

set -e

# Configuration
STACK_NAME="${STACK_NAME:-garage-lights}"
AWS_REGION="${AWS_REGION:-us-east-1}"
ALEXA_SKILL_ID="${ALEXA_SKILL_ID:-}"
ALEXA_CLIENT_SECRET="${ALEXA_CLIENT_SECRET:-$(openssl rand -base64 32)}"

echo "=== Deploying Alexa Integration ==="
echo "Stack Name: $STACK_NAME"
echo "Region: $AWS_REGION"

# Check if Alexa Skill ID is provided
if [ -z "$ALEXA_SKILL_ID" ]; then
    echo ""
    echo "WARNING: ALEXA_SKILL_ID not set. The Alexa Smart Home skill will not be functional."
    echo "Set ALEXA_SKILL_ID environment variable after creating the skill in Amazon Developer Console."
    echo ""
fi

# Copy shared files to all functions
echo "Syncing shared files to all functions..."
cd "$(dirname "$0")/.."

for dir in backend/functions/*/shared; do
    if [ -d "$dir" ]; then
        cp backend/shared/*.go "$dir/" 2>/dev/null || true
        echo "  Updated: $dir"
    fi
done

# Build and deploy with SAM
echo ""
echo "Building and deploying with SAM..."
sam build --parallel

sam deploy \
    --stack-name "$STACK_NAME" \
    --region "$AWS_REGION" \
    --capabilities CAPABILITY_IAM \
    --parameter-overrides \
        AlexaSkillId="$ALEXA_SKILL_ID" \
        AlexaClientSecret="$ALEXA_CLIENT_SECRET" \
    --no-confirm-changeset

# Get outputs
echo ""
echo "=== Deployment Complete ==="
echo ""
echo "Alexa Configuration Values:"
aws cloudformation describe-stacks \
    --stack-name "$STACK_NAME" \
    --region "$AWS_REGION" \
    --query 'Stacks[0].Outputs[?contains(OutputKey, `Alexa`) || contains(OutputKey, `OAuth`)].{Key:OutputKey,Value:OutputValue}' \
    --output table

echo ""
echo "Next Steps:"
echo "1. Create an Alexa Smart Home Skill in the Amazon Developer Console"
echo "2. Configure Account Linking with the OAuth URLs shown above"
echo "3. Set the Lambda ARN as the skill endpoint"
echo "4. Re-deploy with the Alexa Skill ID: ALEXA_SKILL_ID=amzn1.ask.skill.xxx ./scripts/deploy-alexa.sh"
