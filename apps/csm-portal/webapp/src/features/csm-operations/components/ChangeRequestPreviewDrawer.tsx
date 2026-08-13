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

import { Box, Button, Chip, Divider, Drawer, IconButton, Typography } from "@wso2/oxygen-ui";
import { X } from "@wso2/oxygen-ui-icons-react";
import type { JSX, ReactNode } from "react";
import { Link as RouterLink } from "react-router";
import RelativeTime from "@components/RelativeTime";
import type { BeChangeRequestSearchView } from "@api/backend/types";
import {
  changeRequestImpactColor,
  changeRequestImpactLabel,
  changeRequestStateColor,
  changeRequestStateLabel,
} from "@features/csm-operations/utils/changeRequests";

interface ChangeRequestPreviewDrawerProps {
  /** The change request being previewed. `null` keeps the drawer
   * mounted-but-closed, so its close transition can play instead of
   * unmounting mid-animation. */
  changeRequest: BeChangeRequestSearchView | null;
  onClose: () => void;
}

function Field({ label, children }: { label: string; children: ReactNode }): JSX.Element {
  return (
    <Box sx={{ minWidth: 0 }}>
      <Typography variant="caption" color="text.secondary">
        {label}
      </Typography>
      <Typography variant="body2" noWrap>
        {children}
      </Typography>
    </Box>
  );
}

/**
 * Read-only "quick look" for a change request row — renders only fields
 * already present on the `/change-requests/search` response
 * (`BeChangeRequestSearchView`), the same data already on screen in the
 * widget's own table row, no extra fetch. Mirrors `CasePreviewDrawer`'s shell.
 */
export default function ChangeRequestPreviewDrawer({
  changeRequest,
  onClose,
}: ChangeRequestPreviewDrawerProps): JSX.Element {
  return (
    <Drawer
      anchor="right"
      open={!!changeRequest}
      onClose={onClose}
      slotProps={{ paper: { sx: { width: { xs: "100%", sm: 420 } } } }}
    >
      {changeRequest && (
        <Box sx={{ display: "flex", flexDirection: "column", height: "100%" }}>
          <Box sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 1.5 }}>
            <Box sx={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", gap: 1 }}>
              <Box sx={{ minWidth: 0 }}>
                <Typography variant="subtitle2" color="text.secondary" noWrap>
                  {changeRequest.number || "—"}
                </Typography>
                <Typography variant="h6" sx={{ wordBreak: "break-word" }}>
                  {changeRequest.subject || "(no subject)"}
                </Typography>
              </Box>
              <IconButton size="small" onClick={onClose} aria-label="Close preview">
                <X size={18} />
              </IconButton>
            </Box>
            <Box sx={{ display: "flex", gap: 1, flexWrap: "wrap" }}>
              {changeRequest.state && (
                <Chip
                  size="small"
                  variant="outlined"
                  color={changeRequestStateColor(changeRequest.state)}
                  label={changeRequestStateLabel(changeRequest.state)}
                />
              )}
              {changeRequest.impact && (
                <Chip
                  size="small"
                  variant="outlined"
                  color={changeRequestImpactColor(changeRequest.impact)}
                  label={changeRequestImpactLabel(changeRequest.impact)}
                />
              )}
            </Box>
          </Box>

          <Divider />

          <Box sx={{ flex: 1, overflowY: "auto", p: 2.5 }}>
            <Box sx={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 2 }}>
              <Field label="Project">{changeRequest.project?.name || "—"}</Field>
              <Field label="Case">{changeRequest.case?.name || "—"}</Field>
              <Field label="Assigned engineer">
                {changeRequest.assignedEngineer?.name || "Unassigned"}
              </Field>
              <Field label="Assigned team">{changeRequest.assignedTeam?.name || "—"}</Field>
              <Field label="Planned start">
                {changeRequest.plannedStartOn ? (
                  <RelativeTime iso={changeRequest.plannedStartOn} />
                ) : (
                  "—"
                )}
              </Field>
              <Field label="Planned end">
                {changeRequest.plannedEndOn ? (
                  <RelativeTime iso={changeRequest.plannedEndOn} />
                ) : (
                  "—"
                )}
              </Field>
              <Field label="Updated">
                {changeRequest.updatedOn ? <RelativeTime iso={changeRequest.updatedOn} /> : "—"}
              </Field>
            </Box>
          </Box>

          <Divider />

          <Box sx={{ p: 2 }}>
            <Button
              component={RouterLink}
              to={`/operations/change-requests/${changeRequest.id}`}
              variant="contained"
              fullWidth
              onClick={onClose}
              disabled={!changeRequest.id}
            >
              View full details
            </Button>
          </Box>
        </Box>
      )}
    </Drawer>
  );
}
