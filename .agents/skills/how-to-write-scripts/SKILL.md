---
name: how-to-write-scripts
description: Standards for writing, changing, or reviewing automation scripts that coordinate external effects.
---

# How to Write Scripts

A script should read like the outline of one workflow.

- Keep a simple workflow inline.
- For a multi-step workflow, let the entry point call named actions in order.
- In a multi-step workflow, put commands, I/O, parsing, and error or cleanup mechanics inside actions.
- Name actions by intent, such as `prepare_release`, rather than mechanics such as `run_command`.
- A stable workflow step justifies a top-level action. Extract lower-level helpers only for meaningful complexity, an error or cleanup boundary, or real reuse.

```bash
main() {
  parse_request "$@"
  resolve_target
  build_target
  publish_target
}
main "$@"
```
