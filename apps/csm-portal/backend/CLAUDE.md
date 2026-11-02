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
- **Every route goes through `route(pattern, perm, handler)` in `cmd/server/main.go`.** `perm` is a required argument with no default — pick `PermView` for reads/searches/aggregates, `PermViewOperations` for reads under the Operations area (incidents, change requests, problems, incident tasks, outages, alerts), `PermTimeCardsAndUpdates` for every time-card route and the update-level lookups (CS engineer, admin and time-card approver — viewing/managing, not approving, see `PermApproveTimeCard` below), `PermWrite` for other state changes, `PermAdmin` for the handful of actions reserved for admin alone (`POST /users`, creating a new platform user, and `POST /teams/{id}/lead`, reassigning a team's lead), `PermViewSecurityCenter` for Security Center (both `/products/vulnerabilities/*` routes; see "Security Center access" below for how `/cases/*` is handled), `PermViewCustomerHealth` for the Customer Health dashboard (`/customer-health/*` plus the `GET /products` and `GET /abt-teams` lists its filters read; Viewer, `cs_engineer` and `admin` — see its own note below), or one of the narrower ones (`PermEscalate`, `PermDownloadAttachment`). `PermApproveTimeCard` is not a route-level permission at all — see its own note below. Creating a case, incident or change-request comment is `PermWrite` (or `PermCreateWorkNote` for a work note — see below); editing or deleting one by id (`PATCH`/`DELETE /comments/{id}`) is `PermUpdateDeleteComment` — see its own note below. `PermAuthenticated` (no role needed) is only for the caller's own `/users/me`. `/health` is the only route registered directly on the mux.
- **The policy is `NewAccessGuard`.** `admin` satisfies every permission, including `PermAdmin`; `cs_engineer` satisfies every other route permission (view, download, write, including comments, security center, time cards and updates) but **not** `PermAdmin`, `PermEscalate`, or `PermApproveTimeCard` — escalating a case and approving a time card are each a dedicated responsibility, held only by their own role (`escalator` / `timecard-approver`) plus admin, the same way `PermAdmin` is admin-only; `escalator`/`attachment-downloader` grant only their one ability plus view (so a view-only role cannot read Operations — `PermViewOperations` is CS engineer and admin only, matching the frontend's `canUseOperations`); `timecard-approver` grants view plus `PermTimeCardsAndUpdates` plus `PermApproveTimeCard`; `usage-metrics-viewer` also grants `PermUsageMetricsViewer` (the Usage Metrics domain, `/usage-metrics/*` — see that permission's own doc comment in `access.go` for why it's gated directly rather than layered on `PermViewerAccess` like its Viewer-only ex-"Support Portal Lite" siblings (User Scan, the Time report), and why plain `viewer` deliberately does NOT hold it); `dashboard-designer` grants only view here (no backend route for that feature); the frontend gates it. Every role implies view, **except** `PermViewSecurityCenter` — a plain viewer/escalator/attachment_downloader/usage_metrics_viewer/timecard_approver/dashboard_designer holds `PermView` but not this.
- **Customer Health has its own audience: `PermViewCustomerHealth` (Viewer, `cs_engineer`, `admin`).** The dashboard was built for the viewer audience, but CS engineers and admins work the same accounts, so every `/customer-health/*` route, plus the `GET /products` and `GET /abt-teams` lists its filters read, is registered on this permission and the same handlers check it themselves (`requireCustomerHealthAccess`, a sibling of `requireViewerAccess` over one shared `requirePermission`). It is deliberately **not** a widening of `PermViewerAccess`: User Scan, the Time report, the account escalation list and Drive files stay Viewer-only, and `access_test.go` pins both directions. The webapp's `canViewCustomerHealth` (`portalAccess.ts`) must stay in step with this set.
- **`PermApproveTimeCard` gates a state transition inside a shared route, not a route of its own.** `PATCH /time-cards/{id}` carries EITHER a plain field edit OR an approve/reject transition (`state: "approved"`/`"rejected"`, entity-service's `UpdateTimeCardRequest`) — the same shared-endpoint problem Security Center's `POST /cases/search` has, solved the identical way: `TimeCardHandler` (wired with `WithAccessGuard`, same pattern as `CaseHandler`) inspects the request body itself (`timeCardUpdateTargetsStateTransition`, checking for a non-nil `state`) and additionally requires `PermApproveTimeCard` only when it's present. A CS engineer without `timecard-approver`/`admin` can still search/create/edit their own time cards (`PermTimeCardsAndUpdates`), just not approve/reject one.
- **`PermCreateAnnouncement` narrows `PermWrite`, it does not replace it.** Creating and sending a customer announcement needs the `announcement_creator` role (`AUTH_ANNOUNCEMENT_CREATOR_ROLES`) or `admin`, **in addition to** the caller's own `PermWrite`: a `cs_engineer` without the role keeps every other write and loses this one, and a holder of the role who cannot write still cannot send anything. The role implies nothing else, not even view (same shape as `worknote_creator`). Two mechanisms, because the feature is not backed by routes of its own: (1) every write on `/announcement-requests` (create, edit, dry run, submit, schedule, publish, add an update, record deliveries) is registered with `routeAll(pattern, handler, handler.PermWrite, handler.PermCreateAnnouncement)`, which goes through `AccessGuard.RequireAll` (every permission must be held, an empty list denies); the request LIST (`POST /announcement-requests/search`, the webapp's Requests tab: drafts and requests awaiting approval) is the creators' workspace and is registered the same way, while reading ONE request and its updates and deliveries stays `PermView` because a published announcement in the main list opens the same request dialog; (2) `POST /cases` is the one generic create route and makes an announcement (the dry-run case and every per-project case of a publish) as readily as a support case, so `CaseHandler.CreateCase` additionally requires `PermCreateAnnouncement` when the body asks for `type: announcement` (`caseCreateTargetsAnnouncement`, a token scan of the top-level keys so every spelling and every repeat of `type` is judged; a nil guard fails closed, like the Security Center check). **Approving a request ("Mark as approved") is deliberately not gated**: it only records a decision taken over email and may be recorded by someone other than the creator. An unset `AUTH_ANNOUNCEMENT_CREATOR_ROLES` means only `admin` can create announcements, so startup warns about it and it must be set (to the Asgardeo role the announcement creators hold) before this ships to an environment. `cmd/server/announcement_routes_test.go` pins how each `/announcement-requests` route is registered, and `openapi_routes_test.go` reads `routeAll` registrations too. Not covered here: posting an update as ordinary case comments (`POST /cases/{id}/comments`) is the normal comment route and unchanged.
- **`PermUpdateDeleteComment` widens the `PATCH`/`DELETE /comments/{id}` route floor, it does not narrow it.** Unlike `PermCreateAnnouncement`, which removes an ability from `cs_engineer` by requiring a second permission alongside `PermWrite`, this is the same shape as `PermCreateWorkNote`: `comment_updater` (`AUTH_COMMENT_UPDATER_ROLES`) is a new, additional way to reach the handler, on top of `cs_engineer`/`admin`'s existing access — a `cs_engineer` keeps editing/deleting comments exactly as before. This permission only decides who may reach the handler at all; it says nothing about which specific comment they may act on — that finer-grained decision is `PermUpdateDeleteAnyComment`, below. An unset `AUTH_COMMENT_UPDATER_ROLES` is a normal, supported state (same as `AUTH_WORKNOTE_CREATOR_ROLES`): `cs_engineer`/`admin` already hold this permission regardless, so leaving it empty just means the role isn't provisioned yet, never a lockout.
- **`PermUpdateDeleteAnyComment` is the in-handler check that decides whether THIS caller may act on a comment they did not author — entirely inside this backend, nothing forwarded to or re-checked by entity-service.** Held by `admin` and `comment_updater` ONLY — deliberately NOT `cs_engineer`, unlike `PermUpdateDeleteComment` above. `CommentHandler.authorizeCommentActor` (called from both `UpdateComment` and `DeleteComment`) calls `entityCommentClient.GetComment` first, purely to learn the comment's author, then allows the request through when either the caller's own email matches that author (case-insensitively) or `h.access.Permits(PermUpdateDeleteAnyComment, user.Roles)` is true — otherwise it writes a 403 itself and never calls entity-service's `UpdateComment`/`DeleteComment` at all. entity-service's `comment_service.go` performs **no author or role check of its own** on `GetComment`/`UpdateComment`/`DeleteComment`/`GetCommentEditHistory` any more — not even its own former Postgres `admin`-role lookup, dropped as redundant with (and a source of confusion against) this backend's own Asgardeo roles, which is the only caller of this path. Net effect: a `cs_engineer` can still only successfully edit/delete a comment they themselves authored; `comment_updater`/`admin` can act on any comment, decided once, here.
- **`PermManageOnboarding` is checked in-handler on `PATCH /projects/{id}`, behind a wider route floor, `PermUpdateProject`.** Held by `onboarding_lead` (`AUTH_ONBOARDING_LEAD_ROLES`) and `admin` ONLY, not `cs_engineer`. `PermUpdateProject` (`cs_engineer` ∪ `admin` ∪ `onboarding_lead`) only lets an `onboarding_lead` reach the handler; `ProjectHandler.UpdateProject` then reads the body's keys (case-insensitively, like the `closureState` check) and requires `PermManageOnboarding` for `onboardingStatus`, `onboardingOwner` and the four onboarding dates (`onboardingStartDate`, `onboardingGoLivePlanDate`, `onboardingGoLiveDate`, `onboardingExpiryDate`; the `projectOnboardingKeys` set) and `PermWrite` for any other field, so an `onboarding_lead` alone can never edit anything else and a `cs_engineer` alone can never set the onboarding fields or dates. `closureState` stays `PermAdmin`-only. The role is config-only and grants no read permission. A nil guard fails closed. An unset variable is a normal state (admin still holds the permission), so startup does not warn.
- **PLG's routes go through this guard too, and through a second one of their own.** `internal/plg/plg.go` registers all 24 as `accessGuard.Require(perm, identity(fn))` — the guard OUTSIDE PLG's `identity` middleware, deliberately, because the guard is a set lookup while `identity` is an entity-service round trip, so a denied caller costs nothing upstream. Two permissions: `PermUsePlg` (CS engineer and admin) for 20 routes, and `PermManagePlaybooks` (admin only) for the four that author a playbook template. `PermUsePlg` is narrower than `PermView` on purpose — every portal role holds `PermView`, but PLG is a worklist staff act on, and a view-only role that could open it would meet a 403 on every control. Reading a playbook (`GET /plg/playbooks`, `GET /plg/playbooks/{id}`) and assigning one to a pairing (`POST .../playbook-runs`) are `PermUsePlg`, NOT `PermManagePlaybooks`: running a template and writing one are different jobs, so gating on the `/plg/playbooks` path prefix would be wrong. `PermManagePlaybooks` is kept separate from `PermAdmin`, which it currently matches exactly, so granting playbook authoring to a future PLG-admin role does not also hand out platform-user creation. `identity` still runs and is not redundant: it resolves the `"user".id` every PLG write records as `actorId`, and refuses anyone who is not ACTIVE INTERNAL staff — a roles claim cannot tell you somebody was offboarded this morning. `internal/plg/routes_access_test.go` pins every route's permission and the middleware ordering.
- **`GET /users/me` reports `roles`** (from `AccessGuard.RolesFor`): the stable keys of the portal roles the token roles grant — several possible, fixed order — and is **not** the entity service's role data, which the response no longer carries. The frontend decides what to show or hide from these roles (there is deliberately no derived `permissions` list); the backend's `403` is the real gate. Dashboard-designer access is only the `AUTH_DASHBOARD_DESIGNER_ROLES` role (the old `DASHBOARD_DESIGNER_EMAILS` email allow-list is gone).
- **`GET /users/{id}` reports a target's CSM portal-role vocabulary alongside entity-service's own, via SCIM instead of a JWT -- and matches by role ID, not by display name.** `GetUser`'s own `withPortalRoles` (`internal/handler/user_portal_roles.go`) reaches the same vocabulary for the profile being *viewed* in two SCIM calls: `scim.SearchUser` by the target's email to resolve their own Asgardeo user ID (there is no other lookup key available here -- entity-service stores no SCIM id), then `scim.GetUserByID` by that ID for the target's full, canonical role assignment list. The result lands in a separate `csmPlatformRoles` field -- this used to overwrite the response's own `roles` outright for an internal target, but that read as though the Customer Portal's own role vocabulary (`admin`/`commenter`/`customer`/`customer_admin`/`partner`/`partner_admin`/`external`/`internal`/`timecard_approver`/`agent`, the `CSM_USER_ROLES` set -- a genuinely different, unrelated vocabulary from the CSM-portal-access one, not something SCIM has any equivalent for) had disappeared for that person, and left the whole section looking empty for a target holding no configured Asgardeo role at all. Both vocabularies describe real, different things about the same person, so neither hides the other. Things worth knowing:
  - **Why `GetUserByID`, not `SearchUser`'s own `roles` field (which a search response does carry too).** A `/Users/.search` result only returns whichever attributes the caller requested, where a direct `GET /Users/{id}` is the canonical resource read and always returns the complete representation. The decisive reason to pay for the second round trip: matching a held role correctly needs each role's own Asgardeo ID, not just its display name, and two distinct roles can share a display name -- a name-based match can silently credit (or deny) the wrong one. `scim.scimUserByID`/`scimRoleAssignments` (`internal/scim/types.go`) decode the GET-by-id response keeping both `value` (ID) and `display` (name) per role, unlike the plain-search decoder `scimRoles`, which only ever kept `display`.
  - **Matching is by ID, via `AccessGuard.RolesForIDs`, not `RolesFor`.** `RolesFor` (what `GetMe` still uses, unchanged, on the JWT `roles` claim's plain role-name strings) matches a *name* against `AUTH_<ROLE>_ROLES`; `RolesForIDs` matches a *role ID* against `ASGARDEO_ROLE_IDS`, resolved once at startup via `AccessGuard.WithRoleIDs(roleIDsByName)` (`cmd/server/main.go`) -- which cross-references each portal role's own configured `AUTH_<ROLE>_ROLES` name(s) against `ASGARDEO_ROLE_IDS`'s name-\>id map to build an internal id-\>portal-role index. A role with no entry in `ASGARDEO_ROLE_IDS` simply never matches here, the same "no entry means not wired up" posture that config already has for its other two consumers (`GetTimeCardApprovers`, grantable roles on user creation) -- so a deployment that has never populated `ASGARDEO_ROLE_IDS` for a given role gets no `csmPlatformRoles` entry for it, same empty-array outcome as a person holding no role at all.
  - SCIM's role list spans every Asgardeo application the person holds a role in, not just this portal, so `withPortalRoles` hands the whole, unfiltered list straight to `RolesForIDs` -- a role belonging to another application has no configured ID here either, so it never matches anything, no separate narrowing step needed or safe to add. An earlier version first filtered SCIM's roles to a hardcoded prefix (`scim.CSMAppRolePrefix`) before calling `RolesFor` by name, guessed at rather than confirmed against this deployment's real Asgardeo role-naming convention -- found live to not match it at all, silently dropping every role and never resolving anyone's `csmPlatformRoles`. Matching against configuration (by name for `RolesFor`, by ID for `RolesForIDs`) rather than a guessed shape is what rules out that whole class of failure.
  - **A portal role can also be granted by Asgardeo GROUP membership, matched by the group's own display NAME -- via `AccessGuard.RolesForIDsAndGroupNames`, a second, independent path alongside the role-ID matching above, not a replacement for it.** Some deployments assign a portal role (e.g. `cs_engineer`) to whoever belongs to a specific Asgardeo group, rather than (or in addition to) assigning a dedicated role. `scim.GetUserByID`'s response already carries the target's `groups` array alongside `roles` (same canonical GET-by-id read -- see `scim.scimGroupAssignments`, mirroring `scimRoleAssignments`' own `{value,display}`-shape decoding, so no second SCIM call is needed), and `ASGARDEO_ROLE_GROUP_NAMES` (`internal/directory.ParseRoleGroupNames`, wired via `AccessGuard.WithGroupNames(groupNamesByRoleKey)` in `cmd/server/main.go`, same place `WithRoleIDs` is chained) maps a group's display name directly to the portal role key it grants. **Keyed differently from `ASGARDEO_ROLE_IDS` on purpose, and matched by NAME rather than ID by explicit deployment choice**: a group has no "real role name" on the token to cross-reference against `AUTH_<ROLE>_ROLES`, so this config is keyed by the portal role's own stable key (`"cs_engineer"`, `"admin"`, ...) rather than a real Asgardeo name, and each value is the group's own display name (the one visible in the Asgardeo console), not its opaque id -- unlike role matching, which deliberately switched to ID specifically to rule out two roles sharing a display name, group matching here is by name on purpose: a deployment names a group directly, the same way `AUTH_<ROLE>_ROLES` already names a role directly, rather than requiring its id to be looked up separately. Its value is an array since more than one group can grant the same portal role, matched case-sensitively (the same exact-match convention `AUTH_<ROLE>_ROLES` itself uses). `withPortalRoles` calls `RolesForIDsAndGroupNames(roleIDs, groupNames)` -- not `RolesForIDs` alone -- which matches a portal role by EITHER a held role ID or a held group name in one fixed-order pass, so a role reachable through a role assignment, a group, or both is reported exactly once, never twice. A portal role key with no entry in `ASGARDEO_ROLE_GROUP_NAMES` simply never matches by group, the same "no entry means not wired up" posture `ASGARDEO_ROLE_IDS` already has -- and leaving the variable unset entirely (the pre-existing, group-less behavior) is unaffected: `RolesForIDsAndGroupNames` then degrades to exactly `RolesForIDs`' own output, proven by a dedicated regression test (`TestAccessGuard_RolesForIDsAndGroupNames_NoGroupConfigMatchesRolesForIDsExactly`).
  - This is gated on the target's **email domain** (`isWso2Email`), not `userType` -- a wso2.com address is reserved for WSO2 staff regardless of what `userType` the backing data source happens to have recorded for that row, the same edge case `withExternalAccountStatus` (`internal/handler/user_external_account.go`) already special-cases to skip its own SCIM "external" org lookup for a wso2.com address even when `userType` isn't `internal`. A wso2.com account mistakenly tagged `external` still has a real Asgardeo CSM role assignment worth showing; a non-wso2.com address gets no `csmPlatformRoles` field at all, whatever its `userType` says. Best-effort like every other `GetUser` enrichment (teams, external-account status): a failure at either SCIM step, or no AccessGuard wired, leaves `csmPlatformRoles` absent entirely, never a failed request.
  - SCIM's role-list attributes do **not** follow the JWT `roles` claim's "bare string or array of strings" convention (`middleware`'s own unexported `stringList`) — against a real SCIM service each comes back as an array of `{value, display, audienceType, ...}` objects instead (confirmed live: the naive string-array assumption failed every `GetUser` call for an internal user with `"json: cannot unmarshal object into Go value of type string"`). `scim.scimRoles` (search responses) and `scim.scimRoleAssignments` (`GetUserByID` responses) each have their own `UnmarshalJSON` accepting a bare string, an array of strings, a bare object, and an array of objects, since Asgardeo's own singular/plural convention elsewhere means a bare single value can't safely be ruled out either, even though only the array-of-objects shape has actually been observed. **`value` on that object is the role resource's own opaque id (a UUID), not its name** — extracting it instead of `display` was a second bug found in the same original fix, confirmed live: a role id is a UUID and never a readable string, so anything trying to recognize a role by matching against `Value` would always silently match nothing. `display` is the actual role name, in whatever convention this deployment's Asgardeo org actually uses; this package doesn't assume, strip, or otherwise special-case its shape.
- The role check is separate from resource-level guards inside handlers (e.g. the assigned-engineer check on public case comments) — both must pass.
- **`PermViewAllDashboards` (CS engineer and admin) is the one permission with no route.** A `dashboard.Dashboard` may set `"restricted": true`; `DashboardHandler` calls the newly-exported `AccessGuard.Permits(perm, roles)` itself to filter it out of `GET /dashboards` and 403 a direct `GET /dashboards/{id}` for anyone else, while both routes stay `PermView` so the list route still runs for every viewer. `Permits` exists specifically for this — a handler that must gate one specific resource against a caller's roles from inside its own logic, not a whole route via `Require`.
- Handler tests call handlers directly and bypass the guard; test the guard in `access_test.go`. `dashboards_test.go`'s `withRestrictedDashboard` temporarily swaps the package-level `dashboard.Active()` registry for one that adds a `Restricted` dashboard, restored via `t.Cleanup` — every other test in that file depends on the shared two-dashboard fixture's exact count, so never add a permanent third dashboard to it.

## The user's phone (`/users/me`)

`phoneNumber` is read from and written to the platform user record only (the entity
service's `"user".phone`), never the identity provider. `GET /users/me` returns the
record's `phone` as `phoneNumber` (omitted when the record has none), and `PATCH /users/me`
with `phoneNumber` writes it to the record through the entity service, in the same call as a
`timeZone` change when the request carries both. The phone is trimmed first; an empty or blank
one clears it. The identity provider is neither read nor
written for the phone. A user whose record has no phone shows none until they set one.
Tests: `TestGetMe`, `TestPatchMe`.

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

**`firstName`/`lastName`/phone are sourced from SCIM when the Add User form sends neither name field.**
The webapp's Add User dialog only collects an email (plus the type/portal-role selectors) — no name
fields at all. When `CreateUser` sees a request with both `firstName` and `lastName` blank, it runs a
SCIM search by `email` (`scim.Client.SearchUser`, the same call `provisionCsEngineerUser` above uses)
and fills `req.FirstName`/`req.LastName` from the result before building the entity-service request; if
SCIM has no matching user, or has one but no name on file either, the request is rejected with a 400
naming the problem (`please verify the email address` / `please provide a first or last name`) rather
than forwarding a name-less create upstream. A caller that DOES send a name (any other API consumer) is
untouched — this only fires for the email-only case, so it never overrides a caller-supplied name.
Phone is handled the same way `provisionCsEngineerUser` handles it: entity-service's `POST /users` has
no phone field, so a SCIM-sourced phone number is set with a second, best-effort `PATCH /users/{id}`
call once the create has returned the new user's id — a failed phone patch does not undo the create.
Unlike `provisionCsEngineerUser`, the created `lastName` here is **not** marked with `" Ⓦ"` — that
marker is specific to the on-demand `cs_engineer` auto-provisioning path, not the admin Add User flow.

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

## Updating another user's profile (PATCH /users/{id})

Restricted to `cs_engineer` or `admin` (`PermWrite`, held by exactly those two roles — see "Access
control" above). Unlike `PATCH /users/me`, which always resolves the caller's own id from their
token, this lets one of those roles update someone else's profile by id — `timeZone`/`phone`, both
optional pointers, at least one required (`UsersHandler.UpdateUser` forwards whatever was sent
straight to entity-service's own `PATCH /users/{id}`, which applies the identical trim/length/clear
rules `PATCH /users/me` already has for `phone`). Added for the team-lead reassignment feature
below: the incident call-escalation ladder dials the lead's own phone number (`"user".phone`), so a
newly assigned lead with none on file is a silent paging gap until this is set here.

## A team's own Chat space link and lead (GET/PATCH /teams/{id}, POST /teams/{id}/lead)

Two routes on `TeamHandler`, all backed by entity-service's own `team.chat_webhook_url`/
`team_member.role` columns (see that repo's own `CLAUDE.md`):

- **`GET`/`PATCH /teams/{id}`** — view or change the team's own id/name/type and Google Chat space
  link. A single generic team resource (mirroring `GET`/`PATCH /users/{id}`) rather than a dedicated
  `/teams/{id}/chat-webhook` route — `chatWebhookUrl` is the only settable field today, but a second
  one later is a new field on the existing request/response shape, not a new route. The route-level
  floor is `PermViewSharedEntity` (same as `GET /teams/{id}/members`), but
  `TeamHandler.authorizeChatWebhookActor` narrows it further, in-handler: only an Asgardeo admin
  (`AccessGuard.Permits(PermAdmin, ...)`) or **this specific team's own lead**
  (`TeamHandler.callerIsTeamLead`, which fetches the team's roster via the existing
  `GET /teams/{id}/members` client call and matches the caller's email, case-insensitively, against
  whichever member holds `role: "lead"`) may reach either verb — everyone else gets 403, since a Chat
  webhook URL is a bearer credential once set (anyone holding it can post into that space).
  entity-service itself validates the URL on PATCH (must be a well-formed
  `https://chat.googleapis.com/...` URL, or empty to clear) and its refusal reason is echoed
  verbatim (`mapUpstreamError`).
- **`POST /teams/{id}/lead`** — reassign the team's one lead. Admin-only via the route's own
  `PermAdmin` (same posture as `POST /users`: the handler does not re-check the caller's role
  itself — unlike `PATCH /users/{id}`, which is `PermWrite`, i.e. `cs_engineer` or admin, not
  admin-only). **This writes the real, paging-connected `team_member.role` column** — not a
  cosmetic label; entity-service's own incident call-escalation ladder reads this exact column to
  decide who gets paged for this team, so reassigning it changes who gets called during an incident,
  immediately. The UI must make this consequence obvious when offering this control, and should
  prompt to set the new lead's phone number in the same flow (`PATCH /users/{id}` above) if none is
  on file.

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

**`AccessGuard.WithRoleIDs(roleIDsByName)`** (`cmd/server/main.go`, chained onto `NewAccessGuard`
immediately) is a third, separate bridge over the same raw `roleIDsByName` map -- deliberately not
`ResolveGrantableRoles` reused for this, since that function's own candidate list only covers the 11
portal roles relevant to *granting* a role on user creation (missing `security_team`/`comment_updater`,
which cannot be granted that way) where this needs the complete, all-13-role vocabulary `RolesFor` itself
matches against, for a different purpose: recognizing which roles a user *already holds*, not which an
admin may hand out. It extends each `portalRole`'s own `names` set (already built in `NewAccessGuard`) with
the matching Asgardeo role **ids** from `roleIDsByName`, so `AccessGuard.RolesForIDs` (see "`GET /users/{id}`
reports a target's CSM portal-role vocabulary" above) can resolve a role by id with no further lookup. Both
bridges read the identical config; they just project it onto two different, non-overlapping consumer needs.

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

## Granting a portal role to an existing user (`POST /users/{id}/roles`)

`grantRoles` above only ever fires at the moment a new platform user is created — there was no
way to put an *existing* user into a portal role afterward without an admin doing it by hand,
outside the portal. `UsersHandler.GrantUserRoles` (`internal/handler/users.go`) closes that gap:
admin-only, same `PermAdmin` gate as `POST /users` and `GET /roles/grantable`, and reuses the
identical `handler.GrantableRole`/`RoleIDForKey` resolution `CreateUser`'s own grant step uses —
the request body is `{"roles": ["cs_engineer", ...]}`, the same portal role key vocabulary `GET
/roles/grantable` reports.

**Unlike `CreateUser`'s grant step, a SCIM failure here is not best-effort.** `CreateUser` can
treat a failed grant as recoverable because the platform user it just created is the actual point
of that call — granting a role is a bonus on top. Here, granting the role *is* the entire point of
the call, so swallowing a SCIM failure as a logged warning (as `CreateUser` does) would return a
200 to an admin who just watched nothing happen. Every requested role is still attempted
independently — one bad/unreachable role never blocks the others — and the response reports
exactly which roles were granted and which weren't (`grantUserRolesResponse{Granted, Failed}`):
`200` when every role succeeded, `502` when none did, `207 Multi-Status` for a mixed result.

**The target's email is resolved via `entity.GetUser(id)`, not taken from the caller.** SCIM's
`AddRoleMembers` grants by email, not platform user id, so the handler looks the target up by the
`{id}` path segment first (the same identity resolution `PATCH /users/{id}` already does) and uses
whatever email entity-service's own `UserDetail.Email` returns — a caller cannot grant a role to an
email of their own choosing that doesn't match the `{id}` they named.

**Refused with a 400 when the target isn't a `@wso2.com` account** (`isWso2Email`,
`internal/handler/user_external_account.go`) — a portal role only ever resolves back to anything on
the read side for a wso2.com account (`withPortalRoles`' own identical gate), so granting one to any
other target would silently add them to a SCIM role in the wrong org with no way for this portal to
ever display or act on it again; a mistaken grant like that would have looked like it worked (a 200
from SCIM) while doing nothing anyone could observe. `CreateUser`'s own `grantRoles` field got the
identical guard at the same time, for the same reason — it had none before either, despite sharing
the exact same `GrantableRole`/SCIM mechanism.

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

**The create body also sends `systemProvisioned: true`.** Before this, the request
forwarded the caller's own `x-user-id-token` as-is (the same one `Auth` attached for
the rest of the request), so entity-service's own actor resolution
(`emailFromJWT(x-user-id-token)`) stamped `created_by`/`updated_by` on the new row as
the very email being provisioned — a brand-new work-note/escalation author's row read
as though they had created their own platform user record. `systemProvisioned` tells
entity-service's `userService.CreateUser` to stamp `created_by`/`updated_by` as
`domain.SystemActor` ("SYSTEM") instead — see that repo's own `CLAUDE.md`, "POST
/users creates a new `user` row", for the trust gate that keeps this from being a
caller-supplied actor override. The admin-facing Add User flow
(`UsersHandler.CreateUser`, below) never sends this field, so an admin-created row
still carries the admin's own email as usual.

**`CreateCaseComment` skips calling this entirely for a caller who already holds full
`PermWrite`** (`cs_engineer` or `admin`, via the same `hasFullWrite` the
`PermCreateWorkNote`-narrowing check above already computes) — assumed already
provisioned, so the overwhelmingly common path (a CS engineer's own work notes/replies)
pays no extra `GetUserMe` round trip. `CreateCaseEscalation` has no equivalent skip:
`cs_engineer` never holds `PermEscalate` at all (see `NewAccessGuard`'s own doc
comment), so every caller who reaches that handler is, by construction, exactly the
audience this exists for.

**`GET /users/me` has its own, separate on-demand provisioning path for `cs_engineer`,
sourced from the token, not SCIM** (`UsersHandler.provisionCsEngineerUser`, `users.go`,
called from `GetMe`'s existing 404 branch, right where it already checks
`len(resp.Roles) == 0`). A `cs_engineer` caller with no `"user"` row yet — the same
provisioning gap `ensureUserProvisioned` closes for worknote/escalation authors — gets
one created on the first `GET /users/me` after the role is granted, rather than only on
their next case comment or escalation.

**`firstName`/`lastName`/`phone` come straight off the caller's own validated token**
(`middleware.UserInfo`'s `given_name`/`family_name`/`phone_number` claims — the last of
these a new field this feature added, same posture `ensureUserProvisioned` already takes
for name, extended here to phone too since the token carries it). No SCIM call is made at
all: unlike the admin Add User flow below, which has no token for the person it's creating
and so has to look them up in SCIM by email, this caller IS the person being provisioned,
so their own token already has everything needed. The created `lastName` has `" Ⓦ"`
appended (`withCsEngineerLastNameMarker`), so a row created this way is identifiable later
as distinct from one entered by hand or via the admin Add User flow. Unlike
`ensureUserProvisioned`, this path does not fall back to the email's local part when both
name claims are blank — it creates the user with whatever the token has (possibly just the
marker alone as `lastName`).

`POST /users` has no `phone` field (see "Creating a user" above), so a non-blank token
phone is set with a second call, `PATCH /users/{id}` (`entityUserClient.UpdateUser`), once
the create has returned the new user's id — best-effort, same as everything else in this
path: a failed phone patch does not undo the create. A token with no `phone_number` claim
simply creates the user without a phone. Any failure anywhere in this path (create, phone
patch) is logged and falls through to `GetMe`'s existing token-only answer rather than
failing the request.

## Security Center access (PermViewSecurityCenter / PermViewSecurityReportCases)

Security Center (the webapp's Security reports + Vulnerabilities tabs) is restricted to `cs_engineer`,
`admin` and `security_team` — every other role, even though it holds `PermView`, is denied. This needed
two different mechanisms, because the feature isn't backed by its own exclusive routes:

- **`/products/vulnerabilities/search` and `/products/vulnerabilities/{id}`** are genuinely
  Security-Center-exclusive, so they're gated the ordinary way: `route(..., handler.PermViewSecurityCenter, ...)`
  in `cmd/server/main.go`. `PermViewSecurityCenter` is held by `cs_engineer`/`admin`/`comment_updater`/
  `security_team` only — `viewer` does not hold it, and reading these two routes is unaffected by the
  API-only change below.
- **`POST /cases/search`** is the shared, generic case-search endpoint every case-type tab uses
  (Support, Operations sub-tabs, Engagements, Security reports) — it stays registered at `PermView`,
  since narrowing that route-level permission would lock out every other tab too. Instead,
  `CaseHandler.SearchCases` inspects the request body itself: `caseSearchTargetsSecurityReports`
  (`internal/handler/cases.go`) reads the generic filter expression (`filters.filters[]`, and each
  `filters.anyOf[]` branch) for a `{field: "type", op: "in", values: [...]}` predicate naming
  `security_report_analysis`, and if one is found, additionally requires `PermViewSecurityReportCases`
  (not `PermViewSecurityCenter` itself) via `CaseHandler.access` (wired with `WithAccessGuard`, the
  same pattern `UsersHandler` uses) — a caller holding neither gets 403 instead of the search running.
  This only catches an *explicit* request for that type, the same way Security Center's own
  `caseTypes`-locked search (`CsmIssuesView`, webapp) always sends one; a hypothetical unfiltered
  "every case type" search that happens to also return security-report rows is a known, narrower gap,
  not handled here.
  - **`PermViewSecurityReportCases` is a deliberately separate, narrower permission from
    `PermViewSecurityCenter`** — held by everyone `PermViewSecurityCenter` holds (`cs_engineer`/
    `admin`/`comment_updater`/`security_team`) *plus* `viewer`, added by explicit request so `viewer`
    callers can fetch/search `security_report_analysis` cases over the API without also gaining
    `/products/vulnerabilities/*` access, which stays on `PermViewSecurityCenter` alone. This is an
    **API-only** change — the webapp's own Security Center nav/tab gating (`canUseSecurityCenter` in
    `portalAccess.ts`) is untouched, so a `viewer` still sees no Security reports/Vulnerabilities tab
    in the UI; they can only reach this data by calling the API directly.
  - **`service_request` cases have no equivalent gate at all, and never did.** `caseSearchTargetsSecurityReports`
    only ever matched `security_report_analysis`; every `PermView` holder (`viewer` included) could
    already search/fetch `service_request` cases over the API the whole time — the "only admin/cs_engineer
    can view SR" behavior some callers observe is purely the webapp's own Operations nav gating
    (`canUseOperations`), not a backend restriction, and is unaffected by this change.
- **`GET /cases/{id}` has no equivalent check, deliberately.** `CaseView.type` (entity-service's own
  `openapi.yaml`) is only populated for ServiceNow cases — null on Postgres — so there is no reliable
  way for this handler to tell a security-report case apart from any other by inspecting the response
  alone, and a check that silently does nothing on one data source would be worse than no check at
  all (it would look like protection without being any). See `CaseHandler.WithAccessGuard`'s own doc
  comment for the full reasoning. **This is not a hard per-case-type ACL** — only an *explicit*
  `security_report_analysis` type filter on `POST /cases/search` is blocked for a caller holding
  neither `PermViewSecurityCenter` nor `PermViewSecurityReportCases` (see the unfiltered-search caveat
  above); a search that targets some other type, or no type at all, is not inspected for this and can
  still return security-report rows (ids included) in its result set if the upstream query happens to
  match any. A caller who obtains an id this way, from a pre-existing bookmark, or by guessing, can
  still fetch it directly by `GET /cases/{id}`. Closing either gap fully needs entity-service itself to
  resolve and enforce it (it has reliable type data on either data source), not this BFF layer.

## PATCH /projects/{id}: closureState requires PermAdmin, the onboarding fields PermManageOnboarding, every other field PermWrite

Same mechanism as `caseSearchTargetsSecurityReports` above, applied to a write instead of a search:
`PATCH /projects/{id}` is registered at the `PermUpdateProject` route floor (`cs_engineer` ∪ `admin` ∪
`onboarding_lead`, see the `PermManageOnboarding` entry above), and `ProjectHandler.UpdateProject` then
checks each field group itself: every other field on this endpoint (`hasAgent`, `hasKbReferences`, the
three closure sub-state fields, `suspensionProcessState`) needs `PermWrite`, which is ordinary
`cs_engineer` territory — re-gating the whole route to `PermAdmin` would lock `cs_engineer` out of
fields nobody asked to restrict. `closureState`
(entity-service's direct, Postgres-only override of a project's overall closure status — see that
repo's own `CLAUDE.md`, "PATCH /projects/{id}: closureState is a direct admin override") is the one
field that needs the narrower bar: an admin can set a project to OPEN/READ_ONLY/CLOSED/RESTRICTED/
SUSPENDED directly, bypassing the usual derivation from the three sub-state fields, and that's an
admin-only action by product decision, not something every `cs_engineer` should be able to flip.

`ProjectHandler.UpdateProject` inspects the raw request body itself: `projectUpdateTargetsClosureState`
(`internal/handler/projects.go`) reads the top-level keys as a case-insensitive token stream (mirroring
`caseCreateTargetsAnnouncement`'s own reasoning: entity-service's JSON decoder matches a key to a
struct field without regard to case, so a check that only looked for one spelling could be walked
around by a body that spells the key differently) and, if `closureState` is present at all — even an
explicit `null` — additionally requires `PermAdmin` via `ProjectHandler.access` (wired with
`WithAccessGuard`, the same pattern `CaseHandler`/`UsersHandler` already use). A caller holding
`PermWrite` but not `PermAdmin` (a plain `cs_engineer`) gets 403 and the entity service is never
called; every other field on the same PATCH is unaffected and keeps reaching the entity service for a
caller holding `PermWrite` (the onboarding fields, including the four dates, are checked the same way against
`PermManageOnboarding`). A nil `access` (no guard wired) fails closed, same as every other
in-handler permission check in this codebase.

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

## Change request tags go through the case tag routes

`GET /change-requests/{id}` is a raw pass-through, so the `tags` array the
entity service now puts on a change request's detail (the same shape as a
case's `tags`; null when it did not load them) reaches the webapp untouched
(`TestGetChangeRequest_PassesTagsThrough`). There is deliberately **no**
`/change-requests/{id}/tags` route: the entity service's `POST /cases/{id}/tags`,
`DELETE /cases/{id}/tags/{tagId}` and `POST /tags/search` take a work item id
and accept a change request's, and this BFF already routes them at `PermWrite`
/ `PermView` -- the same permissions a change request write / read needs
(`PATCH /change-requests/{id}` is `PermWrite` too), so the change request
detail page calls those three as they are (`CaseHandler.AddCaseTag` /
`RemoveCaseTag` / `SearchTags`, unchanged). Tags are staff bookkeeping: the
entity service leaves them null for a customer, and they are never mirrored to
the previous system (see entity-service's `CLAUDE.md`, "Change requests: Tags").

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
  team id -- `POST /groups/search` lists the same group ids, while `GET /teams/{id}/members` is the team registry.
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
  `"CSM Integration Service"`, `"Engineering Entity Service"`, `"CSM AI Integration Service"` — the response's `name` values are
  human-readable display names, not package/env-var identifiers, so use these exact strings (not
  `csm-notification-service` or similar) when checking a caller against this response. This
  backend's own core `entity-service` is deliberately excluded (this backend depends on it for
  nearly every request; checking it here was an explicit product decision to leave out). Each
  listed dependency reports one of `ok`/`down`/`not_configured` — `not_configured` when that
  dependency's base URL env var is unset (`CSM_NOTIFICATION_SERVICE_BASE_URL`/
  `CSM_INTEGRATION_SERVICE_BASE_URL`/`ENGINEERING_ENTITY_BASE_URL`/`CSM_AI_INTEGRATION_SERVICE_BASE_URL`, all optional; SCIM/Updates are
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
| `entity` | Multiple entity services (see below) | Hosts `CustomerEntityClient` (this repo's entity-service; most case/account/project endpoints, raw `[]byte` passthrough) and `EngineeringEntityClient` (a separate internal engineering entity service; `CreateGitIssue`, typed request/response, plus `Health(ctx)` backing `GET /health/dependencies` — see "Health endpoints" above). `EngineeringEntityClient` is constructed in `cmd/server/main.go` only when `ENGINEERING_ENTITY_BASE_URL` is set, and then `CaseHandler.CreateCaseGithubIssue` uses it (via `WithEngineeringClient`) instead of the entity service: the target is the case product's `product_repo_mapping` row (looked up through the entity service's `GET /products/github-repo`; `repoOverride` is ignored), and after filing it writes the issue URL to the case as a best-effort work note. The issue body is the engineer's description, then the optional Hotfix Required line, then an automated footer of always the same seven lines, in this order and spelling (`Product`, `Update level`, `WSO2 case id`, `Case number`, `Public Git Issue`, `ABT Team`, `Opened by`; a value that is not known is written as `undefined`, a line is never dropped, and a value holding a newline is collapsed onto its line). **Do not change the spelling, order or the `undefined`: other things read this body.** The internal repositories' `ABT Team Notifications` workflow finds `WSO2 case id :`, `Case number :`, `ABT Team :` and `Opened by :` with a case-sensitive fixed-string match to build its Chat card (a different capital letter and the card loses the case id and the opener's name), `Patch Creation Tracker` does the same for `Case number :` / `WSO2 case id :`, the IAM repository's workflows read `Public Git Issue : <url>`, and the Git Internals dashboard reads the last `ABT Team`/`Opened by` line as the automated one (hence the footer goes last). `Opened by` is the signed-in engineer, `ABT Team` the case's account's CRE team (the assigned team only as a fallback), and `Product` is `<abbreviation>-<version>` (e.g. `wso2is-6.1.0`) from a best-effort `POST /deployed-products/search` by the case's deployment (5s timeout, at most four pages, every failure logged with its reason and falling back to the deployed product's display name, then the catalogue name, then the repository label) |
| `scim` | SCIM service | User/group/role lookups. Two orgs: `SearchUser` queries the "internal" org (WSO2 staff — phone number, last password update, and the target's own Asgardeo user id for `GetUserByID`). `SearchExternalUser` queries the "external" org (customer/partner contacts — existence + lock status, mirroring `infra-operations/operations/asgardeo-user-check`'s `{exists, locked}` contract). `GetUser` calls the latter only when the entity response's `userType` isn't `internal`, and treats a lookup failure as best-effort — logged, response returned unchanged, never a failed request. `GetUserByID(ctx, userID)` fetches the full canonical SCIM user record (internal org only) via the SCIM operations service's `GET /organizations/internal/users/{id}` and returns its complete role assignment list AND group assignment list (one `UserAssignments{Roles, Groups}` struct, both off the same response, no second call), each entry keeping both the resource's Asgardeo id and its display name — used by `withPortalRoles` instead of trusting `SearchUser`'s own `roles` field, since a search response only returns whichever attributes were requested. `AccessGuard.RolesForIDsAndGroupNames` is what actually matches these: a role by its id (not just its name, since two roles can share one), but a group by its display NAME, not its id, by deliberate deployment choice — see "`GET /users/{id}` reports a target's CSM portal-role vocabulary" above for why the two are matched differently. `GetRole(ctx, roleID)` fetches an Asgardeo *role's* real member users (internal org only) via the SCIM operations service's `GET /organizations/internal/roles/{id}` — a plain get-by-id, not a search, so the caller supplies the role's own Asgardeo id directly (see `ASGARDEO_ROLE_IDS` below); a member's SCIM `display` (`"<domain>/<email>"`, e.g. `"DEFAULT/jane@wso2.com"`) is unwrapped to a bare email |
| `updates` | Updates service | Product update levels; returns typed structs (not raw passthrough) |
| `csmnotification` | `integrations/csm-notification-service` | Health check only today (`Health(ctx)`, backing `GET /health/dependencies` — see "Health endpoints" below). Optional: unconfigured (`CSM_NOTIFICATION_SERVICE_BASE_URL` unset) means this dependency reports `not_configured` |
| `csmintegration` | `integrations/csm-integration-service` | Same shape and same one purpose as `csmnotification` above, for `integrations/csm-integration-service` |
| `csmai` | `integrations/csm-ai-integration-service` | The dashboard generator's model calls (`POST /ai/dashboards/generate`) plus `Health(ctx)`. Each request carries the correlation id and the signed-in user's id as `X-CSM-User-ID` (never the email), which the service logs with the call's token usage. Optional (`CSM_AI_INTEGRATION_SERVICE_BASE_URL`, `_SCOPES`, `_TIMEOUT` default 55s; https, or http for a local host only). When set, `handler.NewServiceDashboardGenerator` (`dashboard_generator_service.go`) is the generator: the service builds the prompt and calls the model; this backend still converts the draft (`draftToDefinition`), sends the registry teams that have a group, keeps the last candidate `dashboard.ParseGenerated` accepts, swaps team keys for group ids (`WithDashboardTeams`) and maps the service's error `code` to the same statuses and messages (`serviceGenerateError`). Unset: the sample. **This backend holds no model client or model SDK** (`cmd/server/no_model_sdk_test.go` fails if `go.mod` requires, or any file imports, one); the catalogue lives in the service, and `internal/dashboard`'s `TestParseGenerated_TheCatalogueExampleIsValid` reads the service's `catalogue.md` so its example keeps passing this backend's check |

New upstream services get their own package under `internal/` following the same `Client` + `do()` pattern. `entity` is the exception: because it hosts multiple, separately-deployed/differently-authenticated services, it uses a `<Name>Config`/`<Name>Client` pair per service/file instead of one shared `Client` for the whole package — `CustomerEntityClient`/`EngineeringEntityClient`.

**Notifications (email, Google Chat, and future channels like SMS/voice-Twilio) have moved out of this backend** into `integrations/csm-notification-service`, a standalone Go service invoked over HTTP. This backend no longer constructs or calls any notification client directly, and no longer publishes domain events to Azure Event Hub either — that responsibility (case.created/case.acknowledged/case.severity_changed/incident.created, and the like) now belongs entirely to `entity-service`, the one publisher of the `case-events` topic. This backend previously had its own producer-side pipeline (`internal/eventbus`/`internal/events`/`internal/eventpublisher`, publishing `case.comment_added`/`case.status_changed` from `CaseHandler`) — it was removed entirely to avoid two independent services writing to the same topic. Per-recipient portal-link resolution (which of the customer/CSM portal a given recipient's email lands on) **does not live in this backend** either — it's in `csm-notification-service`'s own `internal/recipientlinks`/`internal/entity`, since that's the service actually composing and sending the email.

**Shared OAuth2 credentials**: `CustomerEntityClient` and `EngineeringEntityClient` (both in `entity`), `updates`, `scim`, and PLG's `internal/plg/entityclient` all authenticate as the same OAuth2 client-credentials app — `cmd/server/main.go` reads `OAUTH2_CLIENT_ID`/`OAUTH2_CLIENT_SECRET`/`OAUTH2_TOKEN_URL` once and passes them into every service's `Config`; only `<SERVICE>_BASE_URL`/`<SERVICE>_SCOPES` are per-service. `EngineeringEntityConfig` still has its own `ClientID`/`ClientSecret`/`TokenURL` fields (matching every other service's `Config` shape), but when it's wired into `main.go` those should be filled with the same shared `oauth2ClientID`/`oauth2ClientSecret`/`oauth2TokenURL` values, not new `ENGINEERING_ENTITY_*` env vars. Follow this pattern for any new upstream service client unless it genuinely uses a different OAuth2 app. PLG is the one client that **inherits** rather than being passed a `Config` per field: `main.go` hands `plg.Mount` a `plgconfig.EntityDefaults` carrying `CUSTOMER_ENTITY_BASE_URL` and the three shared `OAUTH2_*` values, because PLG reaches the same entity-service as `CustomerEntityClient` and a second copy of those settings is one that goes stale on the next secret rotation. `PLG_ENTITY_BASE_URL`/`PLG_ENTITY_OAUTH_*` exist as overrides and are normally unset.

**Request timeouts**: `REST_READ_TIMEOUT` / `REST_WRITE_TIMEOUT` (default 60s) and `ENTITY_SERVICE_TIMEOUT` (default 60s, applied to `CustomerEntityClient` only) are parsed by the pure `loadTimeouts` in `cmd/server/timeouts.go`; all must be > 0, else startup fails; no ordering between them is enforced (client timeout < write timeout is advisable, not required). Defaults were raised from 30s/25s for large inline-attachment uploads. Other upstream clients (engineering, updates, SCIM, notifications, SFTPGo) keep their own fixed timeouts.

## Customer-health risk tracking (`internal/risk`) is entity-service-backed, not MySQL

`internal/risk` used to be this backend's own standalone MySQL client (`database/sql` + `github.com/go-sql-driver/mysql`, `SPL_RISK_MYSQL_DSN`) for the `project_health_status`/`project_risk`/`risk_action_item`/`action_item_comment` tables SupportPortalLite's customer-health feature owns. That database has been migrated into entity-service's own Postgres (migration `0219_customer_health_risk_tables.sql` in that repo); this backend no longer opens any database connection of its own at all. `risk.Client` (`internal/risk/client.go`) is now a thin wrapper over the already-shared `customerEntityClient`, delegating to typed methods on `*entity.CustomerEntityClient` (`internal/entity/customer_health.go`) that call entity-service's `/projects/{id}/risk`, `/risks/{id}/close`, `/accounts/{id}/action-items`, etc. `cmd/server/main.go` wires it with a single `risk.NewClient(customerEntityClient)` — no DSN, no ping, no startup failure mode of its own; see `NewCustomerHealthHandler`'s call site.

Two boundary conversions live entirely inside `internal/risk` (never in the handler layer or the frontend) so every existing caller's wire contract stays exactly what it was under the MySQL-backed version:

- **sys_id ↔ UUID.** The old MySQL tables keyed `project_sys_id`/`account_sys_id` as plain upstream sys_ids (32-char hex, no dashes); entity-service's Postgres tables reference `project(id)`/`account(id)` directly, which (per entity-service's own `CLAUDE.md`) store that same sys_id reformatted with dashes as a real UUID. `sysIDToUUID`/`uuidToSysID` (`client.go`) convert at the call boundary, so `ProjectSysID`/`AccountSysID` on every request/response type keep their original sys_id shape. The risk/action-item/comment rows themselves are genuinely new resources with no upstream sys_id at all — their own `id`/`riskId`/`actionItemId` are entity-service-assigned UUID strings, not the MySQL tables' old auto-increment ints (a real, unavoidable wire-contract change — see `types.go`'s own doc comment). The frontend (`apps/csm-portal/webapp`) already types these as `string`, so this needed no frontend change.
- **Enum case.** entity-service's Postgres enums are `UPPER_SNAKE_CASE` (`"AT_RISK"`, `"IN_PROGRESS"`, its own established convention), but every existing caller of this feature — the portal frontend's string-literal comparisons, ported from the original Ballerina backend — compares against lowercase (`"at_risk"`, `"in_progress"`). `lowerEnum`/`upperEnum`/`upperEnumPtr` (`mappers.go`) convert at the same boundary: every status/priority value entity-service returns is lowercased before it reaches a caller, and every one a caller supplies (priority on create/update, status on a transition or filter, the `healthStatus` search/filter value) is uppercased before it reaches entity-service. Found and fixed before this ever reached the frontend (a casing mismatch here would have silently broken every status comparison, defaulting to whatever `else` branch each component falls back to) — if a future endpoint in this feature forgets this conversion, that is the exact failure mode to look for.

The actor is no longer passed as an explicit `email` parameter either: entity-service resolves it server-side from the `x-user-id-token` this backend's own `Auth` middleware already attaches to every request context, forwarded automatically by `entity.CustomerEntityClient.do()` — the same mechanism every other entity-service write in this backend already relies on. Error handling also moved onto this backend's standard `mapUpstreamError`/`mapUpstreamErrorGeneric` convention (see "Handler conventions" below) instead of a bespoke `*risk.ValidationError` type the handler had to explicitly check for.

**The Customer Health dashboard's health-status filter returned the wrong accounts, and silently blocked one of its three values.** Two bugs, found live via a HAR capture: filtering by "At Risk" also returned accounts whose displayed status was Healthy or To Be Reviewed, and selecting "To Be Reviewed" always showed zero accounts.

- `risk.Client.GetAccountsByHealthStatus` (`internal/risk/health.go`) used to short-circuit `healthStatus != "at_risk" && healthStatus != "healthy"` to an empty list with no round trip — on the belief that entity-service only supported those two values. It always supported all three (its own `validProjectHealthStatusValue` set); the guard just never let `"to_be_reviewed"` through. Fixed by widening it to also pass `"to_be_reviewed"` to entity-service.
- The mismatched-accounts half was an entity-service bug, not this backend's: `GetAccountsByHealthStatus` queried a *different* table (`project_risk`, "is there an OPEN risk row?") than the one used to compute the status actually *displayed* for each account (`project_health_status`, the same aggregate `GetBatchAccountHealthSummaries` uses) — the two can disagree for data that predates the synchronized writes `OpenProjectRisk`/`CloseProjectRisk`/`MarkProjectHealthy` perform, or that was written some other way. Fixed in entity-service's own repository to read `project_health_status` for both the filter and the display, so they can no longer disagree by construction; see that repo's own `customer_health_repo.go` doc comment on `GetAccountsByHealthStatus`.
- The webapp's own health-status filter dropdown (`CustomerHealthFilters.tsx`) only ever offered "At Risk"/"Healthy" — "To Be Reviewed" was missing from the UI entirely, independent of the backend bug above. Added as a third `MenuItem`, with the dropdown's and the result-count summary's wording sharing one `HEALTH_STATUS_LABELS` map (`customerHealthTypes.ts`) so the two can't drift apart.

## Viewer-access (SupportPortalLite) routes are always registered; each upstream is independently optional

`viewerHandlerSet` (`cmd/server/main.go`) used to be gated behind one blanket `SPL_ENABLED` flag: off (the default) meant none of its routes were registered at all, and on meant every one of ServiceNow/Google Drive/the Scan User entity service's credentials became required (`mustEnv`) or the process refused to start. That all-or-nothing shape meant a deployment with no real credentials for those three upstreams (a dev environment, for instance) couldn't register even the sub-features that don't need any of them — customer-health's own risk-tracking writes (`internal/risk`, entity-service-backed, see above), usage metrics, user-info, and the product list (`internal/handler/lookups_postgres.go`'s `GetProductList`) are all entity-service-only.

`SPL_ENABLED` is gone. `loadViewerConfig` now reads every remaining upstream's config with plain `os.Getenv` (not `mustEnv`) and every route in `viewerHandlerSet` is registered unconditionally. Google Drive and the Scan User entity service are independently optional the same way: unset leaves only the endpoints that genuinely need them (`GET /files`/`GET /files/search`, Scan User's lookups) failing at call time with a normal mapped upstream error, instead of the whole route being a bare Go 404 or the process refusing to start.

**The ServiceNow integration is gone entirely.** `internal/servicenow` (the Table API / scoped-app client and every type in it) is deleted, `cmd/server/main.go` no longer constructs an `snClient`, and the six routes that could only ever work against ServiceNow are removed along with their handlers and OpenAPI entries: `POST /accounts/{accountId}/cases/{caseId}/escalate`, `GET /cases/{caseId}/attachments-info`, `GET /attachments/{attachmentId}/download`, `GET /abt-team-schedule`, `GET /generate-sla-report` and `GET /report-details`. `TEAM_SCHEDULE_URL` went with `GET /abt-team-schedule`, its only reader. The SLA and CS reports are retired rather than ported: their statistics (SLA percentile breakdowns, monthly case counts, subscription/SLA summaries) only ever existed inside ServiceNow scoped-app scripts (`/api/wso2/case_sla/report`, `/api/wso2/cs_report/project-insights`), with no equivalent data or business rules on the Postgres side. The webapp's SLA and CS report pages (and their `useGetSlaReport`/`useGetCsReportDetails` hooks) were removed in the same change; the Time report (`GET /generate-timelogs-breakdown-report`, entity-service-backed) stays. Every remaining viewer-audience route is entity-service-backed; Google Drive (`GET /files`, `GET /files/search`) and the Scan User entity service (Scan User's lookups) remain the only separate, independently optional upstreams — see `loadViewerConfig`. The wire types those routes still share (the customer-health `AccountDetail`/`ProjectDetail` family and its `GoLiveStatus`, `EscalationDetail`, `TimeLogBreakdownDetails`/`TimeLogBreakdownCase`/`TimeCardDetails`) now live in `internal/viewer`, together with the `ErrAccountNotFound`/`ErrProjectNotFound` sentinels the handlers map to 404; `ValidateMetricsDateRange` moved into `internal/handler/usage_metrics.go` as `validateMetricsDateRange`. `CustomerHealthHandler`'s account-detail dependency, formerly `customerHealthSNClient`, is `customerHealthDetailClient` (implemented by `postgresCustomerHealthClient`).

**`GET /abt-teams` moved onto entity-service.** The ServiceNow hierarchy it used to read (`sys_user_group` children of a `CS_INT_CRT_LK` group) was never mirrored into entity-service's own `"group"` table — checked directly against a real database: every row's `parent_id` is NULL, so there is no group hierarchy there at all. The equivalent data does exist, under a different shape: entity-service's `team` table (the hand-curated CSM_TEAM_REGISTRY, already exposed via `POST /teams/search`) tags each ABT team's own row with a `type` ending in `-abt` (`"Apollo sre-abt"`, `"Atlas cre-abt"`, confirmed against real rows) rather than a separate name/hierarchy. `postgresLookupsClient.GetABTTeamList` (`internal/handler/lookups_postgres.go`) now pages through `SearchTeams`, keeps only rows whose `type` contains `"abt"` (case-insensitive), and returns the bare team name — the same shape the old ServiceNow-backed implementation returned.

**`GET /accounts/{accountId}/escalations` moved onto entity-service; the account-scoped escalate route is removed.** `work_item.account_id` is a direct, real column (unlike ServiceNow's own account-level `sn_customerservice_escalation`, which groups several cases under one escalation record — entity-service's `case_escalation` has no such grouping, only a per-case `current_level`/`previous_level` history), so `postgresViewerAccountClient.GetEscalationsByAccount` (`internal/handler/accounts_postgres.go`) resolves the account's own case ids via `POST /cases/search` (`accountId`, `in`) and feeds them into entity-service's existing `POST /escalations/search`. Each `case_escalation` row becomes its own result row — `severity` is the raw current-level **number** (`"3"`, not `"EL3"`: entity-service's own `Escalation.CurrentLevel.ID` wire shape already strips the `case_escalation_level_enum` prefix, confirmed directly against a real escalation; no equivalent of ServiceNow's "High Severity"/"Medium Severity" picklist exists) and `state` is derived (`"0"` = de-escalated back to baseline, anything else = escalated), since the table has no open/closed column of its own. `POST /accounts/{accountId}/cases/{caseId}/escalate` (`EscalateCase`) is removed with the ServiceNow client: it had zero callers in the webapp or microapp — `CsmAccountDetailPage.tsx`'s `EscalationsSection` is deliberately read-only (see its own doc comment), and the real "escalate a case" UI (`EscalateCaseDialog.tsx`) goes through the unrelated, entity-service-backed `POST /cases/{id}/escalations` instead.

**Found live, against the real database, while verifying this change: `POST /cases/search`'s `accountId` filter is `in`/`notIn` only, never `eq`** (`eq` is a 400 `does not support op "eq"`) — this was caught by actually calling the real entity-service HTTP API end to end rather than trusting a unit test built on a hand-written fake, which happily accepted whatever shape the fake expected. A second, more serious bug surfaced the same way: `accountId`+`in` was *accepted* by entity-service's own filter parser but **silently never applied** by its Postgres repository — a real, pre-existing entity-service bug this discovery led to fixing (see that repo's own `CLAUDE.md`, "case search accountId filter"). Before that fix, `GetEscalationsByAccount`'s own case-id lookup returned every case in the system instead of the account's own — confirmed directly (22,361 cases returned for an account verified by direct SQL to have exactly 1,585). Anyone touching a case-search-backed Postgres filter should verify it the same way: a hand-rolled fake test double proves the Go code calls the client correctly, never that the client's *filter* actually narrows anything on the real data.

**`POST /customer-health/summary` and `GET /customer-health/accounts/{id}` moved onto entity-service.** The original implementation (`GetCustomerHealthSummary`/`GetCustomerHealthDetail` in `internal/servicenow/customer_health.go`) called a legacy upstream service to compute seven account/project risk flags (`hasNoGoLive`, `hasRecentCases`, `hasEolProduct`, `hasAbandonedMigrations`, `hasMigrationDelays`, `hasRecentEscalations`, `noSupportCases6mo`). This was first ported from an approximation, then corrected once the legacy system's own precise rules for each flag were confirmed — see "Risk flags corrected against the legacy system's own rules" below for the precise fixes and how each was verified against real data. `postgresCustomerHealthClient` (`internal/handler/customer_health_postgres.go`) originally derived all seven flags here from real, populated Postgres columns. Both endpoints are now computed server-side by entity-service (the summary by `POST /accounts/search`, the account detail by `GET /accounts/{id}/health-detail`, see "The account detail is one request" below); the rule descriptions that follow are the history of why each flag has the semantics entity-service implements:

- `hasNoGoLive` — `projectGoLiveStatus`/`accountGoLiveStatus`: a project with a recorded `onboarding_go_live_date` (`ProjectView.GoLiveDate`) has gone live; otherwise the account's `activationDate` decides — no activation date is `"N/A (No Start Date)"` (pending, not a risk), an activation before a **fixed** grandfather-clause date (`customerHealthLegacyActivationCutoff`, `2024-01-01` — not a rolling "N years ago" window that moves forward with time) is treated as a long-standing live customer, and only an activation 6+ months old (using a `30.4375`-day average month length, not a calendar-month diff) with no go-live date is the real risk (`"Hasn't Gone Live in 6 months"`, `isRisk: true`). Both "has gone live" branches report the identical plain `"Has Gone Live"` status string — confirmed against a live sample of real accounts predating the cutoff; an earlier version of this port invented a distinct `"Has Gone Live ( Customer for over 2 years )"` string for the legacy-activation branch with no real-data confirmation, since corrected. `project.onboarding_status` plays no part in this decision at all (see the history note below — it used to, incorrectly).
- `hasEolProduct` — any deployed product under the account's projects' deployments has a product version whose `supportEoLDate` has passed, fetched with `includeInactive: true` (`entityDeploymentsSearchRequest.IncludeInactive`, entity-service's own `SearchDeploymentsRequest.IncludeInactive` — additive, see that repo's own `CLAUDE.md`): a deactivated/decommissioned deployment can still carry real EOL product history that must count toward this flag. The project detail view's own plain "Deployments" list (`deploymentRefs`) keeps a separate, `includeInactive: false` fetch, so that list's own active-only semantics are unchanged.
- `hasRecentCases` / `noSupportCases6mo` — a case-**like** work item (`isCustomerHealthCaseLikeType`: case/service_request/engagement/security_report_analysis — explicitly **not** announcement) created within the last 180 days (`customerHealthRecentWindow`); the two flags are exact inverses. Expressed as an inclusion filter (`type`/`in`) rather than an exclusion, since entity-service's case-search `type` filter only supports `op: "in"`, never `notIn`.
- `hasAbandonedMigrations` / `hasMigrationDelays` — a still-open (`state != closed`) `type: engagement, engagementType: migration` case older than 180 days (`customerHealthAbandonedWindow`) / 60 days (`customerHealthDelayWindow`) respectively — two thresholds over the same underlying signal, both this file's own judgment call, not a ported spec.
- `hasRecentEscalations` — any of the account's cases has a `case_escalation` row (via `accounts_postgres.go`'s same case-id-then-escalations-search pattern) created within the last 90 days (`customerHealthEscalationWindow`).

**Superseded:** the summary list no longer scans or derives flags in this backend at all: every `POST /customer-health/summary` request is one `POST /accounts/search` with `includeHealthSummary` (see "The health-status-filtered summary path 400'd" below), and `filteredAccounts`, `scanAccountsPage`, `accountHasProduct`, `accountFlags` and the per-account flag helpers described in this paragraph and the next are deleted. What follows is the history of why each of the six filters (`email`/`phrase`/`risks`/`region`/`product`/`abtTeam`) now has the semantics entity-service implements: `region` matches the account's sub-region, `abtTeam` its CRE/SRE team name, `product` any deployed product, `risks` any one of the seven flags (OR).

**All six filters — `email`/`phrase`/`risks`/`region`/`product`/`abtTeam` — are applied now.** Found live (via a captured record of real traffic): only `email`/`phrase` actually narrowed results; `risks`/`product`/`abtTeam` were accepted on the wire (the `customerHealthSNClient` interface already declared them) but silently never passed into `filteredAccounts` at all, and `region` was compared against the wrong account field (`Region`, a coarser value like `"APAC"`/`"NA"`) instead of the one the filter dropdown's values actually match (`SubRegion`, e.g. `"ASIA"`/`"NA - WEST"` — see entity-service's own `CLAUDE.md`, the `AccountView.SubRegion` addition), so every region value matched zero accounts. `filteredAccounts` now runs a single `customerHealthAccountQuery` (`region`/`risks`/`product`/`abtTeam`) through a bounded scan-and-filter path over the base-filtered (life_cycle+hasProject+email+phrase) account set: `region` matches `a.SubRegion`, `abtTeam` matches `a.CreTeam`/`a.SreTeam.Name`, `product` matches any deployed product (active or not) under the account's projects' deployments via `accountHasProduct` (reusing `deployedEolProducts`'s own second return value), and `risks` matches any of the account's seven computed flags via `customerHealthAccountMatchesAnyRisk` (OR within the requested risk list). `GetCustomerHealthDetail`'s per-project `ProjectDetail` rows reuse the identical flag logic scoped to one project, with drill-down lists (`detailedRecentCases` grouped by severity, `detailedAbandonedCases`/`detailedMigrationDelays`/`detailedEscalatedCases` as case links, `softwareModel` as the EOL product list) built from the same queries each flag's own computation already ran. `riskClient`'s own risk-tracking (`internal/risk` — `OpenRisk`/`CloseRisk`/`MarkHealthy`/`GetAccountHealthStatus`/action items) is unrelated and untouched: that's a human-driven health-status override layered on top of these seven computed flags by `customer_health.go`'s own `enrichAccountSummaries`, not replaced by it.

**Three more bugs found the same way — by calling a real, locally-run entity-service against the real database, not just a hand-written fake — while verifying this change further:**

- `ProjectView.OnboardingStatus` on the wire is `"Not-Started"`/`"In-Progress"`/`"Completed"`/`"Not-Applicable"` (hyphenated, title-case), not the raw `NOT_STARTED`/`IN_PROGRESS` enum label the database column stores — entity-service's own `project_repo.go` reshapes it with `INITCAP(REPLACE(p.onboarding_status::TEXT, '_', '-'))` before it ever reaches a caller. A check written against the raw enum compiles and passes every fake-backed unit test, then matches nothing against real data — `hasNoGoLive` would never fire for any account, silently. Fixed with `projectNeedsGoLive`, which checks the real hyphenated values.
- (Superseded: `GetCustomerHealthDetail` is now one entity-service request and none of the helpers named here exist any more.) `GetCustomerHealthDetail` originally re-fetched a project's entire case list from scratch, separately, once per flag (recent/abandoned/delayed/escalated) and again per project — for a real account with 21 projects and ~1,600 cases this took over two minutes and timed out. Fixed by fetching the account's cases and escalations exactly once (`accountCasesFull`/`searchEscalationsForCaseIDs`, bounded pagination) and grouping/filtering them client-side per project (`projectDetail` now takes the pre-grouped slices instead of making its own network calls). A fake test double that returns instantly can't surface a cost bug like this; it only shows up against real data volume.
- entity-service's own pagination hard cap is 50 rows per search request (`limit cannot exceed 50`), confirmed live — the former `searchEscalationsForCaseIDs` originally requested up to 200 in one call and got a 400. Both this and `GetEscalationsByAccount` (whose own caller-facing `limit` can be up to 100) now loop across entity-service pages instead of assuming one call covers the request.

**A fourth bug, found later a different way — comparing the old (legacy-backed) `/customer-health/summary` response against this one for the identical request, rather than against real entity-service data**: the two endpoints reported very different account counts for the same request, with the new endpoint's extra rows including accounts that are not real, currently-active customers. `filteredAccounts` applied no active/inactive filter at all — every account ever synced, deactivated or not, counted. This was first (incorrectly) addressed by requiring `account.deactivation_date IS NULL`, reasoned from an unrelated legacy account-search endpoint's own exclusion of accounts whose status was "lost" — since superseded (see "Risk flags corrected against the legacy system's own rules" below) once the real account-scoping rule for this specific endpoint was confirmed. `GetCustomerHealthDetail`'s own account lookup (by a specific, already-known account id, not a list) is deliberately left unscoped: a CS engineer may legitimately want to view a deactivated account's health history.

**Risk flags, and the account list itself, corrected against the legacy system's own confirmed rules.** Once those rules were obtained and checked against this port, several things disagreed, each confirmed against real staging data (not just hand-written fakes) by querying the real Postgres database directly and comparing against the legacy endpoint's own response for the identical account/request:

1. **`hasNoGoLive` read the wrong signal entirely.** The old approximation checked `project.onboarding_status` (`Not-Started`/`In-Progress`); the correct rule instead checks whether the project has a recorded go-live date at all, and if not, how long ago the account activated. `ProjectView` had no `GoLiveDate` field on `SearchProjects`'s own response at all — only `GetProjectByID` selected `project.onboarding_go_live_date` — so entity-service's `domain.Project`/`domain.ProjectView`/`project_repo.go`'s `SearchProjects` query/`project_service.go`'s mapping all gained a `GoLiveDate *time.Time` field (the same class of "repository selects it, service layer drops it" bug this file's own history already has precedent for — `ClosureState`/`StartDate`). `entityAccountView` gained `ActivationDate *string` (`account.activation_date`, plain `YYYY-MM-DD`) and `entityProjectView` gained `GoLiveDate *string` (RFC3339 — entity-service marshals `*time.Time`, a different format from `ActivationDate`, so the two needed different `time.Parse` layouts). `projectGoLiveStatus`/`accountGoLiveStatus` (`customer_health_postgres.go`) are the faithful port.
2. **`hasRecentCases`/`noSupportCases6mo` counted announcements as cases.** The correct rule explicitly excludes `Announcement`-type work items; `accountHasCasesSince` applied no type filter at all, so an account whose only recent activity was an announcement was wrongly reported as having recent cases (and, by the inverse, wrongly *not* flagged as having no support cases in 6 months). Since entity-service's case-search `type` filter only supports `op: "in"` (never `notIn` — see that repo's own `CLAUDE.md`, "`case_filters.go`"), "exclude announcement" is expressed as "include every other case-like type" (`customerHealthCaseLikeTypes`), applied consistently to both the recent-cases check and `projectDetail`'s own case grouping.
3. **`hasEolProduct` missed deployments that had been deactivated.** See the bullet above and entity-service's own `CLAUDE.md` for the full fix (`SearchDeploymentsRequest.IncludeInactive`) — found by a live reproduction against a real staging account whose only deployments carrying real, expired product versions were all `is_active = false`, which `SearchDeployments`'s permanent `is_active = TRUE` filter (a deliberate default for every other caller, e.g. a project's own deployment listing) silently excluded.
4. **The account list itself was scoped wrong (see the fourth bug above).** The correct rule is `account.life_cycle = 'Customer'` (a distinct column from `account.classification`, a different, business-segment axis) **and** the account has at least one project — not `account.deactivation_date IS NULL`. Confirmed directly against staging: `life_cycle = 'Customer'` alone still overcounts; adding "has at least one project" narrows that to exactly the legacy endpoint's own reported count for the identical, unfiltered request. Notably the correct rule has **no** active/inactive check at all — several of the matching accounts are themselves deactivated and must still be included, which is why `SearchAccountsFilters` gained `LifeCycle`/`HasProject` (both additive; entity-service's own `CLAUDE.md` has the repository-level details) rather than this continuing to ride on `Active`.

Verified end to end against real staging data for one account exercising the first three fixes at once: `account.activation_date` and both projects' `onboarding_go_live_date` are NULL (→ `"N/A (No Start Date)"`, pending, matching the legacy endpoint exactly); the account's only `work_item` rows are three `ANNOUNCEMENT`-type records (→ `hasRecentCases: false`/`noSupportCases6mo: true`, matching); and every EOL-expired deployed product on the account lives on a deployment with `is_active = false`, while the account's two active deployments carry zero deployed products at all (→ `hasEolProduct: true` only with `includeInactive: true`, matching). All seven flags for this account now match the legacy endpoint's own response exactly, and the account-list count matches for the unfiltered request too.

**`GetCustomerHealthDetail` 404'd for every account — a real, 100%-reproducing bug, not a data gap.** It resolved the account via `POST /accounts/search` with `Filters.SearchQuery` set to the account's own UUID — but entity-service's `SearchQuery` filter only ever matches `account.name`/`sf_id`/`number` with `ILIKE`, never `account.id` (see that repo's own `account_repo.go`), so a UUID search query matches nothing and the account was always reported not-found, regardless of whether it existed. Confirmed against the legacy implementation this ports: it resolves the account with a direct by-id lookup and maps only a genuine upstream not-found response to its own 404 — the same shape this should have had from the start. Fixed by calling entity-service's real by-primary-key `GET /accounts/{id}` (`entity.CustomerEntityClient.GetAccount`, already used elsewhere in this backend — `accounts.go`'s own `GetAccount` handler) instead, mapping only a genuine upstream 404 (`apierror.Error.StatusCode == 404`) to `viewer.ErrAccountNotFound` so a real unknown-account 404 still reaches the caller the same way it always did. Verified end to end against real staging data: a real account id now returns its full detail payload (previously 404 for every account without exception); an unknown id still 404s. The sibling account-detail-page endpoints (`GET /accounts/{id}/project-health-statuses`, `/health-summary`, `/action-items`, all backed by `internal/risk` → entity-service's own by-id routes) were checked too and were never affected — this bug was isolated to the one method that used a search instead of a direct lookup.

**The account detail is one request (`GET /accounts/{id}/health-detail`).** `GET /customer-health/accounts/{id}` used to make a stream of gateway calls per click: `GET /accounts/{id}`, a projects search, a case search that paged every case of the account (50 per request, up to 20,000), an escalation search for those cases, then a deployment search (twice) and a deployed-product search per project, and it got 429-throttled on larger accounts. `postgresCustomerHealthClient.GetCustomerHealthDetail` now makes exactly one call, `entity.CustomerEntityClient.GetAccountHealthDetail`, and `accountDetailFromEntity` reshapes the answer into `viewer.AccountDetail`; entity-service computes the flags, windows and drill-down lists in a fixed number of statements however many projects and cases the account has. The wire format is unchanged: every id (`sysId`, `escalationSysId`, case, deployment and project ids) is the flat 32-character form (`uuidToSysID`, dashes stripped) that `internal/risk` uses, so the webapp can still match a project's card to its health-status row; every list is `[]`, never `null`; a `detailedEscalatedCases` entry carries `number`, `escalationSysId` and `escalationNumber` (the case number: an escalation has no number of its own) and no `sysId`. A 404 from entity-service (no such account) still becomes `viewer.ErrAccountNotFound` and the handler's 404; any other error goes through `mapUpstreamErrorGeneric` (a 429 surfaces as 500, 502/503/504 as 503). Go-live status, abandoned/delayed windows and the escalation rule now live in entity-service only; this backend no longer re-implements them. Deleted with this change: `accountCasesFull`, `searchEscalationsForCaseIDs`, `accountProjects`, `projectDeployments`, `deployedEolProducts`, `projectDetail` and its grouping helpers, `projectGoLiveStatus`, the case-page and window constants, and the entity search view types only they used.

**Removed as dead code**: case attachments (`GET /cases/{caseId}/attachments-info`, `GET /attachments/{attachmentId}/download`) and the ABT team schedule (`GET /abt-team-schedule`) had zero callers anywhere in the webapp or microapp — the real case-attachment feature calls entity-service's `POST /attachments/search`/`GET /attachments/{id}/content` directly, and the real team-schedule feature (`CsmTeamSchedulePage.tsx`) calls the separate, entity-service-backed `/team-schedule/*` routes (`cmd/server/main.go`'s unconditionally-registered `scheduleHandler`). All three went with the ServiceNow client (see the top of this section).

**Marking a project at-risk/healthy silently failed to update on screen — a dashed-vs-flat sys_id mismatch between two sibling endpoints.** `GetCustomerHealthDetail`'s `ProjectDetail.SysID` (and every other sys_id-shaped field it builds — deployment/case/escalation sys_ids) was a raw dashed entity-service UUID, while `internal/risk`'s own `GetAccountHealthStatus` (backing `GET /accounts/{id}/project-health-statuses`) returns `ProjectSysID` in the flat, no-dashes sys_id shape documented in `internal/risk`'s own "sys_id ↔ UUID" conversion (see its CLAUDE.md). The webapp joins these two responses by comparing `s.projectSysId === project.sysId` — a format mismatch meant the join could never match, so every project card silently fell back to its default "to be reviewed" display regardless of the real stored status, and a status mutation appeared to do nothing because the page immediately re-read the now-current data through the same broken join. Fixed by adding `uuidToSysID` (`customer_health_postgres.go`, mirroring `internal/risk`'s own private helper of the same name and purpose) and applying it everywhere `GetCustomerHealthDetail` builds a sys_id-shaped field, so both endpoints now agree on the flat format. A second, related bug in `summaryFilteredByHealthStatus` (the Customer Health list's health-status filter) compared the opposite way — an `idSet` built from flat sys_ids against `acc.AccountSysID`, which is dashed — so every health-status filter selection matched zero accounts; fixed with the inverse helper, `sysIDToUUID`. Verified end to end against real staging data for a real account's projects, confirming `ProjectDetail.SysID` now outputs the same flat value `GET /accounts/{id}/project-health-statuses` reports for the identical project.

**The health-status-filtered summary path 400'd on every single request — a pagination-cap mismatch.** `summaryFilteredByHealthStatus` (`customer_health.go`, now `summaryViaSearch`) paginates through entity-service's `POST /accounts/search` while intersecting a health-status filter against the full account set, and used `batchSize = 200` — but entity-service's own `normalizePagination` hard-caps `limit` at 50 ("limit cannot exceed 50"), rejecting the whole request with a 400 regardless of how many accounts would have matched. Selecting Healthy/At Risk/To Be Reviewed on the Customer Health dashboard could never return a result. Fixed by lowering `batchSize` to 50 (and `maxBatches` from 500 to 2000, to keep the same ~100,000-account ceiling). **Superseded:** that paging loop (and the in-memory intersection with `GetAccountsByHealthStatus`) is gone, and so is the branching: `GetSummary` is now, for EVERY request with or without a health-status filter, exactly one `POST /accounts/search` (`entity.CustomerEntityClient.SearchAccountsWithHealth`) carrying the `lifeCycle`/`hasProject` base filter, `searchQuery`, `ownerEmail` (always the caller's token email), `subRegion`, `risks`, `product`, `abtTeam`, `healthStatus` and `includeHealthSummary`; entity-service filters, paginates, returns the filtered `total` and each row's health flags, so the cost no longer grows with the account count (the old path made thousands of gateway calls and was throttled with a 429, surfaced as a generic 500). `limit` is cut to entity-service's cap of 50 for every request (`healthSummarySearchMaxLimit`; the webapp sends at most 25; the old unfiltered path allowed this backend's 100); the `healthStatus` filter is omitted from the body when the request carries none, and an unrecognised non-empty `healthStatus` still answers empty without an upstream call. Each row's `healthStatus` is the lowercase value from its `healthSummary`; an account with no recorded health rows reads `to_be_reviewed`, exactly what the batch health-summary lookup the unfiltered branch used to call reported. The per-account fan-out (`accountFlags` per account, `GetBatchHealthSummaries`) and the 10,000-account client-side scan that region/risks/product/abtTeam filters used are deleted.

**The "My Accounts" email filter trusted a caller-supplied email verbatim — an IDOR.** The webapp's only caller of this field (`CustomerHealthDashboardPage.tsx`'s "My Accounts" scope toggle) always sends the signed-in user's own email, with no UI path to send anyone else's — but `GetSummary` passed whatever `email` the request body carried straight to entity-service, so any caller could substitute a colleague's email and see that colleague's own "my accounts" results. `GetSummary` now overrides any non-empty `payload.Email` with the authenticated caller's own token email (`middleware.UserInfo.Email`) before forwarding — a non-empty value in the body is only ever read as "filter to my own accounts," never as whose.

**Superseded (deleted):** **Combining more than one client-side filter (region/risks/product/abtTeam) was slow — each of up to 10,000 scanned accounts computed its risk flags via a dozen+ sequential entity-service calls, one account at a time.** `filteredAccounts`' per-page account loop now delegates to `scanAccountsPage`, which evaluates the accounts of one page concurrently (`golang.org/x/sync/errgroup`, bounded to `customerHealthScanConcurrency` = 8 accounts at once) instead of sequentially. Each account's own calls still happen in the same internal order as before (intra-account parallelization was tried and reverted — it broke a test fixture that depends on `SearchCases` call order, and a real `-race` data race once two accounts' calls landed on the same connection concurrently); only the cross-account fan-out is new. Order is preserved exactly: each goroutine writes its verdict into an index-addressed slice, and the matched set is reassembled by walking that slice in the original page order afterward, so pagination stays stable across repeated identical requests (`TestPostgresCustomerHealthClient_RisksFilter_PreservesAccountOrder`). The whole scan, including `scanAccountsPage` and `customerHealthScanConcurrency`, is gone: region/risks/product/abtTeam are server-side filters of the single `POST /accounts/search` call, so there is nothing left to parallelize.

## Custom dashboards (`/dashboards/generate`, `/dashboards/custom/*`, `/dashboards/shared/*`)

A saved dashboard is the definition the user saved, stored by entity-service as opaque text
(`custom_dashboard.definition`). Every read checks it again with `dashboard.ParseGenerated`, the check
it passed when it was saved (`GetCustomDashboard`, `GetSharedDashboard` in
`internal/handler/custom_dashboards.go`).

**Known limitation: saved definitions are not migrated.** When a widget type, field or rule changes so
that a saved definition no longer passes, the owner's and every recipient's view answer 422 (the
message asks the owner to generate it again), and the only repair is regenerating it. Drawing it
anyway could run queries the current rules refuse. A change that would break saved dashboards needs
its own plan: keep accepting the old form, or migrate the stored rows.

**Who can reach what** (pinned by `cmd/server/custom_dashboard_routes_test.go`): generating, saving,
deleting and sharing (`/dashboards/generate`, `/dashboards/custom/*`) are `PermWrite`, CS engineers and
admins. Reading what was shared with you (`/dashboards/shared*`) is `PermView`, like the system
dashboards, because the share picker offers every internal user and a team share reaches every member:
with `PermWrite` a view-only recipient would be handed a dashboard that answers 403. Opening one grants
no data: each widget calls its own search route with the viewer's permissions (time cards, incidents
and change requests keep theirs), so a widget outside the viewer's role fails on its own.

**What one user can spend on model calls.** Every `POST /dashboards/generate` that passes the prompt
checks counts against the caller's `handler.GenerateRateLimiter` (keyed by the token's user id):
`DASHBOARD_GENERATE_LIMIT_PER_MINUTE` (default 10) in any minute and `DASHBOARD_GENERATE_LIMIT_PER_DAY`
(default 100) in any 24 hours, else 429 with `Retry-After` before the generator is asked. The counts are
in memory, so per backend instance and reset on restart: a cost guard, not an exact quota. The draft is
capped too: over `generateDraftMaxBytes` (64 KiB, the size entity-service allows a saved definition) as a
definition is a 413 before any call, and csm-ai-integration-service refuses the same with a 400, so one
call sends the model at most the catalogue, a 2,000-character prompt and a 64 KiB draft.

## Running locally

```bash
# from apps/csm-portal/backend
go run ./cmd/server/main.go
```

The server auto-loads `.env` from the working directory at startup (silently ignored if absent). No need to `source .env` manually.

`cp .env.example .env` is enough to start: `DASHBOARDS_DIR` points at the committed `dashboards.example/` (a missing directory is fatal). For a real dashboard set, `cp -r dashboards.example dashboards` and repoint it — `./dashboards` is gitignored.

`CSM_TEAM_REGISTRY` (team vocabulary: `teamKey|Display Name|FAMILY|groupId` rows, comma separated) and `CSM_USER_ROLES` (assignable-role allow-list) are read **here**, not in `entity-service` — they moved. Both are resolved once at startup into `internal/directory`, so `POST /roles/search` makes no upstream call. `POST /teams/search` lists the entity service's `team` table (`ReferenceHandler.WithEntityClient`) and swaps each team whose name matches a registry row for that row's entry (registry key as `id`, family, group ids) — consumers resolve a team id by key, so a matched team must keep it; an unmatched team is listed by its `team` id and name alone. A request with a `family` filter, or a handler with no entity client, is answered from the registry alone, because family lives only there. A malformed row, an unknown family, or a duplicate team key/display name is fatal at startup, naming the row. Both are flat single-line strings on purpose: the deployment platform's configuration UI is one-dimensional and stringifies nested collections, so a structured registry cannot be deployed at all. Do not introduce nested-collection configuration.

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
- **Upstream errors**: use `mapUpstreamErrorGeneric(w, err, "<fallback message>")` for every endpoint by default — never write custom status mappings inline. Only the ten PATCH/update handlers (`PatchCase`, `PatchCallRequest`, `PatchMe`, `UpdateProject`, `PatchDeployment`, `PatchDeployedProduct`, `PatchChangeRequest`, `UpdateTimeCard`, `UpdateTask`, `PatchIncident`), plus the customer-health lifecycle/mutation handlers (`OpenRisk`, `CloseRisk`, `MarkHealthy`, `RevertReview`, `CreateActionItem`, `UpdateActionItemStatus`, `UpdateActionItem`, `CreateActionItemComment` — `internal/handler/customer_health.go`/`customer_health_action_items.go`), and the custom dashboard writes (`SaveCustomDashboard`, `AddCustomDashboardShare`, `AddCustomDashboardTeamShare` — `internal/handler/custom_dashboards.go`: a duplicate dashboard name, the 50-dashboard limit, an unknown or non-staff user, sharing with yourself), use `mapUpstreamError` instead, which surfaces the upstream 400/409/422 reason (e.g. "Invalid state transition") — appropriate there because the request body just submitted is what's being rejected: entity-service's own lifecycle refusals on these endpoints (e.g. "action items can only be added to open risks", "resolutionComment is required when status is RESOLVED or CANCELLED") are caller-actionable, not internal detail to hide. Every other endpoint (search/create/get/delete) forwards a payload that's only partially validated at this layer, so a 4xx from upstream isn't reliably something the caller could have avoided; `mapUpstreamErrorGeneric` returns the fixed fallback message for those instead of echoing upstream detail. Both log the full reason via the caller's `slog.ErrorContext(ctx, ..., "err", err)` regardless of which is used.
- **Skills handlers are a further documented exception** (`internal/handler/skills.go`, `writeSkillError`): on all twelve `/skills/...` routes a 400, 403, 404 or 409 from entity-service keeps its own `message` (and `errorCode` when named). Those texts are written by entity-service's skills module for the caller (e.g. "level is not part of this skill's scale", "skill is already on your profile", "duplicate skill name") and are the only way a user can tell why an add or edit was refused; they carry no internal detail. 5xx and every other status still go through `mapUpstreamErrorGeneric`.
- **`GetMe` maps an upstream 404 to 403, not 404 — unless the caller's token already grants a portal role.** A 404 here means the caller's own authenticated identity has no backing user row — not a missing resource the caller asked for by ID — and the webapp's data-fetching hook for this endpoint has no handling for a bare 404, so it spun forever instead of showing anything. Since "you don't have permission" is already a handled UI state, `GetMe` treats this upstream 404 as 403 by default, while still logging the real cause at `ERROR` (without the caller's email, to stay off the PII list above). But a caller can hold a real `AUTH_<ROLE>_ROLES` grant via their token with no entity-service `"user"` row yet to show for it (the same provisioning gap `ensureUserProvisioned` exists for, just not yet hit for this caller) — 403ing them outright would lock out someone the access policy already decided should be in. So when `resp.Roles` (computed from the token before the entity-service call) is non-empty, `GetMe` answers 200 instead, built entirely from the token: `email`/`firstName`/`lastName`/`roles` (SCIM phone lookup still runs, best-effort as usual), with `id`/`timeZone`/`team` left `null` since those only ever come from the entity-service row that doesn't exist. Both `id` and `team` are `json:"id"`/`json:"team"` (no `omitempty`) specifically so this null is visible on the wire rather than silently absent. Follow this same substitution if another identity-bound "fetch my own X" endpoint hits the same failure mode.
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
