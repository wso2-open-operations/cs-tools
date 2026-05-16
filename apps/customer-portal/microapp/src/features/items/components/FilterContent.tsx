import { Stack } from "@wso2/oxygen-ui";
import { useFilters, useItems } from "@features/items/hooks";
import { CASE_TYPES } from "@shared/constants";
import { AnnouncementItemsList, CaseItemsList, ChangeRequestItemsList, ChatItemsList, EngagementItemsList, ItemCardSkeleton, SecurityReportAnalysisItemsList, ServiceRequestItemsList } from "@features/items/components";
import { EmptyState } from "@shared/components/common";

export function FilterContent() {
    const { filters } = useFilters();     
    const { total, isResolving } = useItems();

    if (!isResolving && total === 0) return <EmptyState />

    return (
        <Stack gap={2}>
          {filters.types.map((type) => {
            switch (type) {
              case CASE_TYPES.DEFAULT:
                return <CaseItemsList key={type} />;
              case CASE_TYPES.CHAT:
                return <ChatItemsList key={type} />;
              case CASE_TYPES.SERVICE_REQUEST:
                return <ServiceRequestItemsList key={type} />;
              case CASE_TYPES.CHANGE_REQUEST:
                return <ChangeRequestItemsList key={type} />;
              case CASE_TYPES.SECURITY_REPORT_ANALYSIS:
                return <SecurityReportAnalysisItemsList key={type} />;
              case CASE_TYPES.ENGAGEMENT:
                return <EngagementItemsList key={type} />;
              case CASE_TYPES.ANNOUNCEMENT:
                return <AnnouncementItemsList key={type} />;
              default:
                return null;
            }
          })}
        </Stack>
    )
}

export function FilterContentSkeleton() {
  return Array.from({ length: 5 }).map((_, index) => <ItemCardSkeleton key={index} />);
}