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

import { useMemo, type JSX, type ReactNode } from "react";
import type { OxygenTheme } from "@wso2/oxygen-ui/styles/OxygenThemeBase";
import { Box, Button, Card, Chip, Skeleton, Stack, Tooltip, Typography, pxToRem, useTheme } from "@wso2/oxygen-ui";
import { ChevronRight, GitFork, GitPullRequest, Link as LinkIcon, Plus } from "@wso2/oxygen-ui-icons-react";
import { Link } from "react-router-dom";
import { useQueries, useQuery } from "@tanstack/react-query";
import { cases, type ChildCaseRow } from "@src/services/cases";
import { changeRequests } from "@src/services/changeRequests";
import type { CaseDetail, CaseLinkRefDto } from "@src/types";
import { SeverityChip, StatusChip } from "@components/support/Chips";
import { changeRequestStateColor, changeRequestStateLabel } from "@components/operations/config";

interface LinkedItemsTabProps {
  caseId: string;
  caseDetail: CaseDetail;
  isClosed: boolean;
  onLinkCase: () => void;
  onCreateServiceRequest: () => void;
}

/**
 * "Linked Items" tab — mirrors the webapp's CsmCaseDetailPage `related` tab (Child cases, Linked
 * service requests, Linked change requests), adapted from its desktop tables to mobile cards. The
 * webapp's per-widget refresh buttons are dropped — the microapp doesn't have that convention
 * anywhere else in the case detail page, which already re-fetches the whole case on every mutation
 * (see CaseDetailPage.tsx's invalidateCase).
 */
export function LinkedItemsTab({
  caseId,
  caseDetail,
  isClosed,
  onLinkCase,
  onCreateServiceRequest,
}: LinkedItemsTabProps) {
  // Content-relevance, not a data-source gate: shown whenever this is a service request (the only
  // case type that carries the link) or the list already has entries — mirrors the webapp.
  const showLinkedChangeRequests = caseDetail.type === "service_request" || caseDetail.linkedChangeRequests.length > 0;

  return (
    <Stack gap={2}>
      <Tooltip title={isClosed ? "This case is closed — it's read-only." : ""}>
        <Box component="span" sx={{ alignSelf: "flex-start" }}>
          <Button
            size="small"
            variant="outlined"
            startIcon={<LinkIcon size={14} />}
            onClick={onLinkCase}
            disabled={isClosed}
          >
            Link to another case
          </Button>
        </Box>
      </Tooltip>

      <ChildCasesSection caseId={caseId} />

      <LinkedServiceRequestsSection
        caseId={caseId}
        refs={caseDetail.linkedServiceRequests}
        createDisabled={isClosed}
        onCreateServiceRequest={onCreateServiceRequest}
      />

      {showLinkedChangeRequests && <LinkedChangeRequestsSection refs={caseDetail.linkedChangeRequests} />}
    </Stack>
  );
}

// ---------------------------------------------------------------------------
// Shared row shell — a tappable case card compact enough to list several per
// tab, unlike the full support/CaseCard.tsx (which needs fields these linked
// refs don't always carry, e.g. wso2Id/updatedOn/type).
// ---------------------------------------------------------------------------

function LinkedItemsSection({
  icon,
  title,
  count,
  action,
  emptyMessage,
  children,
}: {
  icon: JSX.Element;
  title: string;
  count?: number;
  action?: ReactNode;
  emptyMessage: string;
  children: ReactNode;
}) {
  const isEmpty = count === 0;
  return (
    <Card sx={{ p: 1.5 }}>
      <Stack gap={1.25}>
        <Stack direction="row" alignItems="center" justifyContent="space-between" gap={1} flexWrap="wrap">
          <Stack direction="row" alignItems="center" gap={0.75}>
            {icon}
            <Typography variant="subtitle2">
              {title}
              {typeof count === "number" && count > 0 ? ` (${count})` : ""}
            </Typography>
          </Stack>
          {action}
        </Stack>
        {isEmpty ? (
          <Typography variant="body2" color="text.secondary">
            {emptyMessage}
          </Typography>
        ) : (
          <Stack gap={1}>{children}</Stack>
        )}
      </Stack>
    </Card>
  );
}

function CaseRow({
  to,
  label,
  severity,
  state,
  assigneeName,
  loading,
}: {
  to: string;
  label: string;
  severity?: CaseDetailSeverity;
  state?: CaseDetailState;
  assigneeName?: string | null;
  /** True while enrichment (state/severity/assignee) is still resolving — shows skeletons for
   * those fields instead of a bare "—", matching LinkedServiceRequestsSection's rows before the
   * shared child-cases search resolves. */
  loading?: boolean;
}): JSX.Element {
  const theme = useTheme<OxygenTheme>();
  return (
    <Card component={Link} to={to} variant="outlined" sx={{ textDecoration: "none", p: 1, display: "block" }}>
      <Stack direction="row" alignItems="center" gap={1}>
        <Stack sx={{ minWidth: 0, flex: 1 }} gap={0.5}>
          <Typography variant="body2" color="text.primary" noWrap title={label}>
            {label}
          </Typography>
          <Stack direction="row" alignItems="center" gap={1} flexWrap="wrap">
            {loading ? <Skeleton variant="rounded" width={64} height={20} /> : state && <StatusChip state={state} />}
            {severity && <SeverityChip severity={severity} />}
            {!loading && assigneeName && (
              <Typography variant="caption" color="text.secondary" noWrap>
                {assigneeName}
              </Typography>
            )}
          </Stack>
        </Stack>
        <ChevronRight size={pxToRem(18)} color={theme.vars.palette.text.secondary} style={{ flexShrink: 0 }} />
      </Stack>
    </Card>
  );
}

// Narrow aliases so CaseRow's props stay readable above without importing the full CaseState/
// CaseSeverity union names twice.
type CaseDetailSeverity = NonNullable<CaseDetail["severity"]>;
type CaseDetailState = CaseDetail["state"];

// ---------------------------------------------------------------------------
// 1. Child cases — cases whose own parentId points at this one.
// ---------------------------------------------------------------------------

function ChildCasesSection({ caseId }: { caseId: string }) {
  const { data, isLoading, isError } = useQuery(cases.searchChildren(caseId));
  const rows = data?.cases ?? [];

  return (
    <LinkedItemsSection
      icon={<GitFork size={16} />}
      title="Child cases"
      count={isLoading || isError ? undefined : rows.length}
      emptyMessage="No child cases linked to this case."
    >
      {isError ? (
        <Typography variant="body2" color="error">
          Could not load child cases for this case.
        </Typography>
      ) : isLoading ? (
        <Stack gap={1}>
          {[0, 1].map((i) => (
            <Skeleton key={i} variant="rounded" height={56} />
          ))}
        </Stack>
      ) : (
        rows.map((c) => <ChildCaseCardRow key={c.id} row={c} />)
      )}
    </LinkedItemsSection>
  );
}

function ChildCaseCardRow({ row }: { row: ChildCaseRow }) {
  return (
    <CaseRow
      to={`/cases/${encodeURIComponent(row.id)}`}
      label={`${row.number} — ${row.subject}`}
      severity={row.severity ?? undefined}
      state={row.state}
      assigneeName={row.assigneeName}
    />
  );
}

// ---------------------------------------------------------------------------
// 2. Linked service requests — the case-detail payload's own refs, enriched
//    with state/severity/assignee from the same child-cases search (shared
//    cache entry — service requests are children too, just restricted to the
//    ids the case-detail response already names).
// ---------------------------------------------------------------------------

function LinkedServiceRequestsSection({
  caseId,
  refs,
  createDisabled,
  onCreateServiceRequest,
}: {
  caseId: string;
  refs: CaseLinkRefDto[];
  createDisabled: boolean;
  onCreateServiceRequest: () => void;
}) {
  const { data, isLoading, isError } = useQuery(cases.searchChildren(caseId));
  const enrichedById = useMemo(() => {
    const map = new Map<string, ChildCaseRow>();
    for (const row of data?.cases ?? []) map.set(row.id, row);
    return map;
  }, [data]);
  // isError means enrichment is unavailable, not pending — fall straight to a plain row instead
  // of a skeleton that would spin forever.
  const enriching = isLoading && !isError;

  return (
    <LinkedItemsSection
      icon={<LinkIcon size={16} />}
      title="Linked service requests"
      count={refs.length}
      emptyMessage="No service requests linked to this case."
      action={
        <Tooltip title={createDisabled ? "This case is closed — it's read-only." : ""}>
          <Box component="span">
            <Button
              size="small"
              variant="text"
              startIcon={<Plus size={14} />}
              onClick={onCreateServiceRequest}
              disabled={createDisabled}
              aria-label="Create service request"
            >
              Create
            </Button>
          </Box>
        </Tooltip>
      }
    >
      {refs.map((ref) => {
        const enriched = enrichedById.get(ref.id);
        return (
          <CaseRow
            key={ref.id}
            to={`/cases/${encodeURIComponent(ref.id)}`}
            label={[ref.number, ref.name].filter(Boolean).join(" — ")}
            severity={enriched?.severity ?? undefined}
            state={enriched?.state}
            assigneeName={enriched?.assigneeName}
            loading={!enriched && enriching}
          />
        );
      })}
    </LinkedItemsSection>
  );
}

// ---------------------------------------------------------------------------
// 3. Linked change requests — raised from this service request. Each ref only
//    carries id/number/name; state and target environment are fetched per row
//    (fanned out with useQueries, mirroring the webapp), so a slow or failed
//    row never blocks or hides the others.
// ---------------------------------------------------------------------------

function LinkedChangeRequestsSection({ refs }: { refs: CaseLinkRefDto[] }) {
  const queries = useQueries({
    queries: refs.map((ref) => changeRequests.get(ref.id)),
  });

  return (
    <LinkedItemsSection
      icon={<GitPullRequest size={16} />}
      title="Linked change requests"
      count={refs.length}
      emptyMessage="No change requests have been raised from this service request yet."
    >
      {refs.map((ref, i) => {
        const query = queries[i];
        const detail = query?.data;
        const rowLoading = query?.isLoading ?? false;
        return (
          <Card
            key={ref.id}
            component={Link}
            to={`/operations/change-requests/${encodeURIComponent(ref.id)}`}
            variant="outlined"
            sx={{ textDecoration: "none", p: 1, display: "block" }}
          >
            <Stack direction="row" alignItems="center" gap={1}>
              <Stack sx={{ minWidth: 0, flex: 1 }} gap={0.5}>
                <Typography variant="body2" color="text.primary" noWrap>
                  {[ref.number, ref.name].filter(Boolean).join(" — ")}
                </Typography>
                <Stack direction="row" alignItems="center" gap={1} flexWrap="wrap">
                  {rowLoading ? (
                    <Skeleton variant="rounded" width={64} height={20} />
                  ) : detail ? (
                    <Chip
                      size="small"
                      color={changeRequestStateColor(detail.state)}
                      label={changeRequestStateLabel(detail.state)}
                    />
                  ) : null}
                  {!rowLoading && detail?.deployment?.name && (
                    <Typography variant="caption" color="text.secondary" noWrap>
                      {detail.deployment.name}
                    </Typography>
                  )}
                </Stack>
              </Stack>
            </Stack>
          </Card>
        );
      })}
    </LinkedItemsSection>
  );
}
