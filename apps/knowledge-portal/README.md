# WSO2 Knowledge Portal

A standalone, fully public knowledge base for WSO2 products. Replaces the
ServiceNow-hosted knowledge.wso2.com. Read-only, no authentication.

## Structure

- `backend/` — Go API (own module). Reads published KB articles from
  entity-service and serves them on public, unauthenticated routes. Related
  articles come from Pinecone with a same-knowledge-base fallback.
- `webapp/` — React + Vite + Oxygen UI. Landing page with search, browse,
  and article views.

## Running locally

The backend depends on a running entity-service (default `:8081`).

Backend:

    cd backend
    ENTITY_SERVICE_BASE_URL=http://localhost:8081 PORT=8085 go run ./cmd/server

Frontend:

    cd webapp
    pnpm install
    pnpm run dev   # http://localhost:3100

Copy `backend/.env.example` and `webapp/.env.example` and adjust as needed.
Set `PINECONE_HOST` / `PINECONE_KEY` on the backend to enable semantic
related-article search; without them it falls back to same-KB articles.
