# Customer Portal Backend

Ballerina backend service for the Customer Portal application.

## Quick Start

```bash
# from apps/customer-portal/backend
bal run
```

Backend starts at `http://localhost:9090`.

## Overview

- Service base path: `/`
- Default port: `9090`
- Runtime: Ballerina Swan Lake `2201.12.10`
- Main service file: `service.bal`
- Authentication: JWT-based interceptor (`authorization:JwtInterceptor`) applied to all resources

## Prerequisites

- Ballerina `2201.12.10`

## Configuration

Use `config.toml.local` for local configuration values.

Key configuration groups:

- `[customer_portal.entity]`
- `[customer_portal.authorization.authorizedRoles]`
- `[customer_portal.scim]`
- `[customer_portal.updates]`
- `[customer_portal.ai_chat_agent]`
- `[customer_portal.user_management]`

For each integration, configure the corresponding `clientCredentialsOauth2Config` values (`tokenUrl`, `clientId`, `clientSecret`, `scopes`).

## Authentication

All endpoints are protected by `authorization:JwtInterceptor` and require user context from Asgardeo-authenticated tokens.

### Production

For deployed environments (e.g., Choreo), send:

- `Authorization: Bearer <asgardeo_access_token>`

Choreo handles gateway-level authentication and token propagation.

### Local testing

For local backend testing (`bal run`), the interceptor expects the following headers:

- `x-jwt-assertion`: user token to derive user claims (`email`, `userid`, optional `groups`).
- `x-user-id-token`: user token forwarded for downstream API calls.

Use these headers directly when calling the service locally.

Production request example:

```bash
curl -X GET https://<your-choreo-endpoint>/users/me \
	-H "Authorization: Bearer <asgardeo_user_token>" \
    -H "x-user-id-token: <asgardeo_user_token>"
```

Local request example:

```bash
curl -X GET http://localhost:9090/users/me \
	-H "x-jwt-assertion: <asgardeo_user_token>" \
	-H "x-user-id-token: <asgardeo_user_token>"
```

## Backend Modules

- `modules/authorization` - Interceptors, role checks, and auth context extraction.
- `modules/entity` - Core customer integration operations.
- `modules/scim` - SCIM-based user profile and phone operations.
- `modules/updates` - Update level and recommendation-related operations.
- `modules/ai_chat_agent` - AI-backed classification and conversation helper operations.
- `modules/user_management` - Project contact and user management flows.
- `modules/registry` - Registry token lifecycle and project integration-user operations.
- `modules/types` - Shared public API record and type definitions.

## Run Locally

```bash
bal run
```

Backend starts at `http://localhost:9090`.

### Local (`bal run`) examples

```bash
curl -X POST http://localhost:9090/projects/search \
	-H "Content-Type: application/json" \
	-H "x-jwt-assertion: <jwt_assertion_token>" \
	-H "x-user-id-token: <asgardeo_user_token>" \
	-d '{"filters":{},"pagination":{"limit":10,"offset":0}}'
```

```bash
curl -X GET http://localhost:9090/projects/<project-id>/stats \
	-H "x-jwt-assertion: <jwt_assertion_token>" \
	-H "x-user-id-token: <asgardeo_user_token>"
```

```bash
curl -X GET "http://localhost:9090/cases/<case-id>/comments?limit=10&offset=0" \
	-H "x-jwt-assertion: <jwt_assertion_token>" \
	-H "x-user-id-token: <asgardeo_user_token>"
```

## API Endpoints

The following endpoints are defined in `service.bal`.

### User

- `GET /users/me` - Get logged-in user profile.
- `PATCH /users/me` - Update logged-in user profile.

### Projects

- `POST /projects/search` - Search projects.
- `GET /projects/{id}` - Get project details.
- `PATCH /projects/{id}` - Update project.
- `GET /projects/{id}/filters` - Get case filter metadata for a project.
- `GET /projects/{id}/integration-users` - Get project integration users.

### Project Stats

- `GET /projects/{id}/stats` - Get overall project stats.
- `GET /projects/{id}/stats/cases` - Get project case stats.
- `GET /projects/{id}/stats/conversations` - Get project conversation stats.
- `GET /projects/{id}/stats/support` - Get project support stats.
- `GET /projects/{id}/stats/time-cards` - Get time-card summary stats.
- `GET /projects/{id}/stats/change-requests` - Get change-request summary stats.

### Deployments

- `GET /projects/{id}/deployments` - List project deployments.
- `POST /projects/{id}/deployments` - Create deployment.
- `PATCH /projects/{projectId}/deployments/{deploymentId}` - Update deployment.

### Deployment Attachments

- `GET /deployments/{id}/attachments` - List deployment attachments.
- `POST /deployments/{id}/attachments` - Create deployment attachment.
- `PATCH /deployments/{deploymentId}/attachments/{attachmentId}` - Update deployment attachment metadata.

### Deployment Products and Catalogs

- `GET /deployments/{id}/products` - List products linked to a deployment.
- `POST /deployments/{id}/products` - Link/create product for deployment.
- `PATCH /deployments/{deploymentId}/products/{productId}` - Update deployment product.
- `POST /deployments/products/{id}/catalogs/search` - Search product catalogs for deployment product context.
- `GET /catalogs/{catalogId}/items/{itemId}` - Get catalog item details.

### Cases

- `GET /cases/{id}` - Get case details.
- `POST /cases` - Create case.
- `PATCH /cases/{id}` - Update case.
- `POST /projects/{id}/cases/search` - Search cases for a project.
- `POST /cases/classify` - Classify case content using AI chat agent.

### Case Comments and Attachments

- `GET /cases/{id}/comments` - List case comments.
- `POST /cases/{id}/comments` - Add case comment.
- `GET /cases/{id}/attachments` - List case attachments.
- `POST /cases/{id}/attachments` - Add case attachment.
- `PATCH /cases/{caseId}/attachments/{attachmentId}` - Update case attachment metadata.
- `DELETE /attachments/{id}` - Delete attachment.

### Conversations

- `POST /projects/{id}/conversations/search` - Search conversations for a project.
- `POST /projects/{id}/conversations` - Create conversation.
- `POST /projects/{projectId}/conversations/{conversationId}/messages` - Create conversation message.
- `GET /conversations/{id}` - Get conversation details.
- `GET /conversations/{id}/messages` - List conversation messages.

### Contacts

- `GET /projects/{id}/contacts` - List project contacts.
- `POST /projects/{id}/contacts` - Create project contact.
- `DELETE /projects/{id}/contacts/{email}` - Delete project contact.
- `PATCH /projects/{id}/contacts/{email}` - Update project contact.
- `POST /projects/{id}/contacts/validate` - Validate project contact.

### Call Requests

- `POST /cases/{id}/call-requests/search` - Search case call requests.
- `POST /cases/{id}/call-requests` - Create case call request.
- `PATCH /cases/{caseId}/call-requests/{callRequestId}` - Update case call request.

### Time Cards

- `POST /projects/{id}/time-cards/search` - Search project time cards.

### Change Requests

- `POST /projects/{id}/change-requests/search` - Search project change requests.
- `GET /change-requests/{id}` - Get change request details.
- `PATCH /change-requests/{id}` - Update change request.
- `GET /change-requests/{id}/comments` - List change request comments.

### Registry Tokens

- `POST /projects/{id}/registry-tokens` - Create registry token.
- `POST /projects/{id}/registry-tokens/search` - Search registry tokens for project.
- `DELETE /registry-tokens/{id}` - Delete registry token.
- `POST /registry-tokens/{id}/regenerate` - Regenerate registry token secret.

### Updates and Vulnerabilities

- `GET /updates/recommended-update-levels` - Get recommended update levels.
- `POST /updates/levels/search` - Search update levels.
- `GET /updates/product-update-levels` - Get product update levels.
- `GET /products` - List products.
- `POST /products/vulnerabilities/search` - Search vulnerabilities.
- `GET /products/vulnerabilities/{id}` - Get vulnerability by ID.
- `GET /products/vulnerabilities/meta` - Get vulnerability metadata.

## Notes

- Pagination parameters such as `limit` and `offset` are supported in several list endpoints.
- Some stats endpoints accept `caseTypes` query parameters.
- API behavior and payload types are defined in `service.bal` and module type definitions under `modules/`.
