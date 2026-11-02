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
| `CATALOGUE_DIR` | built-in | A directory of catalogue parts replacing the built-in dashboard catalogue: its `*.md` files, joined in name order (Choreo file mounts, see below). Read once at start: restart to apply a change |

A bad value, a key without `CLAUDE_BASE_URL`, a catalogue directory that cannot be read, has no `*.md` file
or has an empty one, or the old `CATALOGUE_FILE` set without `CATALOGUE_DIR`, fails the start. Left set next
to `CATALOGUE_DIR`, `CATALOGUE_FILE` is ignored, with a warning at start: remove it.

## Mounting the catalogue in Choreo

Choreo limits the size of a mounted file (about 20 KB; the catalogue is larger), so the catalogue is mounted
in parts. The parts are in **`catalogue-parts/`** (`catalogue-01.md`, `catalogue-02.md`, ...), each at most
20,000 bytes, generated from `internal/catalogue/catalogue.md` by `make catalogue-parts`.

1. In the component's Configs & Secrets, add one file mount per file in `catalogue-parts/`, with that
   file's content, all under the same directory and keeping the file names, e.g.
   `/mnt/catalogue/catalogue-01.md`, `/mnt/catalogue/catalogue-02.md`.
2. Set `CATALOGUE_DIR` to that directory (`/mnt/catalogue`).
3. After the restart, check the startup log line `model calls`: `catalogue` names the directory and
   `catalogueParts` lists the parts in the order they were joined.

The service joins the directory's `*.md` files in name order, with one blank line between them, back into
the catalogue. Files starting with `.` or `_`, other files and subdirectories are skipped.

**When `catalogue.md` changes, the mounts must be updated too.** Run `make catalogue-parts`, commit
`catalogue-parts/` (a test fails until you do), then in every environment that mounts the catalogue,
replace each mount's content with the new part, add a mount for a new part and remove one for a part that
is gone. Until then that environment keeps the old catalogue: nothing updates the mounts or flags them as
old.

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
make catalogue-parts   # after editing internal/catalogue/catalogue.md; commit catalogue-parts/
```

In the local compose stack (`apps/csm-portal/README.md`) it runs as `csm-ai-integration-service` on
`127.0.0.1:8087`, with the key and gateway taken from the repository root's git-ignored `.env`; the CSM
portal backend reaches it at `http://csm-ai-integration-service:8080`.
