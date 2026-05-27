import { Box, Skeleton, Stack } from "@wso2/oxygen-ui";

import { useFilters } from "@context/filters";

import { FilterControls, type FilterControlsVariant } from "@features/items/components";

import { ErrorBoundary } from "@shared/components/core";

import { CASE_TYPES, SEARCH_PLACEHOLDER_CONFIG } from "@shared/constants";
import type { CaseType } from "@shared/types";

export function Filters({ type, variant = "full" }: { type: CaseType; variant?: FilterControlsVariant }) {
  const { data, isLoading } = useFilters();

  if (isLoading || !data) return <FiltersSkeleton variant={variant} />;

  const tabs = (() => {
    switch (type) {
      case CASE_TYPES.CHAT:
        return data.conversationStates;
      case CASE_TYPES.CHANGE_REQUEST:
        return data.changeRequestStates;
      default:
        return data.caseStates;
    }
  })();

  return (
    <ErrorBoundary fallback={<FiltersSkeleton />}>
      <FilterControls
        variant={variant}
        placeholder={SEARCH_PLACEHOLDER_CONFIG[type]}
        tabs={tabs.map((filter) => ({ label: filter.label, value: filter.id }))}
      />
    </ErrorBoundary>
  );
}

export function FiltersSkeleton({ variant = "full" }: { variant?: FilterControlsVariant }) {
  const showSearch = variant !== "tabs-only"; /** Hide the search bar */
  const showTabs = variant !== "search-only"; /** Hide the filter tabs section */

  return (
    <Stack gap={2} p={1} width="100%">
      {showSearch && <Skeleton variant="rectangular" height={36} sx={{ mt: 1, borderRadius: 1 }} />}
      {showTabs && (
        <Box sx={{ display: "flex", gap: 2, overflow: "hidden" }}>
          {Array.from({ length: 6 }).map((_, index) => (
            <Skeleton
              key={index}
              variant="rectangular"
              width={80}
              height={40}
              sx={{ borderRadius: 999, flexShrink: 0 }}
            />
          ))}
        </Box>
      )}
    </Stack>
  );
}
