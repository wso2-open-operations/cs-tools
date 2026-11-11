# CSM AI Integration Service

The CSM platform's calls to Claude. The CSM portal backend calls it to generate dashboards from a user's
prompt; it builds the prompt from a catalogue, calls Claude through the AI gateway, and returns what the
model wrote. Other AI features can join under `/ai/`.

```text
 webapp --> CSM portal backend --(OAuth2 client credentials)--> csm-ai-integration-service --> AI gateway --> Claude
            (auth, permissions,                                 (catalogue, prompt, call,
             validation, teams)                                  reading the answer)
```

See `CLAUDE.md` for what lives here and what stays in the backend.

## Configuration

| Variable | Default | Purpose |
|---|---|---|
| `PORT` | `8080` | Listen port |
| `ANTHROPIC_API_KEY` | (none) | Key for the model calls (a secret). Unset: the service starts, model calls are off |
| `CLAUDE_BASE_URL` | (none) | **Required with the key**: the URL of the CSM AI gateway the key belongs to (absolute http(s) URL; an internal address, keep it out of committed files). No fallback to the Anthropic API |
| `CLAUDE_MODEL` | `claude-sonnet-5-5` | Model id |
| `CLAUDE_TIMEOUT` | `45s` | One model call's timeout (Go duration, > 0) |

A bad value or a key without `CLAUDE_BASE_URL` fails the start.

The dashboard catalogue is built into the binary (`internal/catalogue/catalogue.md`): a change to it ships
with the next build. `CATALOGUE_DIR` and `CATALOGUE_FILE` are no longer read; left set, they are ignored
with a warning at start: remove them.

## API endpoints

| Method and path | Purpose |
|---|---|
| `GET /health` | Liveness, always 200 |
| `POST /ai/dashboards/generate` | One model call for a dashboard definition: returns the JSON objects the model wrote (see `CLAUDE.md`) |

## Token usage

Each model call logs one line, `dashboard generate: token usage`, with the portal user's id (`userID`, from
the backend's `X-CSM-User-ID` header; never the email), `correlationID`, `model`, `outcome`, `inputTokens`,
`outputTokens` and `durationMs` (the model call alone). It never holds the prompt or the reply. In Choreo it
is in the component's logs; locally, `docker compose logs csm-ai-integration-service | grep "token usage"`.
See `CLAUDE.md`.

## Developer guide

```bash
cp .env.example .env
go run ./cmd/server
make test
```

In the local compose stack (`apps/csm-portal/README.md`) it runs as `csm-ai-integration-service` on
`127.0.0.1:8087`, with the key and gateway taken from the repository root's git-ignored `.env`; the CSM
portal backend reaches it at `http://csm-ai-integration-service:8080`.
