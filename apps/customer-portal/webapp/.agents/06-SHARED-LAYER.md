# 06 — Shared Layer: Providers, Components, Hooks, Context, Types, Utils

---

## 1. Provider composition

### Outer providers (`src/AppWithConfig.tsx`)

| Order | Provider | Purpose |
|-------|----------|---------|
| 1 | `AsgardeoProvider` | OIDC auth; `periodicTokenRefresh`; scopes: `openid email groups profile` |
| 2 | `BrowserRouter` | Client-side routing (no `basename`) |
| 3 | `LoggerProvider` | Logger context from `loggerConfig` (reads `CUSTOMER_PORTAL_LOG_LEVEL`) |
| 4 | `OxygenUIThemeProvider` | Acrylic Orange theme (or config override) + `GlobalStyles` |
| 5 | `QueryClientProvider` | TanStack Query with custom retry (502/503 only, max 2) |
| 6 | `MobileAppGate` | Block phones/tablets when enabled → `MobileAppPromptPage` |
| 7 | `App` | Route tree + inner contexts |
| 8 | `ReactQueryDevtools` | Dev-only |

### Inner contexts (`src/App.tsx`)

| Provider | Hook | State |
|----------|------|-------|
| `LoaderProvider` | `useLoader()` | `isVisible`, `showLoader()`, `hideLoader()` — ref-counted, 500ms hide delay |
| `ErrorBannerProvider` | `useErrorBanner()` | `showError(message)` — auto-dismisses after `ERROR_BANNER_TIMEOUT_MS` |
| `SuccessBannerProvider` | `useSuccessBanner()` | `showSuccess(message)` — consumes pending session storage message on mount |
| `ErrorPageProvider` | `useErrorPageContext()` | `isErrorPageDisplayed`, `isProjectSuspended` — hides sidebar, adapts layout |

### Layout-level provider
`IdleTimeoutProvider` — wraps content inside `AppLayout` only (not at root):
- 15-minute idle timeout (`IDLE_TIMEOUT_MS`)
- 4s prompt before timeout (`IDLE_PROMPT_BEFORE_MS`)
- Shows `SessionWarningDialog` (Continue / Logout)
- On logout: `clearUserPreferredTimeZone()` + `signOut()`

### Feature flags (no provider)
| Source | How consumed |
|--------|-------------|
| `window.config` | Mock toggles, mobile prompt, banner config |
| `GET /metadata` → `PortalFeatureFlags` | `featureFlags.usageMetricsEnabled` → hides sidebar item |
| `GET /projects/:id/features` → `ProjectFeatures` | `getProjectPermissions(features)` → gates pages/components |

---

## 2. Shared components (`src/components/`)

### `list-view/` — list page primitives

| Component | Props interface | Purpose |
|-----------|-----------------|---------|
| `ListPageHeader` | `title`, `description`, `backLabel?`, `onBack?`, `actions?` | Page title row + optional back + action slots |
| `ListSearchPanel` | `searchTerm`, `filters`, `filterMetadata`, visibility flags (`hideSeverityFilter` etc.), deployment pagination | Composes `ListSearchBar` + `ListFilters` |
| `ListSearchBar` | `searchTerm`, `onSearchChange`, `isFiltersOpen`, `onFiltersToggle`, `activeFiltersCount`, `onClearFilters`, `filtersContent`, `actionsBeforeClearFilters?` | Search field + collapsible filter panel |
| `ListFilters` | `filters: AllCasesFilterValues`, `filterMetadata`, `deployments?`, `onFilterChange` | Case-specific filter grid (driven by `ALL_CASES_FILTER_DEFINITIONS`) |
| `ListFiltersPanel` | `filterDefinitions`, `filters`, `resolveOptions`, `onFilterChange` | Generic filter grid for non-case lists |
| `ListResultsBar` | `totalRecords`, `showing`, sort field/order controls, `rightContent?` | "Showing X of Y" + sort dropdowns |
| `ListItems` | `cases`, `isLoading`, `isError?`, `onCaseClick?`, `hideSeverity?`, `showEngagementType?` | Card list with loading/error/empty states |
| `ListCard` | `caseItem`, `onClick?`, `hideSeverity?` | Single case card |
| `ListPagination` | `totalRecords`, `page`, `rowsPerPage`, handlers | MUI `TablePagination` (hidden when 1 page) |
| `ListStatGrid` | `configs`, `stats`, loading/error, `onStatClick?` | Stat cards grid |
| `ListSkeleton` | `count?` | Loading placeholder cards |
| `CaseCardDescriptionClamp` | `html`, `maxLines?` | Clamped HTML description on cards |

### `header/`

| Component | Notes |
|-----------|-------|
| `Header` | Composes Brand + Actions + optional ProjectSwitcher + SearchBar; stacked layout on narrow viewports |
| `Brand` | Sidebar toggle + logo → `navigate("/", { state: { fromHeader: true } })` |
| `Actions` | GetHelp dropdown, `ColorSchemeToggle`, `UserProfile` |
| `SearchBar` | Global case/CR search popover with `SearchCaseCard`, `SearchChangeRequestCard` |
| `ProjectSwitcher` | Infinite projects `<Select>` |
| `UserProfile` | Menu: profile modal, logout |
| `UserProfileModal` | Full profile editor (name, phone, avatar, timezone, language) |
| `GetHelpDropdown` | Support links |

### `side-nav-bar/`

| Component | Notes |
|-----------|-------|
| `SideBar` | Filters `APP_SHELL_NAV_ITEMS` by permissions + `usageMetricsEnabled`; collapsed/expanded state |
| `SubscriptionWidget` | Subscription summary panel |

### `tab-bar/`
`TabBar` — `tabs: TabOption[]`, `activeTab`, `onTabChange`, optional `compact`, `keepButtonWidth`

### `select-menu-load-more-row/`
`SelectMenuLoadMoreRow` — spinner sentinel row for paginated `<Select>` components;
sentinel value `SELECT_MENU_LOAD_MORE_ROW_VALUE` is excluded from selection handler.

### `error/`
`Error401Page`, `Error403Page`, `Error404Page`, `Error400Page`, `Error500Page` — each accepts optional `message?`
`ApiErrorState` — maps `ApiError.status` to appropriate error page; used in detail/list panels

### `empty-state/`
`EmptyState`, `EmptyIcon`, `SearchNoResultsIcon` — empty state UI

### `filter-panel/`
`ActiveFilters` — chips + clear all for active filter display
`FilterPopover` — popover wrapper

### `rich-text-editor/`
Lexical-based: `Editor`, `ToolBar` (variants: `"full"` | `"describeIssue"`), `ImagesPlugin`, `ImageNode`
Used in case creation (description field) and activity tab comment input.

### `AuthenticatedImage`
Fetches attachment preview via `useAttachmentPreview` (authenticated GET); renders as `<img>`.

### `SessionWarningDialog`
`open`, `onContinue`, `onLogout` — idle timeout prompt

### Banners
- `ErrorBanner`, `SuccessBanner` — driven by contexts
- `GlobalNotificationBanner` — maintenance banner from `notificationBannerConfig`
- `HtmlAnnouncementBanner` — dismissible with localStorage
- `TopBanners` — multiple config-driven HTML banners from `CUSTOMER_PORTAL_TOP_BANNERS`

### Access control pages
`PortalAccessRequiredPage`, `AccountSuspendedPage`, `ProjectSuspendedNoticePage`, `MobileAppPromptPage`

---

## 3. Global hooks (`src/hooks/`)

### `useAuthApiClient`
Returns `authFetch`. See `03-API-LAYER.md` for full contract.

### `useSessionState<T>(key, defaultValue, validate?, options?)`
```typescript
const [value, setValue] = useSessionState<AllCasesFilterValues>(
  `${projectId}-cases-filters`,
  {},
  undefined,
  { popOnly: true }
);
```
- Persists to `sessionStorage` as JSON
- `popOnly: true` — only restores on browser POP (`navigation.type === "POP"`) or `location.state.fromBack === true`
- Values are ALWAYS written on change; only reading is conditional
- Optional `validate` type guard to reject stale shapes

### `useModifierAwareNavigate`
```typescript
const navigate = useModifierAwareNavigate();
navigate("/projects/123/support/cases/456");  // Ctrl/Cmd+click → new tab
```
Tracks Ctrl/Meta via window key listeners. Use everywhere for card/row clicks.

### `useLogger`
Returns `ILogger` from `LoggerContext`. Throws if called outside `LoggerProvider`.
Methods: `debug(msg, ...args)`, `info`, `warn`, `error`.

### `useDebouncedValue<T>(value, delayMs)`
Standard debounce hook. Used for search inputs in project hub, product vulnerability table, etc.

### `useResponsiveLayout`
- `useIsMidSizeTouchViewport()` — sm–md + coarse pointer → overlay sidebar behavior
- `useIsStackedHeaderLayout()` — below `lg` breakpoint → stacked header

### `useDarkMode` (`src/utils/useDarkMode.ts`)
Watches `<html data-color-scheme="dark">` via `MutationObserver`. Works with Oxygen `ColorSchemeToggle`.

---

## 4. Contexts (`src/context/`)

| Module | Value | Provider | Hook |
|--------|-------|----------|------|
| `error-banner/` | `{ showError(message) }` | `ErrorBannerProvider` | `useErrorBanner` |
| `success-banner/` | `{ showSuccess(message) }` | `SuccessBannerProvider` | `useSuccessBanner` |
| `linear-loader/` | `{ isVisible, showLoader, hideLoader }` | `LoaderProvider` | `useLoader` |
| `error-page/` | `{ isErrorPageDisplayed, setIsErrorPageDisplayed, isProjectSuspended, setIsProjectSuspended }` | `ErrorPageProvider` | `useErrorPageContext` |
| `logger/` | `ILogger \| null` | `LoggerProvider` | `useLogger` |

**`useLoader` pattern:**
```typescript
const { showLoader, hideLoader } = useLoader();
// ref-counted: showLoader increments, hideLoader decrements
// hides after 500ms delay when count reaches 0
```

---

## 5. Shared types (`src/types/`)

### `common.ts`
```typescript
type IdLabelRef = {
  id: string; label: string;
  count?: number; number?: string; internalId?: string;
  name?: string; abbreviation?: string;
  createdOn?: string; updatedOn?: string;
};
type MetadataItem = { id: string; label: string };
type AuditMetadata = { createdOn: string; updatedOn: string; createdBy?: string; updatedBy?: string };
type PaginationRequest = { offset?: number; limit?: number };
type PaginationResponse = { offset: number; limit: number; totalRecords: number };
type SortOrder = "asc" | "desc";
type SortBy = { field: string; order: SortOrder };
type SearchRequestBase = { pagination?: PaginationRequest; sortBy?: SortBy };
type SharedEnvContext = { envProducts: ...; region?: string; tier?: string };
```

### `permission.ts`
`ProjectClosureState`, `ProjectType`, `ProjectPermissions` (hasDeployments, hasSR, hasCR, hasEngagements, etc.),
`GetProjectPermissionsOptions`, `ProjectOperationsStatsResult`.

### `mobileDevice.ts`
`MobileOs`, `DeviceType`, `MobileDeviceInfo`, `DetectMobileDeviceOptions`.

---

## 6. Utils (`src/utils/`)

| Module | Key exports |
|--------|-------------|
| `common.ts` | `paginatedSelectMenuListProps`, `stripLightModeInlineStyles`, `DESCRIPTION_PURIFY_CONFIG` |
| `ApiError.ts` | `ApiError`, error predicates, `getApiErrorMessage`, `parseApiResponseMessage` |
| `permission.ts` | `getProjectPermissions(features)`, `shouldExcludeS0`, severity policy, `getProjectSeverityPolicy` |
| `dateTime.ts` | `normalizeBackendTimestamp`, `formatBackendTimestampForDisplay`, `resolveDisplayTimeZone`, `clearUserPreferredTimeZone` |
| `deviceDetection.ts` | `detectMobileDevice`, `shouldPromptForMobileApp` |
| `csv.ts` | `escapeCsvCell`, `buildCsvContent`, download trigger |
| `pdf.ts` | `downloadPdfFile` (jsPDF + autotable) |
| `useDarkMode.ts` | `useDarkMode()` hook |

---

## 7. Constants (`src/constants/`)

### `apiConstants.ts`
- `ApiQueryKeys` — ~50 React Query key strings
- `ApiMutationKeys` — `POST_COMMENT`, `POST_CHANGE_REQUEST_COMMENT`
- WebSocket: `WS_CHOREO_OAUTH2_TOKEN`, `WS_CUSTOMER_PORTAL`
- Auth retry: `TOKEN_RETRY_DELAYS_MS`, `ASGARDEO_UNAUTHENTICATED_CODE`, `AUTH_NOT_READY_ERROR_MESSAGE`

### `common.ts`
`PAGINATED_SELECT_MENU_MAX_HEIGHT_PX`, `EMPTY_DROPDOWN_PLACEHOLDER`, `NULL_PLACEHOLDER`,
banner timeouts, `HEADER_HEIGHT_PX`, `FOOTER_HEIGHT_PX`, `SIDEBAR_DRAWER_WIDTH_PX`

### `authConstants.ts`
`IDLE_TIMEOUT_MS` (15 min), `IDLE_PROMPT_BEFORE_MS` (4s), `IDLE_THROTTLE_MS` (500ms)

---

## 8. Config modules (`src/config/`)

| Module | Reads | Throws if missing |
|--------|-------|-------------------|
| `portalConfig.ts` | `CustomerPortalWindowConfig` type definition on `Window` | No |
| `authConfig.ts` | Auth URLs + clientId | Yes (required) |
| `apiConfig.ts` | `BACKEND_BASE_URL` | Yes |
| `themeConfig.ts` | `CUSTOMER_PORTAL_THEME` | No (defaults to `acrylicOrange`) |
| `loggerConfig.ts` | `CUSTOMER_PORTAL_LOG_LEVEL` | No (defaults to `"ERROR"`) |
| `mobileAppConfig.ts` | Mobile prompt + store URLs | No |
| `notificationBannerConfig.ts` | Maintenance banner fields | No |
| `topBannersConfig.ts` | `CUSTOMER_PORTAL_TOP_BANNERS[]` | No |
| `announcementBannerConfig.ts` | Announcement HTML + storage key | No |

---

## 9. Auth flow (Asgardeo)

### Sign-in
```
1. Unauthenticated user hits protected route
2. AuthGuard → ProtectedRoute → store post_login_redirect in sessionStorage
3. Asgardeo redirects to IdP sign-in page
4. OAuth callback URL (SIGN_IN_REDIRECT_URL) → AppLayout shows "Authenticating..." UI
5. AuthGuard.useEffect: restore post_login_redirect or redirect to last project
```

### Sign-out
```
UserProfile or IdleTimeoutProvider:
  clearUserPreferredTimeZone()
  dispatch("app:signing-out")
  signOut() (Asgardeo)
```

### Token injection (every API call)
```
useAuthApiClient → getIdToken() → authFetch headers:
  Authorization: Bearer {token}
  x-user-id-token: {token}
```

### `useGetUserDetails`
Has its own retry logic using `AUTH_NOT_READY_ERROR_MESSAGE` for auth-not-ready errors.
Returns user timezone → stored in module-level cache → used by `resolveDisplayTimeZone`.
