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
  Alert,
  Box,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { useState, type JSX } from "react";
import AsyncUserIdSingleSelect from "@features/csm-cases/components/AsyncUserIdSingleSelect";
import { useGetUserById } from "@features/csm-users/api/useGetUserById";
import { usePatchUser } from "@features/csm-users/api/usePatchUser";
import { INTERNAL_USER_ROLES } from "@features/csm-users/types/csmUsers";
import { useAssignTeamLead } from "@features/csm-admin/api/useAssignTeamLead";

interface ReassignTeamLeadDialogProps {
  teamEntityId: string;
  currentLeadName?: string;
  onClose: () => void;
  onAssigned: () => void;
}

/**
 * Reassign a team's lead. Admin-only (the caller only renders this behind an
 * admin check — see `TeamLeadCard`), but the real enforcement is server-side
 * (`POST /teams/{id}/lead`, restricted to admin). The picker is scoped to
 * internal staff only (`INTERNAL_USER_ROLES`) via the existing, reusable
 * `AsyncUserIdSingleSelect` (search + infinite scroll already built in) —
 * picking someone who could never actually lead a team makes no sense here.
 *
 * This writes the real, paging-connected `team_member.role` column, so the
 * warning banner is not boilerplate: the incident call-escalation ladder
 * reads this exact column to decide who gets paged for this team. Once a
 * candidate is chosen, their current phone number (the number that ladder
 * dials) is loaded and shown, editable in the same flow, since a newly
 * assigned lead with no number on file is a silent paging gap otherwise.
 */
export default function ReassignTeamLeadDialog({
  teamEntityId,
  currentLeadName,
  onClose,
  onAssigned,
}: ReassignTeamLeadDialogProps): JSX.Element {
  const [selectedUserId, setSelectedUserId] = useState("");
  const [phone, setPhone] = useState("");
  const [phoneTouchedForUserId, setPhoneTouchedForUserId] = useState("");

  const selectedUser = useGetUserById(selectedUserId || undefined);
  const assignLead = useAssignTeamLead(teamEntityId);
  const patchUser = usePatchUser();

  // Pre-fill the phone field from the selected candidate's own profile, but
  // only the first time a given id is selected -- re-seeding on every
  // refetch would stomp whatever the admin is actively typing. Adjusted
  // during render (React's own sanctioned "derive state from a prop change"
  // pattern, https://react.dev/learn/you-might-not-need-an-effect) rather
  // than in a useEffect -- this condition is only ever true once per
  // selection and then immediately false again, so it converges in the same
  // render pass without an extra commit.
  if (
    selectedUserId &&
    selectedUserId !== phoneTouchedForUserId &&
    selectedUser.data
  ) {
    setPhone(selectedUser.data.phone ?? "");
    setPhoneTouchedForUserId(selectedUserId);
  }

  const isSaving = assignLead.isPending || patchUser.isPending;
  const saveError = assignLead.isError
    ? assignLead.error.message
    : patchUser.isError
      ? patchUser.error.message
      : null;

  const handleSave = (): void => {
    if (!selectedUserId) return;
    assignLead.mutate(selectedUserId, {
      onSuccess: () => {
        const trimmedPhone = phone.trim();
        if (!trimmedPhone) {
          onAssigned();
          return;
        }
        patchUser.mutate(
          { id: selectedUserId, patch: { phone: trimmedPhone } },
          { onSuccess: () => onAssigned() },
        );
      },
    });
  };

  return (
    <Dialog open onClose={isSaving ? undefined : onClose} maxWidth="xs" fullWidth>
      <DialogTitle>Change team lead</DialogTitle>
      <DialogContent dividers>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2, pt: 0.5 }}>
          <Alert severity="warning">
            Reassigning the lead also changes who entity-service's incident
            call-escalation ladder pages for this team — this takes effect
            immediately.
          </Alert>

          {saveError && <Alert severity="error">{saveError}</Alert>}

          {currentLeadName && (
            <Typography variant="caption" color="text.secondary">
              Current lead: {currentLeadName}
            </Typography>
          )}

          <AsyncUserIdSingleSelect
            id="reassign-team-lead-user"
            label="New team lead"
            placeholder="Search internal users…"
            value={selectedUserId}
            onChange={(next) => {
              setSelectedUserId(next);
              if (!next) {
                setPhone("");
                setPhoneTouchedForUserId("");
              }
            }}
            roleIds={INTERNAL_USER_ROLES}
            required
          />

          {selectedUserId && (
            <TextField
              id="reassign-team-lead-phone"
              label="Phone number"
              placeholder="+1 555 555 0123"
              value={phone}
              onChange={(e) => setPhone(e.target.value)}
              disabled={isSaving}
              fullWidth
              helperText="Dialed by the incident call-escalation ladder. Leave blank to leave it unchanged."
            />
          )}
        </Box>
      </DialogContent>
      <DialogActions>
        <Button color="inherit" onClick={onClose} disabled={isSaving}>
          Cancel
        </Button>
        <Button
          variant="contained"
          color="warning"
          onClick={handleSave}
          disabled={isSaving || !selectedUserId}
        >
          {isSaving ? "Saving…" : "Reassign lead"}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
