#!/bin/bash
#
# Helper script to obtain Login with Amazon (LWA) tokens for SMAPI
#
# This script helps you get the refresh token needed for automated
# Alexa skill management via SMAPI.
#
# Prerequisites:
# 1. Create a Security Profile in Amazon Developer Console:
#    https://developer.amazon.com/settings/console/securityprofile/overview.html
#
# 2. Add the following Web Settings to your Security Profile:
#    - Allowed Origins: http://localhost:9000
#    - Allowed Return URLs: http://localhost:9000/callback
#
# Usage:
#   export LWA_CLIENT_ID="your-client-id"
#   export LWA_CLIENT_SECRET="your-client-secret"
#   ./scripts/get-lwa-tokens.sh
#

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# Check required variables
if [ -z "$LWA_CLIENT_ID" ] || [ -z "$LWA_CLIENT_SECRET" ]; then
    log_error "Missing required environment variables!"
    echo ""
    echo "Please set the following variables:"
    echo "  export LWA_CLIENT_ID='amzn1.application-oa2-client.xxx'"
    echo "  export LWA_CLIENT_SECRET='xxx'"
    echo ""
    echo "Get these from your Security Profile at:"
    echo "  https://developer.amazon.com/settings/console/securityprofile/overview.html"
    exit 1
fi

REDIRECT_URI="http://localhost:9000/callback"
SCOPES="alexa::ask:skills:readwrite alexa::ask:models:readwrite alexa::ask:skills:test"
STATE=$(openssl rand -hex 16)

# URL encode the scopes
ENCODED_SCOPES=$(echo -n "$SCOPES" | jq -sRr @uri)

# Build authorization URL
AUTH_URL="https://www.amazon.com/ap/oa?client_id=${LWA_CLIENT_ID}&scope=${ENCODED_SCOPES}&response_type=code&redirect_uri=${REDIRECT_URI}&state=${STATE}"

echo ""
echo -e "${CYAN}=== Login with Amazon Token Generator ===${NC}"
echo ""
log_info "Step 1: Open this URL in your browser and log in:"
echo ""
echo -e "${YELLOW}$AUTH_URL${NC}"
echo ""
log_info "Step 2: After authorization, you'll be redirected to localhost:9000"
log_info "        Copy the 'code' parameter from the URL."
echo ""
echo -n "Step 3: Paste the authorization code here: "
read -r AUTH_CODE

if [ -z "$AUTH_CODE" ]; then
    log_error "No authorization code provided"
    exit 1
fi

log_info "Exchanging authorization code for tokens..."

# Exchange code for tokens
RESPONSE=$(curl -s -X POST "https://api.amazon.com/auth/o2/token" \
    -H "Content-Type: application/x-www-form-urlencoded" \
    -d "grant_type=authorization_code" \
    -d "code=$AUTH_CODE" \
    -d "redirect_uri=$REDIRECT_URI" \
    -d "client_id=$LWA_CLIENT_ID" \
    -d "client_secret=$LWA_CLIENT_SECRET")

ACCESS_TOKEN=$(echo "$RESPONSE" | jq -r '.access_token')
REFRESH_TOKEN=$(echo "$RESPONSE" | jq -r '.refresh_token')
EXPIRES_IN=$(echo "$RESPONSE" | jq -r '.expires_in')

if [ "$ACCESS_TOKEN" == "null" ] || [ -z "$ACCESS_TOKEN" ]; then
    log_error "Failed to obtain tokens!"
    echo "$RESPONSE" | jq .
    exit 1
fi

echo ""
log_info "Tokens obtained successfully!"
echo ""
echo -e "${CYAN}=== Your Tokens ===${NC}"
echo ""
echo -e "${GREEN}Access Token:${NC}"
echo "$ACCESS_TOKEN"
echo ""
echo -e "${GREEN}Refresh Token (save this for automation):${NC}"
echo "$REFRESH_TOKEN"
echo ""
echo -e "${GREEN}Expires In:${NC} $EXPIRES_IN seconds"
echo ""

# Get vendor ID
log_info "Getting your Vendor ID..."
VENDOR_RESPONSE=$(curl -s -X GET "https://api.amazonalexa.com/v1/vendors" \
    -H "Authorization: Bearer $ACCESS_TOKEN")

VENDOR_ID=$(echo "$VENDOR_RESPONSE" | jq -r '.vendors[0].id')

if [ "$VENDOR_ID" != "null" ] && [ -n "$VENDOR_ID" ]; then
    echo -e "${GREEN}Vendor ID:${NC}"
    echo "$VENDOR_ID"
    echo ""
fi

echo -e "${CYAN}=== GitHub Secrets to Set ===${NC}"
echo ""
echo "Run these commands to set your GitHub secrets:"
echo ""
echo "  gh secret set ALEXA_CLIENT_ID --body '$LWA_CLIENT_ID'"
echo "  gh secret set ALEXA_SECRET_KEY --body '$LWA_CLIENT_SECRET'"
echo "  gh secret set ALEXA_LWA_TOKEN --body '$REFRESH_TOKEN'"
echo ""
echo "And set these GitHub variables:"
echo ""
echo "  gh variable set ALEXA_VENDOR_ID --body '$VENDOR_ID'"
echo ""

# Save to file for reference
TOKEN_FILE="/tmp/lwa-tokens-$(date +%Y%m%d-%H%M%S).json"
cat > "$TOKEN_FILE" << EOF
{
  "client_id": "$LWA_CLIENT_ID",
  "vendor_id": "$VENDOR_ID",
  "refresh_token": "$REFRESH_TOKEN",
  "note": "Save the refresh_token as ALEXA_LWA_TOKEN secret"
}
EOF

log_info "Token info saved to: $TOKEN_FILE"
log_warn "Remember to keep your refresh token secure!"
