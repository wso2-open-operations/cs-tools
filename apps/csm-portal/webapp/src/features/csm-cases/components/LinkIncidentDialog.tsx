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
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Typography,
} from "@wso2/oxygen-ui";
import { useState, type JSX } from "react";
import AsyncEntitySelect from "@components/AsyncEntitySelect";
import { useSearchIncidentsForSelect } from "@features/csm-operations/api/useSearchIncidentsForSelect";
import type { BeIncident } from "@api/backend/types";

interface LinkIncidentDialogProps {
  /** True while a PATCH is in flight; disables the actions. */
  isLinking: boolean;
  onClose: () => void;
  /** Link the current case to `targetIncidentId` as its parent. */
  onLink: (targetIncidentId: string) => void;
}

function incidentSearchLabel(i: BeIncident): string {
  return [i.number, i.subject].filter(Boolean).join(" — ") || i.id || "";
}

/**
 * Search-and-select an incident to link the current case to as its
 * **parent** (`PATCH { parentId }`) — the same generic task-parent
 * relationship {@link LinkCaseDialog} uses for case-to-case parent links.
 * There is no incident equivalent of the looser `relatedCaseId` cross-link
 * (that field is case-only), so this only offers the one link type. Reuses
 * {@link AsyncEntitySelect}/`useSearchIncidentsForSelect`, the same
 * type-ahead picker `CreateProblemPage`/`CreateIncidentPage` already use for
 * their own incident reference fields, rather than a bespoke list like
 * `LinkCaseDialog`. ServiceNow-source only; the caller surfaces a rejection
 * on another source.
 */
export default function LinkIncidentDialog({
  isLinking,
  onClose,
  onLink,
}: LinkIncidentDialogProps): JSX.Element {
  const [incidentId, setIncidentId] = useState("");

  const handleLink = (): void => {
    if (!incidentId) return;
    onLink(incidentId);
  };

  return (
    <Dialog open onClose={onClose} maxWidth="xs" fullWidth>
      <DialogTitle>Link to incident</DialogTitle>
      <DialogContent dividers>
        <Typography variant="caption" color="text.secondary" sx={{ display: "block", mb: 1.5 }}>
          Links this case to an incident as its parent — the hierarchical
          relationship, e.g. a case reported against an incident that's
          already being tracked.
        </Typography>

        <AsyncEntitySelect<BeIncident>
          id="link-incident-picker"
          label="Incident"
          placeholder="Search incidents…"
          value={incidentId}
          onChange={setIncidentId}
          disabled={isLinking}
          required
          useSearch={useSearchIncidentsForSelect}
          getId={(i) => i.id!}
          getLabel={incidentSearchLabel}
        />

        <Typography variant="caption" color="text.secondary" sx={{ display: "block", mt: 1.5 }}>
          Linking applies to ServiceNow-managed cases.
        </Typography>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose} disabled={isLinking}>
          Cancel
        </Button>
        <Button
          variant="contained"
          onClick={handleLink}
          disabled={!incidentId || isLinking}
          loading={isLinking}
        >
          Link
        </Button>
      </DialogActions>
    </Dialog>
  );
}
