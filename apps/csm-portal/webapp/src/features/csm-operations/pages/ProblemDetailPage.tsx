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
import { ArrowLeft, Link as LinkIcon } from "@wso2/oxygen-ui-icons-react";
import { type JSX, type ReactNode } from "react";
import { useParams } from "react-router";
import { formatBackendTimestampForDisplay } from "@utils/dateTime";
import { useGetProblem } from "@features/csm-operations/api/useGetProblem";
import { problemStateColor, problemStateLabel } from "@features/csm-operations/utils/problems";
import type { BeEntityRef, BeProblemRef } from "@api/backend/types";
import { useNavTransition } from "@hooks/useNavTransition";

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
 * Read-only detail for a single problem (`GET /problems/{id}`): its overview
 * fields, linked records, and resolution/fix notes. There is no Edit dialog
 * — problems are SRE-owned and the portal doesn't yet expose a mutation
 * endpoint for them (unlike change requests/incidents).
 */
export default function ProblemDetailPage(): JSX.Element {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavTransition();
  const { data, isLoading, isError } = useGetProblem(id);

  const back = (): void => {
    navigate(OPERATIONS_PROBLEMS_PATH);
  };

  const BackButton = (
    <Button
      variant="text"
      size="small"
      startIcon={<ArrowLeft size={16} />}
      onClick={back}
      sx={{ alignSelf: "flex-start" }}
    >
      Back to problems
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

      <Box sx={{ display: "flex", flexDirection: "column", gap: 1, minWidth: 0 }}>
        <Box sx={{ display: "flex", alignItems: "center", gap: 1.5, flexWrap: "wrap" }}>
          <Typography variant="h5">{problem.subject || problem.number || "Problem"}</Typography>
          {problem.state && (
            <Chip
              size="small"
              color={problemStateColor(problem.state)}
              label={problemStateLabel(problem.state)}
            />
          )}
        </Box>
        <Typography variant="body2" color="text.secondary" sx={{ fontFamily: "monospace" }}>
          {problem.number || problem.id}
        </Typography>
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
    </Box>
  );
}
