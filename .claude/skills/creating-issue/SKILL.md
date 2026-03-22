---
name: creating-issue
description: Creates a new GitHub issue for this project using the correct template, label, and blocking relationships. Use this skill whenever the user wants to create an issue, track a new feature, report a bug, add a task, or says things like "make an issue for", "create a feature for", "log a bug for", "I want to add", or "track this". Always use this skill when the intent is to capture new work in GitHub Issues.
---

# Creating a GitHub Issue

When the user describes new work to track, create a properly structured GitHub issue following project conventions.

## Step 1: Classify the issue

Determine the type based on what the user described:
- **feat** — new feature or enhancement
- **fix** — bug or broken behaviour
- **chore** — maintenance, dependency update, tooling, refactor

If unclear, ask before proceeding.

## Step 2: Gather missing details

If the user's description is thin, ask for:
- A clear one-line summary (becomes the issue title)
- The problem it solves or the bug behaviour (for the Problem Statement)
- Any known acceptance criteria

Keep questions minimal — one prompt if possible.

## Step 3: Draft the issue

Use the appropriate template from `.github/ISSUE_TEMPLATE/`:
- `feature.md` for `feat`
- `bug.md` for `fix` or `chore`

Fill in all sections from what the user provided. Leave `## Blocking / Blocked By` blank unless the user mentioned dependencies — if they did, note which issue numbers and set the native GitHub relationship after creation.

Issue title format:
```
<type>: <short description>
```
Example: `feat: add user authentication`

## Step 4: Confirm before creating

Show the user the proposed title, type/label, and a summary of the issue body. Wait for approval.

## Step 5: Create the issue

Use `gh issue create` with:
- `--title` matching the formatted title
- `--label` matching the type (`feat`, `fix`, or `chore`)
- `--body` using the filled template content

```bash
gh issue create --title "<type>: <description>" --label "<type>" --body "<filled template>"
```

## Step 6: Report back

Tell the user the issue number and URL. If blocking relationships were mentioned, remind them to set those natively in GitHub — link directly to the issue.
