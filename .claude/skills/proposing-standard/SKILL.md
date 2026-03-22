---
name: proposing-standard
description: Proposes an update to the shared vault standards based on a change discovered while working in a project. Updates the relevant vault template files, bumps the standards version, and adds a log entry. Must only run when explicitly requested — never invoke automatically.
disable-model-invocation: true
---

# Proposing a Standards Update

This skill updates the shared vault templates to reflect a new or changed convention discovered while working in a project. It must only be run when Bailey explicitly asks — never infer or trigger this automatically.

## Step 1: Confirm vault access

Check whether the vault is accessible at one of the known paths:
- **Desktop / NAS:** `\\GALACTICA\home\Git\obsidian\main`
- **Laptop:** `C:\Users\baile\Documents\Obsidian\main\main`

If neither is accessible, stop and tell the user the vault is not reachable from this machine. Do not proceed.

## Step 2: Understand the proposed change

Ask the user to describe the change if they haven't already:
- What convention or standard is changing?
- Which template file(s) does it affect?
- Is this a new addition, a modification to existing content, or a removal?
- Should this change affect all future projects, or is it a project-specific deviation that should not be standardised?

If it is project-specific, do not proceed — note it in the project's `CLAUDE.md` under Project-Specific Notes instead.

## Step 3: Identify affected template files

Map the change to the relevant files in `020 areas/AI/Knowledge/Software Project Templates/`:

| If the change affects... | Update this file |
| ------------------------ | ---------------- |
| Workflow, testing, conventions, doc templates | `CLAUDE.md` |
| GitHub issue structure or checklist | `github-issue-feature.md` or `github-issue-bug.md` |
| Claude Code permissions or hooks | `dotclaude/settings.json` |
| Issue creation workflow | `dotclaude/skills/creating-issue/SKILL.md` |
| Issue work workflow | `dotclaude/skills/working-issue/SKILL.md` |
| Standards sync workflow | `dotclaude/skills/syncing-standards/SKILL.md` |
| Scaffolding structure or template index | `Software Project Setup.md` (vault knowledge doc) |

Multiple files may need updating for a single change.

## Step 4: Draft the changes

Read the current content of each affected file. Draft the proposed edits. Show Bailey a clear summary before writing anything:

```
Proposed standards update:

Affected files:
- `CLAUDE.md` — [describe the change]
- `dotclaude/skills/working-issue/SKILL.md` — [describe the change]

New standards version: X.Y

OK to apply?
```

Wait for explicit approval before writing.

## Step 5: Apply the changes

Once approved:
1. Edit each affected template file in the vault
2. Bump `standards_version` in `Software Project Setup.md` frontmatter
   - Increment the patch version for clarifications or additions (`1.0` → `1.1`)
   - Increment the minor version for meaningful workflow changes (`1.0` → `1.1` → `2.0`)
3. Update the `Standards Version` value in `Software Project Templates/CLAUDE.md` to match
4. Add a log entry to `Software Project Setup.md` describing what changed and why

Log entry format:
```markdown
- [[YYYY-MM-DD]]
	- **Standards vX.Y** — [description of what changed and why it was changed] #ai/generated
```

## Step 6: Update the current project

After the vault is updated, also apply the same change to the current project's files so it is immediately on the new standard. Bump the `Standards Version` line in the project's `CLAUDE.md`.

Commit the project update:
```bash
git add CLAUDE.md .claude/
git commit -m "chore: update standards to vX.Y — [brief description]"
git push
```

## Step 7: Report

Tell the user:
- Which vault files were updated
- The new standards version
- That other projects can be brought up to date with "sync standards"
