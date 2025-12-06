#!/bin/bash
#
# Create and configure Alexa Smart Home Skill using ASK CLI
#
# Required environment variables:
#   ALEXA_LWA_TOKEN           - Login with Amazon refresh token (from ASK CLI config)
#   ALEXA_VENDOR_ID           - Amazon Developer Vendor ID
#   DOMAIN_NAME               - Your domain (e.g., lights.jeremy.ninja)
#   LAMBDA_ARN                - ARN of the Alexa Lambda function
#   ALEXA_OAUTH_CLIENT_ID     - OAuth Client ID for account linking
#   ALEXA_OAUTH_CLIENT_SECRET - OAuth Client Secret for account linking
#
# Optional:
#   ALEXA_SKILL_ID            - Existing skill ID to update (if not provided, creates new)
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

# Set up ASK CLI config from environment variables
setup_ask_cli() {
    log_info "Configuring ASK CLI..."

    mkdir -p ~/.ask

    # Create ASK CLI config with the refresh token and client credentials
    # These must match the credentials used when the refresh token was generated via 'ask configure'
    cat > ~/.ask/cli_config << EOF
{
  "profiles": {
    "default": {
      "vendor_id": "$ALEXA_VENDOR_ID",
      "lwa_client_id": "$ALEXA_CLIENT_ID",
      "lwa_client_secret": "$ALEXA_SECRET_KEY",
      "token": {
        "refresh_token": "$ALEXA_LWA_TOKEN"
      }
    }
  }
}
EOF

    log_info "ASK CLI configured"
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

# Create or update the skill
create_or_update_skill() {
    local temp_dir=$(mktemp -d)
    trap "rm -rf $temp_dir" EXIT

    # Prepare skill manifest
    log_info "Preparing skill manifest..."
    substitute_vars "$SKILL_DIR/skill-manifest.json" "$temp_dir/skill.json"

    # Prepare account linking config
    substitute_vars "$SKILL_DIR/account-linking.json" "$temp_dir/account-linking.json"

    if [ -n "$ALEXA_SKILL_ID" ]; then
        log_info "Updating existing skill: $ALEXA_SKILL_ID"

        # Update skill manifest
        ask smapi update-skill-manifest \
            --skill-id "$ALEXA_SKILL_ID" \
            --stage development \
            --manifest "file:$temp_dir/skill.json" || {
            log_error "Failed to update skill manifest"
            exit 1
        }

        log_info "Skill manifest updated successfully"
        SKILL_ID="$ALEXA_SKILL_ID"
    else
        log_info "Creating new skill..."

        # Debug: show the manifest being used
        log_info "Skill manifest content:"
        cat "$temp_dir/skill.json"
        echo ""

        # Create skill using ASK CLI
        # Note: ask smapi returns JSON on success, but may output non-JSON errors
        set +e
        RESPONSE=$(ask smapi create-skill-for-vendor \
            --manifest "file:$temp_dir/skill.json" 2>&1)
        EXIT_CODE=$?
        set -e

        log_info "ASK CLI exit code: $EXIT_CODE"
        log_info "ASK CLI response: $RESPONSE"

        if [ $EXIT_CODE -ne 0 ]; then
            log_error "Failed to create skill (exit code: $EXIT_CODE)"
            echo "Response:"
            echo "$RESPONSE"
            exit 1
        fi

        # Try to parse the response as JSON
        SKILL_ID=$(echo "$RESPONSE" | jq -r '.skillId // empty' 2>/dev/null)

        if [ -z "$SKILL_ID" ]; then
            log_error "Failed to get skill ID from response"
            echo "Response:"
            echo "$RESPONSE"
            exit 1
        fi

        log_info "Skill created with ID: $SKILL_ID"

        # Wait for skill to be ready
        log_info "Waiting for skill to be ready..."
        sleep 5
    fi

    # Update account linking
    log_info "Configuring account linking..."
    ask smapi update-account-linking-info \
        --skill-id "$SKILL_ID" \
        --stage development \
        --account-linking-request "file:$temp_dir/account-linking.json" || {
        log_warn "Failed to update account linking (may already be configured)"
    }

    # Set Lambda endpoint permission if needed
    log_info "Ensuring Lambda has Alexa trigger permission..."
    aws lambda add-permission \
        --function-name "$LAMBDA_ARN" \
        --statement-id "alexa-skill-$SKILL_ID" \
        --action "lambda:InvokeFunction" \
        --principal "alexa-connectedhome.amazon.com" \
        --event-source-token "$SKILL_ID" 2>/dev/null || {
        log_info "Permission may already exist (this is OK)"
    }

    echo ""
    log_info "=== Skill Setup Complete ==="
    echo ""
    echo "Skill ID: $SKILL_ID"
    echo ""
    if [ -z "$ALEXA_SKILL_ID" ]; then
        echo "This is a new skill. Save the skill ID as a GitHub secret:"
        echo "  gh secret set ALEXA_SKILL_ID --body '$SKILL_ID'"
        echo ""
    fi
    echo "Next steps:"
    echo "  1. Go to https://developer.amazon.com/alexa/console/ask"
    echo "  2. Select the 'Candle Lights' skill"
    echo "  3. Go to 'Test' tab and enable testing"
    echo "  4. Link your account in the Alexa app"
    echo ""

    # Output for GitHub Actions
    echo "skill_id=$SKILL_ID" >> "${GITHUB_OUTPUT:-/dev/null}"
}

# Main
log_info "=== Alexa Smart Home Skill Setup ==="
echo ""

check_env
setup_ask_cli
create_or_update_skill
