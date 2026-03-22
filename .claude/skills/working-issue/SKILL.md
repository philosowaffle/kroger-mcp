---
name: working-issue
description: Starts work on an existing GitHub issue by fetching its details, creating the correct branch, writing the spec file, committing, and pushing — leaving the project in a ready-to-implement state. Use this skill whenever the user says "work on issue", "start issue", "pick up issue", "implement issue", or provides an issue number and implies they want to begin working on it. Always use this skill when the intent is to begin development on a tracked GitHub issue.
---

# Working a GitHub Issue

When the user wants to start work on an issue, execute the full setup sequence before writing any implementation code.

## Step 1: Fetch the issue

```bash
gh issue view <number> --json number,title,body,labels
```

Read the issue fully — understand the problem statement, acceptance criteria, and any blocking relationships. If the issue is marked as blocked by another open issue, flag this to the user before continuing.

## Step 2: Determine branch type

Use the issue label to pick the branch prefix:
- `feat` → `feat/`
- `fix` → `fix/`
- `chore` → `chore/`
- Unlabelled → ask the user

## Step 3: Create and push the branch

```bash
git checkout main
git pull origin main
git checkout -b <type>/issue<number>-<short-description>
git push -u origin <type>/issue<number>-<short-description>
```

Derive the short description from the issue title — lowercase, hyphen-separated, 3–5 words.

## Step 4: Write the spec file

Create `docs/specs/<number>-<short-title>.md` using this template:

```markdown
# Spec: <Issue Title>

**Issue:** #<number>
**Status:** Draft

## Problem
<Extract or summarise from the issue's Problem Statement / Motivation>

## Acceptance Criteria
<Convert each AC from the issue checklist into a testable checkbox here>
- [ ]

## Notes
<Any relevant context, constraints, or open questions from the issue body>
```

Populate it from the issue content — do not leave it empty.

## Step 5: Commit and push the spec

```bash
git add docs/specs/<number>-<short-title>.md
git commit -m "docs: write spec for #<number>"
git push
```

## Step 6: Write the implementation plan

Create `tmp/plan.md` with:
- What needs to be built or changed
- Key files or packages involved
- Order of operations
- Any decisions or trade-offs worth noting upfront

```bash
git add tmp/plan.md
git commit -m "docs: implementation plan for #<number>"
git push
```

## Step 7: Report back and hand off

Tell the user:
- Branch name
- Spec file path
- A brief summary of the plan

The project is now ready for implementation. Wait for the user's direction on what to build first, or proceed if they've already described it.
