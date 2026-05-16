import { useProject } from "@context/project";
import { useInfiniteQuery } from "@tanstack/react-query";
import { cases } from "@features/cases/api/cases.queries";
import { chats } from "@features/chats/api/chats.queries";
import { serviceRequests } from "@features/service-requests/api/service-requests.queries";
import { changeRequests } from "@features/changes/api/changes.queries";
import { securityReportAnalysis } from "@features/security-report-analysis/api/sra.queries";
import { engagements } from "@features/engagements/api/engagements.queries";
import { announcements } from "@features/announcements/api/announcements.queries";
import { useCaseFiltersFromParams, useChangeRequestFiltersFromParams, useChatFiltersFromParams, useFilters } from "@features/items/hooks";
import { CASE_TYPES } from "@shared/constants";

export function useCaseItems(enabled: boolean = true) {
    const { projectId } = useProject(); 
    const filters = useCaseFiltersFromParams(); 
    const unfiltered = useInfiniteQuery({ ...cases.paginated(projectId!, { filters: {} }), enabled });
    const filtered = useInfiniteQuery({ ...cases.paginated(projectId!, { filters }), enabled });

    const total = unfiltered.data?.pages[0].pagination.totalRecords;
    const count = filtered.data?.pages[0].pagination.totalRecords;

    return { query: filtered, total, count };
}

export function useChatItems(enabled = true) {
    const { projectId } = useProject(); 
    const filters = useChatFiltersFromParams(); 
    const unfiltered = useInfiniteQuery({...chats.paginated(projectId!, { filters: {} }), enabled });
    const filtered = useInfiniteQuery({ ...chats.paginated(projectId!, { filters }), enabled });

    const total = unfiltered.data?.pages[0].pagination.totalRecords;
    const count = filtered.data?.pages[0].pagination.totalRecords;

    return { query: filtered, total, count };
}

export function useServiceRequestItems(enabled = true) {
    const { projectId } = useProject(); 
    const filters = useCaseFiltersFromParams(); 
    const unfiltered = useInfiniteQuery({ ...serviceRequests.paginated(projectId!, { filters: {} }), enabled });
    const filtered = useInfiniteQuery({ ...serviceRequests.paginated(projectId!, { filters }), enabled });

    const total = unfiltered.data?.pages[0].pagination.totalRecords;
    const count = filtered.data?.pages[0].pagination.totalRecords;

    return { query: filtered, total, count };
}

export function useChangeRequestItems(enabled = true) {
    const { projectId } = useProject(); 
    const filters = useChangeRequestFiltersFromParams(); 
    const unfiltered = useInfiniteQuery({ ...changeRequests.paginated(projectId!, { filters: {} }), enabled });
    const filtered = useInfiniteQuery({ ...changeRequests.paginated(projectId!, { filters }), enabled });

    const total = unfiltered.data?.pages[0].pagination.totalRecords;
    const count = filtered.data?.pages[0].pagination.totalRecords;

    return { query: filtered, total, count };
}

export function useSecurityReportAnalysisItems(enabled = true) {
    const { projectId } = useProject(); 
    const filters = useCaseFiltersFromParams(); 
    const unfiltered = useInfiniteQuery({ ...securityReportAnalysis.paginated(projectId!, { filters: {} }), enabled });
    const filtered = useInfiniteQuery({ ...securityReportAnalysis.paginated(projectId!, { filters }), enabled });

    const total = unfiltered.data?.pages[0].pagination.totalRecords;
    const count = filtered.data?.pages[0].pagination.totalRecords;

    return { query: filtered, total, count };
}

export function useEngagementItems(enabled = true) {
    const { projectId } = useProject(); 
    const filters = useCaseFiltersFromParams(); 
    const unfiltered = useInfiniteQuery({ ...engagements.paginated(projectId!, { filters: {} }), enabled });
    const filtered = useInfiniteQuery({ ...engagements.paginated(projectId!, { filters }), enabled });

    const total = unfiltered.data?.pages[0].pagination.totalRecords;
    const count = filtered.data?.pages[0].pagination.totalRecords;

    return { query: filtered, total, count };
}


export function useAnnouncementItems(enabled = true) {
    const { projectId } = useProject(); 
    const filters = useCaseFiltersFromParams(); 
    const unfiltered = useInfiniteQuery({ ...announcements.paginated(projectId!, { filters: {} }), enabled });
    const filtered = useInfiniteQuery({ ...announcements.paginated(projectId!, { filters }), enabled });

    const total = unfiltered.data?.pages[0].pagination.totalRecords;
    const count = filtered.data?.pages[0].pagination.totalRecords;

    return { query: filtered, total, count };
}

export function useItems() {
    const { filters } = useFilters();

    const caseItems = useCaseItems(filters.types.includes(CASE_TYPES.DEFAULT));
    const chatItems = useChatItems(filters.types.includes(CASE_TYPES.CHAT));
    const serviceRequestItems = useServiceRequestItems(filters.types.includes(CASE_TYPES.SERVICE_REQUEST));
    const changeRequestItems = useChangeRequestItems(filters.types.includes(CASE_TYPES.CHANGE_REQUEST));
    const securityReportAnalysisItems = useSecurityReportAnalysisItems(filters.types.includes(CASE_TYPES.SECURITY_REPORT_ANALYSIS));
    const engagementItems = useEngagementItems(filters.types.includes(CASE_TYPES.ENGAGEMENT));
    const announcementItems = useAnnouncementItems(filters.types.includes(CASE_TYPES.ANNOUNCEMENT));

    const all = [caseItems, chatItems, serviceRequestItems, changeRequestItems, securityReportAnalysisItems, engagementItems, announcementItems];
    const active = all.filter((_, i) => filters.types.includes(Object.values(CASE_TYPES)[i]));

    const total = active.reduce((sum, q) => sum + (q.count ?? 0), 0);
    const isResolving = active.some((q) => q.count === undefined);

    return { total, isResolving };
}