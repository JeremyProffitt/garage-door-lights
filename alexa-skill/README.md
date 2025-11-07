# Alexa Smart Home Skill

This directory contains the configuration for the Alexa Smart Home skill that provides voice control for the Candle Lights Controller.

## Automated Deployment

The skill can be automatically deployed via GitHub Actions. See the main README for setup instructions.

## Files

- `skill.json` - Alexa skill manifest configuration
- `account-linking.json` - OAuth account linking configuration template

## Supported Voice Commands

Once the skill is deployed and linked to your account, you can use these commands:

### Power Control
- "Alexa, turn on candle lights"
- "Alexa, turn off candle lights"

### Brightness Control
- "Alexa, set candle lights to 50 percent"
- "Alexa, dim candle lights"
- "Alexa, brighten candle lights"

### Color Control
- "Alexa, set candle lights to red"
- "Alexa, set candle lights to warm white"
- "Alexa, change candle lights to blue"

## Smart Home Capabilities

The skill implements the following Alexa Smart Home interfaces:

1. **PowerController** - Turn devices on/off
2. **BrightnessController** - Adjust brightness (0-100%)
3. **ColorController** - Set HSB color values
4. **Alexa.Discovery** - Device discovery

## Device Discovery

After enabling the skill and linking your account:

1. Say "Alexa, discover devices"
2. Or use the Alexa app: Devices → + → Add Device → Other
3. All your registered Particle Argon devices will appear as controllable lights

## Account Linking

The skill uses OAuth 2.0 account linking to connect your Alexa account with your Candle Lights Controller account.

### Flow:
1. User enables skill in Alexa app
2. User is redirected to your website for login
3. User grants permission to Alexa
4. Alexa receives access token
5. All subsequent requests include this token

### Endpoints:
- **Authorization URL**: `https://lights.jeremy.ninja/oauth/authorize`
- **Token URL**: `https://api.lights.jeremy.ninja/oauth/token`

## Deployment Script

The `deploy-alexa.sh` script in the `scripts/` directory automates:

1. Getting Lambda ARN from CloudFormation
2. Updating skill.json with correct Lambda ARN
3. Creating or updating the Alexa skill via ASK CLI
4. Adding Lambda invoke permissions for Alexa
5. Enabling the skill for testing

## Manual Deployment

If you prefer manual deployment:

```bash
# Install ASK CLI
npm install -g ask-cli

# Configure ASK CLI
ask configure

# Update skill.json with your Lambda ARN
# Then deploy
ask smapi create-skill-for-vendor \
  --manifest file:skill.json \
  --vendor-id YOUR_VENDOR_ID
```

## Testing

### Simulation
Use the Alexa Developer Console to test without a physical device:
1. Go to Test tab
2. Enable testing for "Development"
3. Type or say commands

### Real Device
1. Enable skill in Alexa app
2. Link account
3. Discover devices
4. Test with your Alexa device

## Troubleshooting

### Skill Not Discovering Devices
- Ensure account linking is complete
- Check that devices are registered in DynamoDB
- Verify Lambda has correct permissions
- Check CloudWatch logs for errors

### Commands Not Working
- Verify device is online in your web dashboard
- Check that Particle token is configured
- Ensure Lambda can reach Particle Cloud API
- Review Lambda CloudWatch logs

### Account Linking Fails
- Verify OAuth endpoints are accessible
- Check that domain has valid SSL certificate
- Ensure JWT secret is configured correctly
- Test login flow directly in web browser

## Resources

- [Alexa Smart Home API](https://developer.amazon.com/docs/smarthome/understand-the-smart-home-skill-api.html)
- [ASK CLI Documentation](https://developer.amazon.com/docs/smapi/ask-cli-intro.html)
- [Smart Home Skill Testing](https://developer.amazon.com/docs/smarthome/test-your-smart-home-skill.html)
