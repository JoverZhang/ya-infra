# Personal inbox

A password-protected, read-only GitHub task view plus local deployment observations.
Runtime configuration and SQLite can contain private data. Keep both outside Git.

```sh
go test -race -v ./...
go vet ./...
```

The HTTP/SQLite tests exercise authentication, merged GitHub observations, local
read/snooze persistence, upstream failure retention, site fault recovery and safe
error reporting. Fixtures use no real accounts or repositories.

GitHub collection combines open review requests, assigned issues/PRs, authored PRs
with requested changes, and unread assignments/mentions/review requests/comments. One URL is
one item. A completed snapshot resolves missing items. A failed or truncated
snapshot retains the previous tasks and creates a source error. Notifications
need a classic PAT with `repo` coverage. This version requires private-repository
coverage before replacing a cached task snapshot; a public-only token is rejected.
The `notifications` scope can be added explicitly. Rate-limit retry deadlines are
honored for all requests, including site revision checks.

Read/snooze state belongs only to this inbox; no GitHub records are changed.
Read tasks remain active. New upstream updates or recurring faults become unread.
Resolved items remain available in history. There is no push notification sender.

Site observations check local/public HTTP, the selected GitHub branch revision,
the published git-sync symlink, and the git-sync error file/process endpoint.
An unknown upstream revision is explicit, not a claim that the page is current.
Raw synchronization errors, configuration, tokens and upstream response bodies
are never returned to the browser or written to application logs. Site aliases
can omit private project identities.

Collectors run on startup and at least every 60 seconds, honoring a longer GitHub
`X-Poll-Interval`. The UI refresh button reloads cached observations. Source status
shows when collection was attempted and whether the stored tasks may be stale.
