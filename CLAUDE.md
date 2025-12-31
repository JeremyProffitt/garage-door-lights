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
