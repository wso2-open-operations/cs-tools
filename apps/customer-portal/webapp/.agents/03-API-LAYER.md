# 03 — API Layer & Data Fetching

## Auth fetch pattern

### `useAuthApiClient` (`src/hooks/useAuthApiClient.ts`)

Returns `authFetch(input, options?)` — drop-in wrapper around native `fetch`:

```typescript
const authFetch = useAuthApiClient();
const response = await authFetch(`${baseUrl}/projects/${projectId}/cases/search`, {
  method: "POST",
  body: JSON.stringify(requestBody),
});
```

**Headers always injected:**
- `Authorization: Bearer {idToken}`
- `x-user-id-token: {idToken}` (duplicate, required by backend)
- `Accept: application/json` (if not set)
- `Content-Type: application/json` for POST/PUT/PATCH when body is present AND is NOT FormData, Blob, ArrayBuffer, URLSearchParams, ReadableStream, or ArrayBuffer view

**Throws:** `"Unable to retrieve ID token"` if `getIdToken()` returns nothing.

**Note:** `apiConstants.ts` defines `TOKEN_RETRY_DELAYS_MS` and `resolveIdTokenWithRetry` constants,
but the current hook implementation does a single `getIdToken()` call with no retry.

### Standard API hook guard
```typescript
const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
// All hooks use:
enabled: !!projectId && isSignedIn && !isAuthLoading
```

### Backend base URL
```typescript
const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;
if (!baseUrl) throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
```
Hooks read `window.config` directly. The module `src/config/apiConfig.ts` exports it too, but hooks
rarely import from there.

---

## React Query configuration

**File:** `src/AppWithConfig.tsx` (QueryClient setup)

| Setting | Value |
|---------|-------|
| `refetchOnWindowFocus` | `false` |
| `refetchOnReconnect` | `false` |
| `refetchOnMount` | `true` |
| Query retry | Custom: max 2 failures; only retries HTTP 502 or 503 |
| Retry delay | Exponential: `min(1000 * 2^n, 30000)` |

**Devtools:** `@tanstack/react-query-devtools` with `initialIsOpen={false}`.

### `staleTime` defaults
| Value | Used for |
|-------|---------|
| `0` (explicit) | Most feature data (cases, stats, SR, CR, deployments, vulnerabilities, settings) |
| `5 min` | `useInfiniteProjects`, `useGetProjectFilters`, `useGetProjectFeatures` |
| `10 min` | `useGetMetadata`, `useConversationRecommendationsSearch` |
| `Infinity` | `useGetConversationMessages` (stable transcript) |

---

## Query key constants (`src/constants/apiConstants.ts`)

`ApiQueryKeys` contains all string keys. Representative entries:

```typescript
PROJECT_CASES, CASES_STATS, CASE_DETAILS, CASE_COMMENTS, CASE_ATTACHMENTS,
CASE_CALL_REQUESTS, SUPPORT_STATS, CONVERSATION_STATS, CONVERSATION_SUMMARY,
CHAT_HISTORY, CONVERSATIONS_SEARCH, PROJECTS, PROJECT_DETAILS, PROJECT_FEATURES,
METADATA, DEPLOYMENTS, DEPLOYMENT_PRODUCTS, DEPLOYMENT_ATTACHMENTS,
CHANGE_REQUEST_STATS, CHANGE_REQUEST_DETAILS, VULNERABILITIES, REGISTRY_TOKENS_SEARCH,
PROJECT_CONTACTS, TIME_CARDS_STATS
```

`ApiMutationKeys`: `POST_COMMENT`, `POST_CHANGE_REQUEST_COMMENT` (latter defined but unused).

---

## Shared API hooks (`src/api/`)

| Hook | Method | Endpoint | Pattern | Key |
|------|--------|----------|---------|-----|
| `useInfiniteProjects` | POST | `/projects/search` | Infinite | `[PROJECTS, "infinite", search, pageSize]` |
| `useGetProjectDetails` | GET | `/projects/:id` | Query | `[PROJECT_DETAILS, id]` |
| `useGetProjectFeatures` | GET | `/projects/:id/features` | Query | `[PROJECT_FEATURES, id]` |
| `useGetProjectFilters` | GET | `/projects/:id/filters` | Query | `[PROJECT_CASES, "filters", id]` |
| `useGetMetadata` | GET | `/metadata` | Query | `[METADATA]` |
| `useGetProjectCases` | POST | `/projects/:id/cases/search` | **Infinite** | `[PROJECT_CASES, id, request, limit]` |
| `useGetProjectCasesPage` | POST | same | **Single page** | `[PROJECT_CASES, "page", id, request, offset, limit]` |
| `usePostProjectDeploymentsSearchInfinite` | POST | `/projects/:id/deployments/search` | Infinite | `[DEPLOYMENTS, id, "search", ...]` |
| `usePostProjectDeploymentsSearchAll` | POST | same (loops) | Query | `[DEPLOYMENTS, ..., "search-all", ...]` |
| `useGetAttachment` | GET | `/attachments/:id` | **Mutation** | download |
| `useGetAttachmentContent` | GET | `/attachments/:id/content` | **Mutation** | download |
| `useAttachmentPreview` | GET | same | Query per id | blob preview |

---

## Feature API hooks

### Dashboard (`src/features/dashboard/api/`)

| Hook | Method | Endpoint |
|------|--------|----------|
| `useGetProjectCasesStats` | GET | `/projects/:id/stats/cases?caseTypes=...&createdBy=me` |
| `useGetProjectChangeRequestsStats` | GET | `/projects/:id/stats/change-requests` |

### Support (`src/features/support/api/`)

**Queries:**
| Hook | Method | Endpoint |
|------|--------|----------|
| `useGetCaseDetails` | GET | `/cases/:id` |
| `useGetCaseCommentsInfinite` | POST | `/cases/:id/activities/search` (infinite) |
| `useGetCaseAttachments` | GET | `/cases/:id/attachments` (infinite) |
| `useGetCallRequests` | POST | `/cases/:id/call-requests/search` (infinite) |
| `useGetProjectSupportStats` | GET | `/projects/:id/stats/support` |
| `useSearchConversations` | POST | `/projects/:id/conversations/search` |
| `useGetConversationStats` | GET | `/projects/:id/stats/conversations` |
| `useGetConversationSummary` | GET | `/projects/:id/conversations/:cid/summary` |
| `useGetConversationMessages` | GET | `/conversations/:id/messages` (infinite, staleTime: Infinity) |
| `useConversationRecommendationsSearch` | POST | `/conversations/recommendations/search` |
| `useGetChatHistory` | GET | `/projects/:id/chat-history` |
| `useGetAIChatHistory` | POST | `/cases/:id/activities/search` (infinite) |
| `fetchProjectCaseSearchResults` | POST | cases search (imperative loop — for CSV export) |
| `useChatWebSocket` | WebSocket | `CUSTOMER_PORTAL_CHATBOT_WEBSOCKET_URL` |

**Mutations:**
| Hook | Method | Endpoint |
|------|--------|----------|
| `usePostConversations` | POST | `/projects/:id/conversations` |
| `usePostConversationMessages` | POST | `/projects/:id/conversations/:cid/messages` |
| `usePostComment` | POST | `/cases/:id/comments` → invalidates `CASE_COMMENTS` |
| `usePostCaseClassifications` | POST | `/cases/classify` |
| `usePostCallRequest` | POST | `/cases/:id/call-requests` |
| `usePostAttachments` | POST | `/cases/:id/attachments` |
| `usePatchCase` | PATCH | `/cases/:id` → invalidates CASE_DETAILS, CASES_STATS, PROJECT_CASES |
| `usePatchCaseAttachment` | PATCH | `/cases/:id/attachments/:aid` |
| `usePatchCallRequest` | PATCH | `/cases/:id/call-requests/:rid` |
| `useDeleteAttachment` | DELETE | `/attachments/:id` |

### Operations (`src/features/operations/api/`)

| Hook | Method | Endpoint |
|------|--------|----------|
| `useGetChangeRequests` | POST | `/projects/:id/change-requests/search` (paged query) |
| `useGetChangeRequestsInfinite` | POST | same (infinite) |
| `useGetChangeRequestDetails` | GET | `/change-requests/:id` |
| `useGetProjectChangeRequestStats` | GET | `/projects/:id/stats/change-requests` |
| `useSearchCatalogs` | POST | `/deployments/products/:deployedProductId/catalogs/search` |
| `useGetCatalogItemVariables` | GET | catalog item variables endpoint |
| `usePostCase` | POST | `/cases` (creates SR/case) |
| `usePatchChangeRequest` | PATCH | CR by id → invalidates CR_DETAILS, stats |
| `fetchChangeRequestSearchResults` | POST | CR search loop (imperative, for export) |

### Security (`src/features/security/api/`)

| Hook | Method | Endpoint |
|------|--------|----------|
| `useGetVulnerabilitiesMetaData` | GET | `/products/vulnerabilities/meta` |
| `usePostProductVulnerabilitiesSearch` | POST | `/products/vulnerabilities/search` |
| `useGetProductVulnerability` | GET | `/products/vulnerabilities/:id` |

### Updates (`src/features/updates/api/`)

| Hook | Method | Endpoint |
|------|--------|----------|
| `useGetProductUpdateLevels` | GET | `/updates/product-update-levels` |
| `usePostUpdateLevelsSearch` | POST | `/updates/levels/search` |

### Usage Metrics (`src/features/usage-metrics/api/`)

| Hook | Method | Endpoint |
|------|--------|----------|
| `useGetTimeCardsStats` | GET | `/projects/:id/stats/time-cards?startDate&endDate` |
| `useGetTimeTrackingDetails` | GET | `/projects/:id/timetracking` |
| `useSearchProjectTimeCards` | POST | `/projects/:id/time-cards/search` (infinite) |
| `useSearchProjectCaseTimeCards` | POST | `/projects/:id/cases/time-cards/search` (infinite) |
| `usePostDeploymentUsagesImport` | POST | `/deployment-usages` (multipart zip) |

### Settings (`src/features/settings/api/`)

| Hook | Method | Endpoint |
|------|--------|----------|
| `useGetUserDetails` | GET | `/users/me` |
| `usePatchUserMe` | PATCH | `/users/me` → setQueryData + invalidateQueries |
| `useGetProjectContacts` | GET | `/projects/:id/contacts` |
| `usePostProjectContact` | POST | `/projects/:id/contacts` |
| `usePatchProjectContact` | PATCH | `/projects/:id/contacts/:email` |
| `useDeleteProjectContact` | DELETE | same |
| `useValidateProjectContact` | POST | `/projects/:id/contacts/validate` |
| `useGetIntegrationUsers` | GET | `/projects/:id/integration-users` |
| `usePatchProject` | PATCH | `/projects/:id` (Novera/KB settings) |
| `useSearchRegistryTokens` | POST | `/projects/:id/registry-tokens/search` |
| `useCreateRegistryToken` | POST | `/projects/:id/registry-tokens` |
| `useRegenerateRegistryToken` | POST | `/registry-tokens/:id/regenerate` |
| `useDeleteRegistryToken` | DELETE | `/registry-tokens/:id` |

### Project Details (`src/features/project-details/api/` — ~24 hooks)

Covers: deployments CRUD, products CRUD, instances (search/usages/metrics), attachments CRUD,
license download, project stats. Representative endpoints:
- `POST /projects/:id/deployments`, `PATCH .../deployments/:id`
- `POST /deployments/:id/products`, `PATCH .../products/:id`
- `POST /deployments/:id/attachments`, `GET` infinite
- `POST /projects/:id/instances/search`, `usages/search`, `metrics/search`
- `GET /projects/:id/stats`, `/stats/usage`
- `POST /products/:id/versions/search`
- `GET /projects/:id/deployments/:id/license` (mutation download)

### Features without `api/` folders

**Engagements:** No dedicated api/ dir. `useEngagementsPageState` composes:
- `useGetProjectCasesStats` (dashboard api, `caseTypes: [ENGAGEMENT]`)
- `useGetProjectCases` (shared, infinite)
- `fetchProjectCaseSearchResults` (support api, for CSV)

**Announcements:** No api/ dir. Uses `useGetProjectCasesPage` + `useGetProjectFilters` + `useGetCaseDetails`.

---

## Error handling

### `ApiError` class (`src/utils/ApiError.ts`)

```typescript
class ApiError extends Error {
  status: number;
  statusText: string;
  message?: string;
}

isUnauthorizedError(e)  // 401
isForbiddenError(e)     // 403
isNotFoundError(e)      // 404
isBadRequestError(e)    // 400
getApiErrorMessage(e)   // human-readable string
parseApiResponseMessage(text)  // parse JSON { message } from response body
```

### Error propagation patterns
1. **`ApiErrorState` component** — maps `ApiError.status` to `Error400/401/403/404/500Page`; used in detail panels, deployments, etc.
2. **Generic `Error` throw** — most hooks: `throw new Error(\`Error fetching ...: ${response.statusText}\`)`
3. **Inline error state** — `isError` on query → `ErrorIndicator` in list components
4. **Mutation errors** — surfaced via `mutation.error.message` in forms + `useErrorBanner` toast

**Global retry rules** (QueryClient): only 502/503 are retried; 401/403/404/400 do NOT auto-retry.

---

## Pagination patterns

### Infinite query (load more / virtual page)
Used for: project list, case search, change requests (calendar), comments, attachments, calls,
conversation messages, time cards.

```typescript
// getNextPageParam pattern
getNextPageParam: (lastPage) => {
  const nextOffset = lastPage.offset + lastPage.limit;
  return nextOffset < lastPage.totalRecords ? nextOffset : undefined;
}
```

**Engagements bridge pattern:** uses infinite query as page cache — when user jumps to page N,
a `useEffect` calls `fetchNextPage()` until enough pages loaded, then `getEngagementsCurrentPageCases`
slices the correct page.

### Paged query (table pagination)
Used for: dashboard cases, announcements, vulnerabilities, update levels, registry tokens.
```typescript
// offset + limit as query params; re-fetches on page change
useQuery({ queryKey: [..., offset, limit], queryFn: ... })
```

### Imperative fetch-all (for exports)
```typescript
// fetchProjectCaseSearchResults loops POST until all pages fetched
// fetchChangeRequestSearchResults same for CRs
```

---

## Cache invalidation

| After mutation | Invalidates |
|----------------|-------------|
| `usePostComment` | `CASE_COMMENTS` |
| `usePatchCase` | `CASE_DETAILS`, `CASES_STATS`, predicate on `PROJECT_CASES` |
| `usePostAttachments`, `usePatchCaseAttachment` | `CASE_ATTACHMENTS` |
| `useDeleteAttachment` | `CASE_ATTACHMENTS`, `DEPLOYMENT_ATTACHMENTS` |
| `usePostCallRequest`, `usePatchCallRequest` | `CASE_CALL_REQUESTS` |
| `usePatchUserMe` | `setQueryData` on `userDetails` + broad `invalidateQueries` |
| `usePatchProject` | Project features queries |
| `usePatchChangeRequest` | `CHANGE_REQUEST_DETAILS`, CR stats |
| `usePostCase` (SR/case create) | `src/features/operations/utils/caseRefresh.ts` — predicate invalidation + refetch `CASES_STATS`, `PROJECT_CASES` |
| Settings mutations | `PROJECT_CONTACTS`, `REGISTRY_TOKENS_SEARCH` |
| Deployment/product mutations | `DEPLOYMENTS`, `DEPLOYMENT_PRODUCTS`, `DEPLOYMENT_ATTACHMENTS` |

---

## Key types for API contracts

### Case search (`src/features/support/types/cases.ts`)

```typescript
type CaseSearchFilters = {
  issueId?: number;        // legacy single-select (kept for backwards compat)
  issueIds?: number[];     // multi-select (new — confirm backend supports)
  deploymentIds?: string[];
  severityIds?: number[];
  statusId?: number;
  statusIds?: number[];
  searchQuery?: string;
  caseTypes?: string[];
  createdByMe?: boolean;
  closedStartDate?: string;
  closedEndDate?: string;
  engagementTypeKeys?: number[];
};

type CaseSearchRequest = SearchRequestBase & {
  filters?: CaseSearchFilters;
};

type CaseSearchResponse = PaginationResponse & {
  cases: CaseListItem[];
  projects?: IdLabelRef[];
};
```

### Common base types (`src/types/common.ts`)

```typescript
type PaginationRequest = { offset?: number; limit?: number };
type PaginationResponse = { offset: number; limit: number; totalRecords: number };
type SearchRequestBase = { pagination?: PaginationRequest; sortBy?: SortBy };
type SortOrder = "asc" | "desc";
type IdLabelRef = { id: string; label: string; ... };
type MetadataItem = { id: string; label: string };
```

---

## Implementation checklist for new API hooks

1. Import `useAuthApiClient` and `useAsgardeo`
2. Gate with `isSignedIn && !isAuthLoading` (and `!!projectId` if project-scoped)
3. Read base URL from `window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL`
4. Use `ApiQueryKeys` for cache key (include filters/pagination in key array)
5. Set `staleTime: 0` for mutable data; 5–10 min for stable metadata
6. Throw `ApiError` or generic `Error` from `queryFn` on non-ok responses
7. For mutations: call `queryClient.invalidateQueries` in `onSuccess`
8. Choose the right pagination pattern:
   - **Infinite** → `useInfiniteQuery` (scroll/load-more)
   - **Paged** → `useQuery` with offset/limit as key params (table pagination)
   - **Export** → imperative async loop
