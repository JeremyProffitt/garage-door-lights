#!/bin/bash

# Script to help set up Alexa credentials for automated deployment
# This script guides you through getting the necessary Alexa tokens

set -e

echo "======================================"
echo "Alexa Credentials Setup"
echo "======================================"
echo ""
echo "This script will help you get the credentials needed for"
echo "automated Alexa skill deployment via GitHub Actions."
echo ""

# Check if ASK CLI is installed
if ! command -v ask &> /dev/null; then
    echo "ASK CLI is not installed. Installing..."
    npm install -g ask-cli
fi

echo "Step 1: Initialize ASK CLI"
echo "This will open a browser for you to log in to your Amazon Developer account."
echo ""
read -p "Press Enter to continue..."

ask configure

echo ""
echo "======================================"
echo "Extracting Credentials"
echo "======================================"
echo ""

# Extract credentials from ASK CLI config
if [ -f ~/.ask/cli_config ]; then
    CONFIG_FILE=~/.ask/cli_config

    VENDOR_ID=$(cat $CONFIG_FILE | jq -r '.profiles.default.vendor_id')
    REFRESH_TOKEN=$(cat $CONFIG_FILE | jq -r '.profiles.default.token.refresh_token')

    echo "Found credentials!"
    echo ""
    echo "Add these secrets to your GitHub repository:"
    echo "(Settings → Secrets and variables → Actions → New repository secret)"
    echo ""
    echo "Secret Name: ALEXA_VENDOR_ID"
    echo "Value: $VENDOR_ID"
    echo ""
    echo "Secret Name: ALEXA_REFRESH_TOKEN"
    echo "Value: $REFRESH_TOKEN"
    echo ""
    echo "Note: You'll also need ALEXA_CLIENT_ID and ALEXA_CLIENT_SECRET"
    echo "from your Login with Amazon Security Profile:"
    echo "https://developer.amazon.com/settings/console/securityprofile/overview.html"
    echo ""
    echo "After creating a security profile, add these secrets:"
    echo "- ALEXA_CLIENT_ID: Your LWA Client ID"
    echo "- ALEXA_CLIENT_SECRET: Your LWA Client Secret"
    echo ""
    echo "Optional: If updating an existing skill, add:"
    echo "- ALEXA_SKILL_ID: Your existing skill ID"
    echo ""
    echo "======================================"
    echo "Setup Complete!"
    echo "======================================"
else
    echo "Error: Could not find ASK CLI config file"
    echo "Please run 'ask configure' manually first"
    exit 1
fi
