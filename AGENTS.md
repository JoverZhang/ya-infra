# Workspace Instructions

## Purpose

This repository is a workspace for coordinating work across the Git repositories under this directory. It does not contain product code.

The user will specify the target repository or repositories. Do not modify this workspace unless explicitly requested.

## Project Instructions

Before working in a target repository, read its `AGENTS.md` and any related instructions. Do this separately for each repository.

On first access to a Git repository during a session, run `git fetch origin` and fast-forward its local default branch to `origin/<default-branch>`.

## Surgical Changes

When modifying a target repository:

- Touch only what the requested outcome requires.
- Preserve unrelated code, documentation, comments, formatting, and names.
- Refactor existing code only when the user requests it or the requested change requires it.
- Match the target repository's established style and conventions.
- Remove only artifacts that the current change made unused.
- Report unrelated problems instead of fixing them.

Automatically loaded skills guide in-scope work. They do not expand the task or authorize adjacent changes.

## Read-only Work

For investigation, analysis, explanation, and other read-only tasks, use the existing checkout directly. A worktree is not required.

## Repository Changes

Use a dedicated Git worktree when modifying code, tests, documentation, or configuration.

- Keep each repository's primary checkout at the workspace root.
- Create one task worktree per repository at `.worktrees/<repository>/<task-slug>`, where
  `<repository>` is the primary checkout's directory name.
- Create task worktrees for this workspace repository at `.worktrees/_workspace/<task-slug>`.
- Branch name: `agent/<task-slug>`.
- Start from the remote default branch by default.
- Do not modify the original checkout or its current branch.
- Reuse an existing worktree for the same task when available.
- Modify the current checkout directly only when the user explicitly requests it and the checkout is not on the default branch.

Before making changes, prepare the environment and isolated resources according to the target repository's instructions.

## Validation and Delivery

Unless the user explicitly asks to keep changes local, push the task branch and use `gh` to create or update a Draft PR.

Mark the PR as Ready only after all checks pass and `$code-review` issues have been resolved.

## Worktree Cleanup

Remove a worktree only when required by the task or explicitly requested by the user.
