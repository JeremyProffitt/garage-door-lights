# Claude Code Instructions

**NEVER USE `sam deploy` ON THIS PROJECT** - All deployments are via GitHub Actions.

To deploy changes:
1. Commit and push to the `main` branch
2. GitHub Actions will automatically build and deploy the stack

For Lambda-only updates during development, you can manually update individual functions using the AWS SDK/CLI, but infrastructure changes (API Gateway routes, DynamoDB tables, etc.) require a push to trigger the GitHub Actions workflow.
