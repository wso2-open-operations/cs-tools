import { EmptyState } from "@shared/components/common";
import { useFilters } from "@src/features/items/hooks";
import { Typography } from "@wso2/oxygen-ui";

export function useInfiniteListTail(count: number) {
    const { filters } = useFilters(); 

    if (filters.types.length > 1) return undefined;
    if (count === 0) return <EmptyState />;
    return <Typography variant="subtitle2" textAlign="center">You're all caught up!</Typography>;
}