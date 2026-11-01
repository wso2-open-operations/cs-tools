# Knowledge Portal Backend

The Go backend of the public Knowledge Portal, the standalone site that serves WSO2's published
knowledge base articles. It replaces the legacy knowledge site. It is public, read-only and
**unauthenticated by design**: no token is required or accepted, so there is no identity or
authorization middleware here.

This service owns no database. Articles and knowledge bases are read from **entity-service**,
authenticated as a service (OAuth2 client credentials), never as an end user.

## What it serves

| Method | Path | Purpose |
|---|---|---|
| GET | `/health` | Liveness probe |
| GET | `/public/knowledge-bases` | Active knowledge bases |
| POST | `/public/kb-articles/search` | Search published articles, newest first |
| GET | `/public/kb-articles/{id}` | One published article |
| GET | `/public/kb-articles/{id}/related` | Related published articles |

The full contract is in [openapi.yaml](openapi.yaml).

## Safety properties

Because anyone on the internet can call it, these hold regardless of what a caller sends:

- **Published only.** The article state filter is set by the server, is not part of the request
  body, and results are filtered again on the way out. A draft, in-review or retired article is a
  `404`, indistinguishable from one that does not exist.
- **Active knowledge bases only.** An article of an inactive knowledge base is a `404` and is
  dropped from search and related results. The active set is read from entity-service, cached in
  memory for 60 seconds (a deactivation can lag by that long) and never served stale: if it cannot
  be loaded the request fails with `502`. Search without `knowledgeBaseId` filters each upstream
  page after fetching it, so a page can be short and `total` is an upper bound.
- **Whitelist responses.** Handlers build responses from fixed DTOs; author, reviewer, source case,
  rejection comment and version-lineage fields from upstream are dropped at decode time.
- **Ids cannot steer upstream URLs.** Article ids (path) and `knowledgeBaseId` (search body) must
  be UUIDs in canonical 8-4-4-4-12 hex form, any case, and are lower-cased before use. Ids read from
  the knowledge base list or the vector index are held to the same rule and dropped otherwise. A
  malformed id is a `400`, a well-formed id with no published article a `404`. The client path-escapes
  ids again and never follows redirects.
- **Generic errors.** Upstream statuses and bodies are logged, never relayed: on a by-id lookup an
  upstream `404` (no such article) or `400` (malformed id) becomes the portal's `404`; callers see
  `504` on timeout and `502` for anything else.
- **Correlation ids are server-minted.** Every request gets a fresh `X-CSM-Correlation-ID`, returned
  in the response and sent to entity-service. A caller-supplied value is never echoed or forwarded;
  it is logged separately as `clientCorrelationID` (sanitised, capped at 64 characters).
- **Bounded input.** Request bodies are capped at 16 KiB, `limit` 1-50, `offset` 0-10000,
  `searchQuery` at most 200 characters, and unknown JSON fields are rejected.
- **Read-only.** There is no write route and the entity client has no write method.

## Layout

```
cmd/server/            main, config loading and validation, timeouts
internal/apierror/     typed error for a non-2xx upstream response
internal/entity/       entity-service client (OAuth2 client credentials)
internal/pinecone/     optional vector-index client for related articles
internal/middleware/   SecurityHeaders, CorrelationID, Logger, Recovery
internal/handler/      routes, validation, DTOs, error mapping
```

Middleware order, outermost first: SecurityHeaders, CorrelationID, Logger, Recovery, router.

## Running locally

```bash
# from apps/knowledge-portal/backend
cp .env.example .env     # then fill in the required values
go run ./cmd/server
```

The server loads `.env` from the working directory (ignored if absent) and listens on `PORT`
(default `8080`). `make test` runs `go vet` and `go test -race ./...`.

## Configuration

| Variable | Required | Default | Notes |
|---|---|---|---|
| `ENTITY_SERVICE_BASE_URL` | yes | | Absolute https URL; plain http only for a loopback host (`localhost`, `127.0.0.0/8`, `::1`). Trailing slash trimmed |
| `OAUTH2_CLIENT_ID`, `OAUTH2_CLIENT_SECRET` | yes | | Client-credentials app used toward entity-service |
| `OAUTH2_TOKEN_URL` | yes | | Absolute https URL, no exceptions |
| `ENTITY_SERVICE_SCOPES` | no | | Comma-separated |
| `PORT` | no | `8080` | Plain port number |
| `REST_READ_TIMEOUT`, `REST_WRITE_TIMEOUT`, `ENTITY_SERVICE_TIMEOUT` | no | 10s, 30s, 10s | Go durations, all > 0. `REST_WRITE_TIMEOUT` must be at least 20s (the related-articles lookup budget) or startup fails; keep `ENTITY_SERVICE_TIMEOUT` shorter than it |
| `PINECONE_HOST`, `PINECONE_KEY` | no | | Both or neither; enables semantic related articles. `PINECONE_HOST` must be an absolute https URL |

Startup fails on a missing or malformed required value, naming every problem at once.

## Limits worth knowing

- Search is title-substring only and always ordered newest first. entity-service offers no other
  ordering, so `sortBy` accepts only `recent`; there are no view counts or ratings to sort by.
- Article `body` is HTML as authored. Clients must sanitise it before rendering.
