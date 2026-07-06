# cs-tools

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![GitHub last commit](https://img.shields.io/github/last-commit/wso2-open-operations/cs-tools/main)](https://github.com/wso2-open-operations/cs-tools/commits/main)
[![GitHub issues](https://img.shields.io/github/issues/wso2-open-operations/cs-tools.svg)](https://github.com/wso2-open-operations/cs-tools/issues)

Open-source tools built by WSO2 for customer success operations. This repository contains web applications and backend integrations that enable teams to manage customer-facing workflows, project visibility, and support experiences.

## Repository Structure

```
cs-tools/
├── apps/
│   ├── csm-portal/          # CSM Portal (Go backend + React webapp)
│   └── customer-portal/     # Customer Portal (Ballerina backend + React webapp + React microapp)
├── entity-service/          # Shared entity service
└── integrations/
    └── customer-service/    # Customer operations related integration Ballerina service
```

## Components

### CSM Portal (`apps/csm-portal/`)

An internal portal for Customer Success Managers to manage and track customer cases, assignments, and support operations.

| Component | Description |
|-----------|-------------|
| `backend` | Go RESTful service providing CSM Portal APIs |
| `webapp` | Browser-based portal for CSMs (React + TypeScript) |

See the [CSM Portal README](./apps/csm-portal/README.md) for full setup and usage documentation.

### Customer Portal (`apps/customer-portal/`)

An open-source solution for customer success operations built with a modular architecture. It enables teams to manage customer-facing workflows, project visibility, and support experiences through a unified platform.

Delivered through two frontend experiences alongside a shared backend:

| Component | Description |
|-----------|-------------|
| `backend` | [Ballerina](https://ballerina.io/) RESTful service providing Customer Portal APIs and integration points |
| `webapp` | Publicly available browser-based customer portal (React + TypeScript) |
| `microapp` | Microapp experience inside the [WSO2 super app](https://github.com/opensuperapp/opensuperapp/tree/v1) (React + TypeScript) |

See the [Customer Portal README](./apps/customer-portal/README.md) for full setup and usage documentation.

### Entity Service (`entity-service/`)

A Go REST service that manages customer entity data backed by PostgreSQL. Provides APIs for case management with pagination and validation, used by other apps in this repository.

| Layer | Technology |
|-------|------------|
| Language | Go |
| Framework | Gin |
| Database | PostgreSQL 15+ |

See the [Entity Service README](./entity-service/README.md) for full setup and usage documentation.

### Customer Service Integration (`integrations/customer-service/`)

A Ballerina REST service that aggregates and exposes customer data from multiple backend systems. Provides endpoints for contact lookup, deployment search, and deployed product queries. Includes an in-memory cache layer to reduce upstream load.

## GitHub Actions

### Auto Label and Assign PR

When a pull request is opened, the workflow automatically:

- Assigns the PR author as the assignee
- Applies labels based on changed file paths:

| Files changed under… | Label applied |
|---|---|
| `apps/csm-portal/**` | `App/CSM Portal` |
| `apps/customer-portal/**` | `App/Customer Portal` |
| `apps/*/webapp/**` | `Area/Frontend`, `Platform/Web` |
| `apps/*/backend/**` | `Area/Backend` |
| `apps/*/microapp/**` | `Platform/Microapp` |
| `entity-service/**` | `Entity Service` |

## Reporting Issues

### 1. Opening an issue

Please use this repository's issue tracker and include reproduction steps, expected behavior, actual behavior, and relevant logs.

### 2. Reporting security issues

Please do not report security issues through public issues. Follow the [WSO2 Security Vulnerability Reporting Guidelines](https://security.docs.wso2.com/en/latest/security-reporting/vulnerability-reporting-guidelines/).

## Contributing

Contributions are welcome. Create a feature branch, keep changes focused, and submit a pull request with a clear description and verification steps.

## License

cs-tools is licensed under Apache 2.0. See the [LICENSE](LICENSE) file for details.
