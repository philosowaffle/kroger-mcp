---
name: syncing-standards
description: Checks whether this project's standards files are up to date with the vault templates, diffs what has changed, and applies approved updates. Use this skill whenever the user says "sync standards", "are we up to date", "check standards", "update claude settings", or any similar phrase indicating they want to bring the project in line with the latest shared conventions.
---

# Syncing Project Standards

When the user wants to check or update standards, compare the project's current standards files against the vault templates and apply approved changes.

## Step 1: Read the project's current standards version

Read `CLAUDE.md` and find the `Standards Version` near the top of the file. Note the version number and last sync date.

| Standards Version | Last Synced Date |
| ----------------- | ---------------- |
|                   |                  |

## Step 2: Locate the vault templates

The vault templates live at:
```
020 areas/AI/Knowledge/Software Project Templates/
```

The vault path varies by machine:
- **Desktop / NAS:** `\\GALACTICA\home\Git\obsidian\main`
- **Laptop:** `C:\Users\baile\Documents\Obsidian\main\main`

Check whether either path is accessible. If neither is accessible, tell the user the vault is not reachable from this machine and ask them to either mount it or paste in the relevant template content manually. Do not proceed without template access.

Read `Software Project Setup.md` frontmatter from the vault to get the current `standards_version`. If the project version matches the vault version, report that the project is up to date and stop.

## Step 3: Read both versions of each standards file

Read the following file pairs — project version (left) vs vault template (right):

| Project file | Vault template |
| ------------ | -------------- |
| `CLAUDE.md` | `Software Project Templates/CLAUDE.md` |
| `.claude/settings.json` | `Software Project Templates/dotclaude/settings.json` |
| `.claude/skills/creating-issue/SKILL.md` | `Software Project Templates/dotclaude/skills/creating-issue/SKILL.md` |
| `.claude/skills/working-issue/SKILL.md` | `Software Project Templates/dotclaude/skills/working-issue/SKILL.md` |
| `.claude/skills/syncing-standards/SKILL.md` | `Software Project Templates/dotclaude/skills/syncing-standards/SKILL.md` |
| `.github/ISSUE_TEMPLATE/feature.md` | `Software Project Templates/github-issue-feature.md` |
| `.github/ISSUE_TEMPLATE/bug.md` | `Software Project Templates/github-issue-bug.md` |

## Step 4: Diff and classify changes

For each file, identify what changed. Classify each change as one of:
- **Non-breaking** — additions, clarifications, new sections, updated wording. Safe to apply directly.
- **Potentially breaking** — changes to existing workflow steps, permission rules, hook commands, or skill trigger descriptions. Flag for review.
- **Conflict** — the project file has been intentionally customised in a way that differs from the template (e.g. project-specific tasks in `Taskfile`, custom permissions). Do not overwrite — present the conflict and ask how to resolve it.

## Step 5: Present the sync plan

Show a clear summary before changing anything:

```
Standards update: v<project> → v<vault>

Ready to apply:
- CLAUDE.md — [brief description of changes]
- .claude/settings.json — [brief description of changes]
- ...

Needs your review:
- .claude/skills/working-issue/SKILL.md — [describe the breaking change]

Conflicts (will not auto-apply):
- [file] — project has [X], template has [Y]. How would you like to handle this?

OK to proceed?
```

Wait for approval before writing anything.

## Step 6: Apply approved changes

For each approved file:
- Apply non-breaking changes directly
- Apply reviewed breaking changes as approved
- Skip or manually merge conflicts as directed

After all updates, bump the `Standards Version` line in `CLAUDE.md` to match the vault version and current system date.

## Step 7: Commit the update

```bash
git add CLAUDE.md .claude/ .github/ISSUE_TEMPLATE/
git commit -m "chore: sync project standards to v<version>"
git push
```

## Step 8: Report

Summarize what was updated, what was skipped, and any conflicts that still need manual resolution.

---

## Known Limitation

This skill requires the Obsidian vault to be accessible from the current machine. On machines without vault access, the user must provide template content manually or defer the sync to a machine where the vault is mounted.
