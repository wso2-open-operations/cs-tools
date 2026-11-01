# WSO2 Knowledge Portal

A standalone, fully public knowledge base for WSO2 products.
Replaces the legacy knowledge site.
Read-only, no authentication.

## Structure

- `backend/` — Go API (own module). Reads published KB articles from
  entity-service and serves them on public, unauthenticated routes. Related
  articles come from a vector-similarity index with a same-knowledge-base fallback.
- `webapp/` — React + Vite + Oxygen UI. Landing page with search, browse,
  and article views.

## Running locally

Backend: see [backend/README.md](backend/README.md) for setup and configuration.

Frontend (see [webapp/README.md](webapp/README.md)):

    cd webapp
    pnpm install
    cp public/config.js.example public/config.js   # git-ignored, never commit
    pnpm run dev   # http://localhost:3100
