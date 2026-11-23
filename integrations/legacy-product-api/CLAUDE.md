# Legacy Product API

Go HTTP server (`net/http`, Go 1.26+), read-only. It serves the contract of the
retired wso2-product-service (v1) from the CSM product catalogue, which it reads
from entity-service's products API, and reshapes it into the legacy JSON. See
README.md for the consumers and endpoints.

## The contract is frozen

The output is what the old consumers parse: the same paths at the root, key names
and order (encoded from struct field order; Jackson wrote them alphabetically),
`yyyy-M-d` dates without zero padding, Title-case support status, one row per
public listing (`publicId`) with a `deploymentProfile` list, no HTML escaping.
Keep it byte-identical. A contract change is a new version, not an edit to v1.
The deliberate differences from the retired service are listed in the file
comment of `internal/handler/product_api.go`; keep that list current.

Upstream shapes differ from the output: entity-service sends RFC 3339 timestamps
and an upper-case `currentSupportStatus`. `normalizeVersions` /
`normalizeReleaseLines` turn them into date-only, lower-case values first; the
mapping works on those.

## No inbound auth

The Choreo API Manager gateway owns the inbound trust boundary (subscription +
client credentials). Do not add JWT validation here without confirming that has
changed.

## Errors

An entity-service failure answers 500, or 503 when it is unavailable (502/503/504),
with a fixed message; never the upstream 401/403/404/400, as the retired service
only ever failed on its own database (`legacyUpstreamError`). Path-parameter checks
answer 400 with an empty body, and a missing row 404 with an empty body, as legacy
did.

## Upstream

`internal/entity` calls entity-service with OAuth2 client credentials:
`POST /product-versions/search` (paged by 1000 until `hasMore` is false,
`excludeRetired: true`, optional `feed`), `GET /release-lines?includeRetired=`,
`GET /carbon-platform-history`, `GET /carbon-kernel-releases`. The correlation ID
(`lpa-` prefix) is forwarded as `X-CSM-Correlation-ID`.

## Routes

Registered by `ProductAPIHandler.RegisterRoutes`, not one by one in `main.go`, so
the tests exercise the real patterns: literal segments (`support-matrix`,
`versions`, `latest`, `platforms`, `latest-by-platform`) must win over the `{id}`
wildcard beside them.

## Commands

```bash
go vet ./...
go test -race ./...
go build -o server ./cmd/server
```

## OpenAPI spec

`openapi.yaml` is the contract, legacy paths at the root, and the schema of the
Choreo endpoint (`.choreo/component.yaml`). Update it with any change.

## Security

- Never commit secrets; configuration comes from environment variables.
- Log only IDs and error summaries (`summarizeErr` never logs an upstream body).
- Describe security-related changes in neutral functional terms in PRs.
