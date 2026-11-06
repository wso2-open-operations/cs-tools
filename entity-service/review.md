# Pre-PR self-review checklist — entity-service

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
- [ ] No raw upstream error body / driver error (`pgErr.Detail`, a raw `err.Error()`) echoed to a
      caller — it can leak schema internals or an upstream's internal message.
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

## Nullability / scanning

- [ ] A newly-read column that is nullable in the schema is scanned into a pointer local, never
      directly into a non-pointer struct field — `NULL` into `*string`/`*time.Time` etc. is fine,
      into a plain field panics (`cannot scan NULL into *string`). This has crashed multiple search
      endpoints in production.
- [ ] A field read from an optional column keeps a pointer on the wire (or `stringOrEmpty`/
      equivalent default) rather than silently becoming `""` for "unset" and a real empty string
      both — don't collapse "absent" and "empty" unless the field already doesn't need to tell
      them apart.

## Joins

- [ ] A join onto an optional relationship (deployment, project, account, a second extension table)
      is `LEFT JOIN`, not `INNER JOIN` — an `INNER JOIN` on an optional FK has caused false 404s
      and silently dropped valid rows from search results more than once here.
- [ ] A query added to one of a pair of structurally-identical statements (e.g. a COUNT and its
      matching page query, or `GetByID` vs `Search`) selects the same columns/joins as its sibling
      — drift between the two has produced a `total` that disagrees with the rendered list.

## Enums

- [ ] A new Postgres enum value/column written or filtered from Go confirms the Go string's case
      matches the Postgres label exactly (`UPPER_SNAKE` vs lowercase is the usual mismatch), or
      goes through an explicit `xToEnum`/`xFromEnum` mapping function when the two vocabularies
      don't line up 1:1 (severity S0-S4 vs catastrophic/critical/etc. is the canonical example).
- [ ] A value bound into a `WHERE` clause against an enum column is compared as the column's own
      type, not cast to `::text` first — a `::text` cast defeats that column's index.

## Migrations

- [ ] New migration is `NNNN_<description>.sql`, forward-only, one file, in `migrations/` — never
      the old `000NNN_x.up.sql`/`.down.sql` shape.
- [ ] Picked the next free number by checking `operations/csm-sync-service/migrations/` too when
      the migration touches a shared table — not just this repo's own highest number. A second
      PR adding a migration at the same moment is also normal; don't assume a number collision in
      review means something is wrong (filename is the tracking key, not the number).
- [ ] An `ALTER TYPE ... ADD VALUE` is the only statement in its file (can't share a transaction
      with a later statement using the new value).
- [ ] A migration that defines or touches an RLS policy goes in the `1NNNNN_*.sql` track, not the
      4-digit sequence.
- [ ] Renaming an already-applied migration file is never done — the filename is the identity; a
      rename makes it look new and re-run from scratch.

## Access / security

- [ ] A new endpoint that touches case/project/account data calls `AccessService.ResolveScope`
      (or inherits it) rather than assuming every caller may see everything — most endpoints in
      this codebase still don't do this; don't add a new one to that pile without checking.
- [ ] A new table protected by row-level security is written to under the caller's own identity
      context, not as a superuser/bypass path, unless there's a documented, deliberate reason
      (internal ingest, migration backfill).
- [ ] A customer-visible response doesn't leak an internal-only field (WSO2 team routing, Salesforce
      ids, another customer's data) — check the field against what the BFF layer is expected to
      trim, don't assume entity-service's own shape is already customer-safe.

## PATCH / request-shape correctness

- [ ] A PATCH with mutually-exclusive field groups validates the combination before writing
      anything — a new field slotted into an existing "exclusive" or "combinable" bundle without
      updating the count/check has let an invalid combination silently go through before.
- [ ] A dynamic value formatted into a Chat-card/HTML-ish string goes through the existing escaping
      helper (`caseAlertLine` or equivalent) — a raw `%.2f` against an already-`fmt.Sprint`-ed value
      produces a literal `%!f(string=...)` in production output; format numbers before handing them
      to the helper.

## Event-driven / idempotency

- [ ] A new Kafka/event-driven side effect has its own claim/release key if it can be retried
      independently of the record's primary success path — a side effect with no tracking of its
      own gets repeated (a second Chat post, a second email) when an unrelated part of the same
      handler keeps failing and retrying.
- [ ] A claim is released on `NoMoreRetries`/final-attempt semantics, never on a single concurrent
      call's own success alone — two in-flight calls for the same record is a real, recurring race
      in this codebase, and releasing too early lets a losing call's retry resend something a
      still-in-flight winner already sent.
