# CSM Portal

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](../../LICENSE)
[![GitHub last commit](https://img.shields.io/github/last-commit/wso2-open-operations/cs-tools/main?path=apps%2Fcsm-portal)](https://github.com/wso2-open-operations/cs-tools/commits/main/?path=apps/csm-portal)
[![GitHub issues](https://img.shields.io/github/issues/wso2-open-operations/cs-tools.svg)](https://github.com/wso2-open-operations/cs-tools/issues)

CSM Portal is an open-source solution for customer success management operations, built with a modular architecture that combines a Go backend-for-frontend (BFF) and a React web application. It enables customer success managers to handle cases, projects, accounts, deployments, time tracking, and security operations through a unified platform.

## Why CSM Portal?

Customer success teams need a single workspace to track support cases, customer projects, deployments, product updates, and security vulnerabilities across accounts. Managing these in disparate systems creates disconnected workflows and duplicated effort.

CSM Portal addresses this by combining:

- A centralized Go BFF that validates identity and proxies authenticated requests to upstream services,
- A React SPA for CSM workflows including case management, project tracking, time cards, and more.

This setup lets teams deliver consistent, efficient customer success operations without rebuilding shared infrastructure.

## CSM Portal Features

- **Modular Application Structure**
  Backend and webapp are developed as independent units while remaining part of one cohesive product.
- **Centralized BFF Layer**
  The Go backend validates JWTs, applies security headers, and proxies requests to entity, SCIM, and updates upstream services.
- **Rich Case Management**
  Create, search, patch, and comment on support cases, with attachment uploads and call request support.
- **Project & Deployment Tracking**
  View and manage customer projects, deployments, and deployed products across accounts.
- **Time Cards & Operations**
  Track time spent per case and manage customer success operations including change requests.
- **Security Center**
  Search and view product vulnerability disclosures per account.
- **Product Updates**
  Browse product update levels and upgrade paths between versions.
- **Identity Integration**
  Authentication is integrated with OIDC-compatible identity providers for secure access control.

## Project Structure

```bash
.
├── backend                  # Go BFF REST API service
│   └── README.md            # Detailed backend documentation
├── webapp                   # React + TypeScript SPA for CSMs
│   └── README.md            # Detailed webapp documentation
└── README.md                # You're here
```

## Technologies Used

### Backend

- **Language**: [Go](https://go.dev/) 1.26+
- **Authentication**: JWT validation via JWKS endpoint (OIDC-compatible)
- **Upstream Clients**: OAuth2 client credentials for entity, SCIM, and updates services

### Frontend

- **Framework**: React 19 + TypeScript (Vite)
- **UI**: WSO2 Oxygen UI
- **Data Layer**: TanStack Query
- **Authentication**: OIDC React SDK

## Getting Started

### Prerequisites

- Go 1.26+
- Node.js 20+ (LTS recommended)
- `pnpm` 9+

### Backend Setup Guide

- [Backend](./backend/README.md)

### Frontend Setup Guide

- [Webapp](./webapp/README.md)

## Reporting Issues

### 1. Opening an issue

Please use this repository's issue tracker and include reproduction steps, expected behavior, actual behavior, and relevant logs.

### 2. Reporting security issues

Please do not report security issues through public issues. Follow the [WSO2 Security Vulnerability Reporting Guidelines](https://security.docs.wso2.com/en/latest/security-reporting/vulnerability-reporting-guidelines/).

## Contributing

Contributions are welcome. Create a feature branch, keep changes focused, and submit a pull request with a clear description and verification steps.

## License

CSM Portal is licensed under Apache 2.0. See the [LICENSE](../../LICENSE) file for details.
