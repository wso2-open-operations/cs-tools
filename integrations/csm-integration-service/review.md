# Pre-PR self-review checklist — csm-integration-service

Run this against the diff before opening or updating a PR touching this app. Check every item
below and call out anything that matches as a finding before proceeding to open the PR — don't
just silently fix and move on, say what was found and why.

Every item here comes from this codebase's own documented failure history (`CLAUDE.md` has dozens
of "a real, reported bug" / "confirmed live" write-ups) — not generic code-review advice. If a new
failure class turns up during review that isn't on this list, add it here in the same style
(short, one line, grounded in what actually happened) so the next review catches it too.

## Cross-cutting

- [ ] No secret, token, or service URL with embedded credentials staged in the diff.
- [ ] No PII in a log line — IDs and sanitized summaries only.
- [ ] No raw upstream error body echoed to the caller — use the fixed `ErrMsg*` constants or a
      short fallback message, never pass through an entity-service error's full text.
- [ ] New behavior has a test exercising it (follow `accounts_test.go`/`projects_test.go`'s shape:
      empty/invalid path param, body-too-large, invalid JSON, success passthrough, an
      `upstreamErrors` table).
- [ ] If this diff introduces a new convention/pattern/gotcha, `CLAUDE.md` is updated in the same
      PR — don't leave the next reader to rediscover it.
- [ ] `go vet`/`make test` clean before opening the PR.
- [ ] Commits/PR carry this session's attribution lines, not an older convention from memory.
- [ ] `git fetch upstream main` + `git merge-tree --write-tree HEAD upstream/main` checked clean
      before pushing — don't trust a merge that was clean an hour ago.
- [ ] No real name/email/URL/org-specific identifier copied from a screenshot, pasted log, or API
      response into code, tests, docs, or commit/PR text — use a generic placeholder.
- [ ] No ticket/issue number or repo name cited in a code comment — explain the why directly.

## M2M-only identity — the recurring source of confusing bugs here

- [ ] Before documenting or assuming a new endpoint "works" or "always 401s," checked **which**
      entity-service data source it targets and whether that path requires a forwarded end-user
      identity — this service has structurally no mechanism to carry one (removed on purpose, see
      the "no end-user identity is ever forwarded" section), so a ServiceNow-backed operation that
      strictly requires a token is an unconditional 401 here, while the same operation on
      `DATA_SOURCE=postgres` (or one with a configured M2M-credential fallback) can genuinely
      succeed. Don't generalize from one endpoint's behavior to another without checking both the
      data source and whether that specific ServiceNow operation has an M2M fallback.
- [ ] A new endpoint that proxies a entity-service operation requiring a forwarded user token is
      flagged explicitly as such in `CLAUDE.md` — "kept for API-shape completeness, every call
      401s today" is a legitimate, documented state here (see `UpdateProject`), not a bug to hide.
- [ ] A new write endpoint that needs an attributed actor (like `AddCaseTag`/`CreateCaseComment`'s
      `UMT_INTEGRATION_ACTOR_EMAIL` injection) never takes the actor from the caller's request body
      — it's injected server-side from this service's own configured identity and entity-service
      checks it against its own trusted-actor allowlist.
- [ ] A 401 from this service is never assumed to mean "the endpoint is broken" without first
      checking whether it's one of the endpoints with an M2M-credential fallback (incidents,
      IT-service search) — for those, a 401 can mean the *target environment's* ServiceNow M2M
      credential isn't configured, not that this service's code is wrong.

## Passthrough discipline

- [ ] A new endpoint stays a raw `[]byte` passthrough (`writeJSON`) — this service does not reshape
      upstream response bodies, with exactly one standing, documented exception (`upperEnumFields`
      in the catalog handlers). Don't add a second reshaping special-case without a strong reason
      and a doc-comment explaining it the way that one is explained.
- [ ] An empty request body is forwarded as `{}`, not as an empty byte string — entity-service
      rejects a body that isn't a JSON object.
- [ ] A response body that isn't a single JSON object (null, wrong type, trailing data) is handled
      explicitly (the existing enum-rewrite path 500s on this) rather than silently passed through
      malformed.

## Handler conventions

- [ ] Body size is capped with `http.MaxBytesReader` (1 MiB) before reading.
- [ ] A path param that should be a UUID is validated against the package-level `uuidRe` and
      rejected with 400 before the upstream call, not left for entity-service to reject.
- [ ] Every upstream error goes through `mapUpstreamError(w, err, "<fallback message>")` — no
      inline custom status mapping.
- [ ] `openapi.yaml` is updated in the same PR and declares `401`/`403` alongside the others —
      `mapUpstreamError` can forward either straight through, so both belong in the spec for every
      endpoint that can reach a ServiceNow-backed operation.

## Security

- [ ] No inbound JWT/Bearer validation added — this service deliberately has no `Auth` middleware
      (Choreo's gateway owns that trust boundary); don't "fix" a perceived auth gap without
      confirming that model has actually changed.
- [ ] `CorrelationID`'s own prefix convention (`cis-`) isn't broken by a change to that middleware.
