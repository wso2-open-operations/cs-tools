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

import { Box, Button, Chip, IconButton, Tooltip, Typography } from "@wso2/oxygen-ui";
import { Eye } from "@wso2/oxygen-ui-icons-react";
import { Fragment, useState, type JSX } from "react";
import type { BeCallRequestView } from "@api/backend/types";
import {
  CALL_REQUEST_AGENT_ACTIONS,
  type CallRequestAgentAction,
  callRequestStateColor,
  callRequestStateLabel,
  resolveCallRequestStateKey,
} from "@features/csm-cases/utils/callRequestState";
import RelativeTime from "@components/RelativeTime";
import { formatBackendTimestampForDisplay } from "@utils/dateTime";
import CallRequestDetailModal from "@features/csm-cases/components/CallRequestDetailModal";

// ---------------------------------------------------------------------------
// Helper
// ---------------------------------------------------------------------------

// Backend returns these times as UTC wall-clock (unzoned SN format). Convert to
// the user's timezone for display; the timezone name makes the value explicit.
function formatPreferredTimes(times: string[] | undefined): string {
  if (!times || times.length === 0) return "—";
  return times
    .map(
      (t) =>
        formatBackendTimestampForDisplay(t, {
          year: "numeric",
          month: "short",
          day: "numeric",
          hour: "numeric",
          minute: "2-digit",
          timeZoneName: "short",
        }) ?? t,
    )
    .join(", ");
}

const ACTION_LABEL: Record<CallRequestAgentAction, string> = {
  schedule: "Schedule",
  reschedule: "Reschedule",
  reject: "Reject",
  sendNotes: "Send call notes",
  cancel: "Cancel",
};

// Every column is left-aligned for a consistent scan line down the table.
const HEADER_CELLS: string[] = [
  "Request",
  "State",
  "Duration",
  "Preferred / scheduled",
  "Created",
  "Updated",
  "Actions",
];

const GRID =
  "minmax(160px, 1.3fr) minmax(120px, 0.9fr) minmax(80px, 0.6fr) minmax(220px, 1.6fr) minmax(90px, 0.7fr) minmax(90px, 0.7fr) minmax(160px, 1.2fr)";

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export interface CallRequestsTableProps {
  requests: BeCallRequestView[];
  onAction: (action: CallRequestAgentAction, cr: BeCallRequestView) => void;
  /** True when the parent case is closed — existing call requests stay visible
   * but can no longer be updated (scheduled, rejected, etc.). */
  isClosed?: boolean;
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

export function CallRequestsTable({
  requests,
  onAction,
  isClosed,
}: CallRequestsTableProps): JSX.Element {
  const [detailTarget, setDetailTarget] = useState<BeCallRequestView | null>(null);

  return (
    <Fragment>
      <Box
        sx={{
          border: 1,
          borderColor: "divider",
          borderRadius: 1,
          overflow: "hidden",
          display: "grid",
          gridTemplateColumns: GRID,
          columnGap: 2,
        }}
      >
        {/* Header */}
        <Box
          sx={{
            gridColumn: "1 / -1",
            display: "grid",
            gridTemplateColumns: "subgrid",
            columnGap: 2,
            alignItems: "center",
            px: 2,
            py: 1.25,
            bgcolor: "action.hover",
            borderBottom: 1,
            borderColor: "divider",
          }}
        >
          {HEADER_CELLS.map((label) => (
            <Typography
              key={label}
              variant="caption"
              color="text.secondary"
              sx={{ fontWeight: 600, textAlign: "left" }}
            >
              {label}
            </Typography>
          ))}
        </Box>

        {/* Rows */}
        {requests.map((cr) => {
          const stateKey = resolveCallRequestStateKey(cr.state);
          const actions = (stateKey && CALL_REQUEST_AGENT_ACTIONS[stateKey]) ?? [];

          return (
            <Box
              key={cr.id}
              sx={{
                gridColumn: "1 / -1",
                display: "grid",
                gridTemplateColumns: "subgrid",
                columnGap: 2,
                alignItems: "start",
                px: 2,
                py: 1.25,
                borderBottom: 1,
                borderColor: "divider",
                "&:last-of-type": { borderBottom: 0 },
              }}
            >
              {/* Request: number + reason, with notes as a secondary line. */}
              <Box sx={{ minWidth: 0 }}>
                {cr.number && (
                  <Typography
                    variant="caption"
                    color="text.secondary"
                    noWrap
                    title={cr.number}
                    sx={{ fontFamily: "monospace", display: "block" }}
                  >
                    {cr.number}
                  </Typography>
                )}
                {cr.reason && (
                  <Typography variant="body2" noWrap title={cr.reason}>
                    {cr.reason}
                  </Typography>
                )}
                {cr.notes && (
                  <Typography
                    variant="caption"
                    color="text.secondary"
                    noWrap
                    title={cr.notes}
                    sx={{ display: "block" }}
                  >
                    Notes: {cr.notes}
                  </Typography>
                )}
              </Box>

              {/* State: chip + cancellation reason, when present. */}
              <Box sx={{ justifySelf: "start", minWidth: 0 }}>
                <Chip
                  size="small"
                  variant="outlined"
                  label={callRequestStateLabel(cr.state)}
                  color={callRequestStateColor(cr.state)}
                />
                {cr.cancellationReason && (
                  <Typography
                    variant="caption"
                    color="text.secondary"
                    noWrap
                    title={cr.cancellationReason}
                    sx={{ display: "block", mt: 0.5 }}
                  >
                    {cr.cancellationReason}
                  </Typography>
                )}
              </Box>

              <Typography variant="body2">
                {cr.durationMin ? `${cr.durationMin} min` : "—"}
              </Typography>

              {/* Preferred / scheduled times, plus meeting link when present. */}
              <Box sx={{ minWidth: 0 }}>
                <Typography variant="body2" sx={{ whiteSpace: "pre-line" }}>
                  {formatPreferredTimes(cr.preferredTimes)}
                </Typography>
                {cr.scheduleTime && (
                  <Typography variant="caption" color="text.secondary" sx={{ display: "block" }}>
                    Scheduled: {formatPreferredTimes([cr.scheduleTime])}
                  </Typography>
                )}
                {cr.meetingLink && (
                  <Typography
                    variant="caption"
                    component="a"
                    href={cr.meetingLink}
                    target="_blank"
                    rel="noopener noreferrer"
                    sx={{ color: "primary.main", display: "block" }}
                  >
                    Join meeting
                  </Typography>
                )}
              </Box>

              <Typography variant="caption" color="text.secondary" noWrap>
                {cr.createdOn && <RelativeTime iso={cr.createdOn} />}
              </Typography>
              <Typography variant="caption" color="text.secondary" noWrap>
                {cr.updatedOn && <RelativeTime iso={cr.updatedOn} />}
              </Typography>

              {/* Actions: view-detail eye icon plus any agent actions, with
                  the assignee shown alongside once scheduled. */}
              <Box sx={{ minWidth: 0 }}>
                <Box sx={{ display: "flex", gap: 1, alignItems: "center", flexWrap: "wrap" }}>
                  <IconButton
                    size="small"
                    aria-label="View details"
                    data-testid={`call-request-view-${cr.id}`}
                    onClick={() => setDetailTarget(cr)}
                  >
                    <Eye size={16} />
                  </IconButton>
                  {actions.map((action, i) => (
                    <Tooltip
                      key={action}
                      title={isClosed ? "This case is closed — it's read-only." : ""}
                    >
                      <span>
                        <Button
                          size="small"
                          variant={i === 0 && action !== "cancel" ? "contained" : "outlined"}
                          color={action === "reject" || action === "cancel" ? "error" : "primary"}
                          onClick={() => onAction(action, cr)}
                          disabled={isClosed}
                          sx={{ textTransform: "none" }}
                        >
                          {ACTION_LABEL[action]}
                        </Button>
                      </span>
                    </Tooltip>
                  ))}
                </Box>
                {cr.assignee && (
                  <Typography
                    variant="caption"
                    color="text.secondary"
                    noWrap
                    title={cr.assignee}
                    sx={{ display: "block", mt: 0.5 }}
                  >
                    Assignee: {cr.assignee}
                  </Typography>
                )}
              </Box>
            </Box>
          );
        })}
      </Box>

      {detailTarget && (
        <CallRequestDetailModal
          callRequest={detailTarget}
          onClose={() => setDetailTarget(null)}
        />
      )}
    </Fragment>
  );
}
