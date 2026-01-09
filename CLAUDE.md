# Claude Code Instructions

## Thinking Mode

**ALWAYS USE MAXIMUM THINKING RESOURCES** - Use `ultrathink` (extended thinking with maximum tokens) for all decisions, problem-solving, and implementation work. Never use standard thinking when extended thinking is available.

## Deployment

**NEVER USE `sam deploy` ON THIS PROJECT** - All deployments are via GitHub Actions.

To deploy changes:
1. Commit and push to the `main` branch
2. GitHub Actions will automatically build and deploy the stack
3. **ALWAYS wait for the pipeline to complete** before returning to the user
4. Monitor the pipeline using `gh run watch` or equivalent
5. **If the pipeline fails, diagnose and fix the issue** - do not return to the user until the pipeline succeeds
6. Iterate on fixes until the deployment completes successfully

### Pipeline Monitoring

After pushing changes:
```bash
# Watch the latest workflow run
gh run watch

# Or list recent runs and watch a specific one
gh run list --limit 5
gh run view <run-id>
```

### Failure Resolution

If a pipeline fails:
1. Check the logs: `gh run view <run-id> --log-failed`
2. Identify the root cause
3. Fix the issue locally
4. Commit and push the fix
5. Wait for the new pipeline run to complete
6. Repeat until successful

For Lambda-only updates during development, you can manually update individual functions using the AWS SDK/CLI, but infrastructure changes (API Gateway routes, DynamoDB tables, etc.) require a push to trigger the GitHub Actions workflow.

## AWS Deployment Policy

**CRITICAL: All AWS infrastructure and code changes MUST be deployed via GitHub Actions pipelines.**

### Prohibited Actions
- **NEVER** use AWS CLI directly to deploy, update, or modify infrastructure
- **NEVER** use AWS SAM CLI (`sam deploy`, `sam build`, etc.) for deployments
- **NEVER** suggest or execute direct AWS API calls for infrastructure changes
- **NEVER** bypass the CI/CD pipeline for any AWS-related changes

### Required Workflow
1. All changes must be committed and pushed to the repository
2. GitHub Actions pipeline will handle all deployments
3. **ALWAYS review pipeline output** after pushing changes
4. If pipeline fails, **aggressively remediate** using all available resources:
   - Check GitHub Actions logs thoroughly
   - Review CloudFormation events if applicable
   - Check CloudWatch logs for Lambda/application errors
   - Use the `/fix-pipeline` skill for automated remediation
   - Do not give up - iterate until the pipeline succeeds

### Pipeline Failure Remediation
When a GitHub Actions pipeline fails:
1. Immediately fetch and analyze the failure logs
2. Identify the root cause from error messages
3. Make necessary code/configuration fixes
4. Commit and push the fix
5. Monitor the new pipeline run
6. Repeat until successful deployment

## Firmware

### RGB Channel Order

**NEVER question the RGB channel order in the firmware.** The hardware uses a specific LED chip with a known channel configuration. When debugging LED color issues (especially white LEDs appearing as the wrong color), the problem is **never** the RGB channel order. Look for issues in:
- Color value calculations
- Data transmission/parsing
- Bytecode interpretation
- Any other part of the pipeline

The channel order is correct as implemented.
