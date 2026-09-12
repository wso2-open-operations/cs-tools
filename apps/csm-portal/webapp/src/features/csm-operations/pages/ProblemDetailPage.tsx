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

import { Box, Button, Card, Chip, Skeleton, Typography } from "@wso2/oxygen-ui";
import { ArrowLeft, Link as LinkIcon, Pencil } from "@wso2/oxygen-ui-icons-react";
import { type JSX, type ReactNode, useCallback, useEffect, useState } from "react";
import { useLocation } from "react-router";
import { formatBackendTimestampForDisplay } from "@utils/dateTime";
import { BackendApiError } from "@api/backend/client";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import { useGetProblem } from "@features/csm-operations/api/useGetProblem";
import { usePatchProblem } from "@features/csm-operations/api/usePatchProblem";
import EditProblemDialog from "@features/csm-operations/components/EditProblemDialog";
import ProblemActionBar from "@features/csm-operations/components/ProblemActionBar";
import ProblemFixNotesDialog from "@features/csm-operations/components/ProblemFixNotesDialog";
import { problemStateColor, problemStateLabel } from "@features/csm-operations/utils/problems";
import type { BeEntityRef, BeProblemRef, BeUpdateProblemPayload } from "@api/backend/types";
import { useNavTransition } from "@hooks/useNavTransition";
import { useNormalizedIdParam } from "@hooks/useNormalizedIdParam";
import { useRecordRecentView } from "@features/csm-recent/hooks/useRecentViews";

const OPERATIONS_PROBLEMS_PATH = "/operations?tab=problems";

function formatDateTime(value?: string | null): string {
  return (
    formatBackendTimestampForDisplay(value, {
      dateStyle: "medium",
      timeStyle: "short",
    }) ?? "—"
  );
}

function MetaCell({ label, children }: { label: string; children: ReactNode }): JSX.Element {
  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 0.25, minWidth: 0 }}>
      <Typography
        variant="caption"
        color="text.secondary"
        sx={{ textTransform: "uppercase", letterSpacing: 0.4 }}
      >
        {label}
      </Typography>
      <Box sx={{ minWidth: 0 }}>{children}</Box>
    </Box>
  );
}

function RefText({ value }: { value?: BeEntityRef | null }): JSX.Element {
  return <Typography variant="body2">{value?.name || "—"}</Typography>;
}

/**
 * Renders a `BeProblemRef` as a clickable Case-detail chip when `routeBase`
 * is given (the only route this webapp actually has for a linked-record
 * type today is `/cases/:caseId`), or as plain text otherwise. Deliberately
 * does NOT assume `originCase`/`primaryIncident`/`linkedChangeRequest` point
 * at the record type their field name implies — see the caveat on
 * `BeProblemRef` in `api/backend/types.ts`. Callers must only pass
 * `routeBase` when they independently know the target is that record type.
 */
function ProblemRefItem({
  value,
  routeBase,
  onNavigate,
}: {
  value?: BeProblemRef | null;
  routeBase?: string;
  onNavigate: (path: string) => void;
}): JSX.Element {
  if (!value) return <Typography variant="body2">—</Typography>;
  const label = value.number || value.id;
  if (!routeBase) return <Typography variant="body2">{label}</Typography>;
  return (
    <Chip
      size="small"
      variant="outlined"
      clickable
      icon={<LinkIcon size={14} />}
      label={label}
      onClick={() => onNavigate(`${routeBase}/${value.id}`)}
    />
  );
}

/**
 * Detail for a single problem (`GET /problems/{id}`): its overview fields,
 * linked records, and resolution/fix notes. State transitions (`PATCH
 * /problems/{id} { transition }`) go through `ProblemActionBar` — the live
 * ServiceNow Problem Management engine is a strictly linear forward chain
 * (`New -> Assess -> Root Cause Analysis -> Fix in Progress -> Resolved ->
 * Closed`, see `getNextProblemTransition`), so there's only ever one legal
 * next button, unlike Incident/CR. The `fix` transition routes through
 * `ProblemFixNotesDialog` first to optionally collect `causeNotes`/
 * `fixNotes` (unlike Incident's `RESOLVED`/`CLOSED`, these are genuinely
 * optional, not ServiceNow-required — the dialog is skippable). A separate
 * `EditProblemDialog` covers `assignedToId`/`assignmentGroupId`/
 * `workaround`/`targetResolutionDate`, independent of any transition.
 */
export default function ProblemDetailPage(): JSX.Element {
  const id = useNormalizedIdParam("id");
  const navigate = useNavTransition();
  // Prefer the list URL the row link captured (if any) so "back" returns to
  // the exact view the engineer came from, falling back to the bare tab path
  // for a bookmarked or directly-linked problem.
  const backState = useLocation().state as { from?: string } | undefined;
  const backTarget = backState?.from ?? OPERATIONS_PROBLEMS_PATH;
  const { data, isLoading, isError } = useGetProblem(id);
  const { showError } = useErrorBanner();
  const patchProblem = usePatchProblem();
  const [editOpen, setEditOpen] = useState(false);
  // Set only while the `fix` transition's optional-notes dialog is open;
  // `null` otherwise. Every other transition dispatches directly with no
  // intermediate dialog.
  const [fixNotesOpen, setFixNotesOpen] = useState(false);

  const recordView = useRecordRecentView();
  useEffect(() => {
    if (!data?.id) return;
    recordView({
      kind: "problem",
      id: data.id,
      title:
        [data.number, data.subject].filter((s): s is string => !!s?.trim()).join(" · ") ||
        "(no subject)",
      subtitle: data.assignedTo?.name,
      href: `/operations/problems/${data.id}`,
    });
  }, [data, recordView]);

  /**
   * Dispatch a transition from `ProblemActionBar`. `fix` opens the optional
   * notes dialog first; every other transition PATCHes directly, same split
   * of responsibility as `IncidentActionBar` +
   * `CsmIncidentDetailPage.onIncidentAction`.
   */
  const onProblemAction = useCallback(
    (transition: string) => {
      if (!id) return;
      if (transition === "fix") {
        setFixNotesOpen(true);
        return;
      }
      patchProblem.mutate(
        { id, patch: { transition } },
        {
          onError: (err) => {
            const msg =
              err instanceof BackendApiError && err.status < 500 && err.message
                ? err.message
                : "Could not update the problem's state. Please try again.";
            showError(msg, err);
          },
        },
      );
    },
    [id, patchProblem, showError],
  );

  const onFixNotesSubmit = useCallback(
    (fields: { causeNotes: string; fixNotes: string }) => {
      if (!id) return;
      const patch: BeUpdateProblemPayload = { transition: "fix" };
      if (fields.causeNotes) patch.causeNotes = fields.causeNotes;
      if (fields.fixNotes) patch.fixNotes = fields.fixNotes;
      patchProblem.mutate(
        { id, patch },
        {
          onSuccess: () => setFixNotesOpen(false),
          onError: (err) => {
            const msg =
              err instanceof BackendApiError && err.status < 500 && err.message
                ? err.message
                : "Could not update the problem's state. Please try again.";
            showError(msg, err);
          },
        },
      );
    },
    [id, patchProblem, showError],
  );

  const back = (): void => {
    navigate(backTarget);
  };

  const BackButton = (
    <Button
      variant="text"
      size="small"
      startIcon={<ArrowLeft size={16} />}
      onClick={back}
      sx={{ alignSelf: "flex-start" }}
    >
      Back
    </Button>
  );

  if (isLoading) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
        <Skeleton variant="rounded" height={32} width={240} />
        <Skeleton variant="rounded" height={260} />
      </Box>
    );
  }

  if (isError) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        {BackButton}
        <Typography variant="body1" color="error">
          Could not load problem {id}.
        </Typography>
      </Box>
    );
  }

  if (!data) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        {BackButton}
        <Typography variant="h5">Problem not found</Typography>
        <Typography variant="body2" color="text.secondary">
          No problem with id <code>{id}</code>.
        </Typography>
      </Box>
    );
  }

  const problem = data;
  const linkedIncidents = problem.linkedIncidents ?? [];
  const hasLinks = !!(
    problem.originCase ||
    problem.primaryIncident ||
    linkedIncidents.length > 0 ||
    problem.linkedChangeRequest
  );
  const hasResolution = !!(
    problem.resolutionCode ||
    problem.causeNotes ||
    problem.fixNotes ||
    problem.workaround ||
    problem.resolvedOn ||
    problem.resolvedBy
  );

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2.5 }}>
      {BackButton}

      <Box
        sx={{
          display: "flex",
          gap: 2,
          alignItems: "flex-start",
          flexWrap: { xs: "wrap", md: "nowrap" },
          justifyContent: "space-between",
        }}
      >
        <Box
          sx={{
            display: "flex",
            flexDirection: "column",
            gap: 1,
            flex: 1,
            minWidth: 0,
          }}
        >
          <Typography
            variant="h6"
            sx={{
              fontFamily: "monospace",
              fontWeight: 700,
              letterSpacing: 0.2,
              lineHeight: 1.2,
            }}
          >
            {problem.number || problem.id}
          </Typography>
          <Box sx={{ display: "flex", alignItems: "center", gap: 1, flexWrap: "wrap" }}>
            {problem.state && (
              <Chip
                size="small"
                color={problemStateColor(problem.state)}
                label={problemStateLabel(problem.state)}
              />
            )}
          </Box>
          <Typography variant="h5">{problem.subject || "Problem"}</Typography>
        </Box>
        <Box sx={{ flexShrink: 0, alignSelf: { xs: "stretch", md: "flex-start" } }}>
          <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
            <ProblemActionBar
              problem={problem}
              isPending={patchProblem.isPending}
              onAction={onProblemAction}
            />
            <Button
              variant="outlined"
              size="small"
              startIcon={<Pencil size={14} />}
              onClick={() => setEditOpen(true)}
            >
              Edit
            </Button>
          </Box>
        </Box>
      </Box>

      <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2 }}>
        <Typography variant="subtitle2">Overview</Typography>
        <Box
          sx={{
            display: "grid",
            gap: 2,
            gridTemplateColumns: {
              xs: "1fr",
              sm: "repeat(2, minmax(0, 1fr))",
              md: "repeat(3, minmax(0, 1fr))",
            },
          }}
        >
          <MetaCell label="Priority">
            <Typography variant="body2">{problem.priority || "—"}</Typography>
          </MetaCell>
          <MetaCell label="Category">
            <Typography variant="body2">{problem.category || "—"}</Typography>
          </MetaCell>
          <MetaCell label="Subcategory">
            <Typography variant="body2">{problem.subcategory || "—"}</Typography>
          </MetaCell>
          <MetaCell label="Assigned to"><RefText value={problem.assignedTo} /></MetaCell>
          <MetaCell label="Opened">
            <Typography variant="body2">{formatDateTime(problem.openedOn)}</Typography>
          </MetaCell>
          <MetaCell label="Closed">
            <Typography variant="body2">{formatDateTime(problem.closedOn)}</Typography>
          </MetaCell>
        </Box>
      </Card>

      {hasLinks && (
        <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2 }}>
          <Typography variant="subtitle2">Linked records</Typography>
          <Box
            sx={{
              display: "grid",
              gap: 2,
              gridTemplateColumns: {
                xs: "1fr",
                sm: "repeat(2, minmax(0, 1fr))",
                md: "repeat(3, minmax(0, 1fr))",
              },
            }}
          >
            <MetaCell label="Origin record">
              {/* "originCase" can be an Incident's number in real data despite
                  the field name — no route assumption is made for it. */}
              <ProblemRefItem value={problem.originCase} onNavigate={navigate} />
            </MetaCell>
            <MetaCell label="Primary incident">
              <ProblemRefItem
                value={problem.primaryIncident}
                routeBase="/operations/incidents"
                onNavigate={navigate}
              />
            </MetaCell>
            <MetaCell label="Change request">
              <ProblemRefItem
                value={problem.linkedChangeRequest}
                routeBase="/operations/change-requests"
                onNavigate={navigate}
              />
            </MetaCell>
            {linkedIncidents.length > 0 && (
              <MetaCell label={`Linked incidents (${linkedIncidents.length})`}>
                <Box sx={{ display: "flex", flexWrap: "wrap", gap: 0.75 }}>
                  {linkedIncidents.map((incident) => (
                    <ProblemRefItem
                      key={incident.id}
                      value={incident}
                      routeBase="/operations/incidents"
                      onNavigate={navigate}
                    />
                  ))}
                </Box>
              </MetaCell>
            )}
          </Box>
        </Card>
      )}

      {hasResolution && (
        <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2 }}>
          <Typography variant="subtitle2">Resolution</Typography>
          <Box
            sx={{
              display: "grid",
              gap: 2,
              gridTemplateColumns: {
                xs: "1fr",
                sm: "repeat(2, minmax(0, 1fr))",
                md: "repeat(3, minmax(0, 1fr))",
              },
            }}
          >
            <MetaCell label="Resolution code">
              <Typography variant="body2">{problem.resolutionCode || "—"}</Typography>
            </MetaCell>
            <MetaCell label="Resolved by"><RefText value={problem.resolvedBy} /></MetaCell>
            <MetaCell label="Resolved">
              <Typography variant="body2">{formatDateTime(problem.resolvedOn)}</Typography>
            </MetaCell>
          </Box>
          {problem.causeNotes && (
            <Box sx={{ display: "flex", flexDirection: "column", gap: 0.5 }}>
              <Typography variant="body2" color="text.secondary">
                Cause notes
              </Typography>
              <Typography variant="body2" sx={{ whiteSpace: "pre-wrap" }}>
                {problem.causeNotes}
              </Typography>
            </Box>
          )}
          {problem.fixNotes && (
            <Box sx={{ display: "flex", flexDirection: "column", gap: 0.5 }}>
              <Typography variant="body2" color="text.secondary">
                Fix notes
              </Typography>
              <Typography variant="body2" sx={{ whiteSpace: "pre-wrap" }}>
                {problem.fixNotes}
              </Typography>
            </Box>
          )}
          {problem.workaround && (
            <Box sx={{ display: "flex", flexDirection: "column", gap: 0.5 }}>
              <Typography variant="body2" color="text.secondary">
                Workaround
              </Typography>
              <Typography variant="body2" sx={{ whiteSpace: "pre-wrap" }}>
                {problem.workaround}
              </Typography>
            </Box>
          )}
        </Card>
      )}

      {editOpen && (
        <EditProblemDialog
          problem={problem}
          isSaving={patchProblem.isPending}
          saveError={
            patchProblem.isError
              ? (patchProblem.error instanceof BackendApiError && patchProblem.error.message
                  ? patchProblem.error.message
                  : "Could not update the problem. Please try again.")
              : null
          }
          onClose={() => {
            if (!patchProblem.isPending) setEditOpen(false);
          }}
          onSave={(patch) =>
            patchProblem.mutate(
              { id: problem.id, patch },
              {
                onSuccess: () => setEditOpen(false),
                onError: (err) => {
                  const msg =
                    err instanceof BackendApiError && err.status < 500 && err.message
                      ? err.message
                      : "Could not update the problem. Please try again.";
                  showError(msg, err);
                },
              },
            )
          }
        />
      )}

      {fixNotesOpen && (
        <ProblemFixNotesDialog
          isSubmitting={patchProblem.isPending}
          onClose={() => {
            if (!patchProblem.isPending) setFixNotesOpen(false);
          }}
          onConfirm={onFixNotesSubmit}
        />
      )}
    </Box>
  );
}
