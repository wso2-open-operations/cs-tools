import { SEARCH_PLACEHOLDER_CONFIG } from "@root/src/config/constants";
import { FilterSlotBuilder, FilterSlotBuilderSkeleton, type ItemCardProps } from ".";
import { useFilters } from "@context/filters";

export function FilterSlotContent({ type }: { type: ItemCardProps["type"] }) {
  const { data, isLoading } = useFilters();

  if (isLoading || !data) return <FilterSlotBuilderSkeleton />;

  const tabs = (() => {
    switch (type) {
      case "chat":
        return data.conversationStates;
      case "change":
        return data.changeRequestStates;
      default:
        return data.caseStates;
    }
  })().map((filter) => ({ label: filter.label, value: filter.id }));

  return <FilterSlotBuilder searchPlaceholder={SEARCH_PLACEHOLDER_CONFIG[type]} tabs={tabs} />;
}
