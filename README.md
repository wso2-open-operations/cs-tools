# cs-tools

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![GitHub last commit](https://img.shields.io/github/last-commit/wso2-open-operations/cs-tools/main)](https://github.com/wso2-open-operations/cs-tools/commits/main)
[![GitHub issues](https://img.shields.io/github/issues/wso2-open-operations/cs-tools.svg)](https://github.com/wso2-open-operations/cs-tools/issues)

Open-source tools built by WSO2 for customer success operations. This repository contains web applications and backend integrations that enable teams to manage customer-facing workflows, project visibility, and support experiences.

## Repository Structure

```
cs-tools/
├── apps/
│   └── customer-portal/     # Customer Portal (Ballerina backend + React webapp + React microapp)
└── integrations/
    └── customer-service/    # Customer operations related integration Ballerina service
```

## Components

### Customer Portal (`apps/customer-portal/`)

An open-source solution for customer success operations built with a modular architecture. It enables teams to manage customer-facing workflows, project visibility, and support experiences through a unified platform.

Delivered through two frontend experiences alongside a shared backend:

| Component | Description |
|-----------|-------------|
| `backend` | [Ballerina](https://ballerina.io/) RESTful service providing Customer Portal APIs and integration points |
| `webapp` | Publicly available browser-based customer portal (React + TypeScript) |
| `microapp` | Microapp experience inside the [WSO2 super app](https://github.com/opensuperapp/opensuperapp/tree/v1) (React + TypeScript) |

See the [Customer Portal README](./apps/customer-portal/README.md) for full setup and usage documentation.

### Customer Service Integration (`integrations/customer-service/`)

A Ballerina REST service that aggregates and exposes customer data from multiple backend systems. Provides endpoints for contact lookup, deployment search, and deployed product queries. Includes an in-memory cache layer to reduce upstream load.

## Reporting Issues

### 1. Opening an issue

Please use this repository's issue tracker and include reproduction steps, expected behavior, actual behavior, and relevant logs.

### 2. Reporting security issues

Please do not report security issues through public issues. Follow the [WSO2 Security Vulnerability Reporting Guidelines](https://security.docs.wso2.com/en/latest/security-reporting/vulnerability-reporting-guidelines/).

## Contributing

Contributions are welcome. Create a feature branch, keep changes focused, and submit a pull request with a clear description and verification steps.

## License

cs-tools is licensed under Apache 2.0. See the [LICENSE](LICENSE) file for details.
