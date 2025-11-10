# 🕯️ Particle Argon WS2812B Candle Lights Controller

A comprehensive IoT system for controlling WS2812B RGB LED lights attached to a Particle Argon board. Features include web interface, Alexa voice control, cloud storage, and realistic candle simulation effects.

## Features

### Hardware
- **Particle Argon** microcontroller
- **8 WS2812B RGB LEDs** for candle simulation
- Flash memory storage for persistent settings

### Light Patterns
- 🕯️ **Candle** - Realistic flickering candle effect
- 🎨 **Solid** - Single solid color
- 💓 **Pulse** - Breathing/pulsing effect
- 🌊 **Wave** - Wave animation
- 🌈 **Rainbow** - Rainbow color cycle
- 🔥 **Fire** - Realistic fire simulation

### Control Methods
- 🌐 **Web Interface** - Full-featured web dashboard
- 🗣️ **Alexa Voice Control** - "Alexa, turn on candle lights"
- ☁️ **Particle Cloud** - Control from anywhere via Particle.io
- 📱 **API** - RESTful API for integration

### Cloud Platform
- **AWS Lambda** - Serverless backend with Go
- **DynamoDB** - User data, patterns, and device storage
- **API Gateway** - RESTful API endpoints
- **Route53** - Custom domain management
- **ACM** - SSL/TLS certificates

## Architecture

```
┌─────────────────┐
│  Particle Argon │
│   + WS2812B     │
└────────┬────────┘
         │
         │ Particle Cloud
         │
         ▼
┌─────────────────────────────────────────┐
│          AWS Cloud Platform              │
├─────────────────────────────────────────┤
│                                          │
│  ┌──────────────┐    ┌──────────────┐  │
│  │  Web Client  │    │ Alexa Device │  │
│  └──────┬───────┘    └──────┬───────┘  │
│         │                   │           │
│         ▼                   ▼           │
│  ┌──────────────┐    ┌──────────────┐  │
│  │ Go Fiber Web │    │ Alexa Lambda │  │
│  │   Frontend   │    │   Function   │  │
│  └──────┬───────┘    └──────┬───────┘  │
│         │                   │           │
│         ▼                   ▼           │
│  ┌──────────────────────────────────┐  │
│  │      API Gateway (REST API)      │  │
│  └──────┬───────────────────────────┘  │
│         │                               │
│         ▼                               │
│  ┌──────────────────────────────────┐  │
│  │      Lambda Functions (Go)       │  │
│  │  • Auth  • Patterns  • Devices   │  │
│  │  • Particle Integration          │  │
│  └──────┬───────────────────────────┘  │
│         │                               │
│         ▼                               │
│  ┌──────────────────────────────────┐  │
│  │         DynamoDB Tables          │  │
│  │  • Users  • Patterns  • Devices  │  │
│  └──────────────────────────────────┘  │
└─────────────────────────────────────────┘
```

## Project Structure

```
garage-door-lights/
├── firmware/                  # Particle Argon firmware
│   ├── candle-lights.ino     # Main firmware file
│   └── project.properties    # Particle dependencies
├── backend/
│   ├── shared/               # Shared Go packages
│   │   ├── models.go        # Data models
│   │   ├── auth.go          # JWT authentication
│   │   ├── db.go            # DynamoDB helpers
│   │   └── utils.go         # Utility functions
│   └── functions/           # Lambda functions
│       ├── auth/            # Authentication handler
│       ├── patterns/        # Pattern management
│       ├── devices/         # Device management
│       ├── particle/        # Particle.io integration
│       └── alexa/           # Alexa Smart Home skill
├── frontend/                # Go Fiber web application
│   ├── handlers/            # Route handlers
│   ├── middleware/          # Auth middleware
│   ├── templates/           # HTML templates
│   └── static/              # CSS and JavaScript
├── scripts/                 # Utility scripts
│   ├── deploy.sh           # Deployment script
│   ├── create-user-simple.sh # User creation
│   └── load-secrets.sh     # Load environment variables
├── alexa-skill/            # Alexa skill configuration
│   ├── skill.json          # Skill manifest
│   └── account-linking.json # OAuth configuration
├── .github/workflows/      # CI/CD pipelines
│   └── deploy.yml          # GitHub Actions deployment
├── template.yaml           # AWS SAM template
└── README.md              # This file
```

## Prerequisites

### Required
- [AWS Account](https://aws.amazon.com/)
- [Particle.io Account](https://www.particle.io/)
- [AWS CLI](https://aws.amazon.com/cli/) installed and configured
- [AWS SAM CLI](https://docs.aws.amazon.com/serverless-application-model/latest/developerguide/install-sam-cli.html)
- [Go 1.21+](https://golang.org/dl/)
- [Python 3.11+](https://www.python.org/downloads/) (for user creation script)
- Particle Argon board
- 8x WS2812B RGB LEDs
- Domain name (configured in Route53)
- ACM SSL certificate for your domain

### Optional
- [Particle CLI](https://docs.particle.io/getting-started/developer-tools/cli/) for firmware flashing
- [Amazon Developer Account](https://developer.amazon.com/) for Alexa skill

## Hardware Setup

### Wiring

Connect the WS2812B LED strip to your Particle Argon:

```
WS2812B Strip    Particle Argon
─────────────    ──────────────
VCC (5V)    ───► VUSB (5V)
GND         ───► GND
DIN         ───► D2
```

### Parts List
- 1x Particle Argon
- 1x WS2812B LED strip (8 LEDs minimum)
- Power supply (5V, 2A+ recommended for 8 LEDs)
- Jumper wires

## Software Setup

### 1. Clone the Repository

```bash
git clone https://github.com/yourusername/garage-door-lights.git
cd garage-door-lights
```

### 2. Configure Environment Variables

```bash
cp .env.example .env
```

Edit `.env` with your configuration:

```bash
AWS_REGION=us-east-1
AWS_ACCOUNT_ID=123456789012
DOMAIN_NAME=lights.jeremy.ninja
HOSTED_ZONE_ID=Z1234567890ABC
CERTIFICATE_ARN=arn:aws:acm:us-east-1:123456789012:certificate/...
STACK_NAME=candle-lights-prod
```

### 3. Setup AWS Resources

> **Note:** This project uses the **us-east-1** region because Alexa Smart Home skills require Lambda functions to be deployed in us-east-1. While you can use other regions, us-east-1 is recommended for full Alexa compatibility.

#### Create ACM Certificate

```bash
aws acm request-certificate \
  --domain-name lights.jeremy.ninja \
  --domain-name "*.lights.jeremy.ninja" \
  --validation-method DNS \
  --region us-east-1
```

Follow the DNS validation steps in the AWS Console.

#### Get Route53 Hosted Zone ID

```bash
aws route53 list-hosted-zones-by-name \
  --query "HostedZones[?Name=='jeremy.ninja.'].Id" \
  --output text
```

### 4. Deploy the Application

```bash
# Load environment variables
source scripts/load-secrets.sh

# Deploy using SAM
./scripts/deploy.sh
```

Or manually:

```bash
sam build
sam deploy \
  --stack-name candle-lights-prod \
  --region us-east-1 \
  --capabilities CAPABILITY_IAM \
  --parameter-overrides \
    "DomainName=lights.jeremy.ninja" \
    "HostedZoneId=Z1234567890ABC" \
    "CertificateArn=arn:aws:acm:..." \
  --resolve-s3
```

### 5. Create a User

```bash
# Install bcrypt for Python
pip install bcrypt

# Create user
./scripts/create-user-simple.sh candle-lights-prod \
  john \
  mypassword \
  your-particle-token
```

### 6. Flash Firmware to Particle Argon

Using Particle Web IDE:
1. Go to [Particle Build](https://build.particle.io/)
2. Create a new app
3. Copy contents of `firmware/candle-lights.ino`
4. Add library: `neopixel`
5. Flash to your device

Using Particle CLI:

```bash
particle flash your-device-name firmware/
```

## GitHub Actions Setup

### Required Secrets and Variables

Configure these in your GitHub repository:

```
Settings → Secrets and variables → Actions
```

#### Secrets (Required)
- `AWS_CLIENT_ID` - AWS access key ID for programmatic access
- `AWS_SECRET_KEY` - AWS secret access key
- `ADMIN_PASSWORD` - Password for the admin user (optional)

#### Variables (Required)
- `DOMAIN_NAME` - Your domain (e.g., garage-door-lights.jeremy.ninja)
- `HOSTED_ZONE_ID` - Route53 hosted zone ID
- `CERTIFICATE_ARN` - ACM certificate ARN
- `CLOUDFORMATION_S3_BUCKET` - S3 bucket for SAM/CloudFormation artifacts (must be in us-east-1)
- `ADMIN_USER` - Admin username (optional)

**To create AWS credentials:**
1. Go to AWS IAM Console
2. Create a new user or use existing user
3. Attach policy: `AdministratorAccess` (or create custom policy with required permissions)
4. Create access key under Security Credentials
5. Copy Access Key ID and Secret Access Key

**To create the S3 bucket for deployments:**
```bash
# Create S3 bucket in us-east-1 (required for Alexa compatibility)
aws s3 mb s3://your-cloudformation-bucket-name --region us-east-1

# Verify bucket was created
aws s3 ls | grep your-cloudformation-bucket-name
```

Then set `CLOUDFORMATION_S3_BUCKET` to your bucket name in GitHub variables.

#### Alexa Secrets (Optional - for automated skill deployment)
- `ALEXA_VENDOR_ID` - Your Amazon Developer Vendor ID
- `ALEXA_CLIENT_ID` - Login with Amazon (LWA) Client ID
- `ALEXA_CLIENT_SECRET` - Login with Amazon Client Secret
- `ALEXA_REFRESH_TOKEN` - ASK CLI refresh token
- `ALEXA_SKILL_ID` - (Optional) Existing skill ID to update instead of creating new

**To get Alexa credentials:**
```bash
# Run the setup script
./scripts/setup-alexa-credentials.sh

# Or manually:
# 1. Install ASK CLI: npm install -g ask-cli
# 2. Configure: ask configure
# 3. Get vendor ID from ~/.ask/cli_config
# 4. Create LWA Security Profile: https://developer.amazon.com/settings/console/securityprofile
# 5. Get Client ID and Secret from the security profile
```

### Deploy on Push

The GitHub Actions workflow automatically deploys on push to `main`:

```bash
git add .
git commit -m "Update application"
git push origin main
```

To trigger Alexa skill deployment, either:
- Use workflow dispatch (manual trigger)
- Include `[alexa]` in your commit message:
```bash
git commit -m "Update Alexa skill [alexa]"
```

## Alexa Integration Setup

You can deploy the Alexa skill either **automatically via GitHub Actions** or **manually**.

### Option A: Automated Deployment (Recommended)

1. **Configure Alexa secrets** (see GitHub Actions Setup above)
2. **Trigger deployment:**
   ```bash
   # Via commit message
   git commit -m "Deploy Alexa skill [alexa]"
   git push

   # Or manually via GitHub Actions UI
   # Go to Actions → Deploy Candle Lights Controller → Run workflow
   ```

3. **The script will automatically:**
   - Get Lambda ARN from CloudFormation
   - Create or update the Alexa skill
   - Configure Lambda permissions
   - Enable skill for testing

4. **Configure account linking** in Alexa Developer Console:
   - Authorization URL: `https://lights.jeremy.ninja/oauth/authorize`
   - Token URL: `https://api-lights.jeremy.ninja/oauth/token`

### Option B: Manual Deployment

#### 1. Create Alexa Smart Home Skill

1. Go to [Alexa Developer Console](https://developer.amazon.com/alexa/console/ask)
2. Create Skill → Smart Home
3. Use `alexa-skill/skill.json` as template

#### 2. Configure Lambda Function

From SAM deployment outputs, get the `AlexaSkillEndpoint`:

```bash
aws cloudformation describe-stacks \
  --stack-name candle-lights-prod \
  --query "Stacks[0].Outputs[?OutputKey=='AlexaSkillEndpoint'].OutputValue" \
  --output text
```

Add this ARN to your Alexa skill configuration.

#### 3. Enable Account Linking

Configure OAuth using `alexa-skill/account-linking.json` as reference:

- Authorization URL: `https://lights.jeremy.ninja/oauth/authorize`
- Token URL: `https://api-lights.jeremy.ninja/oauth/token`
- Client ID/Secret: Generated from your app

### Test Voice Commands

```
"Alexa, discover devices"
"Alexa, turn on candle lights"
"Alexa, set candle lights to red"
"Alexa, dim candle lights"
"Alexa, brighten candle lights"
```

## API Reference

### Authentication

```bash
# Register
curl -X POST https://api-lights.jeremy.ninja/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "username": "john",
    "password": "mypassword"
  }'

# Login
curl -X POST https://api-lights.jeremy.ninja/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "username": "john",
    "password": "mypassword"
  }'
```

### Patterns

```bash
# List patterns
curl https://api-lights.jeremy.ninja/patterns \
  -H "Authorization: Bearer $TOKEN"

# Create pattern
curl -X POST https://api-lights.jeremy.ninja/patterns \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Warm Candle",
    "type": "candle",
    "red": 255,
    "green": 100,
    "blue": 0,
    "brightness": 128,
    "speed": 50
  }'
```

### Devices

```bash
# Register device
curl -X POST https://api-lights.jeremy.ninja/devices \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Living Room Candles",
    "particleId": "your-particle-device-id"
  }'

# Assign pattern to device
curl -X PUT https://api-lights.jeremy.ninja/devices/{deviceId}/pattern \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "patternId": "pattern-uuid"
  }'
```

### Particle Commands

```bash
# Send command to device
curl -X POST https://api-lights.jeremy.ninja/particle/command \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "deviceId": "device-uuid",
    "patternId": "pattern-uuid"
  }'
```

## Development

### Local Development

```bash
# Backend
cd backend/functions/auth
go run main.go

# Frontend
cd frontend
go run main.go
```

Access at `http://localhost:3000`

### Testing

```bash
# Test firmware (requires Particle device)
particle compile argon firmware/

# Test Lambda functions
sam local invoke AuthFunction -e events/login.json

# Test API
sam local start-api
```

## Troubleshooting

### Device Not Connecting

1. Check WiFi credentials on Particle device
2. Verify Particle token in user settings
3. Check device is breathing cyan (connected to cloud)

### Patterns Not Applying

1. Verify Particle token is configured
2. Check device ID matches Particle console
3. Ensure device is online
4. Check CloudWatch logs for errors

### Alexa Not Discovering Devices

1. Verify account linking is complete
2. Check Lambda function permissions
3. Ensure devices are registered in DynamoDB
4. Try "Alexa, forget all devices" and rediscover

### Deployment Fails

1. Check AWS credentials are configured
2. Verify domain name and certificate
3. Ensure S3 bucket exists in us-east-1 region (run `aws s3api get-bucket-location --bucket YOUR_BUCKET`)
4. Verify `CLOUDFORMATION_S3_BUCKET` GitHub variable is set correctly
5. Check that the IAM user has permissions to write to the S3 bucket
6. Check CloudFormation stack events for errors

## Cost Estimate

### AWS Services (Monthly)
- Lambda: ~$0.20 (free tier)
- DynamoDB: ~$0.25 (free tier)
- API Gateway: ~$3.50 (1M requests)
- Route53: ~$0.50 per hosted zone
- **Total: ~$4-5/month** (after free tier)

### One-Time
- Domain name: ~$12/year
- Particle Argon: ~$25
- WS2812B LEDs: ~$10-20

## Contributing

Contributions are welcome! Please:

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

## License

MIT License - see LICENSE file for details

## Support

- Issues: [GitHub Issues](https://github.com/yourusername/garage-door-lights/issues)
- Documentation: [Particle Docs](https://docs.particle.io/)
- Community: [Particle Community](https://community.particle.io/)

## Acknowledgments

- [Particle.io](https://www.particle.io/) for the Argon platform
- [Adafruit](https://www.adafruit.com/) for NeoPixel library
- [AWS](https://aws.amazon.com/) for serverless infrastructure
- [Go Fiber](https://gofiber.io/) for the web framework

---

Made with ❤️ for IoT enthusiasts
