// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

import {
  Accordion,
  AccordionDetails,
  AccordionSummary,
  Box,
  Button,
  Chip,
  Divider,
  Skeleton,
  Stack,
  Typography,
} from "@wso2/oxygen-ui";
import { ArrowRight, ChevronDown } from "@wso2/oxygen-ui-icons-react";
import { useState, useMemo, useCallback, type JSX } from "react";
import { useParams, useNavigate, useLocation } from "react-router";
import useGetProjectDetails from "@api/useGetProjectDetails";
import useGetProjectFeatures from "@api/useGetProjectFeatures";
import useGetProjectFilters from "@api/useGetProjectFilters";
import { useGetProjectCasesPage } from "@api/useGetProjectCasesPage";
import useGetChangeRequests from "@features/operations/api/useGetChangeRequests";
import { getProjectPermissions } from "@utils/permission";
import {
  CaseType,
  CaseStatus,
} from "@features/support/constants/supportConstants";
import { getLast30DaysUtcRange } from "@features/support/utils/support";
import ListPageHeader from "@components/list-view/ListPageHeader";
import ListItems from "@components/list-view/ListItems";
import ChangeRequestsList from "@features/operations/components/change-requests/ChangeRequestsList";
import ErrorIndicator from "@components/error-indicator/ErrorIndicator";
import type { CaseListItem } from "@features/support/types/cases";
import type { ChangeRequestItem } from "@features/operations/types/changeRequests";

// CR stateKeys are stable integers defined in CHANGE_REQUEST_STATE_API_ID_TO_LABEL.
const CR_ACTION_REQUIRED_STATE_KEYS = [1, 5]; // Customer Review + Customer Approval
const CR_OUTSTANDING_STATE_KEYS = [-5, -4, -3, 5, -2, -1, 0, 1]; // All except Rollback(2), Closed(3), Canceled(4)
const CR_CLOSED_STATE_KEYS = [3]; // Closed

export type DashboardItemsMode =
  | "action-required"
  | "outstanding-interactions"
  | "closed-last-30d";

interface DashboardItemsPageProps {
  mode: DashboardItemsMode;
}

/**
 * Combined summary page for Action Required / Outstanding Interactions dashboard cards.
 * Shows Cases, Service Requests, SRA, and Change Requests in accordion sections,
 * pre-filtered by the relevant statuses for the selected mode.
 *
 * @param {DashboardItemsPageProps} props - Page mode.
 * @returns {JSX.Element} The rendered page.
 */
export default function DashboardItemsPage({
  mode,
}: DashboardItemsPageProps): JSX.Element {
  const navigate = useNavigate();
  const location = useLocation();
  const returnTo = (location.state as { returnTo?: string } | null)?.returnTo;
  const { projectId } = useParams<{ projectId: string }>();

  // --- Permissions ---
  const { data: project, isLoading: isProjectLoading } = useGetProjectDetails(
    projectId || "",
  );
  const { data: projectFeatures } = useGetProjectFeatures(projectId || "");
  const permissions = useMemo(() => {
    if (isProjectLoading || !project)
      return getProjectPermissions(undefined, { projectFeatures: null });
    return getProjectPermissions(project.type?.label, { projectFeatures });
  }, [project, isProjectLoading, projectFeatures]);

  // --- Filter metadata → case status IDs ---
  const { data: filterMetadata, isError: isFilterMetadataError } =
    useGetProjectFilters(projectId || "");

  // Resolve case status IDs from filter metadata labels.
  // Returns undefined while metadata is loading, or a (possibly empty) array once loaded.
  const caseStatusIds = useMemo((): number[] | undefined => {
    if (!filterMetadata?.caseStates) return undefined;
    if (mode === "action-required") {
      return filterMetadata.caseStates
        .filter(
          (s) =>
            s.label === CaseStatus.AWAITING_INFO ||
            s.label === CaseStatus.SOLUTION_PROPOSED,
        )
        .map((s) => Number(s.id));
    }
    if (mode === "closed-last-30d") {
      return filterMetadata.caseStates
        .filter((s) => s.label === CaseStatus.CLOSED)
        .map((s) => Number(s.id));
    }
    return filterMetadata.caseStates
      .filter((s) => s.label !== CaseStatus.CLOSED)
      .map((s) => Number(s.id));
  }, [mode, filterMetadata]);

  const crStateKeys =
    mode === "action-required"
      ? CR_ACTION_REQUIRED_STATE_KEYS
      : mode === "closed-last-30d"
        ? CR_CLOSED_STATE_KEYS
        : CR_OUTSTANDING_STATE_KEYS;

  const isOutstandingMode = mode === "outstanding-interactions";

  // filterMetadataLoaded tracks whether the metadata response has arrived (distinct from having IDs).
  // An error counts as "loaded" so the page does not stay on skeletons forever.
  const filterMetadataLoaded = !!filterMetadata || isFilterMetadataError;

  // For outstanding mode: resolved = closed items shown in the "Resolved Items" sub-section.
  const resolvedCaseStatusIds = useMemo((): number[] | undefined => {
    if (!isOutstandingMode) return undefined;
    if (!filterMetadata?.caseStates) return undefined;
    return filterMetadata.caseStates
      .filter((s) => s.label === CaseStatus.CLOSED)
      .map((s) => Number(s.id));
  }, [isOutstandingMode, filterMetadata]);

  // Queries are enabled once metadata is loaded AND there are status IDs to filter by.
  // An empty caseStatusIds array (no matching statuses) means no items exist — skip the query.
  const hasStatusIds = filterMetadataLoaded && (caseStatusIds?.length ?? 0) > 0;
  const hasResolvedStatusIds =
    isOutstandingMode &&
    filterMetadataLoaded &&
    (resolvedCaseStatusIds?.length ?? 0) > 0;
  const apiResolvedStatusIds = hasResolvedStatusIds
    ? resolvedCaseStatusIds
    : undefined;

  // statusIds sent to the API: use the resolved IDs, or undefined if empty (never send []).
  const apiStatusIds = hasStatusIds ? caseStatusIds : undefined;

  const casesEnabled = !!projectId && !isProjectLoading && hasStatusIds;
  const srEnabled =
    !!projectId && !isProjectLoading && hasStatusIds && permissions.hasSR;
  const sraEnabled =
    !!projectId &&
    !isProjectLoading &&
    hasStatusIds &&
    permissions.hasSecurityReportAnalysis;
  const engEnabled =
    !!projectId &&
    !isProjectLoading &&
    hasStatusIds &&
    permissions.hasEngagements;
  const crEnabled = !!projectId && permissions.hasCR && !isProjectLoading;

  const closedLast30dRange = useMemo(
    () => (mode === "closed-last-30d" ? getLast30DaysUtcRange() : undefined),
    [mode],
  );

  // --- Data fetching (10 items each — single-page query, never loads more) ---
  const {
    data: casesQueryData,
    isLoading: isCasesQuerying,
    isError: isCasesError,
  } = useGetProjectCasesPage(
    projectId || "",
    {
      filters: {
        caseTypes: [CaseType.DEFAULT_CASE],
        statusIds: apiStatusIds,
        ...closedLast30dRange,
      },
    },
    0,
    10,
    { enabled: casesEnabled },
  );

  const {
    data: srQueryData,
    isLoading: isSrQuerying,
    isError: isSrError,
  } = useGetProjectCasesPage(
    projectId || "",
    {
      filters: {
        caseTypes: [CaseType.SERVICE_REQUEST],
        statusIds: apiStatusIds,
        ...closedLast30dRange,
      },
    },
    0,
    10,
    { enabled: srEnabled },
  );

  const {
    data: sraQueryData,
    isLoading: isSraQuerying,
    isError: isSraError,
  } = useGetProjectCasesPage(
    projectId || "",
    {
      filters: {
        caseTypes: [CaseType.SECURITY_REPORT_ANALYSIS],
        statusIds: apiStatusIds,
        ...closedLast30dRange,
      },
    },
    0,
    10,
    { enabled: sraEnabled },
  );

  const {
    data: engQueryData,
    isLoading: isEngQuerying,
    isError: isEngError,
  } = useGetProjectCasesPage(
    projectId || "",
    {
      filters: {
        caseTypes: [CaseType.ENGAGEMENT],
        statusIds: apiStatusIds,
        ...closedLast30dRange,
      },
    },
    0,
    10,
    { enabled: engEnabled },
  );

  const {
    data: crQueryData,
    isLoading: isCrQuerying,
    isError: isCrError,
  } = useGetChangeRequests(
    projectId || "",
    {
      filters: {
        stateKeys: crStateKeys,
        ...closedLast30dRange,
      },
    },
    0,
    10,
    { enabled: crEnabled },
  );

  // --- Resolved item queries (outstanding mode only) ---
  const {
    data: resolvedCasesData,
    isLoading: isResolvedCasesQuerying,
    isError: isResolvedCasesError,
  } = useGetProjectCasesPage(
    projectId || "",
    {
      filters: {
        caseTypes: [CaseType.DEFAULT_CASE],
        statusIds: apiResolvedStatusIds,
      },
    },
    0,
    10,
    { enabled: !!projectId && !isProjectLoading && hasResolvedStatusIds },
  );

  const {
    data: resolvedSrData,
    isLoading: isResolvedSrQuerying,
    isError: isResolvedSrError,
  } = useGetProjectCasesPage(
    projectId || "",
    {
      filters: {
        caseTypes: [CaseType.SERVICE_REQUEST],
        statusIds: apiResolvedStatusIds,
      },
    },
    0,
    10,
    {
      enabled:
        !!projectId &&
        !isProjectLoading &&
        hasResolvedStatusIds &&
        permissions.hasSR,
    },
  );

  const {
    data: resolvedSraData,
    isLoading: isResolvedSraQuerying,
    isError: isResolvedSraError,
  } = useGetProjectCasesPage(
    projectId || "",
    {
      filters: {
        caseTypes: [CaseType.SECURITY_REPORT_ANALYSIS],
        statusIds: apiResolvedStatusIds,
      },
    },
    0,
    10,
    {
      enabled:
        !!projectId &&
        !isProjectLoading &&
        hasResolvedStatusIds &&
        permissions.hasSecurityReportAnalysis,
    },
  );

  const {
    data: resolvedEngData,
    isLoading: isResolvedEngQuerying,
    isError: isResolvedEngError,
  } = useGetProjectCasesPage(
    projectId || "",
    {
      filters: {
        caseTypes: [CaseType.ENGAGEMENT],
        statusIds: apiResolvedStatusIds,
      },
    },
    0,
    10,
    {
      enabled:
        !!projectId &&
        !isProjectLoading &&
        hasResolvedStatusIds &&
        permissions.hasEngagements,
    },
  );

  const {
    data: resolvedCrData,
    isLoading: isResolvedCrQuerying,
    isError: isResolvedCrError,
  } = useGetChangeRequests(
    projectId || "",
    { filters: { stateKeys: CR_CLOSED_STATE_KEYS } },
    0,
    10,
    {
      enabled:
        !!projectId &&
        permissions.hasCR &&
        !isProjectLoading &&
        isOutstandingMode,
    },
  );

  // --- Derived values ---
  // useGetProjectCasesPage returns CaseSearchResponse directly (not InfiniteData).
  const cases = casesQueryData?.cases ?? [];
  const casesTotal = casesQueryData?.totalRecords ?? 0;
  // Loading while: metadata not yet loaded, OR query in-flight without data yet.
  const isCasesLoading =
    !filterMetadataLoaded ||
    (casesEnabled && isCasesQuerying && !casesQueryData);

  const serviceRequests = srQueryData?.cases ?? [];
  const srTotal = srQueryData?.totalRecords ?? 0;
  const isSrLoading =
    permissions.hasSR &&
    (!filterMetadataLoaded || (srEnabled && isSrQuerying && !srQueryData));

  const sraItems = sraQueryData?.cases ?? [];
  const sraTotal = sraQueryData?.totalRecords ?? 0;
  const isSraLoading =
    permissions.hasSecurityReportAnalysis &&
    (!filterMetadataLoaded || (sraEnabled && isSraQuerying && !sraQueryData));

  const engagements = engQueryData?.cases ?? [];
  const engTotal = engQueryData?.totalRecords ?? 0;
  const isEngLoading =
    permissions.hasEngagements &&
    (!filterMetadataLoaded || (engEnabled && isEngQuerying && !engQueryData));

  const changeRequests = crQueryData?.changeRequests ?? [];
  const crTotal = crQueryData?.totalRecords ?? 0;
  const isCrLoading =
    permissions.hasCR &&
    (isProjectLoading || (crEnabled && isCrQuerying && !crQueryData));

  // --- Resolved derived values (outstanding mode only) ---
  const resolvedCases = resolvedCasesData?.cases ?? [];
  const resolvedCasesTotal = resolvedCasesData?.totalRecords ?? 0;
  const isResolvedCasesLoading =
    isOutstandingMode &&
    (!filterMetadataLoaded ||
      (hasResolvedStatusIds && isResolvedCasesQuerying && !resolvedCasesData));

  const resolvedServiceRequests = resolvedSrData?.cases ?? [];
  const resolvedSrTotal = resolvedSrData?.totalRecords ?? 0;
  const isResolvedSrLoading =
    isOutstandingMode &&
    permissions.hasSR &&
    (!filterMetadataLoaded ||
      (hasResolvedStatusIds && isResolvedSrQuerying && !resolvedSrData));

  const resolvedSraItems = resolvedSraData?.cases ?? [];
  const resolvedSraTotal = resolvedSraData?.totalRecords ?? 0;
  const isResolvedSraLoading =
    isOutstandingMode &&
    permissions.hasSecurityReportAnalysis &&
    (!filterMetadataLoaded ||
      (hasResolvedStatusIds && isResolvedSraQuerying && !resolvedSraData));

  const resolvedEngagements = resolvedEngData?.cases ?? [];
  const resolvedEngTotal = resolvedEngData?.totalRecords ?? 0;
  const isResolvedEngLoading =
    isOutstandingMode &&
    permissions.hasEngagements &&
    (!filterMetadataLoaded ||
      (hasResolvedStatusIds && isResolvedEngQuerying && !resolvedEngData));

  const resolvedChangeRequests = resolvedCrData?.changeRequests ?? [];
  const resolvedCrTotal = resolvedCrData?.totalRecords ?? 0;
  const isResolvedCrLoading =
    isOutstandingMode &&
    permissions.hasCR &&
    (isProjectLoading ||
      (!!projectId &&
        permissions.hasCR &&
        !isProjectLoading &&
        isResolvedCrQuerying &&
        !resolvedCrData));

  // --- Accordion state ---
  const [expandedSections, setExpandedSections] = useState<Set<string>>(
    () =>
      new Set([
        "cases",
        "sr",
        "sra",
        "eng",
        "cr",
        "resolved-cases",
        "resolved-sr",
        "resolved-sra",
        "resolved-eng",
        "resolved-cr",
      ]),
  );
  const toggleSection = useCallback((id: string) => {
    setExpandedSections((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }, []);

  // --- Navigation helpers ---
  const handleCaseClick = useCallback(
    (item: CaseListItem) => {
      navigate(`../../support/cases/${item.id}`, { relative: "path" });
    },
    [navigate],
  );

  const handleSrClick = useCallback(
    (item: CaseListItem) => {
      navigate(`../../operations/service-requests/${item.id}`, {
        relative: "path",
      });
    },
    [navigate],
  );

  const handleSraClick = useCallback(
    (item: CaseListItem) => {
      navigate(`../../security-center/security-report-analysis/${item.id}`, {
        relative: "path",
      });
    },
    [navigate],
  );

  const handleEngClick = useCallback(
    (item: CaseListItem) => {
      navigate(`../../engagements/${item.id}`, { relative: "path" });
    },
    [navigate],
  );

  const handleCrClick = useCallback(
    (item: ChangeRequestItem) => {
      navigate(`../../operations/change-requests/${item.id}`, {
        relative: "path",
      });
    },
    [navigate],
  );

  // --- Section definitions ---
  type CaseSection = {
    id: string;
    label: string;
    isLoading: boolean;
    isError: boolean;
    total: number;
    hasPermission: boolean;
    isCr: false;
    items: CaseListItem[];
    hideSeverity: boolean;
    entityName: string;
    viewAllPath: string;
    viewAllLabel: string;
    onItemClick: (item: CaseListItem) => void;
  };

  type CrSection = {
    id: string;
    label: string;
    isLoading: boolean;
    isError: boolean;
    total: number;
    hasPermission: boolean;
    isCr: true;
    items: ChangeRequestItem[];
    viewAllPath: string;
    viewAllLabel: string;
    onItemClick: (item: ChangeRequestItem) => void;
  };

  type Section = CaseSection | CrSection;

  const sections: Section[] = [
    {
      id: "cases",
      label: "Cases",
      isLoading: isCasesLoading,
      isError: isCasesError,
      total: casesTotal,
      hasPermission: true,
      isCr: false,
      items: cases,
      hideSeverity: false,
      entityName: "cases",
      viewAllPath:
        mode === "outstanding-interactions"
          ? "../../support/cases?statusFilter=active"
          : "../../support/cases",
      viewAllLabel: "View all cases",
      onItemClick: handleCaseClick,
    },
    {
      id: "sr",
      label: "Service Requests",
      isLoading: isSrLoading,
      isError: isSrError,
      total: srTotal,
      hasPermission: permissions.hasSR,
      isCr: false,
      items: serviceRequests,
      hideSeverity: true,
      entityName: "service requests",
      viewAllPath: "../../operations/service-requests",
      viewAllLabel: "View all service requests",
      onItemClick: handleSrClick,
    },
    {
      id: "sra",
      label: "Security Report Analysis",
      isLoading: isSraLoading,
      isError: isSraError,
      total: sraTotal,
      hasPermission: permissions.hasSecurityReportAnalysis,
      isCr: false,
      items: sraItems,
      hideSeverity: false,
      entityName: "security reports",
      viewAllPath: "../../security-center",
      viewAllLabel: "View all security reports",
      onItemClick: handleSraClick,
    },
    {
      id: "eng",
      label: "Engagements",
      isLoading: isEngLoading,
      isError: isEngError,
      total: engTotal,
      hasPermission: permissions.hasEngagements,
      isCr: false,
      items: engagements,
      hideSeverity: true,
      entityName: "engagements",
      viewAllPath: "../../engagements",
      viewAllLabel: "View all engagements",
      onItemClick: handleEngClick,
    },
    {
      id: "cr",
      label: "Change Requests",
      isLoading: isCrLoading,
      isError: isCrError,
      total: crTotal,
      hasPermission: permissions.hasCR,
      isCr: true,
      items: changeRequests,
      viewAllPath: "../../operations/change-requests",
      viewAllLabel: "View all change requests",
      onItemClick: handleCrClick,
    },
  ];

  const resolvedSections: Section[] = isOutstandingMode
    ? [
        {
          id: "resolved-cases",
          label: "Cases",
          isLoading: isResolvedCasesLoading,
          isError: isResolvedCasesError,
          total: resolvedCasesTotal,
          hasPermission: true,
          isCr: false,
          items: resolvedCases,
          hideSeverity: false,
          entityName: "cases",
          viewAllPath: "../../support/cases",
          viewAllLabel: "View all cases",
          onItemClick: handleCaseClick,
        },
        {
          id: "resolved-sr",
          label: "Service Requests",
          isLoading: isResolvedSrLoading,
          isError: isResolvedSrError,
          total: resolvedSrTotal,
          hasPermission: permissions.hasSR,
          isCr: false,
          items: resolvedServiceRequests,
          hideSeverity: true,
          entityName: "service requests",
          viewAllPath: "../../operations/service-requests",
          viewAllLabel: "View all service requests",
          onItemClick: handleSrClick,
        },
        {
          id: "resolved-sra",
          label: "Security Report Analysis",
          isLoading: isResolvedSraLoading,
          isError: isResolvedSraError,
          total: resolvedSraTotal,
          hasPermission: permissions.hasSecurityReportAnalysis,
          isCr: false,
          items: resolvedSraItems,
          hideSeverity: false,
          entityName: "security reports",
          viewAllPath: "../../security-center",
          viewAllLabel: "View all security reports",
          onItemClick: handleSraClick,
        },
        {
          id: "resolved-eng",
          label: "Engagements",
          isLoading: isResolvedEngLoading,
          isError: isResolvedEngError,
          total: resolvedEngTotal,
          hasPermission: permissions.hasEngagements,
          isCr: false,
          items: resolvedEngagements,
          hideSeverity: true,
          entityName: "engagements",
          viewAllPath: "../../engagements",
          viewAllLabel: "View all engagements",
          onItemClick: handleEngClick,
        },
        {
          id: "resolved-cr",
          label: "Change Requests",
          isLoading: isResolvedCrLoading,
          isError: isResolvedCrError,
          total: resolvedCrTotal,
          hasPermission: permissions.hasCR,
          isCr: true,
          items: resolvedChangeRequests,
          viewAllPath: "../../operations/change-requests",
          viewAllLabel: "View all change requests",
          onItemClick: handleCrClick,
        },
      ]
    : [];

  const isPageLoading = isProjectLoading || !filterMetadataLoaded;
  const isPageError = !isProjectLoading && isFilterMetadataError;

  const visibleSections =
    isPageLoading || isPageError
      ? []
      : sections.filter(
          (s) => s.hasPermission && (s.isLoading || s.total > 0 || s.isError),
        );

  const visibleResolvedSections =
    isPageLoading || isPageError
      ? []
      : resolvedSections.filter(
          (s) => s.hasPermission && (s.isLoading || s.total > 0 || s.isError),
        );

  return (
    <Stack spacing={3} sx={{ minWidth: 0 }}>
      <ListPageHeader
        title={
          mode === "action-required"
            ? "Action Required Items"
            : mode === "closed-last-30d"
              ? "Closed items (last 30d)"
              : "Outstanding Items"
        }
        description={
          mode === "action-required"
            ? "Items awaiting your response"
            : mode === "closed-last-30d"
              ? "Successfully closed and resolved items during the last 30 days "
              : "View all currently active and unresolved items"
        }
        backLabel="Back"
        onBack={() => navigate(returnTo ?? "..")}
      />

      <Stack spacing={3}>
        {isPageLoading && (
          <>
            <Skeleton variant="rounded" height={56} />
            <Skeleton variant="rounded" height={56} />
            <Skeleton variant="rounded" height={56} />
            <Skeleton variant="rounded" height={56} />
            <Skeleton variant="rounded" height={56} />
          </>
        )}

        {visibleSections.map((section) => (
          <Accordion
            key={section.id}
            expanded={expandedSections.has(section.id)}
            onChange={() => toggleSection(section.id)}
            disableGutters
            elevation={0}
            sx={{
              "&:before": { display: "none" },
              border: "1px solid",
              borderColor: "divider",
              borderRadius: 2,
              overflow: "hidden",
              bgcolor: "background.paper",
            }}
          >
            <AccordionSummary
              expandIcon={<ChevronDown size={20} />}
              sx={{ px: 2.5, py: 1.5, minHeight: 56 }}
            >
              <Box sx={{ display: "flex", alignItems: "center", gap: 1.5 }}>
                <Typography variant="subtitle1" fontWeight={600}>
                  {section.label}
                </Typography>
                {section.isLoading ? (
                  <Skeleton variant="rounded" width={32} height={22} />
                ) : (
                  <Chip
                    label={section.total}
                    size="small"
                    color="default"
                    sx={{ fontWeight: 600 }}
                  />
                )}
              </Box>
            </AccordionSummary>

            <AccordionDetails sx={{ px: 2.5, pb: 1, pt: 0 }}>
              {section.isCr ? (
                <>
                  <Box sx={{ pb: 2.5 }}>
                    <ChangeRequestsList
                      changeRequests={section.items}
                      isLoading={section.isLoading}
                      isError={section.isError}
                      onChangeRequestClick={section.onItemClick}
                    />
                  </Box>
                  {section.total > 10 && (
                    <>
                      <Divider />
                      <Box
                        sx={{
                          display: "flex",
                          justifyContent: "flex-end",
                          pt: 0.5,
                        }}
                      >
                        <Button
                          variant="text"
                          color="warning"
                          endIcon={<ArrowRight size={16} />}
                          onClick={() =>
                            navigate(section.viewAllPath, { relative: "path" })
                          }
                          sx={{
                            textTransform: "none",
                            fontWeight: 500,
                          }}
                        >
                          {section.viewAllLabel}
                        </Button>
                      </Box>
                    </>
                  )}
                </>
              ) : (
                <>
                  <Box sx={{ pb: 2.5 }}>
                    <ListItems
                      cases={section.items}
                      isLoading={section.isLoading}
                      isError={section.isError}
                      onCaseClick={section.onItemClick}
                      entityName={section.entityName}
                      hideSeverity={section.hideSeverity}
                      showInternalId
                    />
                  </Box>
                  {section.total > 10 && (
                    <>
                      <Divider />
                      <Box
                        sx={{
                          display: "flex",
                          justifyContent: "flex-end",
                          pt: 0.5,
                        }}
                      >
                        <Button
                          variant="text"
                          color="warning"
                          endIcon={<ArrowRight size={16} />}
                          onClick={() =>
                            navigate(section.viewAllPath, { relative: "path" })
                          }
                          sx={{
                            textTransform: "none",
                            fontWeight: 500,
                          }}
                        >
                          {section.viewAllLabel}
                        </Button>
                      </Box>
                    </>
                  )}
                </>
              )}
            </AccordionDetails>
          </Accordion>
        ))}

        {isOutstandingMode && (
          <>
            <Typography
              variant="subtitle2"
              color="text.secondary"
              fontWeight={600}
              sx={{ px: 0.5, pt: visibleSections.length > 0 ? 1 : 0 }}
            >
              Resolved Items
            </Typography>
            {visibleResolvedSections.map((section) => (
              <Accordion
                key={section.id}
                expanded={expandedSections.has(section.id)}
                onChange={() => toggleSection(section.id)}
                disableGutters
                elevation={0}
                sx={{
                  "&:before": { display: "none" },
                  border: "1px solid",
                  borderColor: "divider",
                  borderRadius: 2,
                  overflow: "hidden",
                  bgcolor: "background.paper",
                }}
              >
                <AccordionSummary
                  expandIcon={<ChevronDown size={20} />}
                  sx={{ px: 2.5, py: 1.5, minHeight: 56 }}
                >
                  <Box sx={{ display: "flex", alignItems: "center", gap: 1.5 }}>
                    <Typography variant="subtitle1" fontWeight={600}>
                      {section.label}
                    </Typography>
                    {section.isLoading ? (
                      <Skeleton variant="rounded" width={32} height={22} />
                    ) : (
                      <Chip
                        label={section.total}
                        size="small"
                        color="default"
                        sx={{ fontWeight: 600 }}
                      />
                    )}
                  </Box>
                </AccordionSummary>
                <AccordionDetails sx={{ px: 2.5, pb: 1, pt: 0 }}>
                  {section.isCr ? (
                    <>
                      <Box sx={{ pb: 2.5 }}>
                        <ChangeRequestsList
                          changeRequests={section.items}
                          isLoading={section.isLoading}
                          isError={section.isError}
                          onChangeRequestClick={section.onItemClick}
                        />
                      </Box>
                      {section.total > 10 && (
                        <>
                          <Divider />
                          <Box
                            sx={{
                              display: "flex",
                              justifyContent: "flex-end",
                              pt: 0.5,
                            }}
                          >
                            <Button
                              variant="text"
                              color="warning"
                              endIcon={<ArrowRight size={16} />}
                              onClick={() =>
                                navigate(section.viewAllPath, {
                                  relative: "path",
                                })
                              }
                              sx={{ textTransform: "none", fontWeight: 500 }}
                            >
                              {section.viewAllLabel}
                            </Button>
                          </Box>
                        </>
                      )}
                    </>
                  ) : (
                    <>
                      <Box sx={{ pb: 2.5 }}>
                        <ListItems
                          cases={section.items}
                          isLoading={section.isLoading}
                          isError={section.isError}
                          onCaseClick={section.onItemClick}
                          entityName={section.entityName}
                          hideSeverity={section.hideSeverity}
                        />
                      </Box>
                      {section.total > 10 && (
                        <>
                          <Divider />
                          <Box
                            sx={{
                              display: "flex",
                              justifyContent: "flex-end",
                              pt: 0.5,
                            }}
                          >
                            <Button
                              variant="text"
                              color="warning"
                              endIcon={<ArrowRight size={16} />}
                              onClick={() =>
                                navigate(section.viewAllPath, {
                                  relative: "path",
                                })
                              }
                              sx={{ textTransform: "none", fontWeight: 500 }}
                            >
                              {section.viewAllLabel}
                            </Button>
                          </Box>
                        </>
                      )}
                    </>
                  )}
                </AccordionDetails>
              </Accordion>
            ))}
            {!isPageLoading &&
              !isPageError &&
              visibleResolvedSections.length === 0 && (
                <Box sx={{ textAlign: "center", py: 4 }}>
                  <Typography variant="body2" color="text.secondary">
                    No resolved items found.
                  </Typography>
                </Box>
              )}
          </>
        )}

        {isPageError && (
          <Box sx={{ py: 4 }}>
            <ErrorIndicator entityName="items" />
          </Box>
        )}

        {!isPageLoading &&
          !isPageError &&
          !isOutstandingMode &&
          visibleSections.length === 0 && (
            <Box sx={{ textAlign: "center", py: 8 }}>
              <Typography variant="body1" color="text.secondary">
                No items found.
              </Typography>
            </Box>
          )}
      </Stack>
    </Stack>
  );
}
