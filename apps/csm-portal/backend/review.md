# Pre-PR self-review checklist — csm-portal/backend

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
- [ ] `git fetch upstream dev-app-csm-portal` + `git merge-tree --write-tree HEAD upstream/dev-app-csm-portal`
      checked clean before pushing — the target branch moves fast; don't trust a merge that was
      clean an hour ago (see the repo-root `CLAUDE.md`).
- [ ] No real name/email/URL/org-specific identifier copied from a screenshot, pasted log, or API
      response into code, tests, docs, or commit/PR text — use a generic placeholder.
- [ ] No ticket/issue number or repo name cited in a code comment — explain the why directly.

## Access control

- [ ] A new route picks the narrowest correct `Perm*` for `route(...)` — don't default to
      `PermWrite` for something that should be `PermAdmin`-only or gated by a dedicated role.
- [ ] A permission that only applies to part of a shared endpoint's body (not the whole route) is
      checked in-handler by inspecting the decoded/raw body keys case-insensitively — a check that
      only looks for one exact key spelling can be walked around, since the upstream JSON decoder
      matches case-insensitively.
- [ ] A new `AUTH_<ROLE>_ROLES` env var has no default — an unset/empty value must mean "nobody
      holds this role," never "everyone does," and startup should warn about it.
- [ ] A check deciding "may this caller act on a resource they didn't create" is a real author/role
      comparison in this backend, not assumed to already be enforced by a downstream service that
      in fact performs no such check of its own.

## Upstream passthrough

- [ ] A raw `[]byte` passthrough response doesn't leak an internal field a customer-portal-facing
      API already redacts — recheck if the same data shape is reused anywhere outside internal
      staff view.
- [ ] A new upstream error path uses `mapUpstreamErrorGeneric` by default; only use `mapUpstreamError`
      (which echoes the upstream message) for the handful of PATCH/lifecycle endpoints where the
      upstream 4xx reason is genuinely caller-actionable.
- [ ] A filter/search param added to a handler is verified against a real, locally-run upstream
      service — not just a hand-written fake — before trusting that it actually narrows results.
      A fake proves the Go code calls the client correctly; it never proves the filter works.

## Config / vocabulary

- [ ] `CSM_TEAM_REGISTRY` / `CSM_USER_ROLES` stay flat single-line strings — no nested-collection
      config; the deployment platform can't deploy one.
- [ ] A new `CSM_MIGRATION_*` cutover flag is off unless its value is exactly `"true"`, and "off"
      means the route/handler isn't constructed at all, not merely a no-op at call time.

## Routing

- [ ] A new route pattern is checked for `net/http.ServeMux` ambiguity against existing patterns
      with a similar wildcard shape — an ambiguous pair panics the whole server at startup, and has
      actually done so in production in this family of backends (see the equivalent entry in
      customer-portal/backend-v2's own `review.md`).
