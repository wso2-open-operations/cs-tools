# CSM Portal Backend

Go HTTP server (`net/http`, Go 1.26+) that acts as a backend-for-frontend (BFF) for the CSM portal. It authenticates callers, forwards requests to upstream services, and shapes responses for the frontend.

## Middleware chain

`SecurityHeaders → CorrelationID → Auth → Logger → Mux`

- `SecurityHeaders` (`internal/middleware/security_headers.go`): sets `X-Content-Type-Options: nosniff`, `Content-Security-Policy: upgrade-insecure-requests`, and `Strict-Transport-Security: max-age=31536000; includeSubDomains` on every response; outermost so headers are present even on auth failures
- `CorrelationID` (`internal/middleware/correlation.go`): reads `X-CSM-Correlation-ID` from the incoming request or generates a UUID v4; stores the ID in context for the slog handler and for the entity client to forward; echoes the ID in the response header
- `Auth` (`internal/middleware/auth.go`): validates the `x-jwt-assertion` JWT and sets `UserInfo` in context. When `TokenValidatorEnabled` is true, the JWKS fetch runs through `x5cStrippingTransport`, which strips the `x5c` field from every key before `MicahParks/jwkset` parses the response — some IdPs (Asgardeo included) publish JWKS certs with a negative serial number, which Go's `crypto/x509` rejects since Go 1.23 and would otherwise make the whole JWK Set fail to load, even though verification only needs `n`/`e`. In Choreo deployments `TokenValidatorEnabled` is false (the gateway validates the JWT upstream), so this path only runs in local dev.
- `Logger` (`internal/middleware/logger.go`): logs every completed request (method, path, status, elapsed) via slog; runs after Auth so both `correlationID` and `userID` are present in every record

`middleware.ConfigureLogger()` must be called at startup — it wraps the default slog handler so that every `slog.*Context(r.Context(), …)` call anywhere in the codebase automatically includes `correlationID=<id>` when the context carries one.

## Access control (roles)

`Auth` only proves identity. Authorisation is `handler.AccessGuard` (`internal/handler/access.go`): it checks the caller's `roles` claim — decoded into `middleware.UserInfo.Roles` — against the role names configured per role. It makes **no upstream call**; a check is a set lookup. (Do not switch it to the entity service `GET /users/me` role data: that adds an upstream call (and a cache to offset it) per request for no gain.)

- **Per-role config.** Each role has an `AUTH_<ROLE>_ROLES` env var (comma-separated role names, any one grants the role), read once at startup by `loadAccessConfig` in `cmd/server/main.go`. There is deliberately **no default** (role names are organisation vocabulary and must not be committed, same as `CSM_TEAM_REGISTRY`): an unset/empty variable means nobody holds that role, and startup warns naming each one. Tests use dummy names via `testAccessConfig()` in `access_test.go`, never real ones. Matching is exact and case-sensitive. Adding a role means a new `AccessConfig` field, its env var, and its place in `NewAccessGuard`.
- **The `roles` claim is a string for one role and an array for several** (Asgardeo does this). `middleware.stringList` accepts both; a plain `[]string` would reject a single-role user's whole token with a 401. Any other shape fails the token.
- **Every route goes through `route(pattern, perm, handler)` in `cmd/server/main.go`.** `perm` is a required argument with no default — pick `PermView` for reads/searches/aggregates, `PermViewOperations` for reads under the Operations area (incidents, change requests, problems, incident tasks, outages, alerts), `PermTimeCardsAndUpdates` for every time-card route and the update-level lookups (CS engineer, admin and time-card approver — viewing/managing, not approving, see `PermApproveTimeCard` below), `PermWrite` for other state changes, `PermAdmin` for the handful of actions reserved for admin alone (currently just `POST /users`, creating a new platform user), `PermViewSecurityCenter` for Security Center (both `/products/vulnerabilities/*` routes; see "Security Center access" below for how `/cases/*` is handled), or one of the narrower ones (`PermEscalate`, `PermDownloadAttachment`). `PermApproveTimeCard` is not a route-level permission at all — see its own note below. Case, incident and change-request comments are `PermWrite`. `PermAuthenticated` (no role needed) is only for the caller's own `/users/me`. `/health` is the only route registered directly on the mux.
- **The policy is `NewAccessGuard`.** `admin` satisfies every permission, including `PermAdmin`; `cs_engineer` satisfies every other route permission (view, download, write, including comments, security center, time cards and updates) but **not** `PermAdmin`, `PermEscalate`, or `PermApproveTimeCard` — escalating a case and approving a time card are each a dedicated responsibility, held only by their own role (`escalator` / `timecard-approver`) plus admin, the same way `PermAdmin` is admin-only; `escalator`/`attachment-downloader` grant only their one ability plus view (so a view-only role cannot read Operations — `PermViewOperations` is CS engineer and admin only, matching the frontend's `canUseOperations`); `timecard-approver` grants view plus `PermTimeCardsAndUpdates` plus `PermApproveTimeCard`; `usage-metrics-viewer`/`dashboard-designer` grant only view here (no backend route for those features); the frontend gates them. Every role implies view, **except** `PermViewSecurityCenter` — a plain viewer/escalator/attachment_downloader/usage_metrics_viewer/timecard_approver/dashboard_designer holds `PermView` but not this.
- **`PermApproveTimeCard` gates a state transition inside a shared route, not a route of its own.** `PATCH /time-cards/{id}` carries EITHER a plain field edit OR an approve/reject transition (`state: "approved"`/`"rejected"`, entity-service's `UpdateTimeCardRequest`) — the same shared-endpoint problem Security Center's `POST /cases/search` has, solved the identical way: `TimeCardHandler` (wired with `WithAccessGuard`, same pattern as `CaseHandler`) inspects the request body itself (`timeCardUpdateTargetsStateTransition`, checking for a non-nil `state`) and additionally requires `PermApproveTimeCard` only when it's present. A CS engineer without `timecard-approver`/`admin` can still search/create/edit their own time cards (`PermTimeCardsAndUpdates`), just not approve/reject one.
- **PLG's routes go through this guard too, and through a second one of their own.** `internal/plg/plg.go` registers all 24 as `accessGuard.Require(perm, identity(fn))` — the guard OUTSIDE PLG's `identity` middleware, deliberately, because the guard is a set lookup while `identity` is an entity-service round trip, so a denied caller costs nothing upstream. Two permissions: `PermUsePlg` (CS engineer and admin) for 20 routes, and `PermManagePlaybooks` (admin only) for the four that author a playbook template. `PermUsePlg` is narrower than `PermView` on purpose — every portal role holds `PermView`, but PLG is a worklist staff act on, and a view-only role that could open it would meet a 403 on every control. Reading a playbook (`GET /plg/playbooks`, `GET /plg/playbooks/{id}`) and assigning one to a pairing (`POST .../playbook-runs`) are `PermUsePlg`, NOT `PermManagePlaybooks`: running a template and writing one are different jobs, so gating on the `/plg/playbooks` path prefix would be wrong. `PermManagePlaybooks` is kept separate from `PermAdmin`, which it currently matches exactly, so granting playbook authoring to a future PLG-admin role does not also hand out platform-user creation. `identity` still runs and is not redundant: it resolves the `"user".id` every PLG write records as `actorId`, and refuses anyone who is not ACTIVE INTERNAL staff — a roles claim cannot tell you somebody was offboarded this morning. `internal/plg/routes_access_test.go` pins every route's permission and the middleware ordering.
- **`GET /users/me` reports `roles`** (from `AccessGuard.RolesFor`): the stable keys of the portal roles the token roles grant — several possible, fixed order — and is **not** the entity service's role data, which the response no longer carries. The frontend decides what to show or hide from these roles (there is deliberately no derived `permissions` list); the backend's `403` is the real gate. Dashboard-designer access is only the `AUTH_DASHBOARD_DESIGNER_ROLES` role (the old `DASHBOARD_DESIGNER_EMAILS` email allow-list is gone).
- **`GET /users/{id}` reports the SAME portal-role vocabulary for an internal target, via SCIM instead of a JWT.** `RolesFor` only ever takes `[]string` -- it doesn't care whether those strings came from the live caller's own JWT `roles` claim (`GetMe`) or somewhere else, so `GetUser`'s own `withPortalRoles` (`internal/handler/user_portal_roles.go`) reaches the same vocabulary for the profile being *viewed* by calling `scim.SearchUser` for that user's email instead: SCIM's own user search returns the same role assignment for any user, not just the caller. Two things worth knowing:
  - SCIM's `roles` spans every Asgardeo application the person holds a role in, not just this portal, so `withPortalRoles` filters to `scim.CSMAppRolePrefix` ("app-csm-") first -- matching what the JWT's own `roles` claim already narrows to at token-issuance time. Passing the unfiltered list into `RolesFor` would still be *correct* (a role name from another app just never matches `AUTH_<ROLE>_ROLES`), the filter exists so a caller reading the intermediate `[]string` mid-pipeline sees only this portal's roles, not an unrelated app's.
  - This only runs for an **internal** target (`userType == "internal"`) -- an external contact's access is modeled on entity-service's own role vocabulary instead (`admin`/`commenter`/`customer`/`customer_admin`/`partner`/`partner_admin`/`external`/`internal`/`timecard_approver`/`agent`, the `CSM_USER_ROLES` set), which is a genuinely different, unrelated vocabulary from the portal-access one — not something SCIM has any equivalent for, and not something this replacement should touch. Best-effort like every other `GetUser` enrichment (teams, external-account status): any SCIM failure leaves the response's `roles` as entity-service's own, rather than failing the request.
  - SCIM's `roles` attribute does **not** follow the JWT `roles` claim's "bare string or array of strings" convention (`middleware`'s own unexported `stringList`) — against a real SCIM service it comes back as an array of `{value, display, audienceType, ...}` objects instead (confirmed live: the naive string-array assumption failed every `GetUser` call for an internal user with `"json: cannot unmarshal object into Go value of type string"`). `scim.scimRoles` (`internal/scim/types.go`) has its own `UnmarshalJSON` that accepts a bare string, an array of strings, a bare object, and an array of objects, since Asgardeo's own singular/plural convention elsewhere means a bare single value can't safely be ruled out either, even though only the array-of-objects shape has actually been observed. **`value` on that object is the role resource's own opaque id (a UUID), not its name** — extracting it instead of `display` was a second bug found in the same fix, confirmed live: filtering by `CSMAppRolePrefix` against `Value` silently matched nothing (a 200 with an empty `roles` array, not an error), since no role id happens to start with `app-csm-`. `display` is the actual role name, and carries an environment-specific suffix in practice that `AUTH_<ROLE>_ROLES`'s own configured values already have to account for — this package doesn't strip or otherwise special-case it.
- The role check is separate from resource-level guards inside handlers (e.g. the assigned-engineer check on public case comments) — both must pass.
- **`PermViewAllDashboards` (CS engineer and admin) is the one permission with no route.** A `dashboard.Dashboard` may set `"restricted": true`; `DashboardHandler` calls the newly-exported `AccessGuard.Permits(perm, roles)` itself to filter it out of `GET /dashboards` and 403 a direct `GET /dashboards/{id}` for anyone else, while both routes stay `PermView` so the list route still runs for every viewer. `Permits` exists specifically for this — a handler that must gate one specific resource against a caller's roles from inside its own logic, not a whole route via `Require`.
- Handler tests call handlers directly and bypass the guard; test the guard in `access_test.go`. `dashboards_test.go`'s `withRestrictedDashboard` temporarily swaps the package-level `dashboard.Active()` registry for one that adds a `Restricted` dashboard, restored via `t.Cleanup` — every other test in that file depends on the shared two-dashboard fixture's exact count, so never add a permanent third dashboard to it.

## Creating a user (POST /users)

Admin-only (`PermAdmin`, see "Access control" above) — the CSM portal's Add User UI is the one
caller. `UsersHandler.CreateUser` (`internal/handler/users.go`) validates `roles` against
`Directory.IsValidRole` (the same startup-resolved `CSM_USER_ROLES` allow-list `POST /roles/search`
serves) before forwarding a rebuilt body (see `buildEntityCreateUserBody`) carrying just
`firstName`/`lastName`/`email`/`roles` to the entity service's own `POST /users` — entity-service
deliberately does not validate role names itself (see that repo's own `domain.UserRole` doc comment),
so this is the one place that does. `roles` is optional; the Add User form's "User type" selector is
the one caller-facing use of it — it sends exactly one of `["internal"]`/`["external"]`, since
entity-service derives `user_type` from role membership rather than a plain settable column (see that
repo's own `recompute_user_type` trigger, migration 0011). "External" is currently disabled in that
selector and rejected server-side if sent anyway — see the constraint below.

A separate, unrelated field on the same request, `grantRoles`, is the Asgardeo-backed way to also put
the new user into one or more portal-permission roles (`cs_engineer`, `escalator`, ...) at creation
time — see "Granting portal roles on user creation" below for the full mechanism. It is validated and
acted on independently of `roles`/`CSM_USER_ROLES` and never reaches entity-service at all.

**Constraint: an internal-type user must have a `@wso2.com` email.** Found live: the Add User form sent
no `roles` at all, so every user it created resolved to `user_type = NOT_AVAILABLE` (the trigger's
fallback for "no matching role, not a system user") — a real, existing data-quality gap (128 such users
in staging at the time this was checked), not a hypothetical one. Fixing that by wiring up a type
selector raised the obvious next risk: nothing stopped an admin from granting `internal`/`admin` (both
resolve to `user_type = INTERNAL`) to a non-WSO2 address. `requestsInternalUserType`
(`internal/handler/user_external_account.go`, next to `wso2EmailDomain`/`isWso2Email`) checks `roles`
case-insensitively against that same two-name list and rejects the request with 400 before forwarding
to entity-service if the email isn't `@wso2.com` — a fast, friendly failure. **This is not the real
enforcement boundary**: entity-service's own `userService.CreateUser` (`user_service.go`) runs the
identical check against `req.Roles`/`req.Email` and is what actually protects the database, since
`POST /users` is entity-service's own route and this backend is not its only conceivable caller. The
two lists (`internalUserTypeRoles` here, its unexported twin there) are kept in sync by hand, the same
way `wso2EmailDomain` itself already is between the two repos.

**Constraint: creating an external-type user is temporarily disabled.** `requestsExternalUserType`
(same file, `externalUserTypeRoles = ["external", "partner", "customer", "partner_admin",
"customer_admin"]` — every role name entity-service's trigger maps to `user_type = EXTERNAL`) rejects
the request with 400 regardless of email, mirroring entity-service's own identical, authoritative
check. The Add User form's type selector disables its "External" option for the same reason rather
than offering a choice the backend will reject. All three layers (here, entity-service, the webapp)
are meant to come out together once external-type creation is ready.

## Listing time card approvers via SCIM (GET /users/time-card-approvers)

Who may approve a time card is granted by real Asgardeo role membership (see `AUTH_TIMECARD_APPROVER_ROLES`
in "Access control" above), but `POST /users/search`'s `roleIds` filter — the only other way to list
"who holds role X" — reads entity-service's own Postgres `role`/`user_role` tables instead, a separate,
syncable mirror that is not guaranteed to agree with Asgardeo's real membership at any given moment.
`GetTimeCardApprovers` (`internal/handler/users.go`) answers the question directly: it calls
`scim.Client.GetRole` with a configured Asgardeo role id and returns that role's real `users` list
(`{id, email}` per member), authoritative rather than a potentially-stale mirror.

**`ASGARDEO_ROLE_IDS`** (`internal/directory.ParseRoleIDs`) is a JSON object string, e.g.
`{"example-timecard-approver-role":"11111111-1111-1111-1111-111111111111"}` — a general
real-role-name → role-id mapping, not a single-purpose env var, so a second SCIM-backed role lookup
later is a config row plus a small handler, not a redesign. **Keyed by the real role name exactly as
it appears on the token's `roles` claim** (the same strings `AUTH_<ROLE>_ROLES` already lists), not an
invented portal-role key — `AUTH_<ROLE>_ROLES` can list more than one real name granting the same
portal permission, and only a real name identifies a specific role an id can actually belong to.
JSON rather than this package's usual flat `key|value,key|value` form specifically because the key
here is an opaque, deployment-specific role name that may itself contain punctuation (e.g.
`example.namespaced-role`) — delimiter-splitting that reliably would need its own escaping, which JSON
already provides. Parsed once at startup (`cmd/server/main.go`, right after `loadDirectory()`) into a
plain `map[string]string`; deliberately **not** folded into `directory.Directory` itself, since that
type's own charter (team registry + assignable-role allow-list) is a different, narrower concept than
"which roles have a SCIM-backed lookup wired up" — this is role *ids* for a specific feature, not
organisation vocabulary every caller needs.

`handler.ResolveGrantableRoles(accessCfg, roleIDsByName)` (`internal/handler/grantable_roles.go`) is
what bridges this real-name-keyed map back to the portal-role vocabulary (`cs_engineer`, `escalator`,
...): for each portal role, it checks `AccessConfig`'s own configured real name(s) against this map and
keeps the portal role only if at least one resolves to a known id — the stable `[]handler.GrantableRole{Key,
RoleID}` list every caller downstream actually works with. No default and no required keys: a real name
with no entry here just means that role has no SCIM-backed feature wired up in this deployment —
`timecardApproverRoleIDs` in `main.go` is derived from this same resolved list
(`handler.RoleIDsForKey(grantableRoles, "timecard_approver")`), not a second, parallel lookup into the
raw map. It is a slice, not a single id: `AUTH_TIMECARD_APPROVER_ROLES` can list more than one real role
name, each resolved to its own id, and an approver holding any one of them must still show up — unlike
granting (`RoleIDForKey`, singular, used by `CreateUser`), which only ever needs one specific role to add
a new user to, reading membership must not silently miss someone who only holds the second configured
role. `GetTimeCardApprovers` queries `scim.Client.GetRole` once per configured id and merges the results,
deduplicated by member id. **`GET /users/time-card-approvers` is registered unconditionally**, deliberately
unlike this file's other optionally-wired features (`ENGINEERING_ENTITY_BASE_URL`, the `CSM_MIGRATION_*`
flags), which skip registration entirely when off: it collides with the wildcard `GET /users/{id}` route, so
leaving it unregistered would have the request fall through to `GetUser`, which rejects the literal
segment `"time-card-approvers"` as an invalid UUID with 400 — a confusing status for "this feature isn't
configured." Registering it unconditionally and 404ing from inside the handler (when
`timecardApproverRoleIDs` is empty) gives a clean, correct status either way.

## Granting portal roles on user creation (`grantRoles`, `GET /roles/grantable`)

`POST /users`' own `roles` field only ever controls entity-service's `user_type` (internal/external) —
it has no connection to the Asgardeo-backed portal permissions `AUTH_<ROLE>_ROLES` grants
(`cs_engineer`, `escalator`, ...). Before this, there was no way for the Add User flow to also put a new
user into one of those roles; an admin had to do it by hand, outside the portal. `createUserRequest`'s
separate `grantRoles` field (portal role keys, e.g. `["cs_engineer"]`) closes that gap:
`CreateUser` resolves each key against `UsersHandler.grantableRoles` (set once at startup via
`WithGrantableRoles`, the same `[]handler.GrantableRole` `ResolveGrantableRoles` produces) up front —
before anything is created, so an unknown key is a clean 400 — then, once the entity-service user
exists, calls `scim.Client.AddRoleMembers(ctx, roleID, []string{req.Email})` once per resolved role.

**`grantRoles` never reaches entity-service.** `createUserRequest` is still parsed in full (so
`grantRoles` can be validated), but the body actually forwarded to entity-service is rebuilt from
scratch (`buildEntityCreateUserBody`) carrying only `firstName`/`lastName`/`email`/`roles` —
entity-service's own decoder rejects unknown fields, so the raw request body (which does carry
`grantRoles`) can't be forwarded unchanged the way most of this handler's other POST/PATCH bodies are.
Same "rebuild from what was actually validated" precedent `CreateCaseComment`'s own work_note rebuild
follows in `cases.go`.

**The SCIM grant is best-effort, after the fact.** By the time `AddRoleMembers` runs, the platform user
already exists — a SCIM failure (an unreachable SCIM service, a role the configured credentials can't
write to) is logged and does **not** fail the create response, the same posture `ensureUserProvisioned`
(`cases.go`) takes for the mirror-image direction of this same mechanism (provisioning a platform user
on demand for a worknote/escalation author who already holds the Asgardeo role but has no platform row
yet). A failed grant here leaves the user created but without the intended role — recoverable by hand,
unlike a failed *create*, which leaves nothing to recover.

**`GET /roles/grantable`** (`handler.GrantableRolesHandler`, admin-only — the same `PermAdmin` gate
`POST /users` itself sits behind) reports just the resolved portal role keys, never the real role
name/id behind them, so the webapp's Add User dialog can render a checkbox per grantable role without
ever learning anything about the identity provider's own vocabulary. The webapp maps each key to a
display label itself (`grantableRoleLabels.ts`) — this backend has no notion of a human-readable role
name at all.

**A SCIM 401/403 is never passed through to the caller as 401/403.** A failure fetching the role (e.g.
this backend's own OAuth2 app lacking a roles-read scope on `SCIM_SCOPES` — see the "Operational
follow-up" note on the PR that added the SCIM operations service's role endpoint) is this backend's own
credentials problem, not anything about the calling portal user's permissions — `mapUpstreamErrorGeneric`'s
usual 401/403 pass-through would otherwise tell an ordinary `viewer` "you don't have permission" for what
is really a deployment misconfiguration. `GetTimeCardApprovers` checks for those two codes specifically
and reports a sanitized 502 instead; every other SCIM failure status still goes through the normal
`mapUpstreamErrorGeneric` mapping.

## Provisioning a "user" row on demand (`ensureUserProvisioned`)

A `worknote_creator`- or `escalator`-only caller reaches `POST /cases/{id}/comments`
(the `!hasFullWrite` branch) or `POST /cases/{id}/escalations` purely on the strength
of an Asgardeo role grant — unlike the admin-only "Add User" flow above, neither
`AUTH_WORKNOTE_CREATOR_ROLES` nor `AUTH_ESCALATOR_ROLES` provisions a `"user"` row
anywhere. Without one, entity-service's own identity resolution for the write
(`emailFromJWT` → `GetUserByEmail`) fails it outright — a real gap for a caller whose
only path onto the portal is one of these two narrow roles, since neither is routed
through the entity-service-backed Salesforce membership ingest or the admin `POST
/users` flow.

`ensureUserProvisioned` (`internal/handler/ensure_user.go`) closes this: called
immediately before the entity-service write in both `CreateCaseComment`'s
worknote-only branch and `CreateCaseEscalation`, it calls `GetUserMe` first and, only
on a `404` (no row at all — any other failure is logged and treated as best-effort,
same posture as `caseIsClosed`'s own fail-open guard elsewhere in `cases.go`), creates
one via `POST /users` using `FirstName`/`LastName`/`Email` straight off the caller's
own validated token (`middleware.UserInfo`, which now also decodes `given_name`/
`family_name` — see that struct's own doc comment) and `roles: ["internal"]`. Both
work-note creation and escalation are internal-staff-only actions by construction
(their own `AUTH_<ROLE>_ROLES` grants are organisation-internal role names), so
`"internal"` is never a guess the way `CreateUser`'s admin-facing "User type" selector
has to make.

**`CreateCaseComment` skips calling this entirely for a caller who already holds full
`PermWrite`** (`cs_engineer` or `admin`, via the same `hasFullWrite` the
`PermCreateWorkNote`-narrowing check above already computes) — assumed already
provisioned, so the overwhelmingly common path (a CS engineer's own work notes/replies)
pays no extra `GetUserMe` round trip. `CreateCaseEscalation` has no equivalent skip:
`cs_engineer` never holds `PermEscalate` at all (see `NewAccessGuard`'s own doc
comment), so every caller who reaches that handler is, by construction, exactly the
audience this exists for.

## Security Center access (PermViewSecurityCenter)

Security Center (the webapp's Security reports + Vulnerabilities tabs) is restricted to `cs_engineer`
and `admin` only — every other role, even though it holds `PermView`, is denied. This needed two
different mechanisms, because the feature isn't backed by its own exclusive routes:

- **`/products/vulnerabilities/search` and `/products/vulnerabilities/{id}`** are genuinely
  Security-Center-exclusive, so they're gated the ordinary way: `route(..., handler.PermViewSecurityCenter, ...)`
  in `cmd/server/main.go`.
- **`POST /cases/search` and `POST /cases/aggregate`** are the shared, generic case endpoints every
  case-type tab and dashboard widget uses (Support, Operations sub-tabs, Engagements, Security
  reports) — both stay registered at `PermView`, since narrowing that route-level permission would
  lock out every other tab and widget too. Instead, `CaseHandler.SearchCases` and
  `CaseHandler.AggregateCases` inspect the request body themselves (via
  `CaseHandler.securityReportAccessDenied`): `caseSearchTargetsSecurityReports`
  (`internal/handler/cases.go`) reads the generic filter expression (`filters.filters[]`, and each
  `filters.anyOf[]` branch — the two request bodies share the same `filters` object) for a
  `{field: "type", op: "in", values: [...]}` predicate naming `security_report_analysis`, and if one
  is found, additionally requires `PermViewSecurityCenter` via `CaseHandler.access` (wired with
  `WithAccessGuard`, the same pattern `UsersHandler` uses) — a plain `PermView` caller gets 403
  instead of the request running. This only catches an *explicit* request for that type, the same
  way Security Center's own `caseTypes`-locked search (`CsmIssuesView`, webapp) always sends one; a
  hypothetical unfiltered "every case type" search that happens to also return security-report rows
  is a known, narrower gap, not handled here. For aggregate that includes `groupBy: "type"` with no
  type filter, whose response then carries a `security_report_analysis` count bucket; that is
  deliberately not special-cased either, since the same count can be had by subtracting the other
  types' totals. Every dashboard `security_report_analysis` widget carries that type filter
  (`injectImpliedTypeFilters`, `internal/dashboard/widgets.go`), so its search and its group-by
  aggregate both need `PermViewSecurityCenter`, same as the Security reports tab itself.
- **`GET /cases/{id}` has no equivalent check, deliberately.** `CaseView.type` (entity-service's own
  `openapi.yaml`) is only populated for ServiceNow cases — null on Postgres — so there is no reliable
  way for this handler to tell a security-report case apart from any other by inspecting the response
  alone, and a check that silently does nothing on one data source would be worse than no check at
  all (it would look like protection without being any). See `CaseHandler.WithAccessGuard`'s own doc
  comment for the full reasoning. Practically: since `SearchCases` is now locked down, a non-`cs_engineer`/
  `admin` caller can no longer *discover* a security-report case's id through the portal at all — the
  residual gap is a caller who already has one (a pre-existing bookmark, or a guess) fetching it
  directly by id. Closing that fully needs entity-service itself to resolve and enforce it (it has
  reliable type data on either data source), not this BFF layer.

## Redacting raw base64 inline images (`internal/handler/inline_image_redact.go`)

A pasted screenshot in a comment or case/incident/change-request description is normally extracted
into a real, `PermDownloadAttachment`-gated attachment on the way in (see
`internal/handler/inline_images.go`'s `InlineImageProcessor`, `SFTPGO_ATTACHMENT_STORAGE_ENABLED`
only) and rewritten to a `.iix` reference. Content authored before that flag was on — or with it off
— never goes through that extraction: the image stays as a raw `data:image/...;base64,...` `<img>`
src embedded directly in the comment/description HTML itself, which every read response already
returns to *any* caller holding `PermView` — there is no separate attachment resource for
`PermDownloadAttachment` to gate. Found live: a `viewer`/`escalator` role, neither of which holds
`canDownloadAttachment`, could see a pasted screenshot in a case comment despite the `.iix` mechanism
being correctly gated.

`redactRawBase64Images` strips the base64 payload out of raw response bytes (a compiled regex over
`data:image/...;base64,<payload>`, replaced with a short inert placeholder that still starts with
`data:image/` — the frontend's own `useResolvedInlineImageHtml` still recognizes and hides it, see
`apps/csm-portal/webapp`'s own `CLAUDE.md`) for a caller who fails `shouldRedactInlineImages` (no
`PermDownloadAttachment`, or `access == nil`, which fails closed the same way `CaseHandler`'s own
Security Center check does). It operates on the raw `[]byte` response — comment/description HTML
appears under different field names across endpoints (`content`, `bodyHtml`, `description`, ...) and
this backend already treats these responses as raw passthrough (see "Response shape" below); a
byte-level substitution keeps that convention and can't miss a field by name the way a typed reshape
could.

**Wired into every read response that can carry comment/description HTML**: `CaseHandler.SearchCases`/
`SearchCaseComments`/`SearchCaseActivities`/`GetCase`, `IncidentHandler.SearchIncidents`/`GetIncident`/
`SearchIncidentComments`/`SearchIncidentActivities`, `ChangeRequestHandler.SearchChangeRequests`/
`GetChangeRequest`/`SearchChangeRequestComments` — each calls `WithAccessGuard` at construction (same
pattern as `CaseHandler`'s own Security Center wiring) and checks `shouldRedactInlineImages(h.access,
user.Roles)` immediately before its final `writeJSON`. A *create* endpoint (`CreateCaseComment` and
its incident/change-request equivalents) is deliberately **not** redacted: the caller is the one who
just submitted that exact content, so echoing it back leaks nothing new to them.

This is the server-side half of a two-part fix — `apps/csm-portal/webapp`'s own `denyRawBase64`
mitigation (added first, still in place) only ever hid the image *after* the bytes had already
reached the browser; this is what stops them being sent at all to a caller who shouldn't see them.

## Change request create/patch validation (`internal/handler/change_requests.go`)

`POST /change-requests` and `PATCH /change-requests/{id}` forward their JSON body
to the entity service as-is (no field allow-list), with two checks on top:

* `POST` requires `type` of `standard`, `normal` or `emergency` (`validateChangeRequestCreateType`).
* **An Emergency change takes no customer step** (the previous system has no Emergency CAB and acts on an Emergency
  change without the customer's consent): a `POST` with `type: emergency` and either box true, a `PATCH` that
  turns a box on of an Emergency change, and a `PATCH {type: emergency}` that leaves a box ticked are refused by
  the entity service with a 400 ("Emergency changes proceed without customer consent, so customer approval and
  customer review cannot be required ..."), which reaches the form verbatim -- the BFF does not repeat the rule
  (`TestCreateChangeRequest_CustomerGateFlags`, `TestPatchChangeRequest_CustomerGateFlags`). An Emergency
  change's approvals are one `CAB Approval` stage (an in-flight one from an earlier build may read `ECAB Approval`).
* **Every body guard of the change request handlers reads keys the way the entity service's decoder
  does -- without regard to case.** The entity service decodes with `encoding/json`, which matches a
  key to a field case-insensitively, so `ISCUSTOMERAPPROVED`, `iscustomerreviewed` and
  `{"isCustomerApproved": null, "ISCUSTOMERAPPROVED": true}` are the flag there (a body that names a
  field twice is read as the last spelling reached, `null` included). A guard that looked up one
  exact key let them through; on `DATA_SOURCE=servicenow` the entity service forwards the flag. All
  the guards below (the two customer flags, the two requirement boxes, `type`, `projectId`,
  `deploymentIds`, `deploymentProductIds`, `customerGroupId`, `environmentIds`, `category`,
  `comment`, `workNote`) look a field up through `payloadValues`, which returns the value under
  **every** spelling of the key and judges all of them: the customer flags are refused when any
  spelling carries a value that is not `null` (the null/true pair in either order too).
  `TestPatchChangeRequestRefusesTheCustomersAnswer` and
  `TestChangeRequestBodyGuardsIgnoreTheCaseOfKeys`.
* **Compliance rule: nobody here records the customer's approval or review on the customer's
  behalf.** `PATCH` refuses (400, no upstream call) a body that carries `isCustomerApproved` or
  `isCustomerReviewed` with any value but `null` -- `true` or `false`, alone or with a state
  (`validateChangeRequestCustomerOutcomeFlags`): `isCustomerApproved cannot be set on the
  customer's behalf: the customer's approval can only be given by the customer in the Customer
  Portal` (likewise `isCustomerReviewed` / "review"). The customer gives it in the Customer
  Portal (`apps/customer-portal`); the answer is the customer's decision and the change request's record
  of it is audited. The state half -- no manual `{state: "scheduled"}` out of Customer Approval,
  no `{state: "closed"}` out of Customer Review -- needs the change request's state, so it is the
  entity service's refusal, echoed verbatim (below). There is no "Bypass customer approval" /
  "Bypass customer review".
* **A customer's proposed time is answered through the same `PATCH`.** The customer proposes a start in
  the Customer Portal; the change request stays in `customer_approval` and its detail carries
  `customerProposal` (`answer: "pending"`, the proposed `startOn` / `endOn`, `proposerRecorded` and the
  proposer when it is knowable, `canAccept` / `acceptBlockedReason`), passed through untouched. WSO2
  answers with `{confirmCustomerUpdatedDate: "agree", expectedCustomerUpdatedOn, expectedPlannedStartOn,
  expectedPlannedEndOn}` ("Accept proposed time": the change request is Scheduled with the proposal as
  its start, no CAB, no second ask) or `{state: "authorize", plannedStartOn?, plannedEndOn?,
  expectedCustomerUpdatedOn, ...}` ("Propose a different time" / a decline: DISAGREE, the customers
  asked again). The BFF forwards both unchanged and checks only that `confirmCustomerUpdatedDate`,
  `expectedCustomerUpdatedOn`, `expectedPlannedStartOn` and `expectedPlannedEndOn` are strings, in any
  spelling of the key (`validateChangeRequestTimeAnswerFields`: `<field> must be a string`, no upstream
  call); the route is `PermWrite` like every other PATCH, so there is no new permission. The entity
  service's refusals (409 stale proposal / window, 400 on hold, 409 the proposed start has passed, ...)
  are echoed verbatim; the compliance guard above is untouched (an Accept that also carries
  `isCustomerApproved` is refused as the customer's answer). `TestPatchChangeRequest_TimeAnswerFields`.
* Both accept the creation form's two checkboxes, **`customerApprovalRequired`**
  and **`customerReviewRequired`**, and refuse (400, "… must be a boolean (true
  or false)") any value that is not a JSON boolean, `null` included
  (`validateChangeRequestCustomerGateFlags`). Which transitions they enable, and
  until when they are editable, is the entity service's call — see its CLAUDE.md,
  "Customer Approval / Customer Review checkboxes". `PATCH` and `POST` echo the entity service's 400 message (`mapUpstreamError`), so its
  refusals ("customerApprovalRequired can no longer be changed …", "customer
  review is required …") reach the form verbatim (`POST` does the same, see below). Among them,
  the **customer requirements lock's nobody-to-ask refusal**: Request Approval (`{state:
  "assess"}`) on a change that has a customer box set and a Customer Project with nobody who can
  be asked (no registered portal-user contact other than the requester), and turning a box on
  after it, are a 400 `customer approval is required but nobody on this project can be asked (no
  registered contact other than the requester): register a contact for the project first`
  (`customer review is …` / `customer approval and customer review are …`); a change with no
  Customer Project at all keeps `approval cannot be requested: … no Customer Project is set …`.
  The BFF neither decides nor pre-checks any of it (it has no contact data; the entity service's
  `customerGroupCanBeAsked` is the one test, the same the customer stage's provisioning
  applies), so `TestPatchChangeRequest_CustomerGateFlags/surfaces_the_entity_service's_refusal_messages`
  pins that these words come through unchanged.
* Both shape-check the customer-scope and journal fields
  (`validateChangeRequestScopeFields`): `projectId` a UUID string; `deploymentIds`,
  `deploymentProductIds` arrays of UUID strings (at most 100, `null` is not an
  array); `category`, `comment`, `workNote` strings; on PATCH `comment`/`workNote`
  not blank. **`customerGroupId` and `environmentIds` are no longer accepted** —
  any value, `null` included, is refused at the BFF (no upstream call) with the
  entity service's own 400 text: `customerGroupId is no longer accepted: the
  customer group is derived from the customer project's registered contacts` /
  `environmentIds is no longer supported: deployments carry the environment`.
  Whether the deployments belong to the project, the deployment products match
  (they are read-only/derived) and the edit window (only before the change reaches
  `implement`) are the entity service's call — see its CLAUDE.md, "Customer
  project, deployments and deployment products".
* **`POST` now echoes the entity service's 400/409 message too** (`mapUpstreamError`,
  like `PATCH`): the form has to show "deploymentIds contains a deployment that does
  not belong to the selected project: …". 5xx and unmapped statuses still map to the
  generic "Failed to create change request." — upstream internals are never echoed.
* `POST /change-requests/link-options` (`PermViewOperations`) is the form's lookup:
  `{projectId, deploymentIds?}` → `{deployments:[{id,name,type}],
  deploymentProducts, customerContacts:[{id,name,email?}]}`. `projectId` is required
  (400 "projectId is required"). Project search for the picker is the existing
  `POST /projects/search`; the Customer Group is not searched or picked — it is the
  project's registered contacts, returned here as `customerContacts` (read-only).
  The body and the entity response are passed through untouched.
* The detail response carries `customerApprovalRequired`/`customerReviewRequired`
  and `legalNextStates` untouched; the webapp renders `legalNextStates` as-is. It also
  carries `project`, `deployments`, `deploymentProducts`, `customerContacts` (the
  derived, read-only Customer Group) and `category`.
* **Customer Group approvals (not decided here).** The change's customer group (the
  project's registered contacts) answers Customer Approval / Customer Review **in the
  customer portal**, not through this BFF: customers do not sign in to the CSM portal, and
  the decision route (`POST /change-requests/{id}/approvals/decision`) is `PermWrite`
  (cs_engineer / admin) for internal approvers. The stages "Customer Approval" / "Customer
  Review" still appear in `GET .../approvals` (see the entity service's CLAUDE.md,
  "Customer Group") so the Approvals tab can show who was asked and the outcome. While such a
  stage is live `legalNextStates` for Customer Review is just `["canceled"]` (Customer Approval
  keeps Re-schedule / Propose a different time: `["authorize", "canceled"]` -- the state does not move; Assess and Authorize offer `["canceled"]` only --
  the peer / CAB approval moves them on -- and the entity service accepts exactly the states
  `legalNextStates` offers, see its CLAUDE.md, "The transition graph of `PATCH {state}`": a change
  that is closed, canceled or rolled back cannot be moved by any request, and no request skips a
  step), and a manual `{state: "scheduled"}` /
  `{state: "closed"}` PATCH is a 400 -- live stage or not, whatever the project (see the entity
  service's CLAUDE.md, "There is no "Schedule" action") -- whose message is echoed
  verbatim (`state "scheduled" cannot be set manually from customer_approval: the customer's
  approval can only be given by the customer in the Customer Portal; cancel the change or
  re-schedule it instead`); a CSM user's decision on it is a 403 whose reason is shown
  (`mapApprovalDecisionError` already surfaces any 403 reason: `only members of the customer
  group (the registered contacts of this change request's project) can approve or reject …`).
  A rejected Customer Approval cancels the change, a rejected Customer Review moves it to
  `rollback`. `TestCustomerGroupApprovalMessages` pins both refusal messages as they are echoed,
  `TestPatchChangeRequestRefusesTheCustomersAnswer` the BFF's own refusal of the two flags.
* **A stale approval is a 409.** A decision on a stage whose state the change has left
  (a Review approver once the change is in Customer Review / Closed -- the entity
  service cancels such rows when the change moves on, and refuses a decision on one it
  missed; see its CLAUDE.md, "An approval is only actionable in its stage's state") comes
  back as a **409** `this approval is no longer pending: the change request is in
  Closed, but the Review stage can only be decided while it is in Review`.
  `mapApprovalDecisionError` passes the entity service's message through for a **409
  as well as a 403** on the decision endpoint (a 409 with no readable `message`
  envelope stays the generic 409); every other endpoint keeps the generic mapping.
  `TestDecideChangeRequestApproval` pins both ("a 409 carrying the entity service's
  reason shows it", "... without a readable reason stays generic"). `canDecide` is
  `false` on such a row, so the portal does not offer the buttons in the first place.

**The refusals' machine-readable `errorCode`.** entity-service names the refusals a client has
to tell apart with a stable `errorCode` string in its error body (`change_request_approval_not_pending`
for the 409 above, `change_request_not_asked` / `change_request_forbidden` for its 403s,
`change_request_on_hold`, `change_request_schedule_changed`, `change_request_not_proposable`,
`change_request_proposal_not_now`, `change_request_no_planned_window`, and for staff
`change_request_proposer_not_recorded` (Accept proposed time on a stored time nobody is recorded as having
proposed); the table is in its CLAUDE.md, "Error types"). The BFF passes it through, beside the message, with the status it
gives it: `mapUpstreamError` (the PATCH handlers) on the 400, 403, 409 and 422, and
`mapApprovalDecisionError` on the decision route's 403 and 409 (`errorBody.ErrorCode`, `writeErrorCode`,
`upstreamErrorCode`). The code is read from the upstream envelope (`apierror.Error.Body` holds it whole,
`maxEntityErrBody`) and kept only when it is a plain lower-case snake_case name of at most 64
characters, so nothing else reaches a client through it; a refusal with none (an older entity
service) adds no key, and `mapUpstreamErrorGeneric` (every other endpoint) never echoes it, as it never
echoes the message. The CSM webapp does not branch on it today (it keys on the 409 status); it is
there for the next client that has to. Pinned by `TestUpstreamErrorCodesPassThrough`.

## Opening an approval stage's assignment group (`GET /groups/{id}`)

`GET /change-requests/{id}/approvals` now carries `assignmentGroup: {id, name}` on
each stage (`null` for Customer Approval / Customer Review, whose approvers are the
project's registered contacts rather than a group); the response is still passed
through untouched. `GET /groups/{id}` (`GroupHandler.GetGroup`, `PermView` -- the same
level as `POST /groups/search` and `POST /users/search`, which already expose staff
names and emails) forwards to the entity service's `GET /groups/{id}` and returns its
`{id, name, description, email, manager, members: [{id, name, email, userType, role}],
total}` untouched.

* `id` is a **group** id (the `assignmentGroup.id` from the approvals response), not a
  team id -- `POST /groups/search` and `GET /teams/{id}/members` are the team registry.
  It must be a UUID (400 `ErrMsgInvalidUUID` otherwise, no upstream call).
* **Internal staff only.** The BFF does not widen it: entity-service refuses an
  external caller with 403 and the BFF returns that as 403 (`mapUpstreamErrorGeneric`);
  an unknown group is 404, and a group with no members is a 200 with `members: []`.
* PostgreSQL data source only (entity-service does not register the route otherwise).
* Declared in `openapi.yaml` (`/groups/{id}`, `GroupDetail`, and
  `ChangeRequestApprovalGroup` on `ChangeRequestApproval`), so
  `TestEveryRegisteredRouteIsInOpenAPI` does not list it. Tests: `TestGetGroup`,
  `TestGetChangeRequestApprovals_PassesAssignmentGroupThrough` (`groups_test.go`) and
  `TestGetGroupSendsGetToGroupsID` (`internal/entity/customer_client_test.go`).

## Health endpoints

Two, registered directly on the mux in `cmd/server/main.go` (not through `route()`) and both exempt
from `Auth` (`internal/middleware/auth.go` checks the exact path) — the same split entity-service
already established for its own `/health` vs `/health/database`:

- **`GET /health`** — pure liveness, always `200`, zero dependency calls. This is the one Choreo (or
  whatever orchestrator) should wire up as the restart/drain-triggering probe.
- **`GET /health/dependencies`** (`internal/handler/health.go`) — aggregates this backend's own
  upstream integrations: `"SCIM Service"`, `"Updates Service"`, `"CSM Notification Service"`,
  `"CSM Integration Service"`, `"Engineering Entity Service"` — the response's `name` values are
  human-readable display names, not package/env-var identifiers, so use these exact strings (not
  `csm-notification-service` or similar) when checking a caller against this response. This
  backend's own core `entity-service` is deliberately excluded (this backend depends on it for
  nearly every request; checking it here was an explicit product decision to leave out). Each
  listed dependency reports one of `ok`/`down`/`not_configured` — `not_configured` when that
  dependency's base URL env var is unset (`CSM_NOTIFICATION_SERVICE_BASE_URL`/
  `CSM_INTEGRATION_SERVICE_BASE_URL`/`ENGINEERING_ENTITY_BASE_URL`, all optional; SCIM/Updates are
  always configured so only ever report `ok`/`down`). Checks run concurrently, each bounded by its
  own 5s timeout so one slow upstream can't hang the whole response. Overall `status` is `degraded`
  (HTTP `503`) if any dependency is `down`; `not_configured` never counts as a failure on its own.
  **Never wire this one up as a
  liveness/restart probe** — a brief SCIM or Updates outage failing this endpoint must not have the
  orchestrator restart or drain an otherwise-healthy instance of this backend, the same reasoning
  entity-service's own `/health` vs `/health/database` split documents. The response body carries no
  failure detail (no error message, no upstream status code) since this route is unauthenticated and
  public, mirroring entity-service's own explicit "failure bodies carry no detail" convention for the
  same reason. **The result is cached for `dependencyHealthCacheTTL` (10s)**, and a cache-miss
  recomputation holds `HealthHandler.mu` for its full duration rather than just the read/write of the
  cached fields — a request that arrives mid-recomputation blocks on that same mutex instead of
  starting its own concurrent fan-out, collapsing every request within one TTL window onto a single
  set of upstream calls (the same effect `golang.org/x/sync/singleflight` would give, without adding
  it as a dependency for one call site). This exists because the route has no auth check: without it,
  an unauthenticated caller repeating the request could trigger unbounded concurrent calls to every
  configured upstream on every hit. The recomputation runs against `context.Background()`, not the
  triggering request's own context, since it may be serving several other callers besides the one
  that started it — a client disconnect must not cancel a check every queued caller is waiting on.

## Upstream service modules

Each upstream service has its own client package under `internal/`:

| Package | Upstream | Notes |
|---------|----------|-------|
| `entity` | Multiple entity services (see below) | Hosts `CustomerEntityClient` (this repo's entity-service; most case/account/project endpoints, raw `[]byte` passthrough) and `EngineeringEntityClient` (a separate internal engineering entity service; `CreateGitIssue`, typed request/response, plus `Health(ctx)` backing `GET /health/dependencies` — see "Health endpoints" above). `EngineeringEntityClient` is constructed in `cmd/server/main.go` only when `ENGINEERING_ENTITY_BASE_URL` is set, and then `CaseHandler.CreateCaseGithubIssue` uses it (via `WithEngineeringClient`) instead of the entity service: the target is the case product's `product_repo_mapping` row (looked up through the entity service's `GET /products/github-repo`; `repoOverride` is ignored), and after filing it writes the issue URL to the case as a best-effort work note |
| `scim` | SCIM service | User/group/role lookups. Two orgs: `SearchUser` queries the "internal" org (WSO2 staff — phone number, last password update). `SearchExternalUser` queries the "external" org (customer/partner contacts — existence + lock status, mirroring `infra-operations/operations/asgardeo-user-check`'s `{exists, locked}` contract). `GetUser` calls the latter only when the entity response's `userType` isn't `internal`, and treats a lookup failure as best-effort — logged, response returned unchanged, never a failed request. `GetRole(ctx, roleID)` fetches an Asgardeo role's real member users (internal org only) via the SCIM operations service's `GET /organizations/internal/roles/{id}` — a plain get-by-id, not a search, so the caller supplies the role's own Asgardeo id directly (see `ASGARDEO_ROLE_IDS` below); a member's SCIM `display` (`"<domain>/<email>"`, e.g. `"DEFAULT/jane@wso2.com"`) is unwrapped to a bare email |
| `updates` | Updates service | Product update levels; returns typed structs (not raw passthrough) |
| `csmnotification` | `integrations/csm-notification-service` | Health check only today (`Health(ctx)`, backing `GET /health/dependencies` — see "Health endpoints" below). Optional: unconfigured (`CSM_NOTIFICATION_SERVICE_BASE_URL` unset) means this dependency reports `not_configured` |
| `csmintegration` | `integrations/csm-integration-service` | Same shape and same one purpose as `csmnotification` above, for `integrations/csm-integration-service` |

New upstream services get their own package under `internal/` following the same `Client` + `do()` pattern. `entity` is the exception: because it hosts multiple, separately-deployed/differently-authenticated services, it uses a `<Name>Config`/`<Name>Client` pair per service/file instead of one shared `Client` for the whole package — `CustomerEntityClient`/`EngineeringEntityClient`.

**Notifications (email, Google Chat, and future channels like SMS/voice-Twilio) have moved out of this backend** into `integrations/csm-notification-service`, a standalone Go service invoked over HTTP. This backend no longer constructs or calls any notification client directly, and no longer publishes domain events to Azure Event Hub either — that responsibility (case.created/case.acknowledged/case.severity_changed/incident.created, and the like) now belongs entirely to `entity-service`, the one publisher of the `case-events` topic. This backend previously had its own producer-side pipeline (`internal/eventbus`/`internal/events`/`internal/eventpublisher`, publishing `case.comment_added`/`case.status_changed` from `CaseHandler`) — it was removed entirely to avoid two independent services writing to the same topic. Per-recipient portal-link resolution (which of the customer/CSM portal a given recipient's email lands on) **does not live in this backend** either — it's in `csm-notification-service`'s own `internal/recipientlinks`/`internal/entity`, since that's the service actually composing and sending the email.

**Shared OAuth2 credentials**: `CustomerEntityClient` and `EngineeringEntityClient` (both in `entity`), `updates`, `scim`, and PLG's `internal/plg/entityclient` all authenticate as the same OAuth2 client-credentials app — `cmd/server/main.go` reads `OAUTH2_CLIENT_ID`/`OAUTH2_CLIENT_SECRET`/`OAUTH2_TOKEN_URL` once and passes them into every service's `Config`; only `<SERVICE>_BASE_URL`/`<SERVICE>_SCOPES` are per-service. `EngineeringEntityConfig` still has its own `ClientID`/`ClientSecret`/`TokenURL` fields (matching every other service's `Config` shape), but when it's wired into `main.go` those should be filled with the same shared `oauth2ClientID`/`oauth2ClientSecret`/`oauth2TokenURL` values, not new `ENGINEERING_ENTITY_*` env vars. Follow this pattern for any new upstream service client unless it genuinely uses a different OAuth2 app. PLG is the one client that **inherits** rather than being passed a `Config` per field: `main.go` hands `plg.Mount` a `plgconfig.EntityDefaults` carrying `CUSTOMER_ENTITY_BASE_URL` and the three shared `OAUTH2_*` values, because PLG reaches the same entity-service as `CustomerEntityClient` and a second copy of those settings is one that goes stale on the next secret rotation. `PLG_ENTITY_BASE_URL`/`PLG_ENTITY_OAUTH_*` exist as overrides and are normally unset.

**Request timeouts**: `REST_READ_TIMEOUT` / `REST_WRITE_TIMEOUT` (default 60s) and `ENTITY_SERVICE_TIMEOUT` (default 60s, applied to `CustomerEntityClient` only) are parsed by the pure `loadTimeouts` in `cmd/server/timeouts.go`; all must be > 0, else startup fails; no ordering between them is enforced (client timeout < write timeout is advisable, not required). Defaults were raised from 30s/25s for large inline-attachment uploads. Other upstream clients (engineering, updates, SCIM, notifications, SFTPGo) keep their own fixed timeouts.

## Running locally

```bash
# from apps/csm-portal/backend
go run ./cmd/server/main.go
```

The server auto-loads `.env` from the working directory at startup (silently ignored if absent). No need to `source .env` manually.

`cp .env.example .env` is enough to start: `DASHBOARDS_DIR` points at the committed `dashboards.example/` (a missing directory is fatal). For a real dashboard set, `cp -r dashboards.example dashboards` and repoint it — `./dashboards` is gitignored.

`CSM_TEAM_REGISTRY` (team vocabulary: `teamKey|Display Name|FAMILY|groupId` rows, comma separated) and `CSM_USER_ROLES` (assignable-role allow-list) are read **here**, not in `entity-service` — they moved. Both are resolved once at startup into `internal/directory`, so `POST /teams/search` and `POST /roles/search` make no upstream call. A malformed row, an unknown family, or a duplicate team key/display name is fatal at startup, naming the row. Both are flat single-line strings on purpose: the deployment platform's configuration UI is one-dimensional and stringifies nested collections, so a structured registry cannot be deployed at all. Do not introduce nested-collection configuration.

## Feature flags: `CSM_MIGRATION_*`

Flags named `CSM_MIGRATION_<FEATURE>_ENABLED` gate customer-onboarding cutover features. Each is **off unless its value is exactly `true`** (whitespace-trimmed — `1`, `TRUE`, `yes` are all off), deliberately stricter than the `strconv.ParseBool` leniency `SFTPGO_ATTACHMENT_STORAGE_ENABLED` has: these are cutover switches that must never flip on by accident. Off means **no behaviour change at all**: the handler is not constructed and the route is not registered in `cmd/server/main.go`, so the path 404s like any unknown one. Resolve a flag with a `load<Feature>Enabled()` in `main.go` backed by a pure `<feature>Enabled(raw string) bool` that `main_test.go` table-tests.

- `CSM_MIGRATION_ONBOARDING_STATUS_ENABLED` → `GET /projects/{id}/onboarding-steps` (`OnboardingStepHandler`, `internal/handler/onboarding_steps.go`, `PermView`). It pages through the entity service's onboarding ledger (`CustomerEntityClient.SearchOnboardingSteps`, `internal/entity/onboarding.go` — a **typed** method, unlike the raw-passthrough rest of `customer.go`, because the handler regroups rows) with `filters.projectId`, at the upstream cap of 50 rows a page and at most `maxOnboardingStepPages` (40) pages, then regroups the flat newest-first rows into one entry per membership (invited email) with steps in flow order IDENTITY → DATABASE → EMAIL → REGISTRATION. It reports only what the ledger row holds (`step`, `status`, `attemptCount`, `lastError`, `eventType`, `eventModifiedOn`, `updatedOn`) — nothing derived or enriched; `lastError` is upstream error text and is passed through untouched for the webapp to render as plain text. Hitting the page cap sets `truncated: true` and logs a warning rather than failing. The webapp matches memberships to the project's contact rows by lower-cased email because `ProjectContact` carries no membership or `project_contact` id.

## Commands

```bash
make setup   # wire up git hooks (once after clone)
make test    # vet + race-detector tests
make build   # runs tests then compiles ./cmd/server
```

Tests run automatically on `git push` via the pre-push hook.

## Adding a new endpoint

Follow these steps in order:

1. **Upstream client** (`internal/<module>/`) — add a method on `Client` that calls `c.do()`; use `url.PathEscape()` for every path parameter
2. **Handler interface** — extend the local interface in the relevant handler file (e.g. `entityCaseClient` in `cases.go`); keep it minimal — only methods that handler actually calls
3. **Handler func** — auth check → path/body guards → call client → `mapUpstreamErrorGeneric` on failure (see Handler conventions below for the one PATCH-handler exception) → write response
4. **Route** (`cmd/server/main.go`) — register with `route(...)` using Go 1.22 method-prefixed patterns and the permission the caller needs (see Access control): `route("POST /cases/{id}/comments", handler.PermComment, caseHandler.CreateCaseComment)`
5. **OpenAPI spec** (`openapi.yaml`) — add the path with 200/400/401/403/404/500 responses; `403` is always required because `mapUpstreamError`/`mapUpstreamErrorGeneric` can return it
6. **Tests** — add handler tests; update the mock in `helpers_test.go` to satisfy the extended interface
7. **gosec** — run `gosec -fmt=text ./...` (see README's Security Scanning section) before opening the PR; it must report 0 issues

## Handler conventions

- **Auth**: always check `middleware.UserInfoFromContext(r.Context()) == nil` first → 401
- **Body size**: cap with `http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)` (1 MiB) before reading
- **Path params**: guard against empty string after `r.PathValue("id")`; if the param is a UUID, also validate format using the package-level `uuidRe` compiled regex and return 400 on mismatch — fail fast before calling the upstream
- **Field naming**: case create/patch use bare names without `Key`/`Keys` suffix — `state`, `severity`, `workState`, `type`, `engagementType`, `catalogId`, `catalogItemId`, `variables` (PATCH — the latter four only meaningful transferring into `engagement`/`service_request` respectively), `type`, `severity`, `issueType` (POST); search filters use `states`, `severities`, `types`, `issueTypes`, `engagementTypes`; deployment search uses `deploymentTypes`; case comments use `type` (not `typeKey`); case create accepts `type: "case"`, `"service_request"`, or `"security_report_analysis"` (ServiceNow only for the latter two); case-type transfer via PATCH accepts `type: "case"`, `"engagement"`, `"security_report_analysis"`, or `"service_request"` (ServiceNow only)
- **Deployment ID injection**: two helpers exist in `deployments.go` — `injectDeploymentID` (injects `deploymentIds: [id]` array, used by search) and `injectDeploymentIDField` (injects `deploymentId: id` string, used by create/update). Use the correct one for the endpoint's upstream contract.
- **Upstream errors**: use `mapUpstreamErrorGeneric(w, err, "<fallback message>")` for every endpoint by default — never write custom status mappings inline. Only the ten PATCH/update handlers (`PatchCase`, `PatchCallRequest`, `PatchMe`, `UpdateProject`, `PatchDeployment`, `PatchDeployedProduct`, `PatchChangeRequest`, `UpdateTimeCard`, `UpdateTask`, `PatchIncident`) use `mapUpstreamError` instead, which surfaces the upstream 400/409/422 reason (e.g. "Invalid state transition") — appropriate there because the request body just submitted is what's being rejected. Every other endpoint (search/create/get/delete) forwards a payload that's only partially validated at this layer, so a 4xx from upstream isn't reliably something the caller could have avoided; `mapUpstreamErrorGeneric` returns the fixed fallback message for those instead of echoing upstream detail. Both log the full reason via the caller's `slog.ErrorContext(ctx, ..., "err", err)` regardless of which is used.
- **`GetMe` maps an upstream 404 to 403, not 404.** A 404 here means the caller's own authenticated identity has no backing user row — not a missing resource the caller asked for by ID — and the webapp's data-fetching hook for this endpoint has no handling for a bare 404, so it spun forever instead of showing anything. Since "you don't have permission" is already a handled UI state, `GetMe` treats this upstream 404 as 403 for the response while still logging the real cause at `ERROR` (without the caller's email, to stay off the PII list above). Follow this same substitution if another identity-bound "fetch my own X" endpoint hits the same failure mode.
- **Response**: return raw `[]byte` with `writeJSON` for simple passthroughs; unmarshal into typed structs only when the response shape needs to change

## OpenAPI spec

**`openapi.yaml` must be updated whenever the API changes** — new endpoints, removed endpoints, changed request/response shapes, new error codes. It is the contract consumed by the frontend and other teams; an out-of-date spec is worse than no spec.

- Error responses use `$ref: '#/components/schemas/ErrorPayload'`
- Every endpoint must declare a `403` response
- Path parameters that expect UUIDs must declare `format: uuid` on the schema
- The `Case` schema includes a computed `nextStates` read-only field populated server-side from `state`
- Binary-download endpoints (e.g. attachment content) must document the `Content-Disposition: attachment` and `X-Content-Type-Options: nosniff` security headers in their description

## Response shape

- For **portal-owned/transformed** responses (typed structs constructed by the portal), all JSON fields must use **camelCase** (e.g. `createdAt`, `projectId`, `issueType`); use `json:"fieldName"` struct tags to enforce this
- **Raw passthrough** responses may retain upstream field naming as-is — do not reshape them unless there is an explicit requirement to do so
- The `nextStates` field is portal-constructed and follows camelCase like all other portal-owned fields

## Security

- **Never commit secrets** — API keys, tokens, passwords, and service URLs with credentials must not appear in source code or config files; use environment variables
- **No sensitive data in logs** — do not log request bodies, JWT payloads, or user PII; log only IDs and error summaries
- **JWT is the only auth mechanism** — all endpoints must validate the caller via `middleware.UserInfoFromContext`; there are no public endpoints
- **Audience** — `Config.Audiences` is `[]string`; a token is accepted if its `aud` claim contains **any** of the configured values (OR logic). Set via `AUTH_AUDIENCE` as a comma-separated string
- **Input validation** — validate and reject unexpected input at the boundary (path params, body size, JSON structure) before forwarding to upstream services
- **Error messages** — never leak upstream error details or stack traces to the caller; use the fixed `ErrMsg*` constants or a short fallback message. This is what `mapUpstreamErrorGeneric` enforces by default; see the Handler conventions section for the narrow PATCH-handler exception that uses `mapUpstreamError` instead
- **Security fixes in PRs** — when a change is made to fix a security issue (gosec findings, input sanitization, etc.), do not mention it in the PR title or description; describe the change in neutral functional terms only
- **Run gosec on every backend change** — `gosec -fmt=text ./...` (install once: `go install github.com/securego/gosec/v2/cmd/gosec@latest`) must report 0 issues before opening a PR touching this backend; fix the root cause of any finding rather than suppressing it, unless a `#nosec` annotation with a justification comment already covers that exact case
- **Run govulncheck on every backend change** — `govulncheck ./...` (install once: `go install golang.org/x/vuln/cmd/govulncheck@latest`) must report no vulnerabilities before opening a PR touching this backend. Most findings here are Go standard-library CVEs tied to the toolchain patch version pinned in `go.mod`'s `go` directive — bump it to the latest `1.26.x` patch (and run `go mod tidy` so the toolchain download matches) rather than working around the symptom. A finding in a third-party module is fixed with `go get <module>@<fixed-version>`

## Testing

- Mocks live in `internal/handler/helpers_test.go` — when you extend a handler interface, add the new field and method to the mock there
- `upstreamErrors(fallback)` is the error table for the ten `mapUpstreamError` PATCH-handler tests (surfaces the upstream 400/409/422 reason); `upstreamErrorsGeneric(fallback)` is its counterpart for every other handler's tests, which call `mapUpstreamErrorGeneric` and always expect `fallback` for those statuses instead
- `withUser()` injects a test user into the request context
- `decodeJSON[T]()` decodes response bodies in assertions
- Use real UUIDs (e.g. `"11111111-1111-1111-1111-111111111111"`) for UUID path param test values — not fake slugs like `"case-1"`
