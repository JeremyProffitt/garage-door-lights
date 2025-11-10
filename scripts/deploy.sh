#!/bin/bash

# Deployment script for the Candle Lights Controller
# This script builds and deploys the entire application using AWS SAM

set -e

# Load environment variables
if [ -f .env ]; then
    export $(cat .env | grep -v '^#' | xargs)
fi

# Default values
STACK_NAME=${STACK_NAME:-"candle-lights-prod"}
AWS_REGION=${AWS_REGION:-"us-east-1"}
DOMAIN_NAME=${DOMAIN_NAME:-"lights.jeremy.ninja"}

echo "==================================="
echo "Candle Lights Controller Deployment"
echo "==================================="
echo "Stack Name: $STACK_NAME"
echo "Region: $AWS_REGION"
echo "Domain: $DOMAIN_NAME"
echo "==================================="
echo ""

# Check required environment variables
if [ -z "$HOSTED_ZONE_ID" ]; then
    echo "Error: HOSTED_ZONE_ID not set"
    exit 1
fi

if [ -z "$CERTIFICATE_ARN" ]; then
    echo "Error: CERTIFICATE_ARN not set"
    exit 1
fi

if [ -z "$CLOUDFORMATION_S3_BUCKET" ]; then
    echo "Error: CLOUDFORMATION_S3_BUCKET not set"
    exit 1
fi

# Build the application
echo "Building application..."
sam build

# Deploy the application
echo "Deploying application..."
sam deploy \
    --stack-name "$STACK_NAME" \
    --region "$AWS_REGION" \
    --capabilities CAPABILITY_IAM \
    --parameter-overrides \
        "DomainName=$DOMAIN_NAME" \
        "HostedZoneId=$HOSTED_ZONE_ID" \
        "CertificateArn=$CERTIFICATE_ARN" \
    --s3-bucket "$CLOUDFORMATION_S3_BUCKET" \
    --s3-prefix "$STACK_NAME"

echo ""
echo "==================================="
echo "Deployment complete!"
echo "==================================="

# Get outputs
echo ""
echo "Stack Outputs:"
aws cloudformation describe-stacks \
    --stack-name "$STACK_NAME" \
    --region "$AWS_REGION" \
    --query "Stacks[0].Outputs" \
    --output table

echo ""
echo "Next steps:"
echo "1. Create a user: ./scripts/create-user-simple.sh $STACK_NAME username email password"
echo "2. Flash firmware to your Particle Argon device"
echo "3. Visit https://$DOMAIN_NAME to log in"
