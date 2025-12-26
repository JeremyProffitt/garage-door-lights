---
name: watch-action
description: Monitor GitHub Actions workflows and notify when they complete. Use when user wants to watch, monitor, or track a GitHub Actions run and be notified when it finishes.
user-invocable: true
---

# Watch GitHub Action

Monitor a GitHub Actions workflow run and notify when it completes.

## Instructions

When invoked:

1. **Identify the run to monitor**
   - If a run ID is provided as an argument, use it
   - Otherwise, list recent runs and find the most recent in-progress run:
     ```bash
     gh run list --limit 5
     ```

2. **Get initial status**
   ```bash
   gh run view <run-id> --json status,conclusion,displayTitle,url
   ```

3. **If already complete**, report the result immediately and stop.

4. **If in progress**, launch a background agent to monitor:
   - Use the Task tool with `run_in_background: true`
   - The agent should check status every 60 seconds
   - Continue until status is "completed"
   - Report the final conclusion (success/failure)

5. **Notification format** when complete:
   ```
   GitHub Action Complete!

   Run:        <displayTitle>
   Status:     <conclusion> (SUCCESS/FAILURE)
   URL:        <url>
   ```

## Examples

### Watch current run
```
/watch-action
```
Finds the most recent in-progress run and monitors it.

### Watch specific run
```
/watch-action 20509440262
```
Monitors the specified run ID.

## Agent Prompt Template

When launching the background monitoring agent, use this prompt:

```
Monitor GitHub Action run {RUN_ID}. Check every 60 seconds using:
  gh run view {RUN_ID} --json status,conclusion,displayTitle,url

When status becomes "completed":
1. Report clearly: "GitHub Action Complete!"
2. Show the run title, conclusion (SUCCESS/FAILURE), and URL
3. Stop monitoring

Keep checking until complete. Do not stop early.
```
