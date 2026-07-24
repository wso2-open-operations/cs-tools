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

import type { JSX } from "react";
import { Box, Button, Chip, Dialog, DialogActions, DialogContent, DialogTitle, Typography } from "@wso2/oxygen-ui";
import type { BeCallRequestView } from "@api/backend/types";
import RelativeTime from "@components/RelativeTime";
import { callRequestStateColor, callRequestStateLabel } from "@features/csm-cases/utils/callRequestState";
import { formatBackendTimestampForDisplay } from "@utils/dateTime";

interface CallRequestDetailModalProps {
  callRequest: BeCallRequestView;
  onClose: () => void;
}

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

function Field({ label, value }: { label: string; value: JSX.Element | string }): JSX.Element {
  return (
    <Box sx={{ minWidth: 0, display: "flex", flexDirection: "column", gap: 0.5 }}>
      <Typography variant="caption" color="text.secondary">
        {label}
      </Typography>
      {typeof value === "string" ? (
        <Typography variant="body2" sx={{ whiteSpace: "pre-line", wordBreak: "break-word" }}>
          {value}
        </Typography>
      ) : (
        value
      )}
    </Box>
  );
}

/**
 * Read-only detail view of a single call request, opened via the eye icon in
 * `CallRequestsTable`'s Request column — the row itself truncates the reason
 * and notes, this shows them in full.
 */
export default function CallRequestDetailModal({
  callRequest: cr,
  onClose,
}: CallRequestDetailModalProps): JSX.Element {
  return (
    <Dialog open onClose={onClose} maxWidth="sm" fullWidth>
      <DialogTitle>Call request {cr.number ? `· ${cr.number}` : ""}</DialogTitle>
      <DialogContent dividers>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
          {cr.reason && <Field label="Reason" value={cr.reason} />}

          <Field
            label="State"
            value={
              <Box sx={{ mt: 0.5 }}>
                <Chip
                  size="small"
                  variant="outlined"
                  label={callRequestStateLabel(cr.state)}
                  color={callRequestStateColor(cr.state)}
                />
              </Box>
            }
          />

          <Box sx={{ display: "grid", gridTemplateColumns: "1fr 1fr 1fr", gap: 2 }}>
            <Field label="Duration" value={cr.durationMin ? `${cr.durationMin} min` : "—"} />
            <Field
              label="Created"
              value={
                cr.createdOn ? (
                  <Typography variant="body2">
                    <RelativeTime iso={cr.createdOn} />
                  </Typography>
                ) : (
                  "—"
                )
              }
            />
            <Field
              label="Updated"
              value={
                cr.updatedOn ? (
                  <Typography variant="body2">
                    <RelativeTime iso={cr.updatedOn} />
                  </Typography>
                ) : (
                  "—"
                )
              }
            />
          </Box>

          <Box sx={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 2 }}>
            <Field label="Preferred times" value={formatPreferredTimes(cr.preferredTimes)} />
            {cr.scheduleTime && (
              <Field label="Scheduled time" value={formatPreferredTimes([cr.scheduleTime])} />
            )}
          </Box>

          {(cr.actualDurationMin !== undefined || cr.assignee) && (
            <Box sx={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 2 }}>
              {cr.actualDurationMin !== undefined && (
                <Field label="Actual duration" value={`${cr.actualDurationMin} min`} />
              )}
              {cr.assignee && <Field label="Assignee" value={cr.assignee} />}
            </Box>
          )}

          {cr.meetingLink && (
            <Field
              label="Meeting link"
              value={
                <Typography
                  variant="body2"
                  component="a"
                  href={cr.meetingLink}
                  target="_blank"
                  rel="noopener noreferrer"
                  sx={{ color: "primary.main", wordBreak: "break-all" }}
                >
                  {cr.meetingLink}
                </Typography>
              }
            />
          )}

          {cr.cancellationReason && (
            <Field label="Cancellation reason" value={cr.cancellationReason} />
          )}

          {cr.notes && <Field label="Call notes" value={cr.notes} />}
          {cr.plan && <Field label="Follow-up plan" value={cr.plan} />}
          {cr.attendees && <Field label="Attendees" value={cr.attendees} />}
          {cr.actionItems && <Field label="Action items" value={cr.actionItems} />}
        </Box>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>Close</Button>
      </DialogActions>
    </Dialog>
  );
}
