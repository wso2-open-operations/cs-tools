# 05 — Other Features: Operations, Security, Announcements, Project Details, Settings, Updates, Usage Metrics, Project Hub

---

## 1. Operations (`src/features/operations/`)

### Purpose
SR (Service Requests) and CR (Change Requests) management. Hub shows outstanding counts.
SR creation uses **catalog-driven dynamic forms** (deployment → product → catalog → variables).
CR has a **customer approval/review workflow** and calendar view.

### Routes
```
/projects/:projectId/operations                              OperationsPage (hub)
/projects/:projectId/operations/service-requests            ServiceRequestsPage
/projects/:projectId/operations/service-requests/create     CreateServiceRequestPage
/projects/:projectId/operations/service-requests/:id        ServiceRequestDetailsPage
/projects/:projectId/operations/change-requests             ChangeRequestsPage
/projects/:projectId/operations/change-requests/:id         ChangeRequestDetailsPage
```
**All duplicated under `/support/` tree** (same components, `getOperationsNavSegment(pathname)` picks URL prefix).

### Pages & key state
| Page | State |
|------|-------|
| `OperationsPage` | Stats + last 5 SR/CR; permissions gate |
| `ServiceRequestsPage` | **useSessionState**: search, filters, sort, page, rowsPerPage; location.state: `returnTo`, `outstandingOnly`, `actionRequired`; URL: `createdByMe` |
| `CreateServiceRequestPage` | Local: deployment, product, catalog IDs, `variableValues`, attachments; URL params: `deploymentId`, `productId` pre-fill |
| `ServiceRequestDetailsPage` | `useGetCaseDetails`; `returnTo` from state |
| `ChangeRequestsPage` | **useSessionState**: search, filters, page, rowsPerPage, sort; local: `viewMode` (list/calendar); location.state: `returnTo`, `outstandingOnly`, `actionRequired`, `scheduledOnly` |
| `ChangeRequestDetailsPage` | Local: `proposeTimeOpen`; `useGetChangeRequestDetails`, `usePatchChangeRequest` |

### SR creation catalog flow
```
CreateServiceRequestPage:
  1. Pick deployment (from usePostProjectDeploymentsSearchInfinite)
  2. Pick product version (from usePostDeploymentProductsSearch)
  3. Pick catalog + catalog item (useSearchCatalogs, CatalogSelector accordion)
  4. Fill variables (useGetCatalogItemVariables → VariableFormFields dynamic form)
  5. Upload optional attachments
  6. Submit → usePostCase → navigate to SR detail
```

Variable types include: rich text, datetime picker, attachment field, text, boolean.
Context fields (project name, deployment type, product version) are auto-filled from selection.

### CR workflow states (customer-visible)
```
New → Assess → Authorize → Customer Approval → Scheduled → Implement
  → Customer Review → Closed
  → Rollback
  → Canceled
```
Customer actions: Approve, Reject, Propose New Time (Customer Approval state); Successful, Unsuccessful (Customer Review state).

### Operational state presets
| Location state key | Status filter |
|-------------------|---------------|
| `outstandingOnly: true` | Excludes closed/canceled/rollback |
| `actionRequired: true` | Customer Approval + Customer Review states only |
| `scheduledOnly: true` | Scheduled state only |

### Key utils
| File | Purpose |
|------|---------|
| `operationsPages.ts` | `getOperationsNavSegment`, SR/CR search request builders, state ID resolution |
| `serviceRequestValidation.ts` | Required fields, field type detection, title length |
| `changeRequests.ts` | Workflow stages, PDF generation |
| `changeRequestUi.ts` | Impact label and color |
| `changeRequestsCsvExport.ts` | CSV export |
| `changeRequestsSchedulePdf.ts` | Calendar schedule PDF |
| `caseRefresh.ts` | Post-create query invalidation |

### Constants (`operationsConstants.ts` — 408 lines)
- `ChangeRequestStatus` enum, state labels, state order, API ID → label map
- `ChangeRequestImpact` enum + labels
- `CHANGE_REQUEST_STAT_CONFIGS`, `OPERATIONS_LIST_PAGE_SIZE` (10)
- Filter definitions: `CHANGE_REQUEST_FILTER_DEFINITIONS` (state, impact)
- Calendar: weekday labels, legend states
- All page copy (titles for outstanding/action-required/scheduled views)

---

## 2. Security (`src/features/security/`)

### Purpose
Security Center: **Security Report Analysis** (security cases as a filtered list) and
**Component Analysis** (product vulnerability table). Stat cards drill into filtered report list.

### Routes
```
/projects/:projectId/security-center                         SecurityPage (tabs)
/projects/:projectId/security-center/:vulnerabilityId        VulnerabilityDetailsPage
/projects/:projectId/security-center/security-report-analysis/:caseId  → CaseDetailsPage (support)
```

### Pages
| Page | Key state |
|------|-----------|
| `SecurityPage` | URL `?tab` param; local `fixedStatusIds` + `activeStatKey` + `fixedClosedDateRange` when stat card clicked |
| `VulnerabilityDetailsPage` | `useGetProductVulnerability` |

### Component tree
```
SecurityPage
├── SecurityStats (outstanding / resolved-30d stat cards)
├── TabBar (Security Report Analysis | Component Analysis | Security Advisories)
└── Active tab:
    ├── SecurityReportAnalysis
    │   ├── TabBar (My Reports | All Reports)
    │   ├── ListSearchBar + status filters
    │   ├── ListResultsBar
    │   ├── ListCard × n (useGetProjectCases)
    │   └── CaseListCsvExportButton
    ├── ProductVulnerabilitiesTable
    │   ├── ProductVulnerabilitiesTableHeader (search + filter toggle)
    │   ├── ProductVulnerabilitiesFilters (severity, product, version)
    │   └── ProductVulnerabilitiesList (table rows → VulnerabilityDetailsPage)
    └── SecurityAdvisoriesTable (placeholder "coming soon")

VulnerabilityDetailsPage → VulnerabilityDetailsContent
    (CVE, NVD link, component, severity chip, resolution details)
```

### Stat card behavior
```
SecurityStats: Outstanding → fixedStatusIds (open/in-progress)
SecurityStats: Resolved (30d) → fixedStatusIds (resolved) + fixedClosedDateRange (last 30 days)
SecurityReportAnalysis receives fixedStatusIds + fixedClosedDateRange
```

### Key utils
- `securityPage.ts` — parse tab, build security case search, parse view mode/sort
- `vulnerabilities.ts` — severity/status color tokens, chip helpers
- `productVulnerabilitiesTable.ts` — debounced search, filter building, option derivation

### Permissions
`hasSecurityReportAnalysis`, `hasComponentAnalysis`, `hasSraWriteAccess` from `getProjectPermissions`.

---

## 3. Announcements (`src/features/announcements/`)

### Purpose
Browse and read announcement cases (specific `caseType`) with search, state/severity filters, and a read-only HTML detail panel.

### Routes
```
/projects/:projectId/announcements            AnnouncementsPage
/projects/:projectId/announcements/:caseId    AnnouncementDetailsPage
```

### Pages
| Page | Key state |
|------|-----------|
| `AnnouncementsPage` | **useSessionState**: search, filters, sortField, sortOrder, page, rowsPerPage |
| `AnnouncementDetailsPage` | `useGetCaseDetails`; no tab structure |

### Component tree
```
AnnouncementsPage
├── ListPageHeader
├── ListSearchBar
├── ListFiltersPanel (ANNOUNCEMENT_FILTER_DEFINITIONS — status, severity)
├── ListResultsBar (sort: Updated On, Status)
├── AnnouncementList (AnnouncementCard rows)
└── ListPagination

AnnouncementDetailsPage → AnnouncementDetailsPanel
    ├── Back button
    ├── Title, dates, severity/status chips
    ├── DOMPurify-sanitized HTML description
    └── CaseDetailsActionRow (conditional)
```

### Key utils (`announcements.ts`)
- `buildAnnouncementCaseSearchRequest` — announcement case type + filters
- `resolveAnnouncementListFilterOptions` — restricts states to `ANNOUNCEMENT_CASE_STATE_ALLOWED_VALUES` (`"1"`, `"3"`)
- `formatAnnouncementDateDisplay`, `normalizeAnnouncementDescriptionHtml`, `isAnnouncementDescriptionEffectivelyEmpty`

---

## 4. Project Details (`src/features/project-details/`)

### Purpose
Project profile: overview metadata/contacts/service hours, deployments (add/edit/delete environments and products), and time tracking (case time cards).

### Routes
```
/projects/:projectId/project-details    ProjectDetailsPage (tab bar)
```

### Pages
| Page | Tabs |
|------|------|
| `ProjectDetailsPage` | Overview | Deployments | Time Tracking |

### Component tree (overview)
```
ProjectDetailsPage — Overview tab
└── ProjectInformationCard
    ├── ProjectHeader (name, key badge)
    ├── ProjectDescription
    ├── ProjectMetadataPrimaryRow (type, tier, region)
    ├── ProjectMetadataSecondaryRow (additional metadata)
    ├── SubscriptionDetails
    └── ContactInfoCard (contact rows with roles)
└── ServiceHoursAllocationsCard (permission-gated)
└── ProjectStatisticsCard
```

### Component tree (deployments)
```
ProjectDetailsPage — Deployments tab
└── ProjectDeployments
    ├── DeploymentHeader (add deployment button)
    └── DeploymentCard × n
        ├── DeploymentCardToolbar (edit, delete)
        ├── DeploymentProductList
        │   └── product rows (add/manage/delete modals)
        ├── DeploymentDocumentList (attachments)
        ├── UpdateHistoryTab (per product)
        └── DeploymentCardLicenseFooter
    └── Modals: AddDeploymentModal, EditDeploymentModal, DeleteDeploymentModal
    └── Modals: AddProductModal, ManageProductModal, DeleteProductModal
    └── EditDeploymentAttachmentModal
```

### Component tree (time tracking)
```
ProjectDetailsPage — Time Tracking tab
└── ProjectTimeTracking
    ├── ServiceHoursStatCards (hours summary)
    ├── TimeCardsDateFilter (start/end date)
    ├── TimeTrackingCard × n (case time card rows)
    └── ListPagination
```

### Key facts
- `useSearchProjectTimeCards` / `useSearchProjectCaseTimeCards` live in `usage-metrics/api/` but used here
- Tab visibility gated by `hasDeployments`, `hasTimeLogs`, `showServiceHoursAllocationsCard`
- Deployment "create SR" → navigates to CreateServiceRequestPage with `deploymentId` + `productId` URL params

---

## 5. Settings (`src/features/settings/`)

### Purpose
Per-project settings: user management (invite/edit/remove contacts), AI assistant toggles (Novera, KB), registry tokens (WSO2 Updates 2.0 tokens).

### Routes
```
/projects/:projectId/settings    SettingsPage
```

### Pages
| Page | Tabs |
|------|------|
| `SettingsPage` | User Management | AI Assistant | Registry Tokens (hidden for cloud project types) |

Tab restore: `consumePendingSettingsTab()` reads from session storage on mount.

### Component tree
```
SettingsPage
├── TabBar
└── Active tab:
    ├── SettingsUserManagement
    │   ├── User table (roles: admin, portal, security contact)
    │   ├── AddUserModal (email validate → invite)
    │   ├── EditUserModal
    │   └── RemoveUserModal
    ├── SettingsAiAssistant
    │   └── Toggles: Novera enabled, KB suggestions enabled (PATCH project)
    └── SettingsRegistryTokens
        ├── Sub-tabs: User tokens | Service tokens
        ├── Token table rows
        └── Modals: GenerateTokenModal, DeleteTokenModal, RegenerateTokenModal
```

### Permissions
- `isCustomerAdmin` → can add/remove users
- `isRestricted` → limits some token actions
- Registry tab hidden when `ProjectType` is a cloud type

---

## 6. Updates (`src/features/updates/`)

### Purpose
WSO2 product update levels: search product/version/level range, view pending/installed updates,
generate update levels report (PDF), drill into level detail.

### Routes
```
/projects/:projectId/updates                                  UpdatesPage
/projects/:projectId/updates/pending                          PendingUpdatesPage
/projects/:projectId/updates/pending/level/:levelKey          UpdateLevelDetailsPage
```

**Note:** `UpdatesPage` renders only `AllUpdatesTab` currently. `UpdatesStatsGrid`, `UpdateProductGrid`
exist but are NOT wired into the main page (reserved for "My Updates" experience).

### Pages
| Page | State |
|------|-------|
| `UpdatesPage` / `AllUpdatesTab` | Local filter state (product, version, level range) + URL params (`pn`, `pv`, `sl`, `el`) |
| `PendingUpdatesPage` | URL search params → `usePostUpdateLevelsSearch` |
| `UpdateLevelDetailsPage` | URL params + local filter tab (all/security/regular advisories) |

### Component tree
```
AllUpdatesTab
├── Product/version/level range filters
├── Search results list
├── "View Report" button → UpdateLevelsReportModal
│   └── Security advisories + regular updates + PDF export
└── Link to PendingUpdatesPage

PendingUpdatesPage → PendingUpdatesList
    └── Level rows with "View" → UpdateLevelDetailsPage

UpdateLevelDetailsPage
├── Level description
├── Filter tabs: All | Security | Regular
└── Advisory list (HtmlOrText for safe rendering)
```

### Key utils
- `allUpdatesTab.ts` — filter cascade (product → version → levels), validation
- `updates.ts` — chip colors, navigation helpers
- `updateLevelsReportPdf.ts` — build report data, generate PDF

---

## 7. Usage Metrics (`src/features/usage-metrics/`)

### Purpose
Usage & metrics: aggregated metrics per deployment/environment, trend charts, deployment usage
import, and (shared) project/case time card APIs used by project-details time tracking tab.

### Routes
```
/projects/:projectId/usage-metrics    UsageMetricsPage
```

Redirects to dashboard if `usageMetricsEnabled` feature flag is false.

### Component tree
```
UsageMetricsPage → UsageAndMetricsTabContent
├── UsageMetricsTimeRangeSelector (1/3/6/12 months or custom)
├── Inner tabs: Overview | Per-deployment tabs
│   ├── UsageOverviewPanel (cross-environment breakdown)
│   │   └── UsageMetricTrendCard × n
│   └── UsageEnvironmentProductsPanel (per deployment)
│       └── UsageChartSurface
└── DeploymentUsageUploadDialog (import CSV)
```

### Key utils
- `usageMetricsTab.ts` — inner tabs from deployments, date range presets
- `usageMetricsAggregated.ts` — aggregate metric cards
- `usageMetricsEnvironmentProducts.ts` — environment/product panels
- `usageMetricDelta.ts` / `usageMetricSignedDelta.ts` — delta display

### Time cards (shared with project-details)
`useSearchProjectTimeCards` and `useSearchProjectCaseTimeCards` in `usage-metrics/api/` are consumed
by both `UsageMetricsPage` and `project-details/components/time-tracking/ProjectTimeTracking`.

---

## 8. Project Hub (`src/features/project-hub/`)

### Purpose
Landing page after authentication: project grid, search, auto-redirect for single project,
suspended account state, ServiceNow external deep link handling.

### Routes
```
/            ProjectHubPage (inside AuthGuard)
/support     ServiceNowCaseRedirectPage
```

### Pages
| Page | State |
|------|-------|
| `ProjectHubPage` | Local: `searchQuery` (debounced 300ms), `showBackToTop`; `contentView` enum: `loading`, `singleProject`, `suspended`, `loaded` |
| `ServiceNowCaseRedirectPage` | Renders nothing; resolves `?sys_id` → redirect |

### Auto-redirect logic
```
ProjectHubPage mount:
  if projects.length === 1 and !suspended
    → navigate(/projects/{id}/dashboard, replace: true)
  if all projects suspended
    → render AccountSuspendedPage
  if ≥2 projects
    → show grid
```

### Component tree
```
ProjectHubPage
├── Header (search bar, no sidebar)
└── Grid of ProjectCard × n
    ├── ProjectCardBadges (project key)
    ├── ProjectCardInfo (title clamped 2 lines)
    ├── ProjectCardStats (chats, outstanding, action-required, date)
    └── ProjectCardActions ("View Dashboard" CTA)
```

### ServiceNow redirect
```
GET /support?sys_id=XXXXXX
  → useGetCaseDetails(undefined, sys_id_as_caseId)
  → on success: navigate(/projects/{case.projectId}/support/cases/{case.id}, replace: true)
  → on failure: navigate("/404")
```

### Persistence
`setLastSelectedProject(id)` on card click → `localStorage["last_selected_project"]`
→ `AuthGuard` reads on `/` to redirect to last project

---

## Cross-feature dependency graph

```
project-hub ──→ settings (setLastSelectedProject)
             ──→ support (useGetCaseDetails for ServiceNow redirect)

dashboard ──→ support (cases, filters, stats, CasesTable)
          ──→ operations (CR stats, operations chart)
          ──→ engagements (chart nav)

support ←── engagements (case types, list components, CaseDetailsPage)
        ←── operations (SR/CR details reuse CaseDetailsContent)
        ←── security (SRA case list, CaseDetailsPage)
        ←── announcements (case types, list components)
        ←── dashboard (stats API types, filter constants)

project-details ──→ operations (SR create nav)
                ──→ usage-metrics (time card APIs)

security ──→ support (cases API, create case, CaseDetailsPage)
settings ──→ project-hub (setLastSelectedProject)
updates ──→ (mostly standalone)
usage-metrics ──→ project-details (shared time tracking types/utils)
```
