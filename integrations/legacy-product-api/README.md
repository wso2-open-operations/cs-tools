# Legacy Product API

A small, read-only Go HTTP service that serves the contract of the retired
`wso2-product-service` (the "WSO2 Product API", a Spring Boot service on its own
MySQL database) from the CSM product catalogue. The catalogue is edited in the CSM
Portal and stored by entity-service; this service reads it through entity-service's
products API and reshapes it into the legacy JSON, so the old consumers move by
changing only the base URL.

- [Architecture](#architecture)
- [Developer guide](#developer-guide)
- [Configuration](#configuration)
- [API endpoints](#api-endpoints)

## Architecture

### Consumers

| Consumer | Endpoints read |
|---|---|
| wso2.com Support Matrix and Release Matrix (Drupal module `wso2_marketing_services`) | `/products/support-matrix`, `/products/release-matrix`, `/carbon-platform-history`, `/carbon-kernel-releases` |
| security-advisory-automations | `/products`, `/productNameForAbbreviation` |
| UMT | not confirmed, so every legacy read is kept |

### Interactions

```
consumer ──(Choreo gateway: subscription + client credentials)──▶ legacy-product-api
legacy-product-api ──(OAuth2 client credentials, M2M)──▶ entity-service
    POST /product-versions/search   (feed support-matrix / release-matrix, or every live row)
    GET  /release-lines?includeRetired=true
    GET  /carbon-platform-history
    GET  /carbon-kernel-releases
```

entity-service must accept this service's client as an internal M2M caller
(`M2M_CLIENT_IDS`), since the catalogue reads are internal-only.

### Contract

Version 1 is the legacy contract, kept exactly: the same paths at the root, field
names and key order, `yyyy-M-d` dates without zero padding, Title-case support
status, one row per public listing with a `deploymentProfile` list. Writes answer
405. An upstream failure answers 500, or 503 when entity-service is unavailable,
never the upstream 401/403/404/400. The deliberate differences from the retired
service are documented in `internal/handler/product_api.go`. Contract changes go
into a new version, not v1.

### Authentication model

No inbound auth in the app: the Choreo API Manager gateway owns it (subscription +
client credentials). Outbound calls to entity-service use the OAuth2 client
credentials grant.

### Project structure

```
cmd/server/            entry point, config, routing, graceful shutdown
internal/entity/       entity-service client (OAuth2 client credentials, paging)
internal/handler/      the legacy endpoints and the mapping to legacy JSON
internal/middleware/   correlation ID ("lpa-" prefix), request logging, security headers
internal/apierror/     typed upstream error
openapi.yaml           the contract (legacy paths at the root)
.choreo/component.yaml the Choreo endpoint (legacy-product-api, basePath /)
```

## Developer guide

### Build and test

```bash
# from integrations/legacy-product-api
go vet ./...
go test -race ./...
go build -o server ./cmd/server
# or: make build   (vet + race tests + compile)
```

### Run locally

With the repo's docker-compose stack:

```bash
docker compose up -d legacy-product-api
curl http://127.0.0.1:8087/products/support-matrix
```

Or directly, with a `.env` (see `.env.example`):

```bash
go run ./cmd/server
```

### Lint and security scan

```bash
go vet ./...
go install github.com/securego/gosec/v2/cmd/gosec@latest
gosec -fmt=text ./...
```

## Configuration

| Variable | Required | Description |
|---|---|---|
| `ENTITY_BASE_URL` | yes | entity-service base URL |
| `ENTITY_TOKEN_URL` | yes | OAuth2 token endpoint |
| `ENTITY_CLIENT_ID` | yes | client-credentials client id (an internal M2M caller in entity-service) |
| `ENTITY_CLIENT_SECRET` | yes | client-credentials secret |
| `ENTITY_SCOPES` | no | comma-separated scopes |
| `PORT` | no | listen port, default `8080` |

## API endpoints

All `GET`, all at the root. See `openapi.yaml` for the shapes.

| Path | Returns |
|---|---|
| `/products?inclusion=&exclusion=` | every live listing (ProductDto) |
| `/products/support-matrix` | listings shown on the Support Matrix |
| `/products/release-matrix` | public listings |
| `/products/{id}` | one listing |
| `/searchProduct?key=` | listings whose name contains `key`, id order |
| `/getAllProductVersion?product=` | versions of one product |
| `/productNameForAbbreviation?abbrList=` | abbreviation → product name |
| `/deploymentProfile?id=` | a listing's deployment profiles |
| `/release-line` | live release lines |
| `/product-group` | `[]` (product groups were not migrated) |
| `/carbon-platform-history[...]` | Carbon platform history (`/{id}`, `/versions`, `/version/{version}`, `/latest`) |
| `/carbon-kernel-releases[...]` | Carbon kernel releases (`/{id}`, `/platforms`, `/platform/{platform}`, `/latest-by-platform`) |
| `/health` | 200 |

`POST /products`, `PUT`/`DELETE /products/{id}`, `POST /release-line` and
`POST /product-group` answer 405: the catalogue is managed in the CSM Portal.
