# Go style

## Purpose

This standard applies to Go tooling and tests in this repository.

Optimize for a reviewer who has no access to the author's chat history. The code and nearby documentation must explain its behavior, side effects, and failure paths.

Use [Google Go Style](https://google.github.io/styleguide/go/guide.html) as the baseline. The rules below define the project's review priorities.

## Rules

1. **Show the main path.** Keep operations in execution order. Separate meaningful phases, such as reading state, planning changes, and applying changes. Handle errors early. A reader should see the workflow without tracing many small wrappers.
2. **Use types for known data.** Represent known configuration and API fields with structs. Keep dynamic data at external boundaries. Preserve unknown provider fields when an update must retain them.
3. **Match names to scope.** Use `route`, `plan`, and `records` when values cross several steps. Short names such as `i` and `err` fit small scopes and established Go conventions.
4. **Expose effects and failures.** Make file writes, remote changes, and state updates visible in the calling code. Handle errors and add useful operation and resource context. Explain any deliberate error discard.
5. **Make abstractions earn their cost.** Prefer concrete types and direct calls. Add an interface, callback, or generic helper when a current requirement justifies it. A helper with one caller is useful if it names a complete concept. Function length and duplication are signals to inspect, not automatic reasons to split code.
6. **Explain constraints.** Comments should explain ownership rules, ordering, retry behavior, or other constraints that names cannot express. Keep each explanation beside the code it governs.
7. **Make tests readable.** Keep the input, action, and expected result visible. Use fixtures for complex documents. Use tables when cases share the same behavior; use separate tests when cases need different workflows.
8. **Keep changes focused.** Each change must support the requested outcome. Separate behavior changes from structural cleanup when practical, so a reviewer can assess each independently.

## Examples

These snippets illustrate the rules. They do not prescribe new packages or APIs.

### Give known data a type

Before:

```go
func isProxiedCNAME(record map[string]any) bool {
	return record["type"] == "CNAME" && record["proxied"] == true
}
```

After:

```go
type DNSRecord struct {
	Type    string `json:"type"`
	Proxied bool   `json:"proxied"`
}

func isProxiedCNAME(record DNSRecord) bool {
	return record.Type == "CNAME" && record.Proxied
}
```

Decode these fields at the API boundary. Keep the original payload when a later write must preserve other fields.

### Keep the cause of a failure

Before:

```go
if json.Unmarshal(data, &state) != nil {
	return errors.New("invalid state")
}
```

After:

```go
if err := json.Unmarshal(data, &state); err != nil {
	return fmt.Errorf("decode gateway state %q: %w", path, err)
}
```

Report useful context without including credentials or secret values.

## Review gate

Alongside the checks required for the change, a reviewer must be able to answer from the code and nearby documentation:

- What input starts this behavior, and what result is expected?
- Which files or remote resources can change, and in what order?
- What remains after a failure, and what does a retry do?
- Which current requirement justifies each new abstraction?

If an answer depends on the author's chat history, improve the names, structure, or local explanation before approval.

## References

- [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments): common Go review concerns.
- [Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md): concrete style examples.
- [Karpathy-Inspired Claude Code Guidelines](https://github.com/multica-ai/andrej-karpathy-skills): community guidance for focused agent changes, based on Karpathy's observations.
- [Claude Code best practices](https://code.claude.com/docs/en/best-practices#write-an-effective-claudemd): concise project instructions and validation through observed behavior.
