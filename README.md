# cs-tools

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![GitHub last commit](https://img.shields.io/github/last-commit/wso2-open-operations/cs-tools/main)](https://github.com/wso2-open-operations/cs-tools/commits/main)
[![GitHub issues](https://img.shields.io/github/issues/wso2-open-operations/cs-tools.svg)](https://github.com/wso2-open-operations/cs-tools/issues)

Open-source tools built by WSO2 for customer success operations. This repository contains web applications and backend integrations that enable teams to manage customer-facing workflows, project visibility, and support experiences.

## Repository Structure

```
cs-tools/
├── apps/
│   ├── csm-portal/                        # CSM Portal — Go BFF + React webapp + React microapp
│   ├── customer-portal/                   # Customer Portal — Ballerina backend (v1, deprecated) +
│   │                                       #   Go backend-v2 (in progress) + React webapp + React microapp
│   ├── git-internals-dashboard/           # SLA monitor for GitHub issues — Go backend + React webapp
│   ├── knowledge-portal/                  # Public knowledge base — Go backend + React webapp
│   ├── mttr-application/                  # MTTR analytics for ServiceNow homepage widgets — Node.js
│   ├── security-advisory-patches-portal/  # Gated advisory-PDF portal — Ballerina backend + React webapp
│   └── today-i-learned/                   # Today I Learned (Go backend for the feed in One WSO2)
├── entity-service/          # Shared entity service (the CSM platform's system of record)
├── integrations/
│   ├── acp-closure-service/                     # Go CLI: Account Closure Process, Phase 1
│   ├── csm-flow-service/                        # ServiceNow discovery notes only — service code lives elsewhere
│   ├── csm-integration-service/                 # Go M2M service for third-party account/project search
│   ├── csm-notification-service/                # Go worker: email/Chat/voice notifications, SLA + paging engines
│   ├── csm-portal-activity-stream-service/      # Go SSE push service for the CSM portal
│   ├── customer-portal-activity-stream-service/ # Go SSE push service for the customer portal
│   ├── customer-service/                        # Customer operations related integration Ballerina service
│   ├── sftpgo-authentication-service/           # Go auth/provisioning hooks for SFTPGo, backed by Asgardeo
│   ├── sre-alert-core-service/                  # Go: dedups alerts into incidents, forwards to CSM/Chat
│   └── sre-alert-ingestion-service/             # Go: normalizes monitoring webhooks into alert rows
├── operations/
│   ├── csm-scheduled-tasks/   # Go: one Choreo Scheduled Task fanning out to several sub-crons
│   └── csm-webhooks/          # Public inbound-webhook receivers, one folder per source (e.g. GitHub)
└── scripts/
    └── csm-compose/            # Local Docker Compose stack for the CSM platform (seed data, mock IdP, ...)
```

## Components

### CSM Portal (`apps/csm-portal/`)

An internal portal for Customer Success Managers to manage and track customer cases, assignments, and support operations.

| Component | Description |
|-----------|-------------|
| `backend` | Go RESTful service (backend-for-frontend) providing CSM Portal APIs |
| `webapp` | Browser-based portal for CSMs (React + TypeScript) |
| `microapp` | Mobile micro-app variant of the portal UI (React + TypeScript) |

See the [CSM Portal README](./apps/csm-portal/README.md) for full setup and usage documentation.

### Customer Portal (`apps/customer-portal/`)

> **⚠️ Deprecation Notice:** The Ballerina backend (v1) for the Customer Portal is deprecated and being replaced by `backend-v2`, a Go rewrite (a backend-for-frontend over this repo's own `entity-service`). New feature development should target `backend-v2` instead.

An open-source solution for customer success operations built with a modular architecture. It enables teams to manage customer-facing workflows, project visibility, and support experiences through a unified platform.

| Component | Description |
|-----------|-------------|
| `backend` | [Ballerina](https://ballerina.io/) RESTful service providing Customer Portal APIs and integration points (deprecated — see above) |
| `backend-v2` | Go backend-for-frontend rewrite, targeting this repo's `entity-service` (in progress — see its own [README](./apps/customer-portal/backend-v2/README.md)) |
| `webapp` | Publicly available browser-based customer portal (React + TypeScript) |
| `microapp` | Microapp experience inside the [WSO2 super app](https://github.com/opensuperapp/opensuperapp/tree/v1) (React + TypeScript) |

See the [Customer Portal README](./apps/customer-portal/README.md) for full setup and usage documentation.

### Git Internals Dashboard (`apps/git-internals-dashboard/`)

An open-source SLA monitor for GitHub issues. Ingests issues from GitHub's GraphQL API, classifies each status change against a configurable taxonomy, and reports SLA compliance per issue, per priority, and over time.

| Component | Description |
|-----------|-------------|
| `backend` | Go service that ingests issues, classifies status changes, and computes SLA compliance |
| `webapp` | React SPA visualizing issue volume, SLA breach risk, and compliance trends |

See the [Git Internals Dashboard README](./apps/git-internals-dashboard/README.md) for full setup and usage documentation.

### Knowledge Portal (`apps/knowledge-portal/`)

A public, read-only knowledge base. It serves published articles from active knowledge bases through unauthenticated routes, with search, browse and related articles. It reads from this repo's `entity-service`.

| Component | Description |
|-----------|-------------|
| `backend` | Go service that reads published articles from `entity-service` and serves them on public routes |
| `webapp` | React SPA with search, browse and article views |

See the [Knowledge Portal README](./apps/knowledge-portal/README.md) for full setup and usage documentation.

### MTTR Application (`apps/mttr-application/`)

A Mean Time to Resolve (MTTR) analytics service for WSO2 Customer Support. Pulls resolved cases from ServiceNow, computes a 95th-percentile truncated mean MTTR across nine dimensions, caches results in PostgreSQL, and serves them to ServiceNow homepage widgets.

| Layer | Technology |
|-------|------------|
| Language | Node.js |
| Database | PostgreSQL |
| Upstream | ServiceNow |

See the [MTTR Application README](./apps/mttr-application/README.md) for full setup and usage documentation.

### Security Advisory Patches Portal (`apps/security-advisory-patches-portal/`)

A gated portal for sharing security advisory PDFs with customers. Advisory PDFs live in an Azure Files share; the backend streams bytes by path, and users sign in with Asgardeo before any `.pdf`-suffixed path is served.

| Component | Description |
|-----------|-------------|
| `backend` | [Ballerina](https://ballerina.io/) service that authenticates callers and streams PDFs from Azure Files |
| `webapp` | Minimal React app that signs users in with Asgardeo and links to advisory PDFs |

See the [Security Advisory Patches Portal README](./apps/security-advisory-patches-portal/README.md) for full setup and usage documentation.

### Today I Learned (`apps/today-i-learned/`)

Backend for "Today I Learned" — a company-wide feed of learnings from customers, partners, and internal work. Entries are shared, read and edited in the One WSO2 webapp, and each new one is announced in Google Chat through Novera.

| Component | Description |
|-----------|-------------|
| `backend` | Go service (`til-backend`) providing the entry, search, moderation, and customer-suggestion APIs |

See the [Today I Learned README](./apps/today-i-learned/README.md) for full setup and usage documentation.

### Entity Service (`entity-service/`)

A Go REST service that manages customer entity data backed by PostgreSQL. Provides APIs for case management with pagination and validation, used by other apps in this repository.

| Layer | Technology |
|-------|------------|
| Language | Go |
| Framework | Gin |
| Database | PostgreSQL 15+ |

See the [Entity Service README](./entity-service/README.md) for full setup and usage documentation.

### ACP Closure Service (`integrations/acp-closure-service/`)

A Go run-to-completion CLI implementing Phase 1 of the Account Closure Process (ACP) — subscription end-date closure. Unlike every other Go component in this repo, it is not an HTTP server: it performs one full sweep over open projects and exits, with a Choreo Task component's cron owning the schedule.

| Layer | Technology |
|-------|------------|
| Language | Go |
| Shape | Run-to-completion CLI (Choreo Task) |
| Upstream | CSM Integration Service (OAuth2 client credentials) |

See the [ACP Closure Service README](./integrations/acp-closure-service/README.md) for full setup and usage documentation.

### Customer Service Integration (`integrations/customer-service/`)

A Ballerina REST service that aggregates and exposes customer data from multiple backend systems. Provides endpoints for contact lookup, deployment search, and deployed product queries. Includes an in-memory cache layer to reduce upstream load.

### CSM Integration Service (`integrations/csm-integration-service/`)

A Go REST service that exposes customer data to third-party (M2M) consumers, backed by the Entity Service.

| Layer | Technology |
|-------|------------|
| Language | Go |
| Framework | `net/http` (standard library) |
| Upstream | Entity Service (OAuth2 client credentials) |

See the [CSM Integration Service README](./integrations/csm-integration-service/README.md) for full setup and usage documentation.

### CSM Notification Service (`integrations/csm-notification-service/`)

A Go background worker that turns CSM platform domain events into outbound notifications — email, Google Chat posts, and voice calls. Consumes events from Azure Event Hub (Kafka-compatible endpoint) and also runs an optional SLA breach-alerting engine and an optional incident call-escalation ladder. Its only HTTP route is `GET /health`.

| Layer | Technology |
|-------|------------|
| Language | Go |
| Shape | Kafka consumer (no inbound API beyond a health check) |
| Upstream | Azure Event Hub, entity-service, Google Chat, email service, Twilio |

See the [CSM Notification Service README](./integrations/csm-notification-service/README.md) for full setup and usage documentation.

### CSM Portal Activity Stream Service (`integrations/csm-portal-activity-stream-service/`)

A Go service that pushes live "this case changed" notifications to the CSM portal over Server-Sent Events (SSE). Consumes case events from the event bus and fans a minimal signal out to every browser currently watching the affected case.

| Layer | Technology |
|-------|------------|
| Language | Go |
| Shape | SSE push service (`GET /cases/{id}/activities/stream`) |
| Upstream | Azure Event Hub |

See the [CSM Portal Activity Stream Service README](./integrations/csm-portal-activity-stream-service/README.md) for full setup and usage documentation.

### Customer Portal Activity Stream Service (`integrations/customer-portal-activity-stream-service/`)

The Customer Portal's counterpart to the CSM Portal Activity Stream Service above — cloned and adapted for `apps/customer-portal`, with its own consumer group on the same `case-events` topic so each portal scales and fails independently.

| Layer | Technology |
|-------|------------|
| Language | Go |
| Shape | SSE push service (`GET /cases/{id}/activities/stream`) |
| Upstream | Azure Event Hub |

See the [Customer Portal Activity Stream Service README](./integrations/customer-portal-activity-stream-service/README.md) for full setup and usage documentation.

### SFTPGo Authentication Service (`integrations/sftpgo-authentication-service/`)

A secure, individual-account-based authentication and provisioning service for [SFTPGo](https://github.com/drakkan/sftpgo), integrated with Asgardeo. Authenticates users against corporate credentials, enforces MFA, and dynamically provisions virtual folders based on subscriptions or roles via SFTPGo's pre-login hook.

| Layer | Technology |
|-------|------------|
| Language | Go |
| Upstream | SFTPGo Admin API, Asgardeo |

See the [SFTPGo Authentication Service README](./integrations/sftpgo-authentication-service/README.md) for full setup and usage documentation.

### Alert Core Service (`integrations/sre-alert-core-service/`)

Deduplicates incoming monitoring alerts into incidents and forwards them to CSM via the CSM Integration Service, falling back to Google Chat when CSM doesn't confirm. Reads alerts written by Alert Ingestion Service from PostgreSQL and folds them into incidents by fingerprint.

| Layer | Technology |
|-------|------------|
| Language | Go |
| Database | PostgreSQL |
| Upstream | CSM Integration Service, Google Chat |

See the [Alert Core Service README](./integrations/sre-alert-core-service/README.md) for full setup and usage documentation.

### Alert Ingestion Service (`integrations/sre-alert-ingestion-service/`)

Receives monitoring webhooks from ten sources (AWS, Azure, Datadog, Elasticsearch, GCP, Icinga, OpenObserve, OpenSearch, Prometheus, Site24x7), normalizes each alert to a common shape, and writes it to PostgreSQL for Alert Core Service to claim.

| Layer | Technology |
|-------|------------|
| Language | Go |
| Database | PostgreSQL |
| Downstream | Alert Core Service |

See the [Alert Ingestion Service README](./integrations/sre-alert-ingestion-service/README.md) for full setup and usage documentation.

## Operations (`operations/`)

Shared operational components that run on their own schedule or expose their own public endpoints, rather than belonging to one specific app.

### CSM Scheduled Tasks (`operations/csm-scheduled-tasks/`)

A single Go component that runs several independently-scheduled recurring jobs ("sub-crons" — housekeeping, stale/open case reports, availability recalculation, ...) behind one shared platform trigger, with failure tracking and automatic retry backed by entity-service. Each invocation runs exactly one tick and exits.

See the [CSM Scheduled Tasks README](./operations/csm-scheduled-tasks/README.md) for full setup and usage documentation.

### CSM Webhooks (`operations/csm-webhooks/`)

Public endpoints that receive inbound webhooks from external systems and forward them to entity-service. One folder per source, each its own deployable component (currently `github/`, receiving GitHub issue and issue-comment webhooks).

See the [CSM Webhooks README](./operations/csm-webhooks/README.md) for full setup and usage documentation.

## GitHub Actions

### Auto Label and Assign PR

When a pull request is opened, the workflow automatically:

- Assigns the PR author as the assignee
- Applies labels based on changed file paths:

| Files changed under… | Label applied |
|---|---|
| `apps/csm-portal/**` | `App/CSM Portal` |
| `apps/customer-portal/**` | `App/Customer Portal` |
| `apps/*/backend/**` or `apps/*/backend-v2/**` | `Area/Backend` |
| `apps/*/webapp/**` | `Area/Frontend`, `Platform/Web` |
| `apps/*/microapp/**` | `Platform/Microapp` |
| `entity-service/**` | `Entity Service` |
| `integrations/csm-integration-service/**` | `Int/CSM Integration Service` |
| `integrations/csm-notification-service/**` | `Int/CSM Notification Service` |

- Applies one `Type/*` label (`Type/Bug`, `Type/New Feature`, `Type/Improvement`, `Type/Docs`, `Type/UX`, `Type/Task`) inferred from the PR title's conventional-commit prefix (`fix:`, `feat:`, `perf:`/`improve:`, `docs:`, `ux:`, `chore:`/`ci:`/`refactor:`/`build:`/`test:`), after stripping any leading `[TAG]` prefixes.

## Reporting Issues

### 1. Opening an issue

Please use this repository's issue tracker and include reproduction steps, expected behavior, actual behavior, and relevant logs.

### 2. Reporting security issues

Please do not report security issues through public issues. Follow the [WSO2 Security Vulnerability Reporting Guidelines](https://security.docs.wso2.com/en/latest/security-reporting/vulnerability-reporting-guidelines/).

## Contributing

Contributions are welcome. Create a feature branch, keep changes focused, and submit a pull request with a clear description and verification steps.

## License

cs-tools is licensed under Apache 2.0. See the [LICENSE](LICENSE) file for details.
