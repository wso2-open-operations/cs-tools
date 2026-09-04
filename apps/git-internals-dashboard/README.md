# Git Internals Dashboard

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](../../LICENSE)
[![GitHub last commit](https://img.shields.io/github/last-commit/wso2-open-operations/cs-tools/main?path=apps%2Fgit-internals-dashboard)](https://github.com/wso2-open-operations/cs-tools/commits/main/?path=apps/git-internals-dashboard)
[![GitHub issues](https://img.shields.io/github/issues/wso2-open-operations/cs-tools.svg)](https://github.com/wso2-open-operations/cs-tools/issues)

Git Internals Dashboard is an open-source SLA monitor for customer-success-originated
GitHub issues, tracking product-team SLA compliance from ingest through to
reporting. It ships as a single Next.js application — API routes and UI in one
process — so it deploys as one component with no separate frontend/backend to
coordinate.

## Why Git Internals Dashboard?

Customer success teams file GitHub issues on behalf of customers and need
visibility into whether product teams are responding and resolving them
within agreed SLAs. Tracking this by hand across repos and projects doesn't
scale, and naively wiring a dashboard straight to the GitHub API risks
leaking issue titles, assignees, or other customer-identifying detail into a
tool with broader visibility than the source issue.

Git Internals Dashboard addresses this by combining:

- A sync pipeline that ingests GitHub issues and projects on a schedule,
  computes SLA status against a configurable taxonomy, and persists only the
  fields needed for SLA reporting — never titles, assignees, labels, or
  actors,
- A single Next.js app (App Router + Route Handlers) serving both the API
  and the dashboard UI from the same origin.

This setup lets teams monitor SLA compliance without exposing sensitive
issue content or standing up separate infrastructure per environment.

## Features

- **Single-Process Architecture**
  Dashboard UI and API share one Next.js app and one deployable unit — no
  CORS configuration, no separate frontend/backend releases.
- **Privacy-First Ingest**
  The sync pipeline persists only SLA-relevant fields; issue titles,
  assignees, labels, and actors are never stored or sent to the client.
- **Configurable SLA Taxonomy**
  SLA status categorization is driven by a YAML taxonomy file
  ([`config/sla-config.yaml`](./config/sla-config.yaml)), not hardcoded
  status strings.
- **Pluggable Authentication**
  A stub mode for local development and CI, or Bearer JWT verification
  against an Asgardeo tenant with group-based access control.
- **Background Recompute**
  SLA status is recomputed on a schedule via a background job, coordinated
  across replicas with a Postgres advisory lock rather than in-process
  state.
- **Container-Ready**
  Ships with a multi-stage `Dockerfile` for standalone deployment behind any
  container platform.

## Project Structure

```bash
.
├── src/app                  # Next.js App Router: pages + src/app/api Route Handlers
├── src/server                # SLA engine, config loader, GitHub client, ingest/sync, auth, jobs
├── src/components, src/views # Dashboard UI
├── prisma                   # Database schema and migrations
├── config/sla-config.yaml   # SLA taxonomy / status categorization config
├── Dockerfile                # Multi-stage build for standalone deployment
└── README.md                 # You're here
```

## Technologies Used

- **Framework**: [Next.js](https://nextjs.org/) 16 (App Router) + React 19 + TypeScript
- **UI**: [WSO2 Oxygen UI](https://github.com/wso2/oxygen-ui) (MUI-based design system)
- **Data Layer**: TanStack Query, Prisma (Postgres)
- **Authentication**: Stub mode, or Bearer JWT via an OIDC-compatible provider (Asgardeo)
- **Charts**: Recharts / Oxygen UI Charts

## Getting Started

### Prerequisites

- Node.js 20.9+
- Docker (for local Postgres)

### Local Development

```bash
npm install
docker compose up -d          # Postgres on localhost:5433
cp .env.example .env          # defaults to AUTH_MODE=stub, synthetic seed data
npm run db:migrate
npm run db:seed               # synthetic fixtures unless SEED_GITHUB_TOKEN is set
npm run dev                   # http://localhost:3000
```

Run `npm test` (Vitest) for the unit/integration test suite — the DB-backed
tests need the same Postgres instance started above.

### Deployment

This app has no framework-specific buildpack requirement; the included
`Dockerfile` (multi-stage, Next.js `output: 'standalone'`) builds a
self-contained image suitable for any container platform.

Two things worth knowing before deploying:

1. **`NEXT_PUBLIC_*` vars are inlined at build time**, not read at container
   startup — pass them as Docker build args, not runtime env vars (see the
   `ARG`s in `Dockerfile`).
2. **Database migrations aren't run by the container's start command.** Run
   `npx prisma migrate deploy` against the target database once per release,
   separately from starting the app.

## Reporting Issues

### 1. Opening an issue

Please use this repository's issue tracker and include reproduction steps,
expected behavior, actual behavior, and relevant logs.

### 2. Reporting security issues

Please do not report security issues through public issues. Follow the
[WSO2 Security Vulnerability Reporting Guidelines](https://security.docs.wso2.com/en/latest/security-reporting/vulnerability-reporting-guidelines/).

## Contributing

Contributions are welcome. Create a feature branch, keep changes focused,
and submit a pull request with a clear description and verification steps.

## License

Git Internals Dashboard is licensed under Apache 2.0. See the
[LICENSE](../../LICENSE) file for details.
