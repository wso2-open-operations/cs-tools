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
import { ArrowLeft, Pencil } from "@wso2/oxygen-ui-icons-react";
import { type JSX, type ReactNode, useState } from "react";
import { useParams } from "react-router";
import { formatBackendTimestampForDisplay } from "@utils/dateTime";
import { BackendApiError } from "@api/backend/client";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import { useGetIncident } from "@features/csm-operations/api/useGetIncident";
import { usePatchIncident } from "@features/csm-operations/api/usePatchIncident";
import EditIncidentDialog from "@features/csm-operations/components/EditIncidentDialog";
import EntityRefLink from "@features/csm-operations/components/EntityRefLink";
import {
  incidentPriorityColor,
  incidentPriorityLabel,
  incidentStateColor,
  incidentStateLabel,
} from "@features/csm-operations/utils/incidents";
import type { BeEntityRef, BeIncidentDetail, BeUpdateIncidentPayload } from "@api/backend/types";
import { useNavTransition } from "@hooks/useNavTransition";

const OPERATIONS_INCIDENTS_PATH = "/operations?tab=incidents";

/**
 * Two confirmed-live upstream limitations of `PATCH /incidents/{id}`
 * (entity-service/ServiceNow, not this BFF or the FE) — a third,
 * `state: RESOLVED`/`CLOSED` 500ing without a resolution, was fixed by
 * having `EditIncidentDialog` collect `resolutionCode`/`resolutionNotes`
 * (write-only fields, no read-side model — see `BeUpdateIncidentPayload`)
 * once the target state is one of those two:
 *  - `watchList` 404s ("The requested resource was not found!") for *any*
 *    id — confirmed with both an anonymous service account and a real, named
 *    person, so it isn't a bad-id problem on our side.
 *  - `additionalComments` (and, defensively, `workNotes` — same ServiceNow
 *    journal-field shape, not independently confirmed) is the dangerous one:
 *    the PATCH returns 200, but the response's own echoed value comes back
 *    `null` even though we just set it — a silent no-op dressed as success.
 * `watchList` already surfaces as a real error (see `onError` below) — this
 * is correct, if unfortunate, behavior. `checkSilentlyDroppedNotes` exists so
 * the notes case doesn't: it catches a 200 that didn't actually persist what
 * it claims to and treats it like the failure it is, rather than closing the
 * dialog on a false positive.
 */
function checkSilentlyDroppedNotes(patch: BeUpdateIncidentPayload, saved: BeIncidentDetail): string[] {
  const dropped: string[] = [];
  if ("additionalComments" in patch && (saved.additionalComments ?? null) !== patch.additionalComments) {
    dropped.push("Additional comments");
  }
  if ("workNotes" in patch && (saved.workNotes ?? null) !== patch.workNotes) {
    dropped.push("Internal work note");
  }
  return dropped;
}

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
 * Detail for a single incident (`GET /incidents/{id}`), with an Edit dialog
 * (`PATCH /incidents/{id}`) mirroring the change-request detail page's
 * pattern.
 */
export default function CsmIncidentDetailPage(): JSX.Element {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavTransition();
  const { data, isLoading, isError } = useGetIncident(id);
  const { showError } = useErrorBanner();
  const patchIncident = usePatchIncident();
  const [editOpen, setEditOpen] = useState(false);

  const back = (): void => {
    navigate(OPERATIONS_INCIDENTS_PATH);
  };

  const BackButton = (
    <Button
      variant="text"
      size="small"
      startIcon={<ArrowLeft size={16} />}
      onClick={back}
      sx={{ alignSelf: "flex-start" }}
    >
      Back to incidents
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
          Could not load incident {id}.
        </Typography>
      </Box>
    );
  }

  if (!data) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        {BackButton}
        <Typography variant="h5">Incident not found</Typography>
        <Typography variant="body2" color="text.secondary">
          No incident with id <code>{id}</code>.
        </Typography>
      </Box>
    );
  }

  const incident = data;
  const hasNotes = !!(incident.additionalComments || incident.workNotes);
  const hasLinks = !!(incident.parent || incident.changeRequest || incident.problem || incident.causedBy);

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2.5 }}>
      {BackButton}

      <Box sx={{ display: "flex", flexDirection: "column", gap: 1, minWidth: 0 }}>
        <Box sx={{ display: "flex", alignItems: "center", gap: 1.5, flexWrap: "wrap" }}>
          <Typography variant="h5">{incident.subject || incident.number || "Incident"}</Typography>
          {incident.state && (
            <Chip
              size="small"
              color={incidentStateColor(incident.state)}
              label={incidentStateLabel(incident.state)}
            />
          )}
          {incident.priority && (
            <Chip
              size="small"
              variant="outlined"
              color={incidentPriorityColor(incident.priority)}
              label={incidentPriorityLabel(incident.priority)}
            />
          )}
          <Button
            variant="outlined"
            size="small"
            startIcon={<Pencil size={14} />}
            onClick={() => setEditOpen(true)}
            sx={{ ml: "auto", flexShrink: 0 }}
          >
            Edit
          </Button>
        </Box>
        <Typography variant="body2" color="text.secondary" sx={{ fontFamily: "monospace" }}>
          {incident.number || incident.id}
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
          {(
            [
              { label: "Caller", render: () => <RefText value={incident.caller} /> },
              { label: "Category", render: () => <Typography variant="body2">{incident.category || "—"}</Typography> },
              { label: "Subcategory", render: () => <Typography variant="body2">{incident.subcategory || "—"}</Typography> },
              { label: "Contact type", render: () => <Typography variant="body2">{incident.contactType || "—"}</Typography> },
              { label: "Impact", render: () => <Typography variant="body2">{incident.impact || "—"}</Typography> },
              { label: "Urgency", render: () => <Typography variant="body2">{incident.urgency || "—"}</Typography> },
              { label: "Service", render: () => <RefText value={incident.service} /> },
              { label: "Service offering", render: () => <RefText value={incident.serviceOffering} /> },
              { label: "Configuration item", render: () => <RefText value={incident.configurationItem} /> },
              { label: "Assignment group", render: () => <RefText value={incident.assignmentGroup} /> },
              { label: "Assigned to", render: () => <RefText value={incident.assignedTo} /> },
              { label: "Opened", render: () => <Typography variant="body2">{formatDateTime(incident.openedOn)}</Typography> },
              { label: "Created", render: () => <Typography variant="body2">{formatDateTime(incident.createdOn)}</Typography> },
              { label: "Created by", render: () => <Typography variant="body2">{incident.createdBy || "—"}</Typography> },
              { label: "Last updated", render: () => <Typography variant="body2">{formatDateTime(incident.updatedOn)}</Typography> },
            ] satisfies Array<{ label: string; render: () => JSX.Element }>
          ).map((field) => (
            <MetaCell key={field.label} label={field.label}>
              {field.render()}
            </MetaCell>
          ))}
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
                md: "repeat(4, minmax(0, 1fr))",
              },
            }}
          >
            <MetaCell label="Parent incident">
              <EntityRefLink value={incident.parent} routeBase="/operations/incidents" />
            </MetaCell>
            <MetaCell label="Change request">
              <EntityRefLink value={incident.changeRequest} routeBase="/operations/change-requests" />
            </MetaCell>
            <MetaCell label="Problem">
              <EntityRefLink value={incident.problem} routeBase="/operations/problems" />
            </MetaCell>
            {/* "Caused by" has no confirmed target record type (could be a
                change request, a problem, or something else) — same caveat
                as Problem.originCase — so it's left as plain text rather
                than guessing a route. */}
            <MetaCell label="Caused by"><RefText value={incident.causedBy} /></MetaCell>
          </Box>
        </Card>
      )}

      {hasNotes && (
        <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2.5 }}>
          <Typography variant="subtitle2">Comments &amp; notes</Typography>
          {incident.additionalComments && (
            <Box sx={{ display: "flex", flexDirection: "column", gap: 0.5 }}>
              <Typography variant="body2" color="text.secondary">
                Additional comments (customer-visible)
              </Typography>
              <Typography variant="body2" sx={{ whiteSpace: "pre-wrap" }}>
                {incident.additionalComments}
              </Typography>
            </Box>
          )}
          {incident.workNotes && (
            <Box sx={{ display: "flex", flexDirection: "column", gap: 0.5 }}>
              <Typography variant="body2" color="text.secondary">
                Internal work notes
              </Typography>
              <Typography variant="body2" sx={{ whiteSpace: "pre-wrap" }}>
                {incident.workNotes}
              </Typography>
            </Box>
          )}
        </Card>
      )}

      {incident.watchList && incident.watchList.length > 0 && (
        <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 1.5 }}>
          <Typography variant="subtitle2">Watch list</Typography>
          <Box sx={{ display: "flex", flexWrap: "wrap", gap: 1 }}>
            {incident.watchList.map((w) => (
              <Chip key={w.id} size="small" variant="outlined" label={w.name || w.email} />
            ))}
          </Box>
        </Card>
      )}

      {editOpen && (
        <EditIncidentDialog
          incident={incident}
          isSaving={patchIncident.isPending}
          onClose={() => {
            if (!patchIncident.isPending) setEditOpen(false);
          }}
          onSave={(patch) =>
            patchIncident.mutate(
              { id: incident.id as string, patch },
              {
                onSuccess: (data) => {
                  const droppedFields = checkSilentlyDroppedNotes(patch, data.incident);
                  if (droppedFields.length > 0) {
                    showError(
                      `${droppedFields.join(" and ")} didn't save — the request was accepted but that ` +
                        "change wasn't actually persisted. Any other fields in this edit were saved; please retry the note separately.",
                    );
                    return;
                  }
                  setEditOpen(false);
                },
                onError: (err) => {
                  // Real validation messages (e.g. an invalid UUID in one of the
                  // linking fields) are worth surfacing, same as CreateIncidentPage.
                  const msg =
                    err instanceof BackendApiError && err.status < 500 && err.message
                      ? err.message
                      : "Could not update the incident. Please try again.";
                  showError(msg, err);
                },
              },
            )
          }
        />
      )}
    </Box>
  );
}
