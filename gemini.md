# Project Context for Gemini

## CRITICAL: BRANCHING POLICY
**YOU MUST ONLY WORK ON THE `main` BRANCH.**

- **NEVER** switch to `v0`, `develop`, `staging`, or any other branch.
- **NEVER** create new branches unless explicitly instructed by the user.
- **ALWAYS** check `git status` to ensure you are on `main` before committing.
- **REASON:** The CI/CD pipeline (GitHub Actions) **ONLY** triggers on pushes to `main`. Pushing to any other branch results in code that is **NOT DEPLOYED**, wasting time and confusing the user.

## Project Standards
For all other project standards, architectural patterns, and deployment policies, strictly adhere to the guidelines defined in **claude.md**.