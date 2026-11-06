# Pre-PR self-review checklist — csm-notification-service

Run this against the diff before opening or updating a PR touching this app. Check every item
below and call out anything that matches as a finding before proceeding to open the PR — don't
just silently fix and move on, say what was found and why.

Every item here comes from this codebase's own documented failure history (`CLAUDE.md` has dozens
of "a real, reported bug" / "found live" write-ups — several with named regression tests) — not
generic code-review advice. If a new failure class turns up during review that isn't on this list,
add it here in the same style (short, one line, grounded in what actually happened) so the next
review catches it too.

## Cross-cutting

- [ ] No secret, token, webhook URL, or `.env`/`Config.toml` staged in the diff.
- [ ] No PII (customer/user email, name, phone, address) in a log line — IDs and sanitized
      summaries only.
- [ ] No raw upstream error body echoed to a log — a Google Chat error response can echo back part
      of the submitted card (title/text) verbatim; log ids and status codes, never the error body.
- [ ] New behavior has a test exercising it, not just a happy-path smoke test — this service has
      an unusually high density of named regression tests for exactly this reason (real,
      live-observed duplicate-send/race bugs each got one).
- [ ] If this diff introduces a new convention/pattern/gotcha, `CLAUDE.md` is updated in the same
      PR — don't leave the next reader to rediscover it.
- [ ] `gosec -fmt=text ./...` and `govulncheck ./...` both clean before opening the PR.
- [ ] Commits/PR carry this session's attribution lines, not an older convention from memory.
- [ ] `git fetch upstream main` + `git merge-tree --write-tree HEAD upstream/main` checked clean
      before pushing — don't trust a merge that was clean an hour ago.
- [ ] No real name/email/URL/org-specific identifier copied from a screenshot, pasted log, or API
      response into code, tests, docs, or commit/PR text — use a generic placeholder.
- [ ] No ticket/issue number or repo name cited in a code comment — explain the why directly.

## Idempotency / claim-release on Kafka retries

- [ ] A new side effect that can be retried independently of the record's own primary success path
      (a second Chat post, call, or email) has its own claim/release key — this service's whole
      `claim`/`forget`/`beginRecord`/`endRecord` apparatus exists because an untracked side effect
      gets repeated when an unrelated part of the same handler keeps failing and retrying.
- [ ] A claim is released on `record.NoMoreRetries` (no further attempt coming, ever, on any
      topic), never on `record.IsFinalAttempt` alone and never on a single concurrent call's own
      success — two in-flight `Handle` calls racing the same record is a real, recurring scenario
      here (a Kafka rebalance isn't exclusive down to the microsecond), and releasing on the wrong
      signal has caused at least three separate, since-fixed duplicate-send bugs (see
      `TestDispatcher_Handle_LosingConcurrentCallDoesNotReleaseWinnersClaim`,
      `TestDispatcher_Handle_CaseCreated_ConcurrentBlockedEmailDoesNotDuplicateChat`,
      `TestDispatcher_Handle_CaseCreated_RetryDoesNotResendSucceededChatAlert`).
- [ ] A handler with more than one channel (email + Chat, call + Chat) uses the `beginRecord`/
      `endRecord` refcounting pattern, not a single channel's own `owned && err == nil` check, to
      decide when it's safe to release every channel's claim — a losing call for one channel must
      never release a different, still-in-flight call's claim on another.
- [ ] A new idempotency key is derived from `recordBaseKey(record)` (a hash of the record's
      content), never from `topic/partition/offset` — a dead-lettered record is republished with
      the same `Value` but new Kafka coordinates, so a coordinate-based key would look "unclaimed"
      on its DLQ attempt and resend an already-succeeded channel.
- [ ] A `defer` that releases a claim on `NoMoreRetries` is registered **before** the `claim()`
      call itself, not after — on the genuinely final retry `claim()` can return `false` and return
      early, and a `defer` registered after that point never runs, leaking the key forever (a real,
      since-fixed bug — see `TestDispatcher_Handle_CommentAdded_FrustrationDetection_NotRepeatedWhenEmailRetries`).

## Chat cards

- [ ] A dynamic value formatted into a Chat card's text goes through `caseAlertLine` (or an
      equivalent escaping helper), never raw string concatenation — Chat's card text interprets a
      limited HTML subset, so an unescaped `<`/`&` in a dynamic value can break out of its tag.
- [ ] A numeric value handed to `caseAlertLine` is formatted with `fmt.Sprintf` first, then passed
      through `%s` — `caseAlertLine` stringifies every arg via `fmt.Sprint` before escaping, so a
      direct `%.2f` against an already-stringified arg produces a literal `%!f(string=...)` in the
      real card (a real, reported bug).
- [ ] A new card doesn't add a leading icon/emoji to a body line — tried and deliberately dropped
      more than once: Chat's native `decoratedText` icon has a fixed, non-configurable gap, and
      both cost width a narrow (mobile) screen needs for the text itself.
- [ ] A card meant to stand out as its own message (not grouped with a related one) does not set
      `Thread.ThreadKey` — threading buries it as a reply under a possibly old/scrolled-past
      message, the opposite of what an alert needing attention is for (a real, reported bug fixed
      for the frustration-detection card specifically).
- [ ] A new team-routable audience resolves its webhook through the same `webhookURLFor` precedence
      (routed-incident spaces, then static `GOOGLE_CHAT_SPACES`/`INCIDENT_CHAT_SPACES`, then the
      entity-service `team` table) — don't invent a second, parallel routing table for a new
      feature.

## Config and killswitches

- [ ] A new disable-entirely killswitch (like `EMAIL_SENDING_ENABLED`/`CALL_SENDING_ENABLED`) uses
      the `!= "false"` convention (default **on**), while a new `CSM_MIGRATION_*` cutover flag uses
      `envBool(name, false)` (default **off**) — these two families have deliberately opposite
      default-safety directions; don't mix them up.
- [ ] A payload field added to an event this service strictly decodes (`DisallowUnknownFields`) is
      called out with a deployment-order note (deploy this service before the entity-service build
      that starts sending it) — an unrecognized field fails validation, retries, dead-letters, and
      eventually drops every event of that type.
- [ ] A new YAML config file (in the shape of `INCIDENT_ESCALATION_CONFIG`) rejects an unknown key
      outright (`KnownFields(true)`) rather than silently ignoring it — a silently-dropped
      misspelled key (e.g. `maxCallsPerLadder`) leaves an operator certain a safety cap is applied
      when it isn't.
- [ ] No new inbound HTTP ingest endpoint is added without confirming the "this service is a pure
      Kafka consumer, no inbound API beyond health" decision has actually changed — it was
      deliberately removed once already.

## Logging / PII (paging-specific)

- [ ] A new paging log line names a person by `Recipient.LogID()` (a CSM user id, or a hashed
      `e:`-prefixed email) — never a raw email, name, or phone number, even though Chat cards and
      the execution summary are allowed to show names (those are for people to read, not logs).
- [ ] A phone number resolved for a call is never logged or persisted outside the plan.

## Twilio / TwiML

- [ ] A change to how a spoken call message is built keeps using `encoding/xml` struct marshaling
      (`twimlResponse`/`twimlSay`), never raw string concatenation — a caller-supplied message
      shaped like TwiML (`</Say><Redirect>...`) must render as literal spoken text, not inject a
      different verb. This applies to any new attribute added to the `<Say>` struct too.

## SLA engine / incident ladders (if touched)

- [ ] A change to `slaengine`'s Redis-stored clock state accounts for "incarnation" — a severity
      revision cancels a clock and registers a fresh one with a new `startedAt`; any read/write of
      `alertedTier`/wake-index state checks the stored `startedAt` against the one in hand first.
      Three separate incarnation bugs were caught here by review before landing (stale
      `alertedTier` surviving a revision, a stale wake-index removal clobbering a new clock, and
      two incarnations colliding within the same wall-clock second) — don't reintroduce any of the
      three shapes.
- [ ] A new SLA/paging Redis write that could run concurrently with a tick or another handler call
      is content with being overwritten/raced only when that's actually safe — check whether the
      existing Lua-script compare-and-set pattern (`AdvanceAlertedTier`, `ClaimTier`) is the right
      model before adding a plain `SET`.
