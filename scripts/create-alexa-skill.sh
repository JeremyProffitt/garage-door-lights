#!/bin/bash
#
# Create and configure Alexa Smart Home Skill using SMAPI
#
# Required environment variables:
#   ALEXA_CLIENT_ID      - LWA Client ID
#   ALEXA_SECRET_KEY     - LWA Client Secret (for refreshing token)
#   ALEXA_LWA_TOKEN      - Login with Amazon refresh token
#   ALEXA_VENDOR_ID      - Amazon Developer Vendor ID
#   DOMAIN_NAME          - Your domain (e.g., lights.jeremy.ninja)
#   LAMBDA_ARN           - ARN of the Alexa Lambda function
#   ALEXA_OAUTH_CLIENT_ID     - OAuth Client ID for account linking
#   ALEXA_OAUTH_CLIENT_SECRET - OAuth Client Secret for account linking
#
# Optional:
#   ALEXA_SKILL_ID       - Existing skill ID to update (if not provided, creates new)
#

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
SKILL_DIR="$PROJECT_DIR/alexa-skill"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# SMAPI base URL
SMAPI_BASE="https://api.amazonalexa.com/v1"

# Check required environment variables
check_env() {
    local missing=()

    [ -z "$ALEXA_CLIENT_ID" ] && missing+=("ALEXA_CLIENT_ID")
    [ -z "$ALEXA_SECRET_KEY" ] && missing+=("ALEXA_SECRET_KEY")
    [ -z "$ALEXA_LWA_TOKEN" ] && missing+=("ALEXA_LWA_TOKEN")
    [ -z "$ALEXA_VENDOR_ID" ] && missing+=("ALEXA_VENDOR_ID")
    [ -z "$DOMAIN_NAME" ] && missing+=("DOMAIN_NAME")
    [ -z "$LAMBDA_ARN" ] && missing+=("LAMBDA_ARN")
    [ -z "$ALEXA_OAUTH_CLIENT_ID" ] && missing+=("ALEXA_OAUTH_CLIENT_ID")
    [ -z "$ALEXA_OAUTH_CLIENT_SECRET" ] && missing+=("ALEXA_OAUTH_CLIENT_SECRET")

    if [ ${#missing[@]} -gt 0 ]; then
        log_error "Missing required environment variables:"
        for var in "${missing[@]}"; do
            echo "  - $var"
        done
        exit 1
    fi
}

# Get access token from refresh token
get_access_token() {
    log_info "Obtaining access token from LWA..."

    local response
    response=$(curl -s -X POST "https://api.amazon.com/auth/o2/token" \
        -H "Content-Type: application/x-www-form-urlencoded" \
        -d "grant_type=refresh_token" \
        -d "refresh_token=$ALEXA_LWA_TOKEN" \
        -d "client_id=$ALEXA_CLIENT_ID" \
        -d "client_secret=$ALEXA_SECRET_KEY")

    ACCESS_TOKEN=$(echo "$response" | jq -r '.access_token')

    if [ "$ACCESS_TOKEN" == "null" ] || [ -z "$ACCESS_TOKEN" ]; then
        log_error "Failed to obtain access token"
        echo "$response" | jq .
        exit 1
    fi

    log_info "Access token obtained successfully"
}

# Substitute environment variables in a JSON file
substitute_vars() {
    local template="$1"
    local output="$2"

    sed -e "s|\${DOMAIN_NAME}|$DOMAIN_NAME|g" \
        -e "s|\${LAMBDA_ARN}|$LAMBDA_ARN|g" \
        -e "s|\${ALEXA_CLIENT_ID}|$ALEXA_OAUTH_CLIENT_ID|g" \
        -e "s|\${ALEXA_CLIENT_SECRET}|$ALEXA_OAUTH_CLIENT_SECRET|g" \
        "$template" > "$output"
}

# Create a new skill
create_skill() {
    log_info "Creating new Alexa Smart Home skill..."

    # Prepare manifest with substituted variables
    local manifest_file="/tmp/skill-manifest.json"
    substitute_vars "$SKILL_DIR/skill-manifest.json" "$manifest_file"

    local response
    response=$(curl -s -X POST "$SMAPI_BASE/skills" \
        -H "Authorization: Bearer $ACCESS_TOKEN" \
        -H "Content-Type: application/json" \
        -d @"$manifest_file")

    SKILL_ID=$(echo "$response" | jq -r '.skillId')

    if [ "$SKILL_ID" == "null" ] || [ -z "$SKILL_ID" ]; then
        log_error "Failed to create skill"
        echo "$response" | jq .
        exit 1
    fi

    log_info "Skill created with ID: $SKILL_ID"
    echo "$SKILL_ID" > "$SKILL_DIR/.skill-id"
}

# Update existing skill manifest
update_skill_manifest() {
    log_info "Updating skill manifest..."

    local manifest_file="/tmp/skill-manifest.json"
    substitute_vars "$SKILL_DIR/skill-manifest.json" "$manifest_file"

    local response
    response=$(curl -s -X PUT "$SMAPI_BASE/skills/$SKILL_ID/stages/development/manifest" \
        -H "Authorization: Bearer $ACCESS_TOKEN" \
        -H "Content-Type: application/json" \
        -d @"$manifest_file")

    local status
    status=$(echo "$response" | jq -r '.status // "success"')

    if [ "$status" == "FAILED" ]; then
        log_error "Failed to update skill manifest"
        echo "$response" | jq .
        exit 1
    fi

    log_info "Skill manifest updated"
}

# Configure account linking
configure_account_linking() {
    log_info "Configuring account linking..."

    local account_linking_file="/tmp/account-linking.json"
    substitute_vars "$SKILL_DIR/account-linking.json" "$account_linking_file"

    local response
    response=$(curl -s -X PUT "$SMAPI_BASE/skills/$SKILL_ID/stages/development/accountLinkingClient" \
        -H "Authorization: Bearer $ACCESS_TOKEN" \
        -H "Content-Type: application/json" \
        -d @"$account_linking_file")

    # Check for errors
    local error
    error=$(echo "$response" | jq -r '.message // empty')

    if [ -n "$error" ]; then
        log_error "Failed to configure account linking: $error"
        echo "$response" | jq .
        exit 1
    fi

    log_info "Account linking configured"
}

# Add Lambda permission for Alexa
add_lambda_permission() {
    log_info "Adding Lambda permission for Alexa..."

    # Extract region and function name from ARN
    local region
    region=$(echo "$LAMBDA_ARN" | cut -d: -f4)
    local function_name
    function_name=$(echo "$LAMBDA_ARN" | cut -d: -f7)

    # Remove existing permission if it exists (ignore errors)
    aws lambda remove-permission \
        --function-name "$function_name" \
        --statement-id "alexa-smart-home-$SKILL_ID" \
        --region "$region" 2>/dev/null || true

    # Add permission for this skill
    aws lambda add-permission \
        --function-name "$function_name" \
        --statement-id "alexa-smart-home-$SKILL_ID" \
        --action "lambda:InvokeFunction" \
        --principal "alexa-connectedhome.amazon.com" \
        --event-source-token "$SKILL_ID" \
        --region "$region"

    log_info "Lambda permission added"
}

# Get skill status
get_skill_status() {
    log_info "Getting skill status..."

    local response
    response=$(curl -s -X GET "$SMAPI_BASE/skills/$SKILL_ID/status" \
        -H "Authorization: Bearer $ACCESS_TOKEN")

    echo "$response" | jq .
}

# Enable skill for testing
enable_skill() {
    log_info "Enabling skill for testing..."

    local response
    response=$(curl -s -X POST "$SMAPI_BASE/skills/$SKILL_ID/stages/development/enablement" \
        -H "Authorization: Bearer $ACCESS_TOKEN" \
        -H "Content-Type: application/json")

    log_info "Skill enabled for testing"
}

# Get account linking info (to retrieve redirect URLs)
get_account_linking_info() {
    log_info "Getting account linking information..."

    local response
    response=$(curl -s -X GET "$SMAPI_BASE/skills/$SKILL_ID/stages/development/accountLinkingClient" \
        -H "Authorization: Bearer $ACCESS_TOKEN")

    echo ""
    log_info "Account Linking Redirect URLs (add these to your OAuth config):"
    echo "$response" | jq -r '.accountLinkingResponse.redirectUrls[]' 2>/dev/null || echo "  (No redirect URLs yet - skill may still be processing)"
}

# Main execution
main() {
    log_info "=== Alexa Smart Home Skill Setup ==="
    echo ""

    check_env
    get_access_token

    # Check if we're updating an existing skill or creating new
    if [ -n "$ALEXA_SKILL_ID" ]; then
        SKILL_ID="$ALEXA_SKILL_ID"
        log_info "Using existing skill ID: $SKILL_ID"
        update_skill_manifest
    elif [ -f "$SKILL_DIR/.skill-id" ]; then
        SKILL_ID=$(cat "$SKILL_DIR/.skill-id")
        log_info "Found existing skill ID: $SKILL_ID"
        update_skill_manifest
    else
        create_skill
    fi

    # Configure account linking
    configure_account_linking

    # Add Lambda permission
    add_lambda_permission

    # Enable for testing
    enable_skill

    # Get status and account linking info
    echo ""
    get_skill_status
    get_account_linking_info

    echo ""
    log_info "=== Setup Complete ==="
    echo ""
    echo "Skill ID: $SKILL_ID"
    echo ""
    echo "Next steps:"
    echo "1. Open the Alexa app on your phone"
    echo "2. Go to Skills & Games > Your Skills > Dev"
    echo "3. Enable the 'Garage Lights' skill and link your account"
    echo "4. Say 'Alexa, discover devices'"
    echo ""
    echo "To use this skill ID in deployments:"
    echo "  export ALEXA_SKILL_ID=$SKILL_ID"
    echo ""

    # Output skill ID for GitHub Actions
    if [ -n "$GITHUB_OUTPUT" ]; then
        echo "skill_id=$SKILL_ID" >> "$GITHUB_OUTPUT"
    fi
}

main "$@"
