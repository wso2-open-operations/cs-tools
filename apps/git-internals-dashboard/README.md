# git-internals-dashboard

SLA monitor for CS-originated GitHub issues, tracking product-team SLA
compliance. Rewritten from the [`internal-sla-monitor-v2`](../internal-sla-monitor-v2)
monorepo (Fastify API + Vite/React SPA) into a single Next.js 16 (App Router)
application, so it deploys as **one** Choreo component instead of a separate
frontend and backend.

UI is built on [Oxygen UI](https://github.com/wso2/oxygen-ui) (`@wso2/oxygen-ui`,
WSO2's MUI-based design system), themed with **Acrylic Purple** — Oxygen UI's
frosted-glass material theme (translucent surfaces, backdrop blur, purple
gradient accent, `#646cff` primary). Set in `src/lib/theme.ts`. The
dashboard's own design tokens (`--sla-*` in `src/app/globals.css`) alias
Oxygen UI's live theme CSS variables (`--oxygen-palette-*`, `--oxygen-blur-*`)
rather than hardcoding hex, so swapping the theme there (e.g. to
`WSO2Theme`/`AcrylicOrangeTheme`) re-themes the whole app without touching
component code — see the comments in `theme.ts` and `globals.css` for the one
documented exception (`--sla-primary-gradient`) and why.

## Architecture at a glance

- **Frontend + API in one process.** Pages under `src/app/` (App Router,
  client components) call Route Handlers under `src/app/api/**`, same origin
  — no CORS needed (v2 needed `CORS_ORIGINS` because the Vite dev server and
  Fastify API were different origins).
- **Backend logic** (`src/server/`) is a near-verbatim port of v2's pure SLA
  engine, config loader, GitHub client, and ingest/sync pipeline — same
  privacy rules (no titles/assignees/labels/actors ever persisted or put on
  the wire), same taxonomy-driven status categorization.
- **Auth** (`src/server/auth`): `AUTH_MODE=stub` (no auth, default for local
  dev) or `AUTH_MODE=asgardeo` (Bearer JWT verification against an Asgardeo
  tenant + group check), applied via a `requireAuth()` wrapper around each
  Route Handler — not `proxy.ts`/middleware, since that runs on the Edge
  runtime by default and doesn't suit `jose`'s remote JWKS fetch or
  Prisma-touching handlers.
- **Background jobs** (`src/server/jobs/recompute.ts`) start from
  `instrumentation.ts` (Next's boot hook), replacing Fastify's `app.listen`
  callback.
- **Cross-replica job lock** (`src/server/jobs/lock.ts`): a Postgres
  `pg_try_advisory_lock`, not v2's in-process boolean. Choreo does **not**
  guarantee single-instance deployment on paid/private-data-plane tiers with
  autoscaling enabled, so the manual-sync/recompute mutex needed to become
  real cross-process coordination.

See inline comments in `src/server/db/client.ts`, `src/server/jobs/lock.ts`,
and `next.config.ts` for the specific deviations from v2 and why.

## Local development

```bash
npm install
docker compose up -d          # Postgres on localhost:5433
cp .env.example .env          # defaults to AUTH_MODE=stub, synthetic seed data
npm run db:migrate
npm run db:seed               # synthetic fixtures unless SEED_GITHUB_TOKEN is set
npm run dev                   # http://localhost:3000
```

Run `npm test` (Vitest) for the ported unit/integration test suite — the
DB-backed tests need the same Postgres instance from `docker compose up`.

## Deploying to Choreo

Choreo has no dedicated Next.js buildpack; this repo ships a `Dockerfile`
(multi-stage, `next.config.ts`'s `output: 'standalone'`) for Choreo's
"Bring your own Dockerfile" Web Application component path — see
`.choreo/component.yaml` for the endpoint descriptor.

Two things that don't just fall out of `docker build`:

1. **`NEXT_PUBLIC_*` vars are inlined at build time**, not read at container
   startup. Pass them as Docker build args (Choreo: Build Configuration),
   not just runtime env vars — see the `ARG`s in `Dockerfile`.
2. **Database migrations aren't run by the container's `CMD`.** Run
   `npx prisma migrate deploy` against the target database once per release
   (a Choreo pre-deploy/init step, or manually), separately from starting
   `node server.js`.

If using Choreo's managed Postgres "Connection" feature instead of a plain
`DATABASE_URL`, see the comment in `src/server/db/client.ts` — it injects
five separate `CHOREO_<NAME>_*` env vars that get assembled into a connection
string automatically.

## Known gaps vs. v2 (not yet addressed)

- **No global rate limiting.** v2 had `@fastify/rate-limit` (300 req/min per
  IP, 60/min on the titles endpoint). A single-process in-memory limiter
  would be incorrect once Choreo scales beyond one replica; this is better
  enforced at the platform/gateway layer (Choreo policies, or WSO2 API
  Manager in front) than reimplemented per-replica here.
- **No CSP/nonce setup.** MUI/Emotion inject `<style>` tags at runtime;
  supporting a strict `style-src` CSP needs a per-request nonce threaded
  through `OxygenUIThemeProvider`'s `nonce`/`emotionCache` props. Not wired
  up yet — flagged rather than shipped half-done.
