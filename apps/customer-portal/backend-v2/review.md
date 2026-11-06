# Pre-PR self-review checklist — customer-portal/backend-v2

Run this against the diff before opening or updating a PR touching this app. Check every item
below and call out anything that matches as a finding before proceeding to open the PR — don't
just silently fix and move on, say what was found and why.

Every item here comes from this codebase's own documented failure history (`CLAUDE.md` has dozens
of "a real, reported bug" / "found live" write-ups) — not generic code-review advice. If a new
failure class turns up during review that isn't on this list, add it here in the same style
(short, one line, grounded in what actually happened) so the next review catches it too.

## Cross-cutting

- [ ] No secret, token, webhook URL, or `.env`/`Config.toml` staged in the diff.
- [ ] No PII (customer/user email, name, phone) in a log line — IDs and sanitized summaries only.
- [ ] No raw upstream error body / driver error echoed to a caller — it can leak schema internals
      or an upstream's internal message.
- [ ] Every SQL value is parameterized — never string-interpolated, even for a value that "can't"
      contain user input.
- [ ] New/changed endpoint has an access-control check — route-level permission, or an in-handler
      check if the route is shared across audiences. Don't assume the route floor alone is enough.
- [ ] New behavior has a test exercising it, not just a happy-path smoke test.
- [ ] If this diff introduces a new convention/pattern/gotcha, `CLAUDE.md` is updated in the same
      PR — don't leave the next reader to rediscover it.
- [ ] `gosec -fmt=text ./...` and `govulncheck ./...` both clean before opening the PR (`GOTOOLCHAIN=auto`
      for gosec; bump `go.mod`'s `go` directive to the latest `1.26.x` patch if the only finding is
      a stdlib CVE tied to the toolchain version).
- [ ] Commits/PR carry this session's attribution lines, not an older convention from memory.
- [ ] `git fetch upstream main` + `git merge-tree --write-tree HEAD upstream/main` checked clean
      before pushing — `upstream/main` moves fast; don't trust a merge that was clean an hour ago
      (see the repo-root `CLAUDE.md`).
- [ ] No real name/email/URL/org-specific identifier copied from a screenshot, pasted log, or API
      response into code, tests, docs, or commit/PR text — use a generic placeholder.
- [ ] No ticket/issue number or repo name cited in a code comment — explain the why directly.

## Response shaping (the DTO wrapper pattern)

- [ ] A new endpoint's response goes through a `dto.Map*` function, never an entity-service struct
      returned directly — this backend's core convention, unlike csm-portal/backend's raw
      passthrough.
- [ ] Every field on the entity-service struct was read and a deliberate decision made to
      include/exclude it — don't assume a field "looks internal" or "looks customer-safe" from its
      name or entity-service's own doc comment; check what the live frontend TypeScript actually
      reads before excluding something (two fields were wrongly assumed internal-only this way).
- [ ] A write endpoint builds a restricted portal request DTO when the entity-service request mixes
      customer-safe fields with internal-only ones (assignee, internal relinking, `workState`,
      fix-ETA fields) — don't forward the full entity-service struct just because it's convenient.
- [ ] A new paginated response uses `totalRecords`, not `total` — the frontend's shared pagination
      type expects `totalRecords`; this has silently broken a UI tab before (no array-length
      fallback masked it).

## IDs and scoping

- [ ] A `{id}` path segment is the source of truth for project/deployment scope — never trust a
      body field for the same thing even when entity-service's own struct has one; overwrite it
      from the path after decoding.
- [ ] A new request field that carries an entity-validated UUID is added to `sysidBodyKeys` so
      `NormalizeSysIDs` rewrites a bare-hex sysid to a dashed UUID before entity-service sees it.
- [ ] A new route pattern is checked for `ServeMux` registration-ambiguity against every existing
      route with the same-or-fewer path segments — this has taken the server down in production
      already; merge ambiguous patterns under one wildcard and dispatch manually rather than
      renaming an external-contract URL.

## Validation strictness

- [ ] Checked entity-service's own doc comment for "exactly one of" vs "at least one of" on a
      multi-field request before porting the same validation here — copying the wrong one makes
      this backend stricter or looser than the upstream contract (CodeRabbit caught this once).
- [ ] A customer-facing write never lets a customer set a field that records WSO2's own internal
      decision/answer on their behalf (approval flags, assignment, workflow state) — route those
      through the dedicated customer-action field instead of a generic state setter.

## Internal-write-group gate

- [ ] A new write route is registered with `RequirePermission`/`RequirePermissionOneOf`/
      `RequireRoles` (not a bare `mux.HandleFunc`) — a route registered directly on the mux skips
      the internal-write-group gate entirely, and several routes already do this unintentionally
      (see the "Known gap" list in this app's `CLAUDE.md`); don't add a new one to that list.

## Logging

- [ ] A new log call uses `summarizeErr(err)`, not a raw error interpolation — an unrecognized
      error's string form can include the full request URL with query params.
