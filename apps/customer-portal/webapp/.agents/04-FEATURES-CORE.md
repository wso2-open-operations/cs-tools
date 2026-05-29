# 04 — Core Features: Dashboard, Support, Engagements

---

## 1. Dashboard (`src/features/dashboard/`)

### Purpose
Project home: aggregated stat cards, charts (outstanding incidents by severity, operations chart,
engagements by category, cases trend), embedded outstanding cases table, and drill-down pages for
stat card categories.

### Pages

| Page | Route | Key state |
|------|-------|-----------|
| `DashboardPage` | `dashboard` | Local chart nav callbacks; multiple `useGetProjectCasesStats` calls per case type; permissions from `getProjectPermissions`/`getProjectSeverityPolicy` |
| `DashboardItemsPage` | `dashboard/action-required`, `outstanding-interactions`, `closed-last-30d` | `mode` prop; `expandedSections` Set; `returnTo` from location.state |

### Component tree
```
DashboardPage
├── SupportStatGrid
│   └── StatCard × n (action-required, outstanding, closed-30d, avg-response-time)
├── ChartLayout
│   ├── OutstandingIncidentsChart   (severity pie → AllCasesPage ?severityId=)
│   ├── ActiveCasesChart            (SR/CR pie → OperationsPage lists)
│   ├── CasesTrendChart             (line chart)
│   └── EngagementsChart            (category pie → EngagementsPage)
└── CasesTable
    ├── CasesTableHeader            (title + My/All TabBar + filter toggle)
    ├── CasesFilters                (status, severity, category, deployment select)
    └── CasesList                   (paginated rows → CaseDetailsPage)

DashboardItemsPage
└── ListPageHeader + Accordion × sections
    ├── ListItems (cases, SRA, engagements)
    └── ChangeRequestsList
```

### Cases table (`CasesTable`)
- **View modes:** `DashboardCasesViewMode.MyCases` (`createdByMe: true`) / `AllCases`
- **Filter state:** local `useState` (NOT `useSessionState`) — does not persist across navigations
- **Default statuses:** non-closed via `resolveCasesTableDefaultStatusIds(caseStates)` when no explicit status filter
- **Dynamic filters:** built from `ALL_CASES_FILTER_DEFINITIONS`; deployment optional via `includeDeploymentFilter` prop
- **Severity nav:** handled on `AllCasesPage` (not here) — `CasesTable` passes through `issueIds`

### Key utils
| File | Purpose |
|------|---------|
| `dashboard.ts` | S0 detection, severity colors, chart loading aggregation, `navigateToCreateCase` |
| `dashboardNavigation.ts` | `buildDashboardCaseSearchFilters(params)` — merges outstanding statuses for severity nav |
| `casesTable.ts` | `resolveCasesTableDefaultStatusIds`, `resolveCasesTableSearchStatusIds`, active filter count, severity/status color tokens |
| `dashboardCharts.ts` | Pie slice/legend builders for all three charts |

### Chart navigation targets
| Chart | Click | Destination |
|-------|-------|-------------|
| Outstanding Incidents | severity segment | `/support/cases?severityId={id}` + `returnTo` |
| Active Cases | SR slice | `/operations/service-requests` + `outstandingOnly: true` |
| Active Cases | CR slice | `/operations/change-requests` + `outstandingOnly: true` |
| Engagements | category segment | `/engagements` + `engagementTypeId`, `engagementTypeLabel` |

### Permissions
`getProjectPermissions(features)` + `getProjectSeverityPolicy(features)` gate:
- Which case types appear in stat grid
- Whether S0 is shown (`excludeS0` prop to CasesTable)
- Whether deployment filter appears (`includeDeploymentFilter`)

---

## 2. Support (`src/features/support/`)

### Purpose
Support Center: case listing, case creation, case details (tabs: activity/comments, details, attachments, calls, KB, related CRs), Novera AI chat, conversations, and the shared domain types/utils used across the entire portal.

### Pages

| Page | Route suffix | Key state |
|------|-------------|-----------|
| `SupportPage` | `support` | Stats + overview cards; nav to filtered lists |
| `AllCasesPage` | `support/cases` | **useSessionState**: search, filters (incl. `issueTypes: string[]`), sort, page, rowsPerPage — `popOnly: true`; URL: `createdByMe`, `severityId`, `statusFilter` |
| `CaseDetailsPage` | `support/cases/:caseId` | Active tab, focus mode; tab restore via session storage |
| `DescribeIssuePage` | `support/chat/describe-issue` | Local text + env products |
| `NoveraChatPage` | `support/chat`, `support/chat/:conversationId` | Messages, WebSocket, classifications |
| `CreateCasePage` | `support/chat/create-case`, `create-related-case`, `security-report/create` | Large local form state; related-case from location.state |
| `AllConversationsPage` | `support/conversations` | useSessionState; `statusFilter`, `createdByMe` |
| `ConversationDetailsPage` | `support/conversations/:conversationId` | Read-only transcript |

### `AllCasesPage` URL/query behavior details
- `?createdByMe=true` → "My Cases" title + createdByMe filter pre-applied
- `?statusFilter=active` → "Outstanding Cases" title; search panel hidden; builds active status IDs
- `?statusFilter=resolved` → closed + `getLast30DaysUtcRange()` applied to filter
- `?severityId={id}` → dashboard severity title/description; severity filter applied; severity filter hidden in panel; `isDashboardSeverityNavigation: true` → `buildDashboardCaseSearchFilters` uses `resolveCasesTableDefaultStatusIds`

### Component tree (case details)
```
CaseDetailsPage
└── CaseDetailsContent
    ├── CaseDetailsBackButton
    ├── CaseDetailsHeader           (case number, title, dates, severity/status chips)
    ├── CaseDetailsActionRow        (Close, Reopen, Open Related Case — PATCH case)
    ├── CaseDetailsTabs             (tab bar)
    └── CaseDetailsTabPanels
        ├── [0] CaseDetailsActivityPanel
        │       ├── CommentBubble × n (HTML, inline images via useResolvedInlineImageHtml)
        │       ├── ChatMessageCard × n
        │       ├── ActivityCommentInput (Lexical rich text editor)
        │       └── ImageFullscreenModal
        ├── [1] CaseDetailsDetailsPanel
        │       ├── CaseDetailsCard (description, type, deployment, product)
        │       └── AssignedEngineerDisplay
        ├── [2] CaseDetailsAttachmentsPanel
        │       ├── UploadAttachmentDropZone
        │       ├── AttachmentListItem × n
        │       └── modals: Upload, Edit, Delete
        ├── [3] CallsPanel
        │       ├── CallRequestList
        │       ├── RequestCallModal
        │       └── modals: approve, reject, delete
        ├── [4] CaseKnowledgeBaseRecommendations
        └── [5] RelatedChangeRequests (permission-gated)
```

### Component tree (case creation)
```
CreateCasePage
└── CaseCreationLayout
    ├── CaseCreationHeader
    ├── BasicInformationSection     (title, description using Lexical editor)
    ├── CaseDetailsSection          (issue type, severity, deployment, product)
    ├── ConversationSummary         (when from Novera chat)
    └── RelatedCaseSummary          (when from "Open Related Case")
```

### Component tree (Novera AI)
```
NoveraChatPage
├── ChatHeader                  (back, title)
├── EscalationBanner            (shown when AI suggests human support)
├── ChatMessageList
│   └── ChatMessageBubble × n
│       └── LoadingDotsBubble
├── RecommendationsCard         (KB suggestions)
└── ChatInput                   (TextField + send)
```

### Key utils
| File | Key exports |
|------|-------------|
| `support.ts` | Severity/status colors & icons, date formatters, case actions, HTML sanitization, `deriveFilterLabels`, `normalizeCaseSearchIssueIds`, `isS0Case`, `getLast30DaysUtcRange`, `isClosedLikeCaseStatus` |
| `listView.ts` | `hasListSearchOrFilters`, `countListSearchAndFilters`, `normalizeCaseSearchIssueIds` |
| `caseCreation.ts` | Classification payload, deployment/product matching |
| `chat.ts` | Stream token sanitization, typing display, message extraction |
| `casesCsvExport.ts` | CSV column formatting |

### Filter definitions (`ALL_CASES_FILTER_DEFINITIONS`)
Defined in `src/features/support/constants/supportConstants.ts`. Used by `ListFilters` and `CasesTable`:

| id | filterKey | metadataKey | multiSelect |
|----|-----------|-------------|-------------|
| `status` | `statusIds` | `caseStates` | `true` |
| `severity` | `severityIds` | `severities` | `true` |
| `category` | `issueTypes` | `issueTypes` | `true` |
| `deployment` | `deploymentIds` | `deploymentTypes` | `true` |

**Note:** `AllCasesFilterValues.issueTypes` is `string[]` (multi-select). API sends `issueIds: number[]`.
Backend support for `issueIds` array needs confirmation (was previously `issueId: number` singular).

### State management
| Location | Mechanism |
|----------|-----------|
| `AllCasesPage`, `AllConversationsPage` | `useSessionState` `popOnly: true` |
| `CaseDetailsContent` | `useState(activeTab)` + session storage tab restore |
| `NoveraChatPage` | Extensive local state + React Query + WebSocket |
| `CreateCasePage` | Local form state + `usePostCase` mutation |

### User flows
1. Support home → click stat/card → navigate to cases/conversations with pre-applied filters
2. Case list → search, multi-select filter (status, severity, category, deployment), sort, paginate, export CSV
3. From dashboard severity click → land with severity pre-applied + outstanding statuses + dashboard title
4. Case detail → switch tabs; post comment (Lexical editor, HTML); upload/manage attachments; schedule call; view KB
5. Change case state via action row (Close, Reopen, etc.) → PATCH case → invalidates queries
6. Open Related Case → CreateCasePage with `relatedCase` in state
7. Novera: Describe issue → chat with AI → AI escalates → Create Case (pre-filled from classification)
8. Resume previous chat → `/support/chat/:conversationId`

---

## 3. Engagements (`src/features/engagements/`)

### Purpose
Customer engagement cases (`CaseType.ENGAGEMENT`): stats overview, searchable/filterable list,
CSV export, detail via shared `CaseDetailsPage`.

### Pages

| Page | Route suffix | Key state |
|------|-------------|-----------|
| `EngagementsPage` | `engagements` | All state in `useEngagementsPageState` hook |

### Dashboard chart navigation
```
DashboardPage chart segment
  → navigate("/engagements", {
      state: { engagementTypeId: "1,2", engagementTypeLabel: "...", returnTo: dashPath }
    })
EngagementsPage:
  reads location.state.engagementTypeId → pre-filters by engagementTypeKeys
  reads location.state.returnTo → back button destination
```

### Stat card click flow
```
EngagementsStatCards: Active / Completed / On Hold
  → sets fixedStatusIds + activeStatKey in useEngagementsPageState
  → list filtered to that status
  → stat cards hidden; simplified title shown
  → Back clears stat filter (back to full list)
```

### Component tree
```
EngagementsPage
├── Back button (if stat-filtered or chart nav)
├── EngagementsStatCards (3 cards: active, completed, on hold)
└── EngagementsListSection
    ├── ListSearchBar
    ├── Status filters (multi-select checkboxes)
    ├── Engagement type filter (grouped API ids)
    ├── ListResultsBar (sort: updated on, created on, status)
    ├── ListItems (hideSeverity: true)
    └── ListPagination
```

### `useEngagementsPageState` (the entire page state)
Single hook managing:
- React Query: stats (`useGetProjectCasesStats`), cases (`useGetProjectCases` infinite)
- Filter state: `filters`, `fixedStatusIds` (stat card lock), `activeStatKey`
- Chart nav: applies `engagementTypeKeys` + outstanding `statusIds` from metadata
- Pagination: `page`, `rowsPerPage`, infinite query bridge
- Sort: `sortField`, `sortOrder`
- Export: `fetchProjectCaseSearchResults` for CSV
- Navigation: `buildEngagementDetailPath` → case detail

### Filters
Reuses `AllCasesFilterValues` with:
- `statusIds` (multi)
- `engagementTypeKey` (comma-separated group ids)
- `issueTypes`, `deploymentIds`, `searchQuery`
Does NOT use severity.

### Cross-feature dependencies
- `support` — `CaseType.ENGAGEMENT`, `CaseStatus`, list components, case detail, `CaseListCsvExportButton`
- `dashboard` — `useGetProjectCasesStats`, `normalizeEngagementLabel`
- `@api/useGetProjectCases` — infinite list
