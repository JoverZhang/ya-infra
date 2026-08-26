---
name: plain-language-rewrite
description: Rewrite selected responses, prose, or Markdown with one of four plain-language strategies. Invoke explicitly when a rewrite is wanted.
---

# Plain Language Rewrite

Rewrite only the target selected by the user. This skill is explicit-only and never runs as a background hook.

## Target

Use explicitly supplied text or an explicitly named file. When the invocation names no target, rewrite the immediately preceding assistant response.

Use the preceding user request only as context for understanding the target. Rewrite the target rather than answering, repeating, or rewriting that request.

For a named file, return the rewrite without changing the file unless the user explicitly requests a write-back.

## Strategy

Read exactly one strategy reference:

- For the default plain-language rewrite, read [references/plain.md](references/plain.md).
- For a short summary requested as `tldr` or `TL;DR`, read [references/tldr.md](references/tldr.md).
- For a `5y`, “like I'm five,” or equivalent explanation, read [references/five-year-old.md](references/five-year-old.md).
- For a structure-preserving rewrite of Markdown, read [references/markdown.md](references/markdown.md).

An explicitly requested strategy wins. Otherwise, use `markdown` for Markdown that should remain a Markdown document and `plain` for every other target.

## Language

Keep the target's language unless the user explicitly requests another language. A requested language overrides the same-language instruction in the selected reference. Translate prose while preserving code, identifiers, file paths, commands, URLs, and quoted output exactly.

## Output

Return only the rewritten content unless the user requests commentary or a file write-back. Before completing, verify that the selected strategy's preservation requirements are satisfied.

