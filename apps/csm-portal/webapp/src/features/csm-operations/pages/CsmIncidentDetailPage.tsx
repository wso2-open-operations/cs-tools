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
import { ArrowLeft } from "@wso2/oxygen-ui-icons-react";
import { type JSX, type ReactNode } from "react";
import { useParams } from "react-router";
import { formatBackendTimestampForDisplay } from "@utils/dateTime";
import { useGetIncident } from "@features/csm-operations/api/useGetIncident";
import {
  incidentPriorityColor,
  incidentPriorityLabel,
  incidentStateColor,
  incidentStateLabel,
} from "@features/csm-operations/utils/incidents";
import type { BeEntityRef } from "@api/backend/types";
import { useNavTransition } from "@hooks/useNavTransition";

const OPERATIONS_INCIDENTS_PATH = "/operations?tab=incidents";

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
 * Read-only detail for a single incident (`GET /incidents/{id}`). No edit
 * flow — the backend only exposes search/create/get for incidents, no
 * PATCH — so unlike the change-request detail page, there's nothing to
 * open an edit dialog against.
 */
export default function CsmIncidentDetailPage(): JSX.Element {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavTransition();
  const { data, isLoading, isError } = useGetIncident(id);

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
          <MetaCell label="Caller"><RefText value={incident.caller} /></MetaCell>
          <MetaCell label="Category">
            <Typography variant="body2">{incident.category || "—"}</Typography>
          </MetaCell>
          <MetaCell label="Subcategory">
            <Typography variant="body2">{incident.subcategory || "—"}</Typography>
          </MetaCell>
          <MetaCell label="Contact type">
            <Typography variant="body2">{incident.contactType || "—"}</Typography>
          </MetaCell>
          <MetaCell label="Impact">
            <Typography variant="body2">{incident.impact || "—"}</Typography>
          </MetaCell>
          <MetaCell label="Urgency">
            <Typography variant="body2">{incident.urgency || "—"}</Typography>
          </MetaCell>
          <MetaCell label="Service"><RefText value={incident.service} /></MetaCell>
          <MetaCell label="Service offering"><RefText value={incident.serviceOffering} /></MetaCell>
          <MetaCell label="Configuration item"><RefText value={incident.configurationItem} /></MetaCell>
          <MetaCell label="Assignment group"><RefText value={incident.assignmentGroup} /></MetaCell>
          <MetaCell label="Assigned to"><RefText value={incident.assignedTo} /></MetaCell>
          <MetaCell label="Opened">
            <Typography variant="body2">{formatDateTime(incident.openedOn)}</Typography>
          </MetaCell>
          <MetaCell label="Created">
            <Typography variant="body2">{formatDateTime(incident.createdOn)}</Typography>
          </MetaCell>
          <MetaCell label="Created by">
            <Typography variant="body2">{incident.createdBy || "—"}</Typography>
          </MetaCell>
          <MetaCell label="Last updated">
            <Typography variant="body2">{formatDateTime(incident.updatedOn)}</Typography>
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
                md: "repeat(4, minmax(0, 1fr))",
              },
            }}
          >
            <MetaCell label="Parent incident"><RefText value={incident.parent} /></MetaCell>
            <MetaCell label="Change request"><RefText value={incident.changeRequest} /></MetaCell>
            <MetaCell label="Problem"><RefText value={incident.problem} /></MetaCell>
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
    </Box>
  );
}
