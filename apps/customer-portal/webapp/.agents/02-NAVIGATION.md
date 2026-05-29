# 02 — Navigation & Routing

## Architecture summary

- **Router:** React Router 7.1.5 with `BrowserRouter` (no `basename`, no `createBrowserRouter`)
- **Route tree:** Entirely in `src/App.tsx` — no `src/routes/` directory
- **No code splitting:** No `React.lazy`, no `Suspense` route boundaries
- **No basename:** App must be served at domain root; subdirectory deployment requires code changes

---

## Provider stack (outer → inner)

```
main.tsx
  AppWithConfig.tsx
    AsgardeoProvider (auth)
    BrowserRouter
    LoggerProvider
    OxygenUIThemeProvider
    QueryClientProvider
    MobileAppGate
    App.tsx
      LoaderProvider
      ErrorBannerProvider
      SuccessBannerProvider
      ErrorPageProvider
      <Routes>
```

---

## Full route tree

```
Routes
├── /401                                    ErrorLayout + Error401Page
├── /403                                    ErrorLayout + Error403Page
├── /404                                    ErrorLayout + Error404Page
├── AuthGuard (layout — ProtectedRoute → AppLayout + Outlet)
│   ├── /                                   ProjectHubPage
│   ├── /support                            ServiceNowCaseRedirectPage
│   └── /projects/:projectId               ProjectGuard (layout — Outlet)
│       ├── index                           Navigate to="dashboard" replace
│       ├── dashboard                       DashboardPage
│       ├── dashboard/action-required       DashboardItemsPage mode="action-required"
│       ├── dashboard/outstanding-interactions  DashboardItemsPage mode="outstanding-interactions"
│       ├── dashboard/closed-last-30d       DashboardItemsPage mode="closed-last-30d"
│       ├── project-details                 ProjectDetailsPage
│       ├── operations                      OperationsPage
│       ├── operations/service-requests                     ServiceRequestsPage
│       ├── operations/service-requests/create              CreateServiceRequestPage
│       ├── operations/service-requests/:serviceRequestId   ServiceRequestDetailsPage
│       ├── operations/change-requests                      ChangeRequestsPage
│       ├── operations/change-requests/:changeRequestId     ChangeRequestDetailsPage
│       ├── support                                         SupportPage
│       ├── support/cases                                   AllCasesPage
│       ├── support/cases/:caseId                           CaseDetailsPage
│       ├── support/change-requests                         ChangeRequestsPage (duplicate)
│       ├── support/change-requests/:changeRequestId        ChangeRequestDetailsPage (duplicate)
│       ├── support/conversations                           AllConversationsPage
│       ├── support/conversations/:conversationId           ConversationDetailsPage
│       ├── support/service-requests                        ServiceRequestsPage (duplicate)
│       ├── support/service-requests/create                 CreateServiceRequestPage (duplicate)
│       ├── support/service-requests/:serviceRequestId      ServiceRequestDetailsPage (duplicate)
│       ├── support/chat                                    NoveraChatPage
│       ├── support/chat/:conversationId                    NoveraChatPage
│       ├── support/chat/describe-issue                     DescribeIssuePage
│       ├── support/chat/create-case                        CreateCasePage
│       ├── support/chat/create-related-case                CreateCasePage
│       ├── support/security-report/create                  CreateCasePage
│       ├── updates                                         UpdatesPage
│       ├── updates/pending                                 PendingUpdatesPage
│       ├── updates/pending/level/:levelKey                 UpdateLevelDetailsPage
│       ├── security-center                                 SecurityPage
│       ├── security-center/security-report-analysis/:caseId  CaseDetailsPage (reuse)
│       ├── security-center/:vulnerabilityId                VulnerabilityDetailsPage
│       ├── engagements                                     EngagementsPage
│       ├── engagements/:caseId                             CaseDetailsPage (reuse)
│       ├── usage-metrics                                   UsageMetricsPage
│       ├── announcements                                   AnnouncementsPage
│       ├── announcements/:caseId                           AnnouncementDetailsPage
│       └── settings                                        SettingsPage
└── *                                       ErrorLayout + Error404Page (wildcard fallback)
```

**Key shared components used on multiple routes:**
- `CaseDetailsPage` — support cases, engagements, security SRA, announcements (adapts via `location.pathname`)
- `ChangeRequestsPage` / `ChangeRequestDetailsPage` — both operations and support trees
- `ServiceRequestsPage` / `ServiceRequestDetailsPage` / `CreateServiceRequestPage` — both trees
- `getOperationsNavSegment(pathname)` returns `"operations"` or `"support"` for URL prefix decisions

---

## Layouts

### `AuthGuard.tsx`
- Wraps all authenticated routes
- Uses `@asgardeo/react-router` `ProtectedRoute` with `loader={<AppLayout />}` (shows loading chrome during auth)
- **Deep-link preservation:** before sign-in, stores `location.pathname + location.search` in `sessionStorage["post_login_redirect"]` (unless path is `/`)
- **Post-login redirect:** restores `post_login_redirect` with `replace: true`
- **Last-project shortcut:** if path is `/` and user did NOT arrive from header (`fromHeader` state), and `getLastSelectedProjectId()` returns a stored 32-char id → `navigate(/projects/{id}/dashboard, { replace: true })`

### `AppLayout.tsx`
- Full authenticated shell: `IdleTimeoutProvider`, `TopBanners`, `GlobalNotificationBanner`, `HtmlAnnouncementBanner`, `AppShellLayout` (Header + Sidebar + main scroll + Footer), then `<Outlet />`
- **Sidebar hiding:** hidden on project hub (`/`), on portal access error, or when `ErrorPageContext.isErrorPageDisplayed`
- **Detail layout (no padding):** regex on pathname removes horizontal padding for case details, SR details, engagements, vulnerability details, update level pages etc.
- **Scroll:** resets `#main-scroll-container` to top on `location.pathname` change
- **Auth-loading gate:** waits for `!isAuthLoading && !isUserDetailsLoading` before showing outlet; shows "Authenticating…" UI when OAuth `?code=&state=` in URL
- **Portal access:** shows `PortalAccessRequiredPage` on 401-style user-details API error

### `ProjectGuard.tsx`
- Fetches `useGetProjectDetails(projectId)` for the `:projectId` segment
- Loading → centered `LinearProgress`
- API error → `ApiErrorState` (sets `ErrorPageContext`)
- `closureState === SUSPENDED` → `ProjectSuspendedNoticePage`
- Success → `<Outlet />`

### `ErrorLayout.tsx`
- Minimal layout for `/401`, `/403`, `/404`, `*` — top banners only, no auth shell

### `AppShellLayout.tsx`
- Low-level flex shell: header row, sidebar (docked or overlay drawer), main scroll region, footer
- Not route-aware; receives collapsed state + toggle from `AppLayout`

---

## Navigation hooks & utilities

### `useModifierAwareNavigate` (`src/hooks/useModifierAwareNavigate.ts`)
The standard navigation hook used everywhere instead of bare `useNavigate`:
- Tracks Ctrl/Meta via window `keydown`/`keyup`
- If modifier held → `window.open(resolvedUrl, "_blank")` (opens in new tab)
- Otherwise → standard `navigate(path, options)`
- **Use this everywhere** for list row clicks and card links

### `ROUTE_PREVIOUS_PAGE` (`navigationConstants.ts`)
```typescript
export const ROUTE_PREVIOUS_PAGE = -1;  // navigate(-1)
```
Used in case details, conversation details, pending updates, update level detail pages.

### `getOperationsNavSegment(pathname)`
Returns `"operations"` if pathname contains `/operations/`, else `"support"`. Used to build back-links and URLs for SR/CR pages that exist under both trees.

### Path builders
- `buildEngagementDetailPath(projectId, caseId)` → `/projects/{id}/engagements/{caseId}`
- `getOperationsNavSegment(pathname)` → `"operations"` | `"support"`

---

## URL path parameters

| Param | Routes | How read |
|-------|--------|----------|
| `projectId` | All `/projects/:projectId/...` | `useParams()` |
| `caseId` | support/cases, engagements, announcements, security SRA | `useParams()` |
| `serviceRequestId` | SR detail (both trees) | `useParams()` |
| `changeRequestId` | CR detail (both trees) | `useParams()` |
| `conversationId` | conversations detail, chat | `useParams()` |
| `vulnerabilityId` | security-center detail | `useParams()` |
| `levelKey` | updates pending level | `useParams()` |

---

## URL query parameters

| Query key | Page | Purpose |
|-----------|------|---------|
| `sys_id` | `/support` | ServiceNow case ID for redirect |
| `severityId` | `AllCasesPage` | Dashboard severity → filtered cases |
| `statusFilter` | `AllCasesPage`, conversations | `active` / `resolved` / `resolvedViaChat` |
| `createdByMe` | cases, SR, operations hub | `true` → my items |
| `tab` | SecurityPage, case detail return | Security tab id (`SecurityTabId`) |
| `type` | create-case | Case type (e.g. security report) |
| `deploymentId`, `productId` | SR create | Pre-fill deployment/product |
| `productName`, `productBaseVersion`, `startingUpdateLevel`, `endingUpdateLevel`, `mode` | PendingUpdatesPage | Product/update level context |

---

## `location.state` (in-memory, lost on hard refresh)

| Key | Source → Consumer | Purpose |
|-----|-------------------|---------|
| `returnTo` | DashboardPage, SupportPage, OperationsPage → list/detail pages | Explicit back target |
| `fromBack` | List pages → `useSessionState` | Restore filter state on back navigation |
| `fromHeader` | Header brand click | Suppress last-project redirect on `/` |
| `outstandingOnly` | OperationsPage → SR/CR lists | Pre-filter to outstanding |
| `actionRequired` | OperationsPage → SR/CR lists | Pre-filter to action-required |
| `engagementTypeId` | DashboardPage chart → EngagementsPage | Pre-filter engagement type |
| `engagementTypeLabel` | Same | Display label for filter |
| `relatedCase` | CaseDetailsPage → CreateCasePage | Related case for "open related case" |
| `skipChat` | Novera flow → CreateCasePage | Skip chat creation step |
| `scheduledOnly` | OperationsPage → CRPage | Pre-filter to scheduled |

---

## Session/local state persistence

### `useSessionState` (`src/hooks/useSessionState.ts`)
Standard pattern for list page state persistence in sessionStorage:
```typescript
const [filters, setFilters] = useSessionState<AllCasesFilterValues>(
  `${projectId}-cases-filters`,  // project-scoped key
  {},                             // default
  undefined,                      // no validator
  { popOnly: true }               // only restore on browser BACK or fromBack state
);
```

**`popOnly: true`:** restores state only on browser POP (back button) or `location.state.fromBack === true`.
Values are always written on change so back navigation can restore them.

**Pages using `useSessionState`:** `AllCasesPage`, `AllConversationsPage`, `ServiceRequestsPage`,
`ChangeRequestsPage`, `AnnouncementsPage`, `SecurityReportAnalysis` (within SecurityPage).

**Engagements** uses plain `useState` but reads initial filter from `location.state`.

### localStorage / sessionStorage keys

| Key | Storage | Role |
|-----|---------|------|
| `last_selected_project` | localStorage | Last project id; AuthGuard `/` redirect |
| `sidebar_collapsed` | localStorage | Sidebar open/closed persistence |
| `post_login_redirect` | sessionStorage | Deep link preserved through Asgardeo sign-in |
| `pending_settings_tab` | sessionStorage | Restore settings tab after reload |
| `pending_success_message` | sessionStorage | Post-reload success banner |
| `pending_case_details_tab` | sessionStorage | Restore case detail tab after reload |

---

## Cross-feature navigation flows

### Dashboard → AllCasesPage (severity click)
```
DashboardPage: handleSeverityClick(severityId)
  → navigate(`/projects/:id/support/cases?severityId=${id}`, { state: { returnTo: dashPath } })

AllCasesPage:
  reads ?severityId from URL
  buildDashboardCaseSearchFilters({ isDashboardSeverityNavigation: true })
    → resolveCasesTableDefaultStatusIds(caseStates)   ← non-closed status IDs
    → severityIds: [Number(severityId)]
  hides search panel; shows dashboard-specific title
```

### Dashboard → EngagementsPage (chart click)
```
DashboardPage chart segment click:
  → navigate(`/projects/:id/engagements`, { state: { engagementTypeId: "1,2", engagementTypeLabel: "...", returnTo } })

EngagementsPage:
  reads state.engagementTypeId → applies as API filter
  shows "Back" button → navigate(returnTo) or navigate(-1)
```

### SupportPage → AllCasesPage
```
SupportPage:
  → navigate(`support/cases?statusFilter=active`, { state: { returnTo: supportPath } })

AllCasesPage back:
  → navigate(state.returnTo)  // back to Support Center
```

### CaseDetailsPage back priority
1. `location.state.returnTo` → navigate there with `{ fromBack: true }`
2. Engagement route → `/projects/:id/engagements`
3. Security report route → `/projects/:id/security-center?tab=vulnerabilities`
4. Otherwise → `navigate(-1)` via `ROUTE_PREVIOUS_PAGE`

### ServiceNow external deep link
`/support?sys_id=…` → `ServiceNowCaseRedirectPage`:
- Fetches case by `sys_id`
- On success → `navigate(/projects/{projectId}/support/cases/{caseId}, { replace: true })`
- On failure → `navigate("/404")`

### Project hub → project
`ProjectCard` click → `setLastSelectedProject(id)` + `navigate(/projects/${id}/dashboard)`

### Sign-in deep link preservation
1. User visits `/projects/123/support/cases/456` while signed out
2. `AuthGuard.onSignIn` stores `/projects/123/support/cases/456` in `sessionStorage["post_login_redirect"]`
3. After Asgardeo OAuth callback, `AuthGuard` restores the URL with `replace: true`

---

## Sidebar navigation items

`APP_SHELL_NAV_ITEMS` in `src/features/project-hub/constants/appLayoutConstants.ts`:

Items (relative paths under `/projects/:projectId/`):
- `dashboard`, `support`, `operations`, `updates`, `security-center`, `engagements`, `usage-metrics`, `project-details`, `announcements`
- Settings in footer: `/projects/:projectId/settings`

Items filtered in `SideBar.tsx` by:
- `getProjectPermissions(projectFeatures)` → `hasSR`, `hasCR`, `hasEngagements`, `hasSecurityReportAnalysis`, etc.
- `featureFlags.usageMetricsEnabled` → hides "Usage & Metrics"
