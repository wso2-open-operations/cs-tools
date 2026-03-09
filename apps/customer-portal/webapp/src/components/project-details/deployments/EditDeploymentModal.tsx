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

import type { ProjectDeploymentItem } from "@models/responses";
import {
  Box,
  Button,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  IconButton,
  MenuItem,
  Skeleton,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { X } from "@wso2/oxygen-ui-icons-react";
import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type ChangeEvent,
  type JSX,
} from "react";
import useGetProjectFilters from "@api/useGetProjectFilters";
import { usePatchDeployment } from "@api/usePatchDeployment";

export interface EditDeploymentModalProps {
  open: boolean;
  deployment: ProjectDeploymentItem | null;
  projectId: string;
  onClose: () => void;
  onSuccess?: () => void;
  onError?: (message: string) => void;
}

const INITIAL_FORM = {
  name: "",
  typeKey: "",
  description: "",
};

/**
 * Modal for editing a deployment (name, type, description).
 * Only sends changed fields to the API (PATCH behavior).
 * Deployment type options come from useGetProjectFilters.
 *
 * @param {EditDeploymentModalProps} props - open, deployment, projectId, onClose, optional onSuccess/onError.
 * @returns {JSX.Element} The edit deployment modal.
 */
export default function EditDeploymentModal({
  open,
  deployment,
  projectId,
  onClose,
  onSuccess,
  onError,
}: EditDeploymentModalProps): JSX.Element {
  const { data: filtersData, isLoading: isFiltersLoading } =
    useGetProjectFilters(projectId);
  const patchDeployment = usePatchDeployment();

  const deploymentTypes = filtersData?.deploymentTypes ?? [];

  const [form, setForm] = useState(INITIAL_FORM);
  const prevDeploymentIdRef = useRef<string | null>(null);

  const isSubmitting = patchDeployment.isPending;
  const isValid = !!projectId && !!deployment?.id;

  // Reset form when modal closes or deployment changes
  useEffect(() => {
    if (!open) {
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setForm(INITIAL_FORM);
      prevDeploymentIdRef.current = null;
    } else if (deployment && deployment.id !== prevDeploymentIdRef.current) {
      prevDeploymentIdRef.current = deployment.id;
      setForm({
        name: deployment.name ?? "",
        typeKey: deployment.type?.id ?? "",
        description: deployment.description ?? "",
      });
    }
  }, [open, deployment]);

  const handleClose = useCallback(() => {
    setForm(INITIAL_FORM);
    onClose();
  }, [onClose]);

  const handleTextChange =
    (field: "name" | "description") =>
    (event: ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) => {
      setForm((prev) => ({ ...prev, [field]: event.target.value }));
    };

  const handleTypeChange = useCallback(
    (event: ChangeEvent<HTMLInputElement>) => {
      setForm((prev) => ({ ...prev, typeKey: event.target.value }));
    },
    [],
  );

  const handleSubmit = useCallback(async () => {
    if (!isValid || !deployment) return;

    // Only send changed fields (PATCH behavior)
    const body: Record<string, string | number | boolean | undefined> = {};

    const newName = form.name.trim();
    if (newName !== (deployment.name ?? "")) {
      body.name = newName;
    }

    const newDescription = form.description.trim();
    if (newDescription !== (deployment.description ?? "")) {
      body.description = newDescription;
    }

    const newTypeKey =
      form.typeKey && form.typeKey.trim() ? Number(form.typeKey) : undefined;
    const originalTypeKey = deployment.type?.id
      ? Number(deployment.type.id)
      : undefined;
    if (newTypeKey !== undefined && newTypeKey !== originalTypeKey) {
      body.typeKey = newTypeKey;
    }

    // If nothing changed, just close
    if (Object.keys(body).length === 0) {
      handleClose();
      return;
    }

    try {
      await patchDeployment.mutateAsync({
        projectId,
        deploymentId: deployment.id,
        body,
      });
      handleClose();
      onSuccess?.();
    } catch (error) {
      onError?.(
        error instanceof Error ? error.message : "Failed to update deployment",
      );
    }
  }, [
    isValid,
    deployment,
    form.name,
    form.description,
    form.typeKey,
    projectId,
    patchDeployment,
    handleClose,
    onSuccess,
    onError,
  ]);

  return (
    <Dialog
      open={open}
      onClose={handleClose}
      maxWidth="sm"
      fullWidth
      aria-labelledby="edit-deployment-dialog-title"
      aria-describedby="edit-deployment-dialog-description"
    >
      <DialogTitle
        id="edit-deployment-dialog-title"
        sx={{ pr: 6, position: "relative", pb: 0.5 }}
      >
        Edit Deployment
        <Typography
          id="edit-deployment-dialog-description"
          variant="body2"
          color="text.secondary"
          sx={{ mt: 0.5, fontWeight: "normal", fontSize: "0.875rem" }}
        >
          Update deployment name, type, and description.
        </Typography>
        <IconButton
          aria-label="close"
          onClick={handleClose}
          sx={{ position: "absolute", right: 12, top: 12 }}
          size="small"
        >
          <X size={20} aria-hidden />
        </IconButton>
      </DialogTitle>

      <DialogContent
        sx={{
          pt: 1,
          "& .MuiInputBase-input::placeholder": {
            color: "text.secondary",
            opacity: 1,
          },
        }}
      >
        <Box sx={{ mt: 2, mb: 2 }}>
          <TextField
            id="edit-deployment-name"
            label="Deployment Name"
            placeholder="e.g., Production US-East"
            value={form.name}
            onChange={handleTextChange("name")}
            fullWidth
            size="small"
            disabled={isSubmitting}
          />
        </Box>

        {isFiltersLoading ? (
          <Skeleton
            variant="rounded"
            height={40}
            sx={{ mb: 2, borderRadius: 1 }}
          />
        ) : (
          <Box sx={{ mb: 2 }}>
            <TextField
              select
              fullWidth
              size="small"
              id="edit-deployment-type"
              label="Deployment Type"
              value={form.typeKey}
              onChange={handleTypeChange}
              disabled={isSubmitting}
              sx={{
                "& .MuiSelect-select": {
                  color: !form.typeKey ? "text.secondary" : undefined,
                },
              }}
            >
              <MenuItem value="">Select</MenuItem>
              {deploymentTypes.map(({ id, label }) => (
                <MenuItem key={id} value={id}>
                  {label}
                </MenuItem>
              ))}
            </TextField>
          </Box>
        )}

        <TextField
          id="edit-deployment-description"
          label="Description"
          placeholder="Describe this deployment environment..."
          value={form.description}
          onChange={handleTextChange("description")}
          fullWidth
          size="small"
          multiline
          rows={3}
          sx={{ mb: 2 }}
          disabled={isSubmitting}
        />
      </DialogContent>

      <DialogActions
        sx={{ px: 3, pb: 3, pt: 1, justifyContent: "flex-end", gap: 1 }}
      >
        <Button
          variant="outlined"
          onClick={handleClose}
          disabled={isSubmitting}
        >
          Cancel
        </Button>
        {isSubmitting ? (
          <Button
            variant="contained"
            color="primary"
            startIcon={<CircularProgress color="inherit" size={16} />}
            disabled
          >
            Updating...
          </Button>
        ) : (
          <Button
            type="button"
            variant="contained"
            color="primary"
            onClick={handleSubmit}
            disabled={!isValid}
          >
            Update
          </Button>
        )}
      </DialogActions>
    </Dialog>
  );
}
