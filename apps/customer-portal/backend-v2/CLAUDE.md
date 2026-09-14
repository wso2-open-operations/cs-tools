# Customer Portal Backend (v2)

Go HTTP server (`net/http`, Go 1.26+) that acts as a backend-for-frontend (BFF) for the customer
portal. It authenticates callers, forwards requests to `cs-tools/entity-service`, and shapes
responses for the frontend. This is a rewrite of the existing backend at
`apps/customer-portal/backend`, modeled on `apps/csm-portal/backend`'s conventions — read that
backend's own CLAUDE.md too if something here is underspecified.

**Status: in progress.** 104 routes are wired up so far, across seven upstream services:
entity-service, the WSO2 Updates service, SCIM, the AI chat agent, the product-consumption
service, the registry (robot-account) service, and the project-contact onboarding service (see
"The AI chat agent", "The product-consumption service", "The registry service", and "The
project-contact onboarding service" below — none of the last four is entity-service-backed at
all). Route list: `GET /health`, `GET`/`PATCH /users/me`,
`GET /accounts/{id}`, `POST /projects/search`, `GET /projects/{id}`,
`POST /projects/{id}/cases/search`,
`GET /cases/{id}`, `POST /cases`, `PATCH /cases/{id}`, `POST /cases/{id}/comments`,
`POST /cases/{id}/activities/search`, `POST /cases/{id}/attachments`,
`POST /projects/{id}/deployments/search`, `POST /projects/{id}/deployments`,
`PATCH /projects/{projectId}/deployments/{id}`,
`POST /deployments/{deploymentId}/products/search`, `POST /deployments/{deploymentId}/products`,
`PATCH /deployments/{deploymentId}/products/{id}`, `POST /attachments`,
`GET /attachments/{id}/content`, `DELETE /attachments/{id}`, `GET /products`,
`POST /products/search`,
`POST /products/{id}/versions/search`, `POST /products/vulnerabilities/search`,
`GET /products/vulnerabilities/{id}`,
`POST /deployments/products/{deployedProductId}/catalogs/search`,
`GET /catalogs/{catalogId}/items/{itemId}`, `POST /projects/{id}/time-cards/search`,
`POST /comments`, `POST /change-requests`,
`POST /projects/{id}/change-requests/search`, `GET /change-requests/{id}`, `PATCH /change-requests/{id}`,
`GET /change-requests/{id}/approvals`, `POST /change-requests/{id}/approvals/decision`,
`POST /cases/{caseId}/call-requests`, `POST /cases/{caseId}/call-requests/search`,
`PATCH /cases/{caseId}/call-requests/{id}`,
`POST /cases/classify`, `POST /conversations/recommendations/search`,
`POST /projects/{id}/conversations/search`, `POST /projects/{id}/conversations`,
`GET /conversations/{id}`, `PATCH /conversations/{id}`, `GET /conversations/{id}/messages`,
`POST /projects/{projectId}/conversations/{conversationId}/messages`,
`GET /projects/{id}/conversations/{conversationId}/summary`, `GET /ws`,
`POST /projects/{projectId}/deployments/{deploymentId}/license`, `POST /deployment-usages`,
`GET /projects/{id}/filters`, `GET /projects/{id}/features`, `GET /projects/{id}/stats`,
`GET /projects/{id}/stats/cases`, `GET /projects/{id}/stats/conversations`,
`GET /projects/{id}/stats/support`, `GET /projects/{id}/stats/time-cards`,
`GET /projects/{id}/stats/change-requests`, `GET /projects/{id}/stats/usage`,
`POST /projects/{id}/cases/time-cards/search`,
`GET /metadata`, `POST /search`, `GET /products/vulnerabilities/meta`,
`GET /cases/{id}/feedback`, `POST /cases/{id}/feedback`, `GET /cases/{id}/attachments`,
`GET /attachments/{id}`,
`GET`/`POST /deployments/{deploymentId}/attachments`,
`PATCH /deployments/{deploymentId}/attachments/{attachmentId}`,
`PATCH /cases/{caseId}/attachments/{attachmentId}`,
`POST /deployments/{deploymentId}/products/{productId}/metrics/search`,
`POST /deployments/{deploymentId}/products/{productId}/metrics/usage-counts/search`,
`POST /cases/{caseId}/escalations`, `POST /cases/{caseId}/escalations/search`, the 15-route
instances fan-out (see "Instances — fan-out, not passed through" below),
`POST /projects/{id}/registry-tokens`, `POST /projects/{id}/registry-tokens/search`,
`DELETE /registry-tokens/{id}`, `POST /registry-tokens/{id}/regenerate`,
`GET /projects/{id}/integration-users`, `GET /projects/{id}/contacts`,
`POST /projects/{id}/contacts`, `DELETE /projects/{id}/contacts/{email}`,
`PATCH /projects/{id}/contacts/{email}`, `POST /projects/{id}/contacts/validate`,
`GET /updates/product-update-levels`, `POST /updates/levels/search`.

The existing backend exposes a handful more routes still not ported here: escalations/incidents/
problems/task SLAs/tasks are already partly covered above where they exist on `cs-tools/entity-service`;
what remains unported is mostly generic user search, project update, groups/service-offerings/
configuration-items, and anything genuinely lacking a `cs-tools/entity-service` equivalent (see
"Which entity-service" below on how to tell before porting one). Follow the recipe below to add the
next one.

## Which entity-service

This backend targets **`cs-tools/entity-service`** (this repo, `../../../entity-service`), *not*
the `digiops-cs/entity-service` that `apps/customer-portal/backend` (the original implementation)
calls. The two are different services with overlapping but not identical APIs — before porting an
endpoint, verify it actually exists on `cs-tools/entity-service` (check
`entity-service/internal/server/routes.go` and `entity-service/openapi.yaml`), and note that many
of its routes are **ServiceNow-only** (registered only when the service runs with
`DATA_SOURCE=servicenow`; see `entity-service/internal/config/config.go`) — a Postgres-mode
deployment will 404 on those. If the existing backend has an endpoint with no `cs-tools/entity-service`
equivalent at all (e.g. `GET /metadata` — entity-service has no metadata endpoint), do not invent
one; add a code comment at the call site noting the gap and flag it instead of fabricating a
response.

**Existing on `cs-tools/entity-service` is necessary but not sufficient — also confirm the
existing backend actually exposes it as a customer-portal feature.** `cs-tools/entity-service`
implements plenty of routes this backend should *not* port: some are genuinely customer-facing but
belong to a different portal (e.g. `POST /cases/{id}/github-issues` — filing an engineering bug
against an internal repo is a support-agent action with zero precedent anywhere in the existing
customer-portal backend), and some read like customer features but the existing backend actually
serves the equivalent from an entirely different, non-`cs-tools` microservice (e.g.
`POST /accounts/{id}/contacts/search` / `POST /projects/{id}/contacts/search` look like read
analogues of the existing backend's project-contact endpoints, but those are actually backed by
the separate `user_management` module/microservice, not entity-service at all — porting the
`cs-tools/entity-service` version would expose a different, unrelated dataset under a
similar-looking URL). Before implementing anything new, grep
`apps/customer-portal/backend/modules/entity/entity.bal` (or the relevant sibling module) for the
function that would call it, and check what it actually resolves to — a route only earns
a place in this backend once both checks pass.

## Other upstream services

Not everything comes from entity-service. Two more upstream service clients exist, each following
the same `Config{BaseURL, TokenURL, ClientID, ClientSecret, Scopes}` + `Client` + `NewClient` +
private `do()` shape as `internal/entity`:

- **`internal/updates`** — the WSO2 Updates service (product update levels, update descriptions
  between levels). Its own types are already portal-shaped camelCase (`internal/updates/types.go`
  defines both the upstream snake_case wire structs and the portal camelCase structs, translated by
  `internal/updates/mapper.go`) — so handlers write its return values directly via `writeJSONValue`
  with **no** further `internal/dto` mapping layer. This is the one deliberate exception to the
  "always map through dto" rule below, because the mapping already happened inside the client.
- **`internal/scim`** — the SCIM operations service, used only for a user's phone number
  (`SearchUser`, `UpdateUserPhone`). Its `UserInfo` return type is already a small portal-clean
  struct, so it's merged directly into `dto.UserMeResponse`/`dto.UserUpdateResponse` in
  `internal/handler/users.go` — again no separate mapping layer needed.

All seven service clients (entity, updates, SCIM, the AI chat agent, the product-consumption
service, the registry service, the project-contact onboarding service) authenticate as the same
shared OAuth2 client-credentials app in `cmd/server/main.go` — only each service's
`*_BASE_URL`/`*_SCOPES` env vars differ.

## The AI chat agent

`internal/aichatagent` is a fourth upstream client, but unlike entity/updates/SCIM it talks to a
**separate Python service that has no relationship to `cs-tools/entity-service` at all**. It has
its own HTTP API (`internal/aichatagent/client.go`) and a distinct WebSocket endpoint
(`internal/aichatagent/ws.go`, using `github.com/gorilla/websocket` — the one third-party
dependency in this otherwise-stdlib-only backend, since `net/http` has no server-side WebSocket
support).

`internal/handler/ai_chat.go` (case classification, KB recommendations, conversation
create/get/update/search, conversation messages, conversation summary) and
`internal/handler/websocket.go` (the real-time chat proxy) both mix calls to the AI agent with
calls to entity-service's conversation/comment routes by design: a conversation thread lives in
entity-service; the AI agent only handles the live message exchange. `entity-service` now has
`createConversation`/`getConversation`/`updateConversation` (see
`internal/entity/conversations.go`), so `POST /projects/{id}/conversations`
(`AIChatHandler.CreateConversation`) implements the full composite flow: create the conversation →
call the AI agent → optionally fetch KB recommendations (only when both `region` and `tier` are
supplied on the request) → persist the AI's reply as a comment → auto-resolve the conversation if
the agent reports `resolved: true`. Conversation creation, the AI call, and comment persistence are
fatal (mapped via `mapUpstreamError`) with **no compensating rollback** of earlier steps if a later
one fails (a conversation can exist in entity-service even though the client got a 500 from a later
step); recommendations and auto-resolve are best-effort and never fail the request. The same
persist-reply-and-auto-resolve pattern is replicated in `SendConversationMessage` (follow-up
messages — no recommendations call there; KB recommendations are attached only on a conversation's
first message) and in `websocket.go`'s `handleMessage`.

One gap remains, flagged with a doc comment at each call site rather than worked around — do not
build a workaround for this; wait for `entity.CreateCommentRequest` to gain the field:

- **No `createdBy` override on `entity.CreateCommentRequest`** — a distinct "chat agent" identity
  for the AI's own replies isn't available; `cs-tools/entity-service` always attributes a created
  comment to the caller's own authenticated identity. Both the customer's message and the AI's
  reply are therefore saved under the *same* (the caller's) identity here — there is currently no
  way to distinguish them by author alone. See the `agentReplyCreatedByCaveat` doc comment in
  `internal/handler/ai_chat.go`.

`GET /ws?sessionId={projectId}` names its query parameter `sessionId` even though it actually
carries the *project* ID, not a session ID, for wire compatibility with existing clients — the AI
agent's own per-conversation session key is derived as `"{projectId}:{conversationId}"` inside the
handler. This connection still only supports *resuming* an existing conversation — the browser must
supply a `conversationId` in its first message, or the handler returns an `error` event rather than
silently failing; start a brand-new conversation via `POST /projects/{id}/conversations` first,
then resume it over the WebSocket. Go's `http.Server` runs each upgraded connection in its own
goroutine, and the handler's `ReadMessage` → `handleMessage` loop is a single blocking sequence — it
does not read or process the next frame until `handleMessage` returns, and never starts a
concurrent read or upstream stream. So there's no need for an explicit "already streaming"
busy-flag/mutex; the client
can still send another frame at any time, this handler simply won't look at it until the current
one finishes.

**`GET /ws` is the one route that does NOT go through `middleware.Auth`, and it runs on its own
listener and port (`WS_PORT`, default 8081) rather than 8080.** This mirrors
`apps/customer-portal/backend`, which serves REST on 9090 and its WebSocket on 9091 — read that
backend's `ws` upgrade resource in `service.bal` before changing anything here.

The reason is a browser limitation, not a preference: **a browser cannot set custom headers on a
WebSocket handshake**, so the `x-jwt-assertion` header `middleware.Auth` requires can never arrive.
The frontend instead smuggles its tokens through the `Sec-WebSocket-Protocol` header as

```
choreo-oauth2-token, <accessToken>, cs-customer-portal, <userIdToken>
```

(see `WS_CHOREO_OAUTH2_TOKEN`/`WS_CUSTOMER_PORTAL` in the webapp's `constants/apiConstants.ts` and
`features/support/api/useChatWebSocket.ts`). Choreo's gateway consumes the leading
`choreo-oauth2-token, <accessToken>` pair for its own authorization and forwards only the
remainder, so **the token this backend needs is always the last comma-separated value** — which
holds whether or not the gateway is in the path, so a direct local connection works identically.
`handler.userIDTokenFromRequest` implements exactly that, trying the `x-user-id-token` header first
for non-browser callers.

**The token that arrives is NOT the same token the `Auth` middleware validates, and this trips
people up.** `middleware.Auth` validates the `x-jwt-assertion` that Choreo's gateway injects. What
the WebSocket handshake carries is the browser's **Asgardeo-issued ID token** — different issuer
(`https://api.asgardeo.io/t/<org>/oauth2/token`), different audience (the SPA's client ID plus
`choreo:deployment:<env>`), different signing key. Passing it to `TokenValidator.Validate` therefore
fails on issuer/audience/signature every single time, which is exactly how this surfaced: **every
WebSocket connection returned 401**. `HandleWebSocket` calls
`middleware.TokenValidator.DecodeUnverified` instead, which decodes the claims without verifying
signature/issuer/audience, then rebuilds the context (`middleware.WithUserInfo` +
`entity.WithUserIDToken`) that `Auth` would normally have populated.

That is safe **only** because Choreo's API Manager gateway has already authenticated the caller's
access token (the leading `choreo-oauth2-token, <accessToken>` subprotocol pair) before forwarding
the handshake — the same trust boundary described under "Why no Auth middleware". The ID token
*identifies* an already-authenticated caller; it does not authenticate one. The Ballerina backend
does the same thing for the same reason (`authorization:getUserInfoFromTokens` calls plain
`jwt:decode`). **Never use `DecodeUnverified` on a route reachable without the gateway** — it will
accept a forged, unsigned, or expired token. If that assumption ever changes, the fix is to validate
the ID token against Asgardeo's own JWKS/issuer/audience via separate config, not to point
`Validate` at it.

`Upgrader.Subprotocols` must keep listing `cs-customer-portal`: the client offers it, and a browser
aborts the connection if the server selects a subprotocol it never offered. It is not cosmetic.

The second reason the listener is separate: the REST server's `ReadTimeout`/`WriteTimeout` would
cap the lifetime of an upgraded connection. The handler clears those deadlines after upgrading, but
a listener without them is safe by construction. `handler.wsIdleTimeout` bounds the connection
instead, per frame.

`WebSocketHandler`'s `gorilla/websocket.Upgrader.CheckOrigin` is a defense-in-depth hook against
cross-site WebSocket hijacking that could restrict which browser `Origin`s may open the connection,
but `main.go` currently passes it `nil` (allow any origin) — there is no env var for this.

## The product-consumption service

`internal/productconsumption` is a fifth upstream client for **another separate service unrelated
to entity-service** — it actually covers two independently-configurable upstream endpoints, a
subscription/license API and a usage-tracking API. This backend models them as one Go package (one
shared `*Client`, one OAuth2 app) but keeps their base URLs distinct: `Config.SubscriptionBaseURL`
backs the subscription/license API, `Config.TrackingBaseURL` backs the usage-tracking API. They may
happen to point at the same host in some deployments, which is why `TrackingBaseURL` falls back to
`SubscriptionBaseURL` when left unset — but they are independently configurable
(`PRODUCT_CONSUMPTION_SUBSCRIPTION_URL` vs `PRODUCT_CONSUMPTION_TRACKING_BASE_URL`) and not
guaranteed to always match, so don't collapse them into a single field.

It backs two routes:
- `POST /projects/{projectId}/deployments/{deploymentId}/license` — provisions (or resumes
  provisioning) a WSO2 API Manager application/subscription/credentials for a deployment and
  returns the resulting license. `ProcessLicenseDownload` (`internal/productconsumption/subscription.go`)
  implements the upstream state machine — it is **not idempotent-by-accident-safe to reimplement
  casually**: it can make up to 5 sequential upstream calls, several with side effects (creating an
  application, subscribing it, generating credentials), and each step only runs if the project's
  upstream-tracked status hasn't reached it yet. Read the whole function before touching it — a
  subtly wrong condition could create a duplicate WSO2 API Manager application. The handler first
  calls `entity.GetProject` purely as an access-control gate, discarding the result —
  entity-service is still the actual authorization boundary for "does this caller own this
  project." Because up to 5 sequential upstream calls can plausibly exceed the server's global
  `WriteTimeout` (see `cmd/server/main.go`) even when no single step is slow, the handler extends
  its own write deadline via `http.NewResponseController(w).SetWriteDeadline` — which only works
  because `middleware.responseWriter` forwards `SetWriteDeadline`/`SetReadDeadline` to the
  underlying `ResponseWriter`, the same reason it forwards `Hijack` for the WebSocket route above.
- `POST /deployment-usages` — imports a deployment-usage zip file. Unlike every other endpoint in
  this backend, the request body is **raw binary**, not JSON — see `readBinaryBody` in
  `internal/handler/response.go` (a `readJSONBody` counterpart with a larger size cap,
  `maxZipUploadBytes`) and the `Content-Type: application/zip`/`application/x-zip-compressed`
  check in the handler, which together validate the upload the way the upstream service requires.
  The Go client base64-encodes the bytes before forwarding to the upstream service, matching its
  JSON contract exactly (`{"email": "...", "zip": "<base64>"}`).

## The registry service

`internal/registry` is a sixth upstream client for **another separate service unrelated to
entity-service** — a container/robot-account registry (Harbor-style) that issues registry access
tokens ("robot accounts") scoped to a project, and looks up a project's integration (service
account) users.

It backs five routes (`internal/handler/registry.go`, `RegistryHandler`):
- `POST /projects/{id}/registry-tokens` — create a token. Only admins (per `AUTH_ADMIN_ROLE`,
  checked against `entity.GetUserMeResponse.Roles`) may create **service** tokens or tokens
  `createdFor` someone other than themselves; non-admins can only ever create **user** tokens for
  their own email.
- `POST /projects/{id}/registry-tokens/search` — non-admins are forced to `userEmail: <themselves>`
  server-side; admins see every token for the project.
- `DELETE /registry-tokens/{id}` / `POST /registry-tokens/{id}/regenerate` — **authorization here is
  indirect, not path-scoped.** Neither route is nested under `/projects/{id}`; instead,
  `RegistryHandler.authorizeTokenAction` fetches the token, parses its opaque `Description` field
  (`registry.DeriveTokenInfoFromDescription`, format
  `<snAccountId>##<snProjectId>##<TokenType>##<createdFor>##<createdBy>`) to recover which project
  it belongs to, then re-verifies the caller's access to *that* project via `entity.GetProject` —
  the registry service itself has no concept of project membership at all. Only then does it check
  admin/ownership and proceed. Read this function before changing either route; it's the one
  non-obvious authorization pattern in this backend.
- `GET /projects/{id}/integration-users` — a simple project-access-gated passthrough, keyed on the
  project's Salesforce ID (`entity.ProjectDetailsView.SfID`), same as the project-contact routes
  below.

Several of the registry service's own error responses are surfaced to the caller verbatim (not a
generic fallback) — see `writeUpstreamMessage` in `internal/handler/registry.go`, which reads
`apierror.Error.Body` (the upstream's own `message` field, extracted by the client) and maps it to
the matching HTTP status, passing through the upstream's own error text rather than a generic one.
Regular entity-service calls in the same handlers still use the shared `mapUpstreamError`.

## The project-contact onboarding service

`internal/usermanagement` is a seventh upstream client for **yet another separate service** (not
entity-service, not SCIM) that manages a project's customer-side contacts/memberships — add,
remove, list, update role, and validate before onboarding. It is keyed on the project's Salesforce
ID (`entity.ProjectDetailsView.SfID`), same as the registry service's integration-users route
above — not the project's `id` or `key`.

**The core reshaping this package does: the upstream service represents a contact/membership's
roles as a single semicolon-delimited string** (e.g. `"Admin;Lead"`), but this backend exposes them
as four separate booleans (`isCsAdmin`, `isLead`, `isPortalUser`, `isSecurityContact`), matching the
portal's own contract. `internal/usermanagement/types.go`'s unexported
`getRoles`/`hasRole`/`toContact`/`toMembership` do this translation in both directions. When adding
a new field here, translate through these helpers, don't reach for the raw `role` string directly.

It backs five routes (`internal/handler/contacts.go`, `ContactHandler`):
- `GET /projects/{id}/contacts` — list.
- `POST /projects/{id}/contacts` — add; `adminEmail` sent upstream is always the caller's own
  email, never client-supplied (`dto.BuildOnBoardContactPayload`).
- `DELETE /projects/{id}/contacts/{email}` / `PATCH /projects/{id}/contacts/{email}` — remove /
  update role; same `adminEmail`-is-always-the-caller rule.
- `POST /projects/{id}/contacts/validate` — validates a contact before onboarding. The upstream
  service uses the **HTTP status code itself** to distinguish three outcomes (not just success/
  failure), so `usermanagement.Client.ValidateProjectContact` returns a 3-way result
  `(contact *Contact, conflict bool, err error)` rather than the usual `(T, error)`: `201` means a
  deactivated contact with this email already exists (`contact` non-nil), `202` means the contact
  is new and can be onboarded (`contact` nil, `err` nil), `409` means an active contact already
  exists (`conflict` true), anything else is a genuine error. Don't collapse this back into a plain
  2-tuple when touching this code — the three cases produce different HTTP responses to the portal.

Like the registry service, several of this service's error responses are surfaced verbatim via
`writeUpstreamMessage` rather than `mapUpstreamError`'s generic fallback — see
`usermanagement.extractErrorMessage`, which parses the upstream's own `{"message": "..."}` body.

## Caller-scoped project/case search (always enforced, no kill switch)

`POST /projects/search`, `POST /projects/{id}/cases/search`, and `GET /cases/{id}` currently return
results for **any** authenticated caller regardless of which projects they actually belong to — there
is no bulk *email → projects* reverse lookup anywhere in this stack. But entity-service itself has a
native, per-project forward lookup that CSM's backend already calls in production:
`POST /projects/{id}/contacts/search` (`entity.SearchProjectContacts` here — see
`entity-service/internal/handler/project_handler.go`'s `ProjectContactHandler`), whose
`ProjectContact.GrantsCaseAccess` field is a purpose-built answer to "can this email actually see
this project's cases" (ServiceNow's own access rule: a linked contact record *and* the invited
address matching that record's own address). This is a **different, separate** system from the
project-contact *onboarding* service above (`internal/usermanagement`, Salesforce-ID-keyed,
`isPortalUser`/`isCsAdmin`/etc.) — don't confuse the two; an earlier version of this resolver used
that one, but it required an extra `GetProject`→Salesforce-ID hop and a heuristic (`isPortalUser`)
where entity-service already has an authoritative field, keyed on the same platform UUID everything
else here already uses. ServiceNow data source only — no Postgres equivalent for project contacts.

`handler.CallerScopeResolver` (`internal/handler/caller_scope.go`) answers membership one project at
a time: page through `entity.SearchProjectContacts(projectID, ...)` (bounded —
`callerScopeContactsLimit`/`callerScopeContactsMaxPages` — independent of what `Total` reports) and
check for a case-insensitive email match with `GrantsCaseAccess: true`.

**The single reusable gate every handler calls is `requireProjectMember`** (same file) — not just
`CallerScopeResolver` itself. It runs the membership check and writes the caller-supplied
status/message (403 for a search that already names its project in the URL, 404 for a single-item
detail fetch — see below) on failure. Every handler below calls this one function; none duplicates
the check-then-write-error logic itself. There is no env var or flag gating any of this — it's always
enforced in production.

Each handler gets its own `callerScope` field and a `SetCallerScope(resolver)` setter — a setter
rather than a constructor parameter *purely* so the many pre-existing tests across this package that
construct handlers directly, unrelated to this feature, keep compiling without change. `main.go`
calls `SetCallerScope` unconditionally on every one of them with the *same* `CallerScopeResolver`
instance — there's no way to opt a handler out in production. `requireProjectMember` (and
`ProjectHandler.scopeToCallerProjects`) treat a `nil` resolver as unscoped rather than panicking —
that path is only ever taken by tests that never call `SetCallerScope` at all, since they don't
exercise this feature.

Endpoints covered, grouped by how they resolve to a project id:

- **Direct — `{id}`/`{projectId}` in the URL path is already the project's platform UUID**:
  `ProjectHandler.SearchProjects` (post-filters the response instead of gating the request — see
  below), `CaseHandler.SearchCases`, `RegistryHandler.SearchRegistryTokens`,
  `RegistryHandler.CreateRegistryToken`, `RegistryHandler.GetProjectIntegrationUsers`,
  `ChangeRequestHandler.SearchChangeRequests`, `TimeCardHandler.SearchTimeCards`,
  `DeploymentHandler.SearchDeployments`, all 10 `ProjectStatsHandler` endpoints
  (`SearchProjectCaseTimeCards`, `GetProjectFilters`, `GetProjectFeatures`,
  `GetProjectDashboardStats`, `GetProjectCaseStats`, `GetProjectConversationStats`,
  `GetProjectSupportStats`, `GetProjectTimeCardStats`, `GetProjectChangeRequestStats`,
  `GetProjectUsageStats`), `AIChatHandler`'s direct-project endpoints (`SearchConversations`,
  `CreateConversation`, `SendConversationMessage`, `GetConversationSummary`), and `InstanceHandler`'s
  project-scoped fan-out variants (`SearchProjectInstances`/`*Metrics`/`*Usage`/`*MetricsStats`/`*UsageStats` — see below).
- **Resolved via a case, conversation, or token** — the path carries a case/conversation/token id, so the handler
  fetches/derives the resource first and checks `ProjectDetails.ID` / `Project.ID` / `SnProjectID`: `CaseHandler.GetCase`,
  `CaseHandler.SearchCaseActivities`, `CaseHandler.SearchCaseEscalations`,
  `CallRequestHandler.SearchCallRequests`, `AIChatHandler.GetConversation`,
  `AIChatHandler.UpdateConversation`, `AIChatHandler.GetConversationMessages`,
  `RegistryHandler.DeleteRegistryToken`, `RegistryHandler.RegenerateRegistryToken`.

`SearchProjects` is the one exception to "gate the request": it scans entity-service project search
results in 50-item batches (up to 10 pages / 500 projects ceiling), checks `IsProjectMember` for each
project, collects all accessible projects, and applies client-side slice pagination (`Offset` and
`Limit`). `Total` (`totalRecords`) and `HasMore` accurately describe the caller's scoped matching set.

`GetCase`/`SearchCaseActivities`/`SearchCaseEscalations`/`SearchCallRequests`/`GetConversation`/`UpdateConversation`/`GetConversationMessages`
all 404 (not 403) on a non-member — don't confirm to a caller that a resource id exists at all if they
can't see it. Every direct-project endpoint 403s instead, since the URL already names the project.

**`InstanceHandler`'s 15-route fan-out (project/deployment/deployed-product variants of the same 5
metric types) only covers the *project*-scoped variants.** Its 5 shared private methods
(`searchInstances`/`searchInstanceMetrics`/etc.) all take an `instanceIDFilters` struct that's
non-empty in exactly one of three fields; `checkProjectScope` is a no-op unless `scope.projectIDs` is
set, so the deployment- and deployed-product-scoped variants pass through unchecked. Resolving a
deployment or deployed-product id back to a project (needed for those) is a separate, not-yet-addressed
gap — there's no `GetDeployment`/`GetDeployedProduct` single-item fetch in `internal/entity` at all
today, and deployed products only reference their deployment (`DeployedProductView.Deployment`), not
a project directly — a two-hop resolution once deployment resolution exists.

**Endpoints deliberately left out of this pass** (deployment/deployed-product or global resolution gap):
the `deployments/{deploymentId}/products/*` and `deployments/products/{id}/*` fan-outs,
and globally-unscoped endpoints with no project id in their path (`products/vulnerabilities/search`,
global `/search`) — the mechanism here doesn't generalize to those without an account-level (not
project-level) equivalent. (Note: the three unused global endpoints `POST /accounts/search`,
`POST /attachments/search`, and `POST /comments/search` were removed from this backend).

Note that cases already have a *separate*, already-fully-wired "my own cases" mechanism independent
of this flag: the frontend can send `{"filters":{"createdByMe":true}}` to
`POST /projects/{id}/cases/search` today, which entity-service
resolves server-side from the caller's own JWT (`__current_user_email__` placeholder, see
`entity-service/internal/service/case_filters.go`) — that's about filtering to cases the caller
personally *created*, a different, narrower thing than the project-membership check above.

## Instances — fan-out, not passed through

`POST /projects/{id}/instances/*`, `/deployments/{id}/instances/*`, and
`/deployments/products/{id}/instances/*` (15 routes total) all read from just 5 entity-service
endpoints (`SearchInstances`, `SearchInstanceMetrics`, `SearchInstanceUsage`,
`SearchInstanceMetricsStats`, `SearchInstanceUsageStats`). Each is fanned out into three
differently-scoped views (project/deployment/deployed-product) rather than exposed as one generic
filterable endpoint, and `internal/handler/instances.go` implements that fan-out. Each of the 5
unexported `search*` methods on `InstanceHandler` takes an `instanceIDFilters` struct (exactly one
of `projectIDs`/`deploymentIDs`/`deployedProductIDs` non-empty by construction — never
client-controlled, always derived from which of the 3 public wrapper methods was called) and the
3 exported wrapper methods per metric type just supply that struct plus the path param. When adding
a 6th instance metric type from a future entity-service endpoint, follow this same shape rather
than inventing a new fan-out mechanism.

**One deliberate asymmetry, by design, not "fixed":** `InstanceMetricsStatsRequest`'s `DataSource`
field is never forwarded to entity-service in `searchInstanceMetricsStats` (all 3 scopes), even
though the request type carries it, while the sibling `.../instances/stats/usages/search` family
(`searchInstanceUsageStats`) does forward it. This asymmetry is intentional, not a bug — see the
doc comment above `searchInstanceMetricsStats` before "fixing" it.

## Project metadata and stats — reshaped, not passed through

`GET /projects/{id}/filters`, `/features`, `/stats`, `/stats/cases`, `/stats/conversations`,
`/stats/support`, `/stats/time-cards`, and `/stats/change-requests` all read from seven
entity-service endpoints (`GetProjectMetadata`, `GetProjectCaseStats`,
`GetProjectConversationStats`, `GetProjectDeploymentStats`, `GetProjectStats`,
`GetProjectTimeCardStats`, and `GetProjectChangeRequestStats`). A handful of raw entity-service
responses are fanned out into eight differently-shaped, purpose-built views rather than exposed
1:1, and `internal/dto/project_stats.go` implements that fan-out:

- **`/filters` and `/features` both call `GetProjectMetadata`** — there is no `GET /projects/{id}/metadata`
  passthrough endpoint in this backend at all; the metadata response is only ever exposed split into
  these two narrower views. `ChoiceListItem`/`ReferenceTableItem` (entity-service's two "list of
  valid options" shapes) both collapse into one `dto.ReferenceItem{id, label, count?}` for the
  frontend. `/filters`' `changeRequestStates` additionally drops three internal ServiceNow workflow
  state IDs (`dto.restrictedChangeRequestStateIDs`) that were never meant to be a customer-facing
  filter option.
- **`/stats` and `/stats/support` are composite, graceful-degradation endpoints** — each combines
  multiple independent entity-service calls (`/stats` combines case/conversation/deployment/activity
  stats; `/stats/support` combines case/conversation stats) and returns `200` even if every one of
  them fails, simply omitting that source's fields from the response (`dto.BuildProjectDashboardStats`/
  `dto.BuildProjectSupportStats` take `*entity.XxxResponse`, nil meaning "this source failed to
  load"). It logs each failure and moves on rather than failing the whole request. `/stats/cases`,
  `/stats/conversations`, `/stats/time-cards`, and `/stats/change-requests`, by contrast, are **not**
  graceful — each is a single entity-service call and a failure there is a hard failure
  (`mapUpstreamError`), verified individually per endpoint rather than assumed from the composite
  endpoints' pattern.
- **State-ID-based derived counts are hardcoded, not configurable.** `dto.caseStateIDOpen` and the
  `conversationStateID*` constants pick specific counts out of a state-count breakdown (e.g. "how
  many cases are in the *open* state") using this deployment's default ServiceNow state IDs. If
  cs-tools' ServiceNow instance uses different state IDs for these, these constants need to become
  configurable here too — they are not currently.

## Middleware chain

`CORS → SecurityHeaders → CorrelationID → Auth → Logger → Mux`

Apart from `CORS`, identical to `apps/csm-portal/backend`'s chain — see that backend's CLAUDE.md for
the rationale of each layer. `middleware.ConfigureLogger()` must be called at startup.

**This chain covers the REST listener (`PORT`, 8080) only.** The WebSocket listener (`WS_PORT`,
8081) runs a shorter chain — `SecurityHeaders → CorrelationID → Logger → Mux` — with **no `Auth`
and no `CORS`**: `Auth` is impossible there (a browser cannot send `x-jwt-assertion` on a WebSocket
handshake, so `WebSocketHandler` authenticates the token itself), and `CORS` is irrelevant since a
WebSocket handshake is not subject to preflight. See "The AI chat agent" above.

**`CORS` must be outermost, wrapping everything including `Auth`.** A CORS preflight is a bare
`OPTIONS` request with no JWT at all; if `Auth` ran before `CORS`, it would reject every preflight
with 401 before the browser ever received a CORS header — which the browser then reports as
"blocked by CORS policy", masking the real cause. `main.go` currently calls `middleware.CORS(nil)`
(allow any origin, no env var) — `middleware.CORS` accepts an allow-list parameter if this ever
needs to be restricted, but nothing in this backend currently sets one.

## Response shaping — the "wrapper" pattern

**Never return an entity-service response struct directly to the frontend.** This is the one
deliberate difference from `apps/csm-portal/backend` (which does raw `[]byte` passthrough for most
entity responses) — every entity-service response is reshaped into a portal-owned DTO before it
reaches the frontend.

Concretely:

- `internal/entity` decodes entity-service's raw JSON into typed Go structs that mirror its wire
  format 1:1 (see `internal/entity/types.go`) — these types are internal to that package.
- `internal/dto` defines the portal's own response structs and one `Map*` function per entity type
  that translates entity → portal, dropping fields the customer portal has no business showing:
  - Salesforce/internal IDs (e.g. `ProjectDetailsView.SfID`)
  - CSM/WSO2-internal-only fields that entity-service itself documents as such (e.g.
    `CaseView`'s `BestCaseFixEta`/`MostLikelyFixEta`/`WorstCaseFixEta` — see the comments in
    `entity-service/internal/domain/entity.go` on `CaseView`)
  - WSO2-internal team routing (e.g. `AccountRef.CreTeam`/`SreTeam`)
  - Internal opaque identifiers not meaningful to a customer (e.g. `SearchCaseView.InternalID`,
    though `CaseDetails.internalId` on the single-case read path IS exposed — the frontend's own
    `CaseDetails` type reads it directly, unlike the search-list item)

  Not everything that looks internal-only actually is — `ProjectAccountRef.AgentEnabled`/
  `KbReferencesEnabled` and `CaseView.WatchList` both looked like CSM/engineer-only fields from
  their entity-service-side doc comments, but the frontend reads both directly
  (`ProjectAccount.hasAgent`/`hasKbReferences`, `CaseDetails.watchList` — a customer's own
  self-service watch-list, not an internal annotation) — see `internal/dto/project.go`'s
  `ProjectAccount` and `internal/dto/case.go`'s `CaseDetails` for the corrected mapping. Verify
  against the live frontend TypeScript, not just an entity-service doc comment, before excluding a
  field as "obviously internal."
- `internal/handler` calls the entity client, passes the result through the matching `dto.Map*`
  function, and writes the DTO with `writeJSONValue`.

When you add a new endpoint, add the equivalent trimming — read the field carefully before
including it; when in doubt whether a field is customer-appropriate, leave it out and note why in
a comment on the DTO struct (see `internal/dto/case.go` for examples).

**Data-source normalization is a second job of the DTO layer.** Unlike projects and cases,
entity-service's account endpoints (`GET /accounts/{id}`) return a
genuinely different wire shape depending on whether it's deployed with `DATA_SOURCE=postgres` or
`DATA_SOURCE=servicenow` — see `internal/entity/types.go`'s `AccountDetail`/`AccountSummary`
comments for how the two shapes are unioned into one Go struct (their JSON keys never collide) and
`internal/dto/account.go` for how the DTO mapper picks whichever fields the active data source
populated (e.g. `Tier` prefers entity-service's `tier`, falling back to `classification`) to
produce one consistent contract for the frontend regardless of which data source is live. This is
a genuine advantage of the DTO-mapping convention over raw passthrough — `apps/csm-portal/backend`
has to model this as an OpenAPI `oneOf` in its spec and pass the ambiguity on to the frontend;
here, it's resolved once in the mapper. Also deliberately excluded from account DTOs: `ArrToday`
(annual recurring revenue — WSO2-internal financial data, never expose to the customer) and `Pod`
(WSO2-internal account routing). `POST /products/search` and `POST /products/{id}/versions/search`
(`internal/entity/types.go`'s `ProductView`/`ProductVersionView`) use the exact same superset-struct
technique — e.g. `ProductVersionView.ReleaseDate` is typed `*string` even though the Postgres shape
is `time.Time` and the ServiceNow shape is a plain string, because a Go `*string` field decodes a
JSON string value regardless of which the source type actually was.

**When the frontend's contract expects a thinner shape than entity-service's own struct, match the
frontend's contract, not entity-service.** `POST /projects/{id}/time-cards/search`'s `dto.TimeCardSummary`
excludes entity-service's per-category time breakdowns (`timeAnalyzing`, `timeSettingUp`, etc.),
`issueComplexity`, `workLogComment`, `rejectionReason`, and the eligible-approvers list — not
because any of them are individually dangerous, but because the frontend's existing contract never
exposed them either. When porting an endpoint, read the frontend's existing response shape for the
equivalent feature, not just the request/response pair on `cs-tools/entity-service` — that shape is
itself a design decision about what a customer should see, and entity-service's superset shouldn't
leak through it by default.

**The DTO layer's job isn't only response trimming — it also absorbs entity-service's own request
contract changes, so the frontend never has to know they happened.**
`POST /projects/{id}/cases/search` is the example: entity-service redesigned its case-search
filters from named fields into a generic predicate array (`filters: [{field, op, values}]`,
`internal/entity/types.go`'s `CaseFieldFilter`) and now rejects any request using the old shape
outright (`decodeRequest`'s `DisallowUnknownFields`). The frontend was never updated — it still
sends the *old Ballerina backend's* named-field shape (`statusIds`, `severityIds`, `issueIds`,
`caseTypes`, `engagementTypeKeys`, date-range fields — see `CaseSearchFilters` in
`apps/customer-portal/webapp/src/features/support/types/cases.ts`, the one shared type every
case-search call site imports), not entity-service's own vocabulary — so
`dto.CaseSearchRequest`/`CaseSearchFilters` mirror *that* shape as this backend's own stable
contract, frozen at whatever the frontend already sends, and `dto.BuildEntitySearchCasesRequest` is
the only place that builds an `entity.CaseFieldFilter` array, translating each portal filter field
into the "field in/eq/gte/lte values" entry entity-service's `case_filters.go` expects (see that
file for the authoritative field/op table if entity-service adds a new filter). `CreatedByMe`
becomes a `createdBy`+`eq` filter carrying the literal string `"__current_user_email__"` — this must
match entity-service's own `currentUserFilterPlaceholder` constant exactly, not just be "some
sentinel". If entity-service ever changes another request contract this way, the fix is the same
shape: keep the portal-facing dto type frozen at whatever the frontend already sends, and write a
`BuildEntityXRequest` translator rather than pushing the new shape onto the frontend or, worse,
decoding the incoming request directly into an `internal/entity` type (which is how this bug
happened in the first place — there was no dto/entity separation on the request side for this one
endpoint, unlike every response, which already goes through `dto.Map*`).

**Some fields need a second translation layer on top of the dto/entity split: ServiceNow/Choreo
numeric choice-list ids ⇄ entity-service's string enums.** The frontend was built against the old
Ballerina backend, which forwarded these raw numeric choice-list keys (case status/severity/
issue-type/engagement-type, call-request state, change-request state/impact, conversation state,
deployment type, product-vulnerability severity) directly — it still sends and expects those exact
numbers today, even though `cs-tools/entity-service`'s own contract is plain lowercase-snake-case
string enums (or, for conversation state, an upper-snake-case one). Six files hold this backend's
own copies of the numeric-id↔enum tables, one pair of directions each:
`internal/dto/case_enum_mapping.go` (mirrors `snStateIDMap`/`snSeverityIDMap`/`snIssueTypeIDMap`/
`snEngagementTypeIDMap` in `internal/service/sn_case_service.go`),
`internal/dto/call_request_enum_mapping.go` (mirrors `callRequestKeyToState` in
`sn_call_request_service.go`), `internal/dto/change_request_enum_mapping.go` (mirrors
`snCRStateIDMap`/`snCRImpactIDMap` in `sn_change_request_service.go`),
`internal/dto/conversation_enum_mapping.go` (mirrors `snConversationStateKeyMap` in
`sn_conversation_service.go`), `internal/dto/deployment_enum_mapping.go` (mirrors
`deploymentTypeToKey` in `sn_deployment_service.go`), and
`internal/dto/product_vulnerability_enum_mapping.go` (mirrors `vulnerabilityPriorityToSeverityID`
in `sn_product_vulnerability_service.go`). These ids are each service's own stable upstream
configuration, not something entity-service computes, so duplicating them is safe, but if
entity-service's copies ever change, update the matching file here too. Two directions:
- **Request → entity-service**: the frontend's numeric filter ids (`statusIds`, `severityIds`,
  `issueIds`, `engagementTypeKeys`, `stateKeys`) translate via an id→enum map
  (`caseIDsToEnums`/`callRequestStateKeyToEnum`) before reaching `BuildEntityXRequest`; an
  unrecognized id is silently dropped (search) or produces an empty enum that entity-service's own
  validation then rejects with 400 (update) — never forwarded to entity-service as a raw number,
  which would always 400 anyway (entity-service only accepts its own enum vocabulary).
- **Response → frontend**: entity-service's plain-string enum-valued fields (case search's `state`/
  `severity`/`issueType`/`engagementType`, already ServiceNow's raw display *label* text for the
  ServiceNow-backed search path, not a normalized enum — see `SearchCaseView`'s doc comment) become
  `{id, label}` (`IDLabelRef`) via a label-parsing helper per field (`caseStatusRef`,
  `caseSeverityRef`, etc.) that mirrors entity-service's own label-parsing functions
  (`snCaseStateMap`, `snSeverityLabelMap`, `snIssueTypeToEnum`) closely enough to resolve the same
  id, falling back to a label-only ref (empty id) for a label neither table recognizes rather than
  dropping the field.

If a future endpoint has this same problem (frontend still expects a legacy numeric id/label pair
entity-service no longer speaks), follow this same two-file, two-direction pattern rather than
inventing a new one per endpoint.

**Project scoping via a `{id}` path parameter is the source of truth — never a client-settable body
field, even when entity-service's own request struct has one.**
`POST /projects/{id}/cases/search` and `POST /projects/{id}/deployments/search` both scope every
search to the one project in the URL: `dto.CaseSearchFilters` has no `projectIds` field at all (the
frontend has never sent one), and `dto.BuildEntitySearchCasesRequest(projectID, req)` always adds a
`projectId`+`in` filter carrying that path value. `entity.SearchDeploymentsRequest` does still have
a `ProjectIDs` field (it mirrors entity-service's own struct 1:1, per the no-DTO-layer exception for
this endpoint — see "Response shaping" below), but the handler forcibly overwrites it with the path
value right after decoding, the same way `AIChatHandler.SearchConversations` already does for
`entity.SearchConversationsRequest.Filters.ProjectIDs`. Letting the body set project scope
independently of the path would both be a needless IDOR surface and risk the same
unsatisfiable-AND-filter bug `CreatedBy`/`CreatedByMe` had if the two values ever disagreed.

**`json.RawMessage` on a request field usually means "preserve three states," not "skip validation."**
`entity.UpdateDeployedProductRequest.Description` is `json.RawMessage` specifically so entity-service
can distinguish "field absent" (omit), `"description": null` (clear the value), and
`"description": "value"` (set it) — a plain `*string` can't represent "absent vs. explicitly null."
When a portal request decodes straight into a struct with such a field (no restricted DTO needed
here, since Cores/TPS/Description/Active are all customer-appropriate), the raw bytes the client
sent pass through unchanged and this three-state semantic is preserved automatically — don't
"simplify" the field to `*string` when porting a similar endpoint.

**Binary responses use a `doBinary`-shaped client method, not `getJSON`/`postJSON`.**
`GET /attachments/{id}/content` returns a raw file, not JSON — `entity.Client.doBinary` (added in
`internal/entity/client.go` alongside `do`) returns `(body []byte, contentType string, error)`, and
the handler (`internal/handler/attachments.go`) writes `Content-Type` from entity-service's own
(already-sanitized) header value and explicitly sets `Content-Disposition: attachment` itself —
never render an attachment inline, since entity-service's own allowlist coercion to
`application/octet-stream` for unrecognized types is a stored-XSS mitigation this backend must not
undo by, say, echoing a client-supplied filename into a `Content-Disposition` you construct
yourself.

Request bodies are usually the exception: incoming search/filter/create payloads are decoded
directly into the entity package's request structs (e.g. `entity.SearchProjectsRequest`,
`entity.CreateCaseRequest`) with no separate DTO layer, since those shapes are already what the
frontend needs to send and every field is customer-appropriate — there is nothing to hide.

**But when entity-service's request contract mixes customer-safe fields with internal-only ones,
build a restricted portal request DTO too.** `entity.UpdateCaseRequest` (`PATCH /cases/{id}`) is
the example: it has 18 optional fields, but `workState`, `assigneeEmail`, `parentId`/
`relatedCaseId`/`deploymentId`/`deployedProductId` (case relinking), `autocloseHoldUntil`, and the
`fixEta`/`bestCaseFixEta`/`mostLikelyFixEta`/`worstCaseFixEta` quartet are internal WSO2 support
operations, not things a customer should be able to set on their own case. `dto.UpdateCaseRequest`
(`internal/dto/case.go`) exposes only the customer-safe subset (state, severity, subject,
description, watchList, resolutionCode, cause, closeNotes), and
`dto.BuildEntityUpdateCaseRequest(id, req)` builds the full entity-service request from it, leaving
every excluded field zero/nil — so even if a client sends `{"workState": "ongoing"}`, it's silently
dropped (the portal struct has no such field) rather than forwarded. The same pattern applies to
`POST /cases/{id}/comments`: entity-service accepts `type: work_note|comment|activity`, but
`dto.BuildEntityCreateCaseCommentRequest` always forces `type: comment`, regardless of what the
client sends — a customer should never be able to create an internal work-note or system-activity
entry. When you add a write endpoint, check the entity-service request struct for fields that read
as "internal support operation" rather than "customer self-service action" before deciding whether
to pass it through directly or build a restricted DTO.

Two more examples, both in this same "restrict, don't mirror" category:
- `PATCH /change-requests/{id}` (`dto.ChangeRequestUpdateRequest`) excludes case/project/deployment
  relinking and `assignedEngineerId`/`assignedTeamId` (support assignment) for the same reasons as
  case updates, plus `state` specifically — state transitions go through the dedicated
  `isCustomerApproved`/`isCustomerReviewed`/`requestApproval` fields (which *are* exposed, since
  they're literally the customer's own approval actions) rather than letting the customer set an
  arbitrary ServiceNow workflow state directly.
- `PATCH /cases/{caseId}/call-requests/{id}` (`dto.CallRequestUpdateRequest`) excludes `meetingDate`/`assignee`/
  `notes`/`plan`/`attendees`/`actionItems`/`actualDurationMin` — entity-service's own doc comment
  labels these "agent-side fields, set when an engineer schedules or concludes the call." They're
  still exposed on the *read* side (`dto.CallRequestSummary`) since the customer should be able to
  see the outcome of their own call, just not set it themselves.
- `POST /comments` (and `GET /conversations/{id}/messages`, which searches comments internally)
  restrict in *both* directions: `dto.BuildEntityCreateCommentRequest` forces `type: comment` on
  write for the same reason as case comments, and `dto.BuildEntitySearchCommentsRequest` forces
  `filters.type: comment` on **read** too — entity-service's search endpoint returns `work_note`
  entries verbatim unless the caller filters them out, and those are internal WSO2 annotations that
  must never reach the customer regardless of which reference entity they're attached to.

**Not every field worth restricting is a security decision — some are just an entity-service scoping
convenience, and the path (not the body) is the more reliable source for it.**
`PATCH /deployments/{deploymentId}/products/{id}`'s `deploymentId` field is an IDOR-style scope
guard entity-service documents (verify the deployed product belongs to this deployment before
mutating it), not a way to relink the resource — but the frontend's own `PatchDeploymentProductRequest`
body never actually carries a `deploymentId` field, so trusting the body would leave the guard
permanently unset. `dto.BuildEntityUpdateDeployedProductRequest` always injects it from the URL's
`{deploymentId}` path segment instead, the same way project/deployment scoping is forced from the
path everywhere else in this backend (see "Project scoping via a `{id}` path parameter" below). Read
the entity-service doc comment on a field like this before deciding whether path-injection or
body-passthrough is the right source — don't pattern-match on field name alone (e.g. "any ID field"
or "any assignee-shaped field").

**"Exactly one" vs. "at least one" is per-entity, read entity-service's own doc comment.**
`PATCH /cases/{id}` requires *exactly one* of its primary fields (entity-service's doc comment says
so explicitly); `PATCH /change-requests/{id}` requires *at least one* (its doc comment says "at
least one must be provided"). Don't assume one pattern generalizes to the other — copying the wrong
validation produces a portal that's stricter or looser than the upstream contract, and CodeRabbit
caught exactly this mismatch once already (see the case-update fix in this backend's PR history).

Where entity-service defines many enum-like fields (change request category/priority/impact/type/
state/risk, call request state, etc.) as named Go string types with const blocks, this file's
convention is to flatten them to plain `string` in the mirrored struct (matching how `CaseView`
already treats `Severity`/`State` as plain strings) — skip re-declaring the const blocks unless a
specific value needs to be checked in Go code (e.g. `ChangeRequestApprovalDecisionRequest.Decision`
validation in the handler compares against literal `"approved"`/`"rejected"` strings, not enum
constants).

**All pagination responses use `totalRecords`. Do not introduce `total` on a paginated envelope.** The frontend's shared
`PaginationResponse` type (`apps/customer-portal/webapp/src/types/common.ts`) is
`{offset, limit, totalRecords}`, and the large majority of this backend's paginated responses were
renamed to match it during a full request/response type-alignment pass against the old Ballerina
backend and the live frontend (see git history for `fix/customer-portal-v2-align-response-types`).
The last three stragglers (`internal/dto/account.go`, `attachment.go`, `escalation.go`) were
renamed from `total` to `totalRecords` after the legacy key silently broke the case-detail
Attachments tab: `useGetCaseAttachments` destructures `totalRecords` with **no** array-length
fallback (unlike the calls and escalations panels, which fall back to `data.length` and so masked
the same drift), so `attachmentCount` stayed `undefined` and the tab rendered "Attachments (0)"
against a perfectly good 200 response.

`internal/dto/project_stats.go`'s `ResolvedCountBreakdown.total` is **not** part of this drift and
must keep its name — it is a stats breakdown (`{total, currentMonth, pastThirtyDays}`), not a
pagination envelope, and the frontend reads `total` there (`features/dashboard/types/charts.ts`).
Renaming it would break the dashboard charts.

New endpoints should default to `totalRecords`; verify against the specific frontend hook's decoded
field name before assuming (check `apps/customer-portal/webapp/src/api/` and
`src/features/*/api/`), and prefer a key-name assertion test (`internal/dto/case_display_labels_test.go`)
over trusting the struct tag by eye.

**Some routes nest a resource's own ID in the path purely for RESTful shape, not because the
handler needs it.** `PATCH /cases/{caseId}/call-requests/{id}` is the example: entity-service's
`UpdateCallRequest` is keyed on the call request's own `id` alone, so `caseId` is read from the URL
only to make the path match the frontend's nesting (`/cases/{caseId}/call-requests/{id}`) — the
handler never looks at it. Don't add a spurious "does this call request belong to this case" check
just because the path implies one; that authorization already happens for `POST` and `POST .../search`
on the same nested resource (`CaseID` is forced from the path there because entity-service's request
struct actually carries it), and a stray extra check on `PATCH` would just be dead weight.

## Adding a new endpoint

1. **Confirm the route exists on `cs-tools/entity-service`** — check `routes.go` and note whether
   it's Postgres-only, ServiceNow-only, or both (see "Which entity-service" above). If it doesn't
   exist, stop and add a comment instead of faking it.
2. **Entity types** (`internal/entity/types.go`) — add the request/response structs, copied field-
   for-field (name, type, `json` tag) from `entity-service/internal/domain/entity.go`. Don't guess
   — read the actual struct.
3. **Entity client method** (`internal/entity/<feature>.go`) — add a method on `Client` using
   `c.getJSON`/`c.postJSON`; `url.PathEscape()` every path parameter.
4. **Portal DTO** (`internal/dto/<feature>.go`) — add the trimmed response struct and a `Map*`
   function. See "Response shaping" above for what to exclude.
5. **Handler** (`internal/handler/<feature>.go`) — extend or add a local interface naming only the
   entity-client methods this handler needs; handler method sequence: auth check → path/body
   guards → call entity client → `mapUpstreamError` on failure → map to DTO → `writeJSONValue`.
6. **Route** (`cmd/server/main.go`) — register using Go 1.22 method-prefixed patterns:
   `"POST /cases/{id}/comments"`. **`net/http.ServeMux` panics at startup — crashing the whole
   server — if a new pattern is ambiguous with an existing one** (neither is strictly more specific;
   this can happen even with a different number of literal segments in a different position, e.g.
   `/deployments/products/{id}/instances/metrics/search` vs.
   `/deployments/{deploymentId}/products/{productId}/metrics/search`, which both match
   `/deployments/products/products/instances/metrics/search`). Registration order does NOT matter
   for this — the panic fires regardless of which one is registered first. This is NOT a
   theoretical concern: it took the server down in production once already (see the merged fix for
   exactly this pair of routes). Before opening a PR that adds a route with a wildcard segment,
   sanity-check it against every other route sharing that many-or-fewer path segments; if two
   patterns are ambiguous, merge them under one wildcard pattern and dispatch manually using
   `r.SetPathValue` to inject the values each handler expects (see
   `dispatchDeploymentsProductsMetricsSearch` in `main.go` for the pattern) rather than renaming
   either route — the URL shapes themselves are an external contract with the frontend.
7. **README** — add the endpoint under "API Endpoints" in `README.md`.
8. **OpenAPI spec** (`openapi.yaml`) — add the path with `200`/`400`/`401`/`403`/`500` responses
   (`404` too for get-by-id, `413` too for endpoints with a request body); every endpoint must
   declare `403` since `mapUpstreamError` can return it.
9. **gosec** — run `gosec -fmt=text ./...` (must report 0 issues) before opening a PR.

## Handler conventions

- **Auth**: always check `middleware.UserInfoFromContext(r.Context()) == nil` first → 401.
- **Body size**: use the shared `readJSONBody(w, r)` helper (`internal/handler/response.go`) — caps
  at `maxRequestBodyBytes` (1 MiB) and validates the body is well-formed JSON.
- **Path params**: guard against empty string after `r.PathValue("id")`; validate UUID-shaped IDs
  with the package-level `uuidRe` and return 400 on mismatch before calling entity-service.
- **Upstream errors**: always use `mapUpstreamError(w, err, "<fallback message>")` — never write
  custom status mappings inline. For a 400, this now returns entity-service's own message
  (`apiErr.Body`) verbatim to the caller instead of a generic string — entity-service's validation
  errors are already written to be safe and specific (see its `apierror.ValidationError`), so
  swallowing them loses real, actionable detail for no security benefit. 401/403/404 still always
  use a fixed message regardless of the upstream body — never pass through upstream text for those
  statuses.
- **Logging**: use `slog.ErrorContext` with `summarizeErr(err)`, never the raw error — an
  unrecognized error can stringify with the full request URL including query params.
  `summarizeErr` DOES include the upstream status and message for a typed `*apierror.Error` (e.g.
  `"upstream status 400: caseTypes must be valid UUIDs"`) — entity-service's error bodies are
  already caller-safe validation text, not sensitive internal detail, so logging them verbatim is
  fine. `apierror.NewUpstreamError` (`internal/apierror/apierror.go`) is what populates `apiErr.Body`
  with just the extracted `message` field (not the upstream's raw response) — every upstream
  client in this backend (entity, registry, updates, scim, productconsumption, aichatagent,
  usermanagement) constructs its non-2xx errors through this one shared function rather than each
  reinventing its own inline body-truncation/excerpt fallback. Body is left empty when the response
  isn't the expected `{"message": "..."}` shape, relying on each caller's existing "empty Body → generic
  fallback" logic (`mapUpstreamError`'s 400 case, `writeUpstreamMessage`) — never add a new
  upstream-error construction site that falls back to a raw excerpt instead of calling this function.

## Security

- **Never commit secrets** — `.env`, `Config.toml`, or any file with real credentials must never be
  staged.
- **No sensitive data in logs** — do not log request bodies, JWT payloads, or PII such as email/name. The opaque `userID` claim (`UserInfo.UserID`) is not PII and may be logged for correlation/support purposes, as every handler does today.
- **JWT is the only inbound auth mechanism** — every non-health endpoint must go through
  `middleware.Auth`, with exactly one exception: `GET /ws`, which cannot receive the
  `x-jwt-assertion` header at all and so validates its own token through the same
  `middleware.TokenValidator` instead (see "The AI chat agent"). It is still JWT-authenticated —
  do not add a second exception without the same browser-level justification.
- **Run gosec on every change** — `gosec -fmt=text ./...` must report 0 issues before opening a PR.
