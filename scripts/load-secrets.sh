#!/bin/bash

# Script to load GitHub secrets and variables into environment
# This script reads from a .env file or GitHub Actions secrets

set -e

echo "Loading GitHub secrets and environment variables..."

# Check if running in GitHub Actions
if [ -n "$GITHUB_ACTIONS" ]; then
    echo "Running in GitHub Actions - secrets are already loaded"
    exit 0
fi

# Check for .env file
if [ ! -f .env ]; then
    echo "Error: .env file not found"
    echo "Please create a .env file with the following variables:"
    echo "  DOMAIN_NAME=lights.jeremy.ninja"
    echo "  HOSTED_ZONE_ID=your-route53-hosted-zone-id"
    echo "  CERTIFICATE_ARN=your-acm-certificate-arn"
    echo "  AWS_REGION=us-east-1"
    echo "  AWS_ACCOUNT_ID=your-aws-account-id"
    echo "  PARTICLE_TOKEN=your-particle-access-token"
    exit 1
fi

# Load .env file
export $(cat .env | grep -v '^#' | xargs)

echo "Environment variables loaded successfully!"
echo ""
echo "Configured values:"
echo "  Domain: ${DOMAIN_NAME}"
echo "  AWS Region: ${AWS_REGION}"
echo "  AWS Account: ${AWS_ACCOUNT_ID}"
echo ""
echo "Use these values when deploying with SAM CLI"
