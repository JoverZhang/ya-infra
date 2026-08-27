---
name: how-to-write-scripts
description: Standards for writing, changing, or reviewing automation scripts that coordinate external effects.
---

# How to Write Scripts

A script should read like the outline of one workflow.

- Keep a simple workflow inline; for a multi-step workflow, let the entry point call named actions in order.
- In a multi-step workflow, put commands, I/O, parsing, and error or cleanup mechanics inside actions.
- Name actions by intent, such as `prepare_release`, rather than mechanics such as `run_command`.
- A stable workflow step justifies a top-level action. Extract lower-level helpers only for meaningful complexity, an error or cleanup boundary, or real reuse.
- Give a multi-step entry point a one-line comment stating the workflow outcome.
- Separate top-level action calls with blank lines. Add a comment above a call only for non-obvious workflow context, such as an ordering constraint or external-effect boundary.

```bash
# Publishes one resolved target.
main() {
  parse_request "$@"

  resolve_target

  build_target

  # Publication is the first externally visible step.
  publish_target
}
main "$@"
```
