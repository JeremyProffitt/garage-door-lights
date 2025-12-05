# Alexa Smart Home Integration

This directory contains the configuration and scripts needed to create and manage the Alexa Smart Home skill for controlling LED light strips.

## Overview

The Alexa integration enables voice control of your LED strips:

- **"Alexa, turn on the garage lights"**
- **"Alexa, set garage lights to 50 percent"**
- **"Alexa, set garage lights to red"**
- **"Alexa, set garage lights to fire mode"**

## Prerequisites

### 1. Amazon Developer Account

Sign up at [developer.amazon.com](https://developer.amazon.com/)

### 2. Login with Amazon (LWA) Security Profile

1. Go to [Security Profile Management](https://developer.amazon.com/settings/console/securityprofile/overview.html)
2. Click **Create a New Security Profile**
3. Fill in:
   - **Name**: Garage Lights SMAPI
   - **Description**: Security profile for Garage Lights skill management
4. Under **Web Settings**, add:
   - **Allowed Origins**: `http://localhost:9000`
   - **Allowed Return URLs**: `http://localhost:9000/callback`
5. Note your **Client ID** and **Client Secret**

### 3. Get LWA Tokens

Run the helper script to obtain your refresh token:

```bash
export LWA_CLIENT_ID="amzn1.application-oa2-client.xxx"
export LWA_CLIENT_SECRET="your-secret"
./scripts/get-lwa-tokens.sh
```

This will give you:
- **Refresh Token** (for `ALEXA_LWA_TOKEN`)
- **Vendor ID** (for `ALEXA_VENDOR_ID`)

## GitHub Configuration

### Secrets (Settings > Secrets and variables > Actions > Secrets)

| Secret | Description |
|--------|-------------|
| `ALEXA_CLIENT_ID` | LWA Security Profile Client ID |
| `ALEXA_SECRET_KEY` | LWA Security Profile Client Secret |
| `ALEXA_LWA_TOKEN` | LWA Refresh Token (from get-lwa-tokens.sh) |
| `ALEXA_SKILL_ID` | Skill ID (set after first deployment) |
| `ALEXA_OAUTH_CLIENT_SECRET` | OAuth secret for account linking |
| `AWS_ACCESS_KEY_ID` | AWS credentials for deployment |
| `AWS_SECRET_ACCESS_KEY` | AWS credentials for deployment |

### Variables (Settings > Secrets and variables > Actions > Variables)

| Variable | Description |
|----------|-------------|
| `ALEXA_VENDOR_ID` | Amazon Developer Vendor ID |
| `ALEXA_CUSTOMER_ID` | Amazon Customer ID |
| `ALEXA_OAUTH_CLIENT_ID` | OAuth client ID (default: `garage-lights-alexa`) |

## Deployment

### Option 1: GitHub Actions (Recommended)

1. Set up all required secrets and variables
2. Go to **Actions** > **Deploy Alexa Integration**
3. Click **Run workflow**
4. After first run, add the `ALEXA_SKILL_ID` secret with the created skill ID

### Option 2: Manual Deployment

```bash
# Set environment variables
export ALEXA_CLIENT_ID="amzn1.application-oa2-client.xxx"
export ALEXA_SECRET_KEY="your-lwa-secret"
export ALEXA_LWA_TOKEN="your-refresh-token"
export ALEXA_VENDOR_ID="your-vendor-id"
export DOMAIN_NAME="lights.jeremy.ninja"
export LAMBDA_ARN="arn:aws:lambda:us-east-1:xxx:function:garage-lights-AlexaFunction-xxx"
export ALEXA_OAUTH_CLIENT_ID="garage-lights-alexa"
export ALEXA_OAUTH_CLIENT_SECRET="your-oauth-secret"

# Create/update skill
./scripts/create-alexa-skill.sh
```

## Files

```
alexa-skill/
├── README.md              # This file
├── skill-manifest.json    # Alexa skill configuration
├── account-linking.json   # OAuth account linking config
└── .skill-id             # Created skill ID (gitignored)

scripts/
├── create-alexa-skill.sh  # Main skill creation script
├── get-lwa-tokens.sh      # Helper to get LWA tokens
├── deploy-alexa.sh        # Full deployment script
└── deploy-alexa.bat       # Windows deployment script

.github/workflows/
└── deploy-alexa.yml       # GitHub Actions workflow
```

## Architecture

```
┌─────────────┐     ┌─────────────────┐     ┌──────────────────┐
│   Amazon    │────▶│  Alexa Smart    │────▶│  AWS Lambda      │
│   Echo      │     │  Home Service   │     │  (AlexaFunction) │
└─────────────┘     └─────────────────┘     └────────┬─────────┘
                                                     │
                    ┌────────────────────────────────┤
                    │                                │
                    ▼                                ▼
           ┌─────────────────┐             ┌─────────────────┐
           │  DynamoDB       │             │  Particle Cloud │
           │  (State/Auth)   │             │  API            │
           └─────────────────┘             └────────┬────────┘
                                                    │
                                                    ▼
                                           ┌─────────────────┐
                                           │  Particle IoT   │
                                           │  Devices        │
                                           └─────────────────┘
```

## Account Linking Flow

1. User enables skill in Alexa app
2. User is redirected to `https://lights.jeremy.ninja/oauth/authorize`
3. User logs in with Garage Lights credentials
4. Authorization code is exchanged for tokens
5. Alexa stores tokens for future requests

## Supported Capabilities

| Capability | Interface | Example Commands |
|------------|-----------|------------------|
| Power | `Alexa.PowerController` | "Turn on/off [light]" |
| Brightness | `Alexa.BrightnessController` | "Set [light] to 50%" |
| Color | `Alexa.ColorController` | "Set [light] to red" |
| Pattern | `Alexa.ModeController` | "Set [light] to fire mode" |

## Troubleshooting

### "Unable to link account"

1. Check OAuth URLs are correct in skill configuration
2. Verify `ALEXA_OAUTH_CLIENT_ID` and `ALEXA_OAUTH_CLIENT_SECRET` match
3. Check Lambda logs for errors: `aws logs tail /aws/lambda/garage-lights-OAuthFunction`

### "Device is unresponsive"

1. Check Particle device is online
2. Verify Particle token is configured in user account
3. Check Alexa Lambda logs: `aws logs tail /aws/lambda/garage-lights-AlexaFunction`

### "No devices found"

1. Ensure LED strips are configured on your devices
2. Check device has `isReady: true` (valid firmware)
3. Verify account linking is complete

### Skill creation fails

1. Verify LWA tokens are valid (refresh if needed)
2. Check Lambda ARN is correct
3. Ensure Lambda has Alexa permission

## Development

### Testing Locally

You can test the Alexa Lambda locally using SAM:

```bash
# Create test event
cat > /tmp/alexa-discovery.json << 'EOF'
{
  "directive": {
    "header": {
      "namespace": "Alexa.Discovery",
      "name": "Discover",
      "payloadVersion": "3",
      "messageId": "test-123"
    },
    "payload": {
      "scope": {
        "type": "BearerToken",
        "token": "your-access-token"
      }
    }
  }
}
EOF

# Invoke locally
sam local invoke AlexaFunction -e /tmp/alexa-discovery.json
```

### Updating Skill Manifest

1. Edit `alexa-skill/skill-manifest.json`
2. Run deployment script or GitHub Action
3. Changes are applied to development stage

### Submitting for Certification

Once testing is complete:

1. Go to [Alexa Developer Console](https://developer.amazon.com/alexa/console/ask)
2. Select your skill
3. Go to **Distribution** tab
4. Complete all required information
5. Submit for certification
