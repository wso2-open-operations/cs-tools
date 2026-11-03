# CSM AI Integration Service

Go HTTP server (`net/http`, Go 1.26+) that holds **every call the CSM platform makes to Claude**. Today
that is dashboard generation for the CSM portal; other AI features join under `/ai/`. It is the only
component with a Claude client, the Anthropic SDK and the model credentials: the CSM portal backend
(`apps/csm-portal/backend`) must hold none of them.

## What lives here and what does not

| Here | In the CSM portal backend |
|---|---|
| The Claude client (gateway base URL, API key, model, timeout, retries) | Authentication and permissions of the end user (`PermWrite` for generation) |
| The catalogue the model reads (built-in copy; a directory of Choreo file mounts overrides it) | The route the webapp calls (`/dashboards/generate`) |
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

### `POST /ai/dashboards/generate`

Body `{prompt, draft?, teams}`: the prompt (trimmed, 1-2000 characters), the dashboard being edited in the
definition file format (the backend's `draftToDefinition`; absent or null on a first message; at most 64 KiB,
the size entity-service allows a saved definition) and the teams a dashboard can filter by
(`{key, name, family, field}`, only teams with a group). Unknown fields are a 400. The draft cap bounds what
one call can send the model; the per-user rate limit is the CSM portal backend's.

The service makes one call and returns **every JSON object in the reply, in order** (`candidates`), or the
model's sentence when its last object is the catalogue's refusal shape `{"error": "..."}` (`declined`), plus
`usage`. It does **not** judge the candidates: the backend runs `dashboard.ParseGenerated` from the last one
back and keeps the first that passes, so the per-type rules live in one place. Errors are `{code, message}`;
the backend maps the code to its own status and user-facing text:

| Status | `code` | When |
|---|---|---|
| 400 | `invalid_request` | the body, prompt, draft or teams |
| 503 | `not_configured` | no `ANTHROPIC_API_KEY` |
| 422 | `refused` | the model refused (stop reason `refusal`) |
| 502 | `truncated` / `unusable` | the reply was cut off / holds nothing usable (no JSON object; for code, no usable code block) |
| 504 | `timeout` | the model call ran out of time (`CLAUDE_TIMEOUT`) |
| 502 | `model_call_failed` | any other call failure; a gateway 401/403 is this service's key problem, never passed on |

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
| `outcome` | `ok`, `declined`, `refused`, `truncated`, `unusable`, or `failed` (no reply: a gateway error, a connection failure or a timeout, with 0 tokens) |
| `stopReason` | the model's, empty when `failed` |
| `inputTokens`, `outputTokens` | the reply's `usage` |
| `durationMs` | the model call alone, the SDK's retry included |

The line goes to the service's log (stderr), like every other line, so in Choreo it is under the
component's Observability -> Logs; search for `token usage`, or a `userID`. No file is written: a
Choreo container's file system is read-only apart from `/tmp`, which a restart wipes. A request that
fails before the call (an invalid body, `not_configured`) writes no usage line.

## Catalogue

The model is given a catalogue: what a dashboard definition may contain, written for the model
(`internal/catalogue/catalogue.md`). It is **built into the binary** as the default and can be
**overridden by a directory of parts**, named by `CATALOGUE_DIR`: Choreo limits the size of a mounted file
(about 20 KB, the limit the backend's `DASHBOARDS_DIR` part files are cut to, per its `loadDir` comment) and
the catalogue is larger, so it is mounted as one file mount per part (Configs & Secrets -> file mount), all
under one directory. The parts are generated and committed in **`catalogue-parts/`** (see below). The parts are that directory's `*.md` files, **joined in name order** with one blank line between
them (`catalogue-01.md`, `catalogue-02.md`, ...; two digits, since names sort as text). Directories, other
files, and names starting with `.` (Kubernetes' `..data` entries beside mounted files) or `_` are skipped.
Each part's trailing whitespace is trimmed, so the catalogue split at a blank line before a `## ` heading
joins back into the same text (`TestTheBuiltInCatalogueSplitAtHeadingsJoinsBackUnchanged`).

**`catalogue-parts/` is generated: never edit it by hand.** `make catalogue-parts` (`cmd/splitcatalogue`)
cuts the built-in `catalogue.md` with `catalogue.Split` into parts of at most `catalogue.MaxPartBytes`
(20,000 bytes, under 20 KB however it is counted). It cuts before `## ` sections and packs whole sections
into as few parts as fit; a section too large for one part is cut before its `### ` subsections, then at
blank lines (a single paragraph over the limit is an error). It writes `catalogue-01.md`, `catalogue-02.md`,
... and removes old `catalogue-NN.md` files, so a shorter catalogue leaves no stale part. `Split` refuses a
text its parts would not join back into (extra blank lines at a cut, no final newline).

**Changing `catalogue.md` is three steps, and the third is not automatic:**

1. Run `make catalogue-parts` in this directory.
2. Commit `catalogue-parts/` with the `catalogue.md` change. `TestTheCommittedPartsAreUpToDate` fails while
   the committed parts are not exactly what `Split` makes of `catalogue.md` (or a part is over the limit),
   so `go test ./...` and CI catch a forgotten step 1.
3. **Update the Choreo file mounts** in every environment whose component sets `CATALOGUE_DIR`: replace
   each mounted part's content with the new `catalogue-parts/catalogue-NN.md`, add a mount for a new part,
   and remove the mount of a part that is gone (the `README.md`'s "Mounting the catalogue in Choreo").
   Nothing checks this step: a deployment keeps serving the old catalogue from its mounts, and starts
   normally, until they are updated. Check the startup log's `catalogueParts` after the restart.

The directory is **read once, when the service starts**: a missing directory, one with no `*.md` file, and
an empty or unreadable part each fail the start, so a mistyped path shows there. Requests use that text
with no file access, so an edited mount applies on the next start (in Choreo, changing a mount restarts the
pod anyway). The startup log line `model calls` names the origin (`catalogue`: `built-in` or the directory)
and the parts in the order they were joined (`catalogueParts`). `CATALOGUE_FILE`, the single file this
replaced, is a start error when `CATALOGUE_DIR` is unset, so a deployment still setting only it does not
quietly fall back to the built-in copy; next to `CATALOGUE_DIR` it is ignored, with a warning at start. Keep the built-in copy current: it is what local runs and tests use, and what a
deployment without the mount gets.

**Editing `catalogue.md` can break a backend test, by design.** The backend's
`TestParseGenerated_TheCatalogueExampleIsValid` (`apps/csm-portal/backend/internal/dashboard`) reads this
file and runs its section 10 example through the backend's own check, so the catalogue and the rules
that judge the model's output cannot drift apart. After editing it, run the backend's
`go test ./internal/dashboard`, then the three steps above. In the local stack, an edit to the built-in catalogue needs this service's
image rebuilt (`docker compose up -d --build csm-ai-integration-service`), not the backend's.

The Teams section is not in the file: the backend sends the team list with each request and
`internal/teams` appends it to the catalogue (the same text the backend wrote before the move).

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
