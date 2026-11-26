# CSM AI Integration Service

Go HTTP server (`net/http`, Go 1.26+) that holds **every call the CSM platform makes to Claude**. Today
that is dashboard generation for the CSM portal; other AI features join under `/ai/`. It is the only
component with a Claude client, the Anthropic SDK and the model credentials: the CSM portal backend
(`apps/csm-portal/backend`) must hold none of them.

## What lives here and what does not

| Here | In the CSM portal backend |
|---|---|
| The Claude client (gateway base URL, API key, model, timeout, retries) | Authentication and permissions of the end user (`PermWrite` for generation) |
| The catalogues the model reads (built into the binary) | The routes the webapp calls (`/dashboards/generate`, `/dashboards/generate-code`) |
| Building the prompt (catalogue + the team list the backend sends + prompt + current draft) | Validating what the model wrote (`dashboard.ParseGenerated`, the per-type rules) |
| The call, its timeout, and reading the answer (the JSON objects, a decline, a refusal, a truncation) | The team registry and the team key -> id / name swap; saving and sharing |
| Mapping those outcomes to a stable error vocabulary | The sample generators used when this service is not configured |

The backend sends each request's **team list** (from its `CSM_TEAM_REGISTRY`); this service never reads
the registry itself, so it is configured in one place.

## Routes

| Route | Status |
|---|---|
| `GET /health` | liveness, always 200 |
| `POST /ai/dashboards/generate` | the JSON dashboard generator (`internal/dashboards`, `internal/handler`) |
| `POST /ai/dashboards/generate-code` | the code approach, phase 2 (`internal/dashboards/code.go`); **registered only when `DASHBOARD_GENERATE_CODE_ENABLED` is exactly `true`** |

### `POST /ai/dashboards/generate`

Body `{prompt, draft?, teams, playbookAnswers?}`: the prompt (trimmed, 1-2000 characters), the dashboard being edited in the
definition file format (the backend's `draftToDefinition`; absent or null on a first message; at most 64 KiB,
the size entity-service allows a saved definition), the teams a dashboard can filter by
(`{key, name, family, field}`, only teams with a group) and the PLG playbook tasks a widget can filter or
group by answer (`{code, name, valueType, options: [{code, label}]}`, `valueType` `SINGLE_SELECT` or
`CHECKLIST`; at most 200, each with 1 to 100 options; absent when there are none). Unknown fields are a 400,
so this service is deployed before a backend that sends a new one. The draft cap bounds what
one call can send the model; the per-user rate limit is the CSM portal backend's.

The service makes one call and returns **every JSON object in the reply, in order** (`candidates`), or the
model's sentence when its last object is the catalogue's refusal shape `{"error": "..."}` (`declined`), plus
`usage`. It does **not** judge the candidates: the backend runs `dashboard.ParseGenerated` from the last one
back and keeps the first that passes, so the per-type rules live in one place. A candidate may carry a
top-level `note` (catalogue section 1: a request that cannot be built exactly gets the closest dashboard, and
the note says what differs); it is passed on like the rest of the object, and the backend takes it off and
starts the chat reply with it. `declined` is only for a request nothing close answers. Errors are `{code, message}`;
the backend maps the code to its own status and user-facing text:

| Status | `code` | When |
|---|---|---|
| 400 | `invalid_request` | the body, prompt, draft, teams or playbook answers |
| 503 | `not_configured` | no `ANTHROPIC_API_KEY` |
| 422 | `refused` | the model refused (stop reason `refusal`) |
| 502 | `truncated` / `unusable` | the reply was cut off / holds nothing usable (no JSON object; for code, no usable code block) |
| 504 | `timeout` | the model call ran out of time (`CLAUDE_TIMEOUT`) |
| 429 | `usage_limit` | the gateway answered 429 (a request, token or spend limit on the AI side), also after the SDK's one retry; the user is told the AI's usage limit was reached, not that it could not be reached |
| 502 | `model_call_failed` | any other call failure, an overloaded gateway (529 / 503) included; a gateway 401/403 is this service's key problem, never passed on |

### `POST /ai/dashboards/generate-code`

**Off by default.** The model writes the dashboard as JavaScript (`async function render(portal, root)`)
that the webapp runs in a Web Worker inside a sandboxed frame, with no network and no page to navigate
(see the webapp's `CLAUDE.md`); it is not released yet. The code catalogue's section 2 says what that
worker offers and which names the webapp refuses; keep it in step with the webapp's `code-dashboard-sandbox/`. `newMux` (`cmd/server/routes.go`)
registers this route only when `DASHBOARD_GENERATE_CODE_ENABLED` is exactly `true` (`codeGenerationEnabled`:
`1`, `TRUE`, `yes` are off); off, the path answers 404 and `main.go` builds no code generator. The start log
has `codeGeneration`. The CSM portal backend (`POST /dashboards/generate-code`) and webapp (the
`/dashboard/my/new-code` page and its button) read the same name, so the three are turned on together; with
only some on, the feature fails (the backend's call here gets a 404).

Body `{prompt, current?, teams?, playbookAnswers?}`: the prompt (as above), the source being edited (at most 60,000 characters,
`CodeMaxLength`; absent or "" on a first message) and the team list, checked as on `/generate` (`checkTeams`)
and appended to the code catalogue as the same Teams section (`teams.WithTeams`), then the PLG playbook answer fields
(`playbookAnswers`, `checkPlaybookAnswers`, `playbooks.WithPlaybookAnswers`), as on `/generate`. The reply checks are all here, because they only read the code catalogue's output contract (one
fenced block, the last one wins, it must define `function render(`, at most 60,000 characters; a reply with
no block that starts with `CANNOT:` is the model declining). The prose before the block starts with a
`Name: …` line, the name the webapp's Save dialog suggests (`codeName`: the first such line, quotes and bold
dropped, at most 255 characters, the saved-name limit); it is left out of the chat sentence, and a reply
without one is still usable, with no `name`; on a decline a Name line is dropped. A reply is capped at 10,000 output tokens
(`codeMaxTokens`). 200 is `{source, reply, name?, usage}` or `{declined, usage}`; errors and codes are the table above.

## Model settings

`ANTHROPIC_API_KEY` (a secret) and `CLAUDE_BASE_URL` go together: the key is the CSM AI gateway's, so
**`CLAUDE_BASE_URL` is required whenever the key is set**, and a key without it fails the start
(`loadConfig`). There is deliberately no fallback to the Anthropic API's own address: with the gateway's
key it would start looking healthy and fail every call. Without a key the service still starts with model
calls off (the AI routes answer 503 `not_configured`), so a local stack needs no secrets. In Choreo both
are set per environment in Configs & Secrets (the key as a secret).

## Auth model

- **Inbound: no authentication in the application.** The Choreo endpoint is **Organization**-visible
  (`.choreo/component.yaml`), and the only caller is the CSM portal backend, which uses the platform's
  shared OAuth2 client-credentials app (the same pattern as `csm-notification-service`). The backend has
  already authenticated and authorised the end user. Do not make the endpoint Public, and do not add
  end-user token validation here without confirming that assumption has changed.
- **The end user's id is forwarded, for the logs only.** The backend sends the signed-in user's id
  (never their email) as `X-CSM-User-ID`; `middleware.UserID` puts it in the context and the log handler
  adds it to every line as `userID`. Nothing is authorised by it, and the model call is the same for
  every user. A header over 128 characters is ignored.

## Token usage log

Every model call writes one line, `dashboard generate: token usage` (`logUsage` in
`internal/dashboards/generate.go`), whatever its outcome, since every reply spends tokens:

| Field | |
|---|---|
| `userID`, `correlationID` | added by the log handler from the request (the same keys as the CSM portal backend's lines) |
| `model` | `CLAUDE_MODEL` |
| `outcome` | `ok`, `declined`, `refused`, `truncated`, `unusable`, `usage_limit` (no reply: the gateway's 429, with 0 tokens), or `failed` (no reply: any other gateway error, a connection failure or a timeout, with 0 tokens) |
| `stopReason` | the model's, empty when `failed` |
| `inputTokens` | input neither read from nor written to the prompt cache: mostly the user message once the system prompt is cached (see "Prompt caching") |
| `cacheCreationInputTokens`, `cacheReadInputTokens` | input written to / read from the prompt cache: the system prompt, on a call that found no cache / within its lifetime |
| `outputTokens` | the reply's |
| `durationMs` | the model call alone, the SDK's retry included |
| `approach` | `code` on a `generate-code` call's line (`logUsage`'s extra fields); absent on the JSON route's |

The line goes to the service's log (stderr), like every other line, so in Choreo it is under the
component's Observability -> Logs; search for `token usage`, or a `userID`. No file is written: a
Choreo container's file system is read-only apart from `/tmp`, which a restart wipes. A request that
fails before the call (an invalid body, `not_configured`) writes no usage line.

## Prompt caching

The system prompt (the catalogue, the Teams section and the PLG playbook answers section) is the same for every user and call of one
deployment (the playbook answers change only when a PLG admin edits a playbook, and the backend sorts them), so `Generate` marks it with `cache_control` (`ephemeral`, the default 5-minute lifetime, renewed
by each hit). A call within that time reads it at the model's cache-hit price (0.05 times the input
price on Sonnet 5.5, the default, and Opus 5.5; 0.1 times on most other models); the first call after it
expires writes it at 1.25 times. Generation happens in short sessions (a prompt, then follow-ups), so the
5-minute lifetime fits: a 1-hour cache would cost 2 times at the start of nearly every session. Anything
that changes per request must stay after the system prompt (the prompt and the draft are in the user
message), or no call would find the cache. A deployment's input is `inputTokens` plus the two cache counts
of the token usage line; a warm call shows most of it in `cacheReadInputTokens`. The code generator marks
its system prompt (the code catalogue, the Teams and PLG playbook answers sections) the same way.

## Catalogue

The model is given a catalogue: what a dashboard definition may contain, written for the model
(`internal/catalogue/catalogue.md`). It is **built into the binary** (embedded at build time) and nothing
at run time replaces it: a catalogue change is a code change, reviewed like one, and reaches a deployment
with the next build. `CATALOGUE_DIR` and `CATALOGUE_FILE`, which once named a mounted copy, are no longer
read; a deployment still setting either starts normally and logs a warning naming them, so they can be
removed.

**Editing `catalogue.md` can break a backend test, by design.** The backend's
`TestParseGenerated_TheCatalogueExampleIsValid` (`apps/csm-portal/backend/internal/dashboard`) reads this
file and runs its section 10 example through the backend's own check, and
`TestParseGenerated_TheCatalogueTimeCardTotalsExamplesAreValid` does the same for every grouped time card
example in section 5a (at least three), and `TestParseGenerated_TheCataloguePlgExamplesAreValid` for every
PLG example in section 5e (filling in `id`, `displayName` and `gridWidth` when the example leaves them
out), so the catalogue and the rules that judge the model's output cannot drift apart. After editing it, run the backend's
`go test ./internal/dashboard`, then the three steps above. In the local stack, an edit to the built-in catalogue needs this service's
image rebuilt (`docker compose up -d --build csm-ai-integration-service`), not the backend's.

**The chat tells users what they can ask about.** The webapp's `GenerateChatPanel` (`ASK_ABOUT`) lists, in
plain words, the data types and filters this catalogue supports. It is not generated from this file: a data
type or filter added here (or taken out) is changed in that list in the same change.

**The model writes names as the user wrote them.** Products, versions, projects (key or name),
customers (name or account number) and regions (the customer's sales region) are written as the request gives them, never ids; the backend
resolves them after the reply and tells the user when one names nothing or several, so the catalogue
never asks the model to refuse a name as not exact (the backend's
`TestParseGenerated_TheCatalogue{Product,ProductVersion,Project,Region}ExamplesAreValid` run section 7's
examples through its check, as the tests above do for theirs). An edit's draft holds the resolved `projectId` /
`accountId` filters, which the model keeps as they are. The `{"error": ...}` text is shown to the user
(section 1: at most two plain sentences, no field names or catalogue words), and a "not", "except" or
"without" the catalogue has no filter for is declined rather than dropped. Never put a real customer's
name, project key or account number in the catalogue: the repository is public; the examples use
made-up or seed names.

The Teams section is not in the file: the backend sends the team list with each request and
`internal/teams` appends it to the catalogue (the same text the backend wrote before the move). Nor is the
"PLG playbook answers" section: the backend reads the playbooks with each request and `internal/playbooks`
appends their tasks and answer codes after the Teams section, so the catalogue's section 5e names no code
of its own beyond an example; the backend checks every code the model writes against the same list.

**The code catalogue** (`internal/catalogue/code_catalogue.md`, `catalogue.NewCode()`) is the code
approach's, embedded the same way and read only when the code generator is built. Its section 4 lists the
only `portal.*` functions the code may call, and its section 5 the filters, groupings and sort fields; they
must match what the webapp's bridge answers (`useCodeDashboardPortal.ts`) and what the backend's
`/cases/aggregate` and `/cases/search` accept, or the model is taught calls that fail in the frame.

## Conventions

- Middleware chain: `SecurityHeaders -> CorrelationID -> UserID -> Logger -> Mux`, as in
  `csm-integration-service` plus `UserID`. Correlation IDs carry the `cai-` prefix.
- **No prompts, drafts or model answers in logs.** They are users' text; log lengths, token counts,
  outcome and duration only.
- The server's `WriteTimeout` follows the model's: `CLAUDE_TIMEOUT` plus a minute, never under two minutes
  (`writeTimeout` in `cmd/server/main.go`), because a generate call waits for the model. The backend's own
  client timeout (`CSM_AI_INTEGRATION_SERVICE_TIMEOUT`, 55 s) must stay above `CLAUDE_TIMEOUT` (45 s).
- Update `openapi.yaml` with every route change.

## Commands

```bash
make test    # vet + race-detector tests
make build   # tests, then compiles ./cmd/server
```

Run gosec and govulncheck before a PR, as for the other Go services.
