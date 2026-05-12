import { SEARCH_PLACEHOLDER_CONFIG } from "@root/src/config/constants";
import { FilterSlotBuilder, FilterSlotBuilderSkeleton, type ItemCardProps } from ".";
import { useFilters } from "@context/filters";
import { getAllowedFilters } from "@root/src/utils/filters";
import type { ModeType } from "@root/src/pages/AllItemsPage";

export function FilterSlotContent({
  type,
  state,
  showSearch = true,
  showTabs = true,
}: {
  type: ItemCardProps["type"];
  state?: unknown;

  /** Hide the search bar */
  showSearch?: boolean;
  /** Hide the filter tabs section */
  showTabs?: boolean;
}) {
  const { data, isLoading } = useFilters();

  if (isLoading || !data) return <FilterSlotBuilderSkeleton />;

  const mode: ModeType | undefined = (state as { mode: ModeType })?.mode;

  const tabs = (() => {
    switch (type) {
      case "chat":
        return data.conversationStates;
      case "change":
        return data.changeRequestStates;
      default:
        return data.caseStates;
    }
  })();

  return (
    <FilterSlotBuilder
      searchPlaceholder={SEARCH_PLACEHOLDER_CONFIG[type]}
      tabs={getAllowedFilters(tabs, mode).map((filter) => ({ label: filter.label, value: filter.id }))}
      state={state}
      showSearch={showSearch}
      showTabs={showTabs}
    />
  );
}
