# Git Internals Dashboard

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](../../LICENSE)
[![GitHub last commit](https://img.shields.io/github/last-commit/wso2-open-operations/cs-tools/main?path=apps%2Fgit-internals-dashboard)](https://github.com/wso2-open-operations/cs-tools/commits/main/?path=apps/git-internals-dashboard)
[![GitHub issues](https://img.shields.io/github/issues/wso2-open-operations/cs-tools.svg)](https://github.com/wso2-open-operations/cs-tools/issues)

Git Internals Dashboard is an open-source SLA monitor for GitHub issues, built with a Go backend and a React web application. It tracks how issues move through a configurable status taxonomy and reports SLA compliance per issue, per priority, and over time.

## Why Git Internals Dashboard?

Teams that track support-originated work as GitHub issues need visibility into how long issues sit in each status and whether they're on track against their SLA. Piecing this together by hand from GitHub's own UI doesn't scale once several repos and projects are involved.

Git Internals Dashboard addresses this by combining:

- A Go backend that ingests issues from GitHub's GraphQL API, classifies each status change against a configurable taxonomy, and computes SLA compliance,
- A React SPA that visualizes issue volume, SLA breach risk, and compliance trends over time.

This setup gives a single, always-current view of SLA health without manually cross-referencing GitHub project boards.

## Features

- **GitHub Issue Ingestion**
  Issues and their status history are pulled from GitHub's GraphQL v4 API, either via an incremental sync or a one-off seed for local development.
- **Configurable SLA Engine**
  A YAML taxonomy maps each board status to a category (accrues SLA, paused, or terminal) and defines per-priority time budgets and coverage windows; SLA compliance is computed from this configuration, not hardcoded.
- **Metrics & Trends**
  Overview and time-series endpoints summarize issue volume, SLA breach risk, and compliance over time for the dashboard's charts.
- **Background Recompute**
  A scheduler periodically recomputes SLA state so the dashboard reflects elapsed time even between syncs, coordinated across replicas with a database advisory lock.
- **Privacy-Conscious by Design**
  Issue titles, labels, assignees, and other identifying details are never persisted — see [Privacy](#privacy) below.
- **Identity Integration**
  The webapp authenticates via an OIDC-compatible identity provider (Asgardeo).

## Project Structure

```bash
.
├── backend                  # Go service — ingestion, SLA engine, metrics API
│   └── README.md             # Detailed backend documentation
├── webapp                   # React + TypeScript SPA
│   └── README.md             # Detailed webapp documentation
└── README.md                 # You're here
```

## Architecture

```
Browser (webapp SPA)
  │  Authorization: Bearer <access token>
  ▼
API gateway   ← JWT validation, rate limiting, CORS all live HERE
  ▼
backend (Go, authless) ──► Postgres
                      ──► GitHub GraphQL API (sync, seed, on-demand titles)
```

- The backend trusts the upstream gateway completely — it parses no auth token and has no auth
  middleware of its own. Locally, with no gateway in front, `CORS_ALLOWED_ORIGINS` is what lets
  the webapp dev server reach the backend directly.
- A Postgres advisory lock guards the in-process recompute scheduler so multiple backend
  replicas never interleave the same work.
- Ingestion is a single write path (`internal/ingest`) shared by the manual sync and the seed
  command, so any future ingestion trigger (e.g. a webhook) can reuse it unchanged.

## Technologies Used

### Backend

- **Language**: [Go](https://go.dev/) 1.26+
- **Database**: PostgreSQL, via `pgx/v5`
- **Upstream**: GitHub GraphQL v4 API

### Frontend

- **Framework**: React 19 + TypeScript (Vite)
- **UI**: WSO2 Oxygen UI
- **Data Layer**: TanStack Query
- **Authentication**: Asgardeo (`@asgardeo/react`)

## Getting Started

### Prerequisites

- Go 1.26+
- Node.js 24 (LTS)
- `pnpm` 10
- Docker (for local Postgres)

### Backend Setup Guide

- [Backend](./backend/README.md)

### Frontend Setup Guide

- [Webapp](./webapp/README.md)

## Privacy

No issue title, label, assignee, opener, or status-event actor is ever persisted to the database
or returned by any endpoint except `POST /issues/titles`, which fetches titles live from GitHub
on demand and caches them in memory only (never written to Postgres, never sent anywhere else).
Labels are read transiently during ingest solely to derive an issue's priority, then discarded.

## Reporting Issues

### 1. Opening an issue

Please use this repository's issue tracker and include reproduction steps, expected behavior, actual behavior, and relevant logs.

### 2. Reporting security issues

Please do not report security issues through public issues. Follow the [WSO2 Security Vulnerability Reporting Guidelines](https://security.docs.wso2.com/en/latest/security-reporting/vulnerability-reporting-guidelines/).

## Contributing

Contributions are welcome. Create a feature branch, keep changes focused, and submit a pull request with a clear description and verification steps.

## License

Git Internals Dashboard is licensed under Apache 2.0. See the [LICENSE](../../LICENSE) file for details.
