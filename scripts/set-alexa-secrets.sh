#!/bin/bash
#
# Extract Alexa credentials from ASK CLI config and set as GitHub secrets
#
# Usage: ./scripts/set-alexa-secrets.sh
#

set -e

CONFIG_FILE="$HOME/.ask/cli_config"

if [ ! -f "$CONFIG_FILE" ]; then
    echo "Error: ASK CLI config not found at $CONFIG_FILE"
    echo "Run 'ask configure' first to set up the ASK CLI"
    exit 1
fi

echo "Reading credentials from $CONFIG_FILE..."

# Extract values using grep and sed (works on Windows Git Bash)
CLIENT_ID=$(grep -o '"client_id"[[:space:]]*:[[:space:]]*"[^"]*"' "$CONFIG_FILE" | head -1 | sed 's/.*: *"\([^"]*\)"/\1/')
CLIENT_SECRET=$(grep -o '"client_secret"[[:space:]]*:[[:space:]]*"[^"]*"' "$CONFIG_FILE" | head -1 | sed 's/.*: *"\([^"]*\)"/\1/')
REFRESH_TOKEN=$(grep -o '"refresh_token"[[:space:]]*:[[:space:]]*"[^"]*"' "$CONFIG_FILE" | head -1 | sed 's/.*: *"\([^"]*\)"/\1/')

# Validate
if [ -z "$CLIENT_ID" ]; then
    echo "Error: Could not find client_id in config"
    exit 1
fi

if [ -z "$CLIENT_SECRET" ]; then
    echo "Error: Could not find client_secret in config"
    exit 1
fi

if [ -z "$REFRESH_TOKEN" ]; then
    echo "Error: Could not find refresh_token in config"
    exit 1
fi

echo ""
echo "Found credentials:"
echo "  client_id:     ${CLIENT_ID:0:20}...${CLIENT_ID: -10}"
echo "  client_secret: ${CLIENT_SECRET:0:10}...${CLIENT_SECRET: -5}"
echo "  refresh_token: ${REFRESH_TOKEN:0:20}...${REFRESH_TOKEN: -10}"
echo ""

# Confirm before setting
read -p "Set these as GitHub secrets? (y/n) " -n 1 -r
echo ""

if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "Aborted."
    exit 0
fi

echo ""
echo "Setting GitHub secrets..."

gh secret set ALEXA_CLIENT_ID --body "$CLIENT_ID"
echo "  ✓ ALEXA_CLIENT_ID set"

gh secret set ALEXA_SECRET_KEY --body "$CLIENT_SECRET"
echo "  ✓ ALEXA_SECRET_KEY set"

gh secret set ALEXA_LWA_TOKEN --body "$REFRESH_TOKEN"
echo "  ✓ ALEXA_LWA_TOKEN set"

echo ""
echo "Done! All secrets have been updated."
echo ""
echo "You can now re-run the Alexa deployment workflow:"
echo "  gh workflow run deploy-alexa.yml"
