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
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { useMemo, useState, type JSX } from "react";
import type {
  BeDeployment,
  BeDeploymentDetailUpdatePayload,
} from "@api/backend/types";
import { deploymentTypeLabel } from "@features/csm-projects/utils/deployments";

interface EditDeploymentDialogProps {
  deployment: BeDeployment;
  /** True while the PATCH is in flight; disables the actions. */
  isSaving: boolean;
  onClose: () => void;
  /** Persist the changed detail fields via `PATCH /deployments/{id}`. */
  onSave: (payload: BeDeploymentDetailUpdatePayload) => void;
}

const NAME_MAX = 200;
const DESCRIPTION_MAX = 4000;

/**
 * Edit a deployment's name and description (`PATCH /deployments/{id}` detail
 * fields). The deployment type is shown read-only: changing it needs the
 * ServiceNow type→key integer, which no endpoint exposes yet, so type edits are
 * deferred. Mount only while open.
 */
export default function EditDeploymentDialog({
  deployment,
  isSaving,
  onClose,
  onSave,
}: EditDeploymentDialogProps): JSX.Element {
  const [name, setName] = useState(deployment.name ?? "");
  const [description, setDescription] = useState(deployment.description ?? "");

  const trimmedName = name.trim();
  const trimmedDescription = description.trim();
  const originalName = (deployment.name ?? "").trim();
  const originalDescription = (deployment.description ?? "").trim();

  const nameChanged = trimmedName !== originalName;
  const descriptionChanged = trimmedDescription !== originalDescription;
  const nameError = trimmedName.length === 0;

  // Send only the fields the user actually changed. The backend requires at
  // least one field, so Save stays disabled until something differs.
  const payload = useMemo<BeDeploymentDetailUpdatePayload>(() => {
    const next: BeDeploymentDetailUpdatePayload = {};
    if (nameChanged) next.name = trimmedName;
    // An emptied description clears the value (null), matching the BE's
    // nullable detail field.
    if (descriptionChanged) {
      next.description = trimmedDescription.length > 0 ? trimmedDescription : null;
    }
    return next;
  }, [nameChanged, descriptionChanged, trimmedName, trimmedDescription]);

  const canSave =
    !isSaving && !nameError && (nameChanged || descriptionChanged);

  return (
    <Dialog open onClose={onClose} maxWidth="sm" fullWidth>
      <DialogTitle>Edit deployment</DialogTitle>
      <DialogContent dividers>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2, pt: 0.5 }}>
          <TextField
            label="Name"
            value={name}
            onChange={(e) => setName(e.target.value)}
            size="small"
            fullWidth
            required
            autoFocus
            error={nameError}
            helperText={nameError ? "Name is required." : " "}
            slotProps={{ htmlInput: { maxLength: NAME_MAX } }}
          />

          <TextField
            label="Description"
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            size="small"
            fullWidth
            multiline
            minRows={3}
            slotProps={{ htmlInput: { maxLength: DESCRIPTION_MAX } }}
          />

          <Box sx={{ display: "flex", flexDirection: "column", gap: 0.75 }}>
            <Typography
              variant="caption"
              color="text.secondary"
              sx={{ textTransform: "uppercase", letterSpacing: 0.4 }}
            >
              Type
            </Typography>
            <Box sx={{ display: "flex", alignItems: "center", gap: 1, flexWrap: "wrap" }}>
              <Chip size="small" variant="outlined" label={deploymentTypeLabel(deployment.type)} />
              <Typography variant="caption" color="text.secondary">
                Type changes aren't available yet.
              </Typography>
            </Box>
          </Box>
        </Box>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose} disabled={isSaving}>
          Cancel
        </Button>
        <Button variant="contained" disabled={!canSave} onClick={() => onSave(payload)}>
          Save changes
        </Button>
      </DialogActions>
    </Dialog>
  );
}
