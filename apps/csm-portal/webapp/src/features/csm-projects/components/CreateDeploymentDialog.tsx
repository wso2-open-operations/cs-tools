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
  FormHelperText,
  InputLabel,
  MenuItem,
  Select,
  TextField,
} from "@wso2/oxygen-ui";
import { useState, type JSX } from "react";
import type { BeDeploymentCreatePayload, BeDeploymentType } from "@api/backend/types";
import { deploymentTypeLabel } from "@features/csm-projects/utils/deployments";

interface CreateDeploymentDialogProps {
  /** The project this deployment belongs to (pre-populated, read-only). */
  projectId: string;
  /** True while the POST is in flight; disables the actions. */
  isSaving: boolean;
  onClose: () => void;
  /** Submit the create payload via `POST /deployments`. */
  onCreate: (payload: BeDeploymentCreatePayload) => void;
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
 * Create a new deployment under the current project. All four fields are
 * required per `POST /deployments`. The project is locked to the page context;
 * `type` is the string enum introduced by PR #957 (no `typeKey` integer).
 * Mount only while open.
 */
export default function CreateDeploymentDialog({
  projectId,
  isSaving,
  onClose,
  onCreate,
}: CreateDeploymentDialogProps): JSX.Element {
  const [name, setName] = useState("");
  const [type, setType] = useState<BeDeploymentType | "">("");
  const [description, setDescription] = useState("");

  const trimmedName = name.trim();
  const trimmedDescription = description.trim();
  const nameError = trimmedName.length === 0;
  const typeError = type === "";
  const descriptionError = trimmedDescription.length === 0;

  // All four fields required by the BE contract.
  const canSave =
    !isSaving && !nameError && !typeError && !descriptionError;

  const handleCreate = (): void => {
    if (!canSave || !type) return;
    onCreate({
      projectId,
      name: trimmedName,
      type,
      description: trimmedDescription,
    });
  };

  return (
    <Dialog open onClose={onClose} maxWidth="sm" fullWidth>
      <DialogTitle>Create deployment</DialogTitle>
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
            error={name.length > 0 && nameError}
            helperText={name.length > 0 && nameError ? "Name is required." : " "}
            slotProps={{ htmlInput: { maxLength: NAME_MAX } }}
          />

          <FormControl size="small" fullWidth required error={type === "" && description.length > 0}>
            <InputLabel id="create-deployment-type-label">Type</InputLabel>
            <Select
              labelId="create-deployment-type-label"
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
            {type === "" && description.length > 0 && (
              <FormHelperText>Type is required.</FormHelperText>
            )}
          </FormControl>

          <TextField
            label="Description"
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            size="small"
            fullWidth
            required
            multiline
            minRows={3}
            error={description.length > 0 && descriptionError}
            helperText={
              description.length > 0 && descriptionError
                ? "Description is required."
                : " "
            }
            slotProps={{ htmlInput: { maxLength: DESCRIPTION_MAX } }}
          />
        </Box>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose} disabled={isSaving}>
          Cancel
        </Button>
        <Button
          variant="contained"
          disabled={!canSave}
          onClick={handleCreate}
        >
          Create
        </Button>
      </DialogActions>
    </Dialog>
  );
}
