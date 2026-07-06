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
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControl,
  InputLabel,
  MenuItem,
  Select,
  TextField,
} from "@wso2/oxygen-ui";
import { useMemo, useState, type JSX } from "react";
import type {
  BeDeployment,
  BeDeploymentDetailUpdatePayload,
  BeDeploymentType,
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

const DEPLOYMENT_TYPES: BeDeploymentType[] = [
  "primary_production",
  "staging",
  "qa",
  "stress",
  "uat",
  "development",
];

/**
 * Edit a deployment's name, type, and description (`PATCH /deployments/{id}`
 * detail fields). Type is now a string enum per the BE contract introduced in
 * PR #957 — `typeKey` integer is gone. Mount only while open.
 */
export default function EditDeploymentDialog({
  deployment,
  isSaving,
  onClose,
  onSave,
}: EditDeploymentDialogProps): JSX.Element {
  const [name, setName] = useState(deployment.name ?? "");
  const [type, setType] = useState<BeDeploymentType | "">(deployment.type ?? "");
  const [description, setDescription] = useState(deployment.description ?? "");

  const trimmedName = name.trim();
  const trimmedDescription = description.trim();
  const originalName = (deployment.name ?? "").trim();
  const originalType = deployment.type ?? "";
  const originalDescription = (deployment.description ?? "").trim();

  const nameChanged = trimmedName !== originalName;
  const typeChanged = type !== originalType;
  const descriptionChanged = trimmedDescription !== originalDescription;
  const nameError = trimmedName.length === 0;

  // Send only the fields the user actually changed. The backend requires at
  // least one field, so Save stays disabled until something differs.
  const payload = useMemo<BeDeploymentDetailUpdatePayload>(() => {
    const next: Record<string, unknown> = {};
    if (nameChanged) next.name = trimmedName;
    if (typeChanged && type) next.type = type;
    // An emptied description clears the value (null), matching the BE's
    // nullable detail field.
    if (descriptionChanged) {
      next.description = trimmedDescription.length > 0 ? trimmedDescription : null;
    }
    return next as BeDeploymentDetailUpdatePayload;
  }, [nameChanged, typeChanged, descriptionChanged, trimmedName, type, trimmedDescription]);

  const canSave =
    !isSaving && !nameError && (nameChanged || typeChanged || descriptionChanged);

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

          <FormControl size="small" fullWidth required>
            <InputLabel id="edit-deployment-type-label">Type</InputLabel>
            <Select
              labelId="edit-deployment-type-label"
              label="Type"
              value={type}
              onChange={(e) => setType(e.target.value as BeDeploymentType)}
            >
              {DEPLOYMENT_TYPES.map((t) => (
                <MenuItem key={t} value={t}>
                  {deploymentTypeLabel(t)}
                </MenuItem>
              ))}
            </Select>
          </FormControl>

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
