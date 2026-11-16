# Pre-PR self-review checklist — csm-mail-paging-service

Run this against the staged diff before every commit, and against the branch diff versus the target branch before opening or updating a PR
(after a commit the staged diff is empty), touching this app. Check every item
below and call out anything that matches as a finding before proceeding to commit or open the PR — don't
just silently fix and move on, say what was found and why.

Every item here comes from this service's own documented design rules (`CLAUDE.md`) — not generic
code-review advice. If a new failure class turns up during review that isn't on this list, add it here
in the same style (short, one line, grounded in what actually happened) so the next review catches it too.

## Cross-cutting

- [ ] No secret, OAuth client secret, refresh token, or event-bus connection string staged in the diff.
- [ ] No real group address, sender, or subject copied into code, tests, docs, or commit/PR text — tests use
      `example.com`; no PII in a log line (message ids and rule names only).
- [ ] New behavior has a test exercising it, not just a happy-path smoke test.
- [ ] If this diff introduces a new convention/pattern/gotcha, `CLAUDE.md` is updated in the same
      PR — don't leave the next reader to rediscover it. Keep the addition short.
- [ ] `gosec -fmt=text ./...` and `govulncheck ./...` both clean (`GOTOOLCHAIN=auto` for gosec; bump
      `go.mod`'s `go` directive if the only finding is a stdlib CVE tied to the toolchain version).
- [ ] `go test -race ./...` clean.
- [ ] Commits/PR carry this session's attribution lines, not an older convention from memory.
- [ ] `git fetch upstream main` + `git merge-tree --write-tree HEAD upstream/main` checked clean
      before pushing — don't trust a merge that was clean an hour ago.
- [ ] No ticket/issue number or repo name cited in a code comment — explain the why directly.

## API design and naming

- [ ] The only endpoint is `GET /health`. Adding another needs a stated reason; if added, name/shape checked
      against GitHub's REST API conventions (https://docs.github.com/en/rest), and an existing one reused where
      existing callers' behavior stays unchanged.
- [ ] Non-breaking for others: the event `type`, envelope and payload field names, `dedupeKey` format, rule
      fields and env var names are unchanged or only added to — csm-notification-service decodes them strictly.
      A removal/rename/tightening needs a stated migration plan.
- [ ] Methods, variables and types are named for what they mean (`caseID`, `fetchOpenCases` — not
      `id2`, `doStuff`) and follow Go idiom (MixedCaps, no package-name stutter).

## Mail handling

- [ ] A message is labelled processed only after every matching rule's event was published — the label is the
      only idempotency record. Not moved earlier, and no second state store added for correctness.
- [ ] Headers only (`format=metadata`) — no code path reads a message body.
- [ ] Still one `main` package (`cmd/server`); any helper is a subcommand, not a second `cmd/` directory
      (the Choreo buildpack fails with "no Go files" otherwise).
- [ ] Auth stays an OAuth refresh token for one mailbox user — no domain-wide delegation or service account.
- [ ] No call/paging logic added here — calls belong to csm-notification-service's `internal/mailpaging`.
- [ ] `eventbus` and `poller.Envelope` still match csm-notification-service's copies (separate modules, kept in
      sync by hand); a payload change is mirrored there and deployed consumer-first, since it decodes strictly.
- [ ] A new rule or event type keeps `name` unique and `dedupeKey` distinct per rule (one email matching two
      rules must yield two events).
- [ ] Config stays flat env strings (`MAIL_TRIGGER_RULES` is one JSON string); `.env.example` and the README
      are updated for any new variable.
- [ ] Still single-replica (Choreo); nothing added that assumes more than one poller.
