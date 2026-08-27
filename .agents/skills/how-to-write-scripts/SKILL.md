---
name: how-to-write-scripts
description: Standards for Bash, Python, and PowerShell workflow scripts that orchestrate ordered external effects. Use when creating, changing, or reviewing automation scripts such as setup, build, release, migration, and repository tooling. Do not use for business or data computation, one-off command snippets, task-runner or CI configuration, or interactive wizards.
---

# How to Write Scripts

A script owns one workflow. Keep a single-phase workflow inline. When a script has multiple external-effect phases, give it one workflow entry point that calls named actions in execution order.

The workflow entry point expresses sequence. Commands, I/O, output parsing, default selection, rollback mechanics, and other implementation details belong in actions.

An `action` names a stable workflow step and hides its concrete mechanics, such as `git`, `gh`, task-runner, filesystem, or process calls. It also owns the relevant error or cleanup responsibility.

Extract an action when at least one condition holds:

- Real callers reuse it.
- It hides enough mechanics to make the call site materially simpler.
- It provides a clear state, error, or cleanup seam.
- It represents a stable workflow step whose name carries more intent than its implementation.

Keep a single command inline instead of adding a synonymous wrapper. Name actions after intent; names such as `step_1`, `run_command`, and `do_work` only restate mechanics.

Expected shape:

```bash
main() {
  parse_task_request "$@"
  resolve_task_worktree
  reserve_base_port
  create_task_worktree
  initialize_task_worktree
}

main "$@"
```

The same shape applies across languages: the workflow shows the ordered story, while actions contain the mechanics.

Mechanics exposed at the workflow level obscure that story:

```bash
task_slug="$1"
git worktree add "../project-$task_slug" -b "agent/$task_slug"
cd "../project-$task_slug"
scripts/setup_config.sh 21000
```
