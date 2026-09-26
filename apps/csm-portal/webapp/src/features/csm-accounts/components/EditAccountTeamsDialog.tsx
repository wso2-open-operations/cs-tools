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
  Typography,
} from "@wso2/oxygen-ui";
import { useMemo, useState, type JSX } from "react";
import { BE_MAX_PAGE_LIMIT } from "@constants/apiConstants";
import AsyncEntitySelect from "@components/AsyncEntitySelect";
import { useSearchTeams } from "@features/csm-admin/api/useSearchTeams";
import type { AccountTeamsPatch } from "@features/csm-accounts/api/usePatchAccountTeams";
import type { BeTeam } from "@api/backend/types";

interface TeamRef {
  id: string;
  name: string;
}

interface EditAccountTeamsDialogProps {
  currentCreTeam?: TeamRef | null;
  currentSreTeam?: TeamRef | null;
  /** True while the PATCH is in flight; disables the actions/pickers. */
  isSaving: boolean;
  /** User-facing message for the most recent failed save, if any. */
  saveError?: string | null;
  onClose: () => void;
  /** Submit only the changed field(s) (`PATCH /accounts/{id}`). */
  onSave: (patch: AccountTeamsPatch) => void;
}

/**
 * Type-ahead team search, scoped by family via `AsyncEntitySelect`'s
 * `searchExtra` — reused for both the CRE and SRE pickers below rather than
 * two near-identical hooks. `POST /teams/search` is a small, curated
 * catalogue (see `useSearchTeams`'s own docstring), so a full page covers
 * every match; the family filter narrows it to teams actually usable in
 * that slot. `account.creTeamId`/`sreTeamId` reference the team registry id
 * directly (unlike case filters, which match on `creGroupId`/`sreGroupId`),
 * so any team of the right family is eligible regardless of group-id.
 */
function useTeamsSearch(
  query: string,
  enabled: boolean,
  extra?: string,
): { data: BeTeam[] | undefined; isFetching: boolean; isError: boolean } {
  const { data, isFetching, isError } = useSearchTeams(
    {
      ...(query ? { filters: { searchQuery: query } } : {}),
      pagination: { limit: BE_MAX_PAGE_LIMIT },
    },
    enabled,
  );

  const teams = useMemo(() => {
    const all = data?.teams ?? [];
    if (extra === "cre-abt" || extra === "sre-abt") {
      return all.filter((t) => t.family === extra);
    }
    return all;
  }, [data, extra]);

  return { data: teams, isFetching, isError };
}

/**
 * Edit an account's CRE team / SRE team assignment independently — either
 * field may be set or left alone. There is currently no way to clear an
 * assignment back to "no team": the backend treats a nil field as "leave
 * unchanged," not "clear," so this dialog never submits a cleared selection
 * (see `usePatchAccountTeams`'s `AccountTeamsPatch` doc comment). Mirrors
 * `EditProblemDialog`'s shape (a plain controlled dialog; the caller owns
 * the mutation and passes `isSaving`/`saveError` down) rather than driving
 * its own PATCH call.
 */
export default function EditAccountTeamsDialog({
  currentCreTeam,
  currentSreTeam,
  isSaving,
  saveError,
  onClose,
  onSave,
}: EditAccountTeamsDialogProps): JSX.Element {
  const initialCreTeamId = currentCreTeam?.id ?? "";
  const initialSreTeamId = currentSreTeam?.id ?? "";
  const [creTeamId, setCreTeamId] = useState(initialCreTeamId);
  const [sreTeamId, setSreTeamId] = useState(initialSreTeamId);

  const patch = useMemo<AccountTeamsPatch>(() => {
    const next: AccountTeamsPatch = {};
    // Only a non-empty, changed selection is submitted — sending an explicit
    // null does not clear the assignment server-side (it means "leave
    // unchanged," identically to omitting the field), so a cleared picker
    // must never reach the payload as a no-op that looks like a real edit.
    if (creTeamId && creTeamId !== initialCreTeamId) next.creTeamId = creTeamId;
    if (sreTeamId && sreTeamId !== initialSreTeamId) next.sreTeamId = sreTeamId;
    return next;
  }, [creTeamId, initialCreTeamId, sreTeamId, initialSreTeamId]);

  const hasChanges = Object.keys(patch).length > 0;

  return (
    <Dialog open onClose={onClose} maxWidth="xs" fullWidth>
      <DialogTitle>Edit CRE / SRE team</DialogTitle>
      <DialogContent dividers>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2, pt: 0.5 }}>
          {saveError && <Alert severity="error">{saveError}</Alert>}

          <AsyncEntitySelect<BeTeam>
            id="edit-account-cre-team"
            label="CRE team"
            placeholder="Search teams…"
            value={creTeamId}
            onChange={setCreTeamId}
            disabled={isSaving}
            useSearch={useTeamsSearch}
            searchExtra="cre-abt"
            getId={(t) => t.id}
            getLabel={(t) => t.name}
            knownLabel={currentCreTeam?.name}
          />

          <AsyncEntitySelect<BeTeam>
            id="edit-account-sre-team"
            label="SRE team"
            placeholder="Search teams…"
            value={sreTeamId}
            onChange={setSreTeamId}
            disabled={isSaving}
            useSearch={useTeamsSearch}
            searchExtra="sre-abt"
            getId={(t) => t.id}
            getLabel={(t) => t.name}
            knownLabel={currentSreTeam?.name}
          />

          <Typography variant="caption" color="text.secondary">
            Each team can be set or changed independently. Clearing a field
            back to "no team" isn't supported yet — an emptied field won't be
            saved.
          </Typography>
        </Box>
      </DialogContent>
      <DialogActions>
        <Button color="inherit" onClick={onClose} disabled={isSaving}>
          Cancel
        </Button>
        <Button
          variant="contained"
          onClick={() => onSave(patch)}
          disabled={isSaving || !hasChanges}
        >
          {isSaving ? "Saving…" : "Save"}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
