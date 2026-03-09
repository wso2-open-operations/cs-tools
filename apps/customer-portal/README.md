# Customer Portal

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](../../LICENSE)
[![GitHub last commit](https://img.shields.io/github/last-commit/wso2-open-operations/cs-tools/customer-portal-milestone-1)](https://github.com/wso2-open-operations/cs-tools/commits/customer-portal-milestone-1)
[![GitHub issues](https://img.shields.io/github/issues/wso2-open-operations/cs-tools.svg)](https://github.com/wso2-open-operations/cs-tools/issues)

Customer Portal is an open-source solution for customer success operations, built with a modular architecture that combines a Ballerina backend and multiple React frontends. It enables teams to manage customer-facing workflows, project visibility, and support experiences through a unified platform.

The portal is delivered through two frontend experiences:

- `webapp`: Publicly available browser-based customer portal
- `microapp`: Micro app experience inside the WSO2 mobile app

## Why Customer Portal?

Customer organizations often need a single workspace to track customer projects, support activity, updates, and user operations. Building these as separate apps creates disconnected workflows and duplicated integration effort.

Customer Portal addresses this by combining:

- A centralized backend service for business APIs and integrations,
- A publicly available web application for end-user workflows,
- A microapp experience in the WSO2 mobile app.

This setup helps teams deliver consistent, scalable customer experiences without rebuilding shared infrastructure.

## Customer Portal Features

- **Modular Application Structure**
  Backend, webapp, and microapp are developed as independent units while remaining part of one cohesive product.
- **Centralized API Layer**
  The Ballerina backend provides customer portal APIs and integration points for external systems.
- **Dual Frontend Delivery**
  The webapp targets public browser access, while the microapp targets in-app usage within the WSO2 mobile app.
- **Identity Integration**
  Authentication is integrated with Asgardeo/OIDC-compatible identity providers for secure access control.

## Project Structure

```bash
.
├── backend                  # Ballerina backend service
├── webapp                   # Public React + TypeScript web application
│   └── README.md            # Detailed webapp documentation
├── microapp                 # WSO2 mobile app embedded React microapp
│   └── README.md            # Detailed microapp documentation
└── README.md                # You're here
```

## Technologies Used

### Backend

- **Language**: [Ballerina](https://ballerina.io/)
- **Runtime**: Ballerina Swan Lake `2201.12.10`
- **Authentication**: JWT and OIDC-based integration with Asgardeo-compatible flows

### Frontend

- **Framework**: React + TypeScript (Vite)
- **UI**: WSO2 Oxygen UI
- **Data Layer**: TanStack Query
- **Authentication**: Asgardeo React SDK

## Getting Started

### Prerequisites

- Ballerina `2201.12.10`
- Node.js 20+ (LTS recommended)
- `pnpm` 9+

### Backend Setup Guides

- [Backend](./backend/README.md)

### Frontend Setup Guides

- [Webapp](./webapp/README.md)
- [Microapp](./microapp/README.md)

## Reporting Issues

### 1. Opening an issue

Please use this repository's issue tracker and include reproduction steps, expected behavior, actual behavior, and relevant logs.

### 2. Reporting security issues

Please do not report security issues through public issues. Follow the [WSO2 Security Vulnerability Reporting Guidelines](https://security.docs.wso2.com/en/latest/security-reporting/vulnerability-reporting-guidelines/).

## Contributing

Contributions are welcome. Create a feature branch, keep changes focused, and submit a pull request with a clear description and verification steps.

## License

Customer Portal is licensed under Apache 2.0. See the [LICENSE](../../LICENSE) file for details.
