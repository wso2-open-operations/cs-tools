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
  Box,
  Button,
  Chip,
  Typography,
} from "@wso2/oxygen-ui";
import { type JSX } from "react";
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

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export interface CallRequestRowProps {
  cr: BeCallRequestView;
  onAction: (action: CallRequestAgentAction, cr: BeCallRequestView) => void;
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

export function CallRequestRow({ cr, onAction }: CallRequestRowProps): JSX.Element {
  const stateKey = resolveCallRequestStateKey(cr.state);
  const actions = (stateKey && CALL_REQUEST_AGENT_ACTIONS[stateKey]) ?? [];

  return (
    <Box
      sx={{
        display: "flex",
        flexDirection: "column",
        gap: 0.5,
        py: 1.25,
        borderTop: 1,
        borderColor: "divider",
        "&:first-of-type": { borderTop: 0, pt: 0 },
      }}
    >
      <Box
        sx={{
          display: "flex",
          alignItems: "flex-start",
          justifyContent: "space-between",
          gap: 1,
          flexWrap: "wrap",
        }}
      >
        <Box sx={{ display: "flex", alignItems: "center", gap: 1, flexWrap: "wrap" }}>
          {cr.number && (
            <Typography
              variant="caption"
              sx={{ fontFamily: "monospace", color: "text.secondary" }}
            >
              {cr.number}
            </Typography>
          )}
          <Chip
            size="small"
            label={callRequestStateLabel(cr.state)}
            color={callRequestStateColor(cr.state)}
          />
        </Box>
        {actions.length > 0 && (
          <Box sx={{ display: "flex", gap: 1, flexShrink: 0, flexWrap: "wrap" }}>
            {actions.map((action, i) => (
              <Button
                key={action}
                size="small"
                variant={i === 0 && action !== "cancel" ? "contained" : "outlined"}
                color={action === "reject" || action === "cancel" ? "error" : "primary"}
                onClick={() => onAction(action, cr)}
                sx={{ textTransform: "none" }}
              >
                {ACTION_LABEL[action]}
              </Button>
            ))}
          </Box>
        )}
      </Box>

      {cr.reason && (
        <Typography variant="body2">{cr.reason}</Typography>
      )}

      <Box
        sx={{
          display: "grid",
          gridTemplateColumns: "repeat(auto-fill, minmax(180px, 1fr))",
          gap: 0.75,
          mt: 0.5,
        }}
      >
        <Box>
          <Typography variant="caption" color="text.secondary" display="block">
            Duration
          </Typography>
          <Typography variant="body2">
            {cr.durationMin ? `${cr.durationMin} min` : "—"}
          </Typography>
        </Box>
        <Box>
          <Typography variant="caption" color="text.secondary" display="block">
            Preferred times
          </Typography>
          <Typography variant="body2" sx={{ whiteSpace: "pre-line" }}>
            {formatPreferredTimes(cr.preferredTimes)}
          </Typography>
        </Box>
        {cr.scheduleTime && (
          <Box>
            <Typography variant="caption" color="text.secondary" display="block">
              Scheduled time
            </Typography>
            <Typography variant="body2">
              {formatPreferredTimes([cr.scheduleTime])}
            </Typography>
          </Box>
        )}
        {cr.assignee && (
          <Box>
            <Typography variant="caption" color="text.secondary" display="block">
              Assignee
            </Typography>
            <Typography variant="body2">{cr.assignee}</Typography>
          </Box>
        )}
        {cr.meetingLink && (
          <Box>
            <Typography variant="caption" color="text.secondary" display="block">
              Meeting link
            </Typography>
            <Typography
              variant="body2"
              component="a"
              href={cr.meetingLink}
              target="_blank"
              rel="noopener noreferrer"
              sx={{ color: "primary.main", wordBreak: "break-all" }}
            >
              Join meeting
            </Typography>
          </Box>
        )}
        {cr.cancellationReason && (
          <Box>
            <Typography variant="caption" color="text.secondary" display="block">
              Cancellation reason
            </Typography>
            <Typography variant="body2">{cr.cancellationReason}</Typography>
          </Box>
        )}
        {cr.createdOn && (
          <Box>
            <Typography variant="caption" color="text.secondary" display="block">
              Created
            </Typography>
            <Typography variant="body2">
              <RelativeTime iso={cr.createdOn} />
            </Typography>
          </Box>
        )}
        {cr.updatedOn && (
          <Box>
            <Typography variant="caption" color="text.secondary" display="block">
              Last updated
            </Typography>
            <Typography variant="body2">
              <RelativeTime iso={cr.updatedOn} />
            </Typography>
          </Box>
        )}
      </Box>

      {cr.notes && (
        <Box sx={{ mt: 0.5 }}>
          <Typography variant="caption" color="text.secondary" display="block">
            Call notes
          </Typography>
          <Typography variant="body2" sx={{ whiteSpace: "pre-line" }}>
            {cr.notes}
          </Typography>
        </Box>
      )}
    </Box>
  );
}
