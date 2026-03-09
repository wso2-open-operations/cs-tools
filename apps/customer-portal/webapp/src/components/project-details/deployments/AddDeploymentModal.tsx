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
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControl,
  IconButton,
  InputLabel,
  MenuItem,
  Select,
  Skeleton,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { X } from "@wso2/oxygen-ui-icons-react";
import {
  useCallback,
  useState,
  useEffect,
  type JSX,
  type ChangeEvent,
} from "react";
import type { SelectChangeEvent } from "@wso2/oxygen-ui";
import { usePostCreateDeployment } from "@api/usePostCreateDeployment";
import useGetProjectFilters from "@api/useGetProjectFilters";
import ErrorIndicator from "@components/common/error-indicator/ErrorIndicator";
import ErrorBanner from "@components/common/error-banner/ErrorBanner";

export interface AddDeploymentModalProps {
  open: boolean;
  projectId: string;
  onClose: () => void;
  onSuccess?: () => void;
  onError?: (message: string) => void;
}

const INITIAL_FORM = {
  name: "",
  deploymentTypeKey: "",
  description: "",
};

/**
 * Modal for creating a new deployment environment.
 * Fetches deployment types dynamically from the filters API.
 *
 * @param {AddDeploymentModalProps} props - open, projectId, onClose, optional onSuccess/onError.
 * @returns {JSX.Element} The add deployment modal.
 */
export default function AddDeploymentModal({
  open,
  projectId,
  onClose,
  onSuccess,
  onError,
}: AddDeploymentModalProps): JSX.Element {
  const createDeployment = usePostCreateDeployment(projectId);
  const {
    data: filtersData,
    isLoading: isFiltersLoading,
    isError: isFiltersError,
  } = useGetProjectFilters(projectId);

  const deploymentTypes = filtersData?.deploymentTypes ?? [];

  const [form, setForm] = useState(INITIAL_FORM);
  const [filtersErrorBanner, setFiltersErrorBanner] = useState(false);

  useEffect(() => {
    if (isFiltersError) {
      setFiltersErrorBanner(true);
    }
  }, [isFiltersError]);

  const isValid =
    form.name.trim() !== "" &&
    form.deploymentTypeKey !== "" &&
    form.description.trim() !== "";

  const handleClose = useCallback(() => {
    setForm(INITIAL_FORM);
    setFiltersErrorBanner(false);
    onClose();
  }, [onClose]);

  const handleTextChange =
    (field: keyof typeof INITIAL_FORM) =>
    (event: ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) => {
      setForm((prev) => ({ ...prev, [field]: event.target.value }));
    };

  const handleSelectChange = (event: SelectChangeEvent<string>) => {
    setForm((prev) => ({ ...prev, deploymentTypeKey: event.target.value }));
  };

  const handleSubmit = useCallback(() => {
    if (!isValid) return;

    createDeployment.mutate(
      {
        name: form.name.trim(),
        description: form.description.trim(),
        deploymentTypeKey: Number(form.deploymentTypeKey),
      },
      {
        onSuccess: () => {
          handleClose();
          onSuccess?.();
        },
        onError: (error: Error) => {
          onError?.(error.message ?? "Failed to create deployment.");
        },
      },
    );
  }, [isValid, form, createDeployment, handleClose, onSuccess, onError]);

  return (
    <>
      {/* Filters fetch error banner — shown outside dialog so it persists */}
      {isFiltersError && filtersErrorBanner && (
        <ErrorBanner
          message="Failed to load deployment types. Please close and try again."
          onClose={() => setFiltersErrorBanner(false)}
        />
      )}

      <Dialog
        open={open}
        onClose={handleClose}
        maxWidth="sm"
        fullWidth
        aria-labelledby="add-deployment-dialog-title"
        aria-describedby="add-deployment-dialog-description"
      >
        {/* Dialog header */}
        <DialogTitle
          id="add-deployment-dialog-title"
          sx={{ pr: 6, position: "relative", pb: 0.5 }}
        >
          Add New Deployment
          <Typography
            id="add-deployment-dialog-description"
            variant="body2"
            color="text.secondary"
            sx={{ mt: 0.5, fontWeight: "normal", fontSize: "0.875rem" }}
          >
            Create a new deployment environment for your project.
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

        <DialogContent sx={{ pt: 1 }}>
          <TextField
            id="deployment-name"
            label="Deployment Name *"
            placeholder="e.g., Production US-East"
            value={form.name}
            onChange={handleTextChange("name")}
            fullWidth
            size="small"
            sx={{ mt: 4, mb: 2 }}
            disabled={createDeployment.isPending}
          />

          {/* Deployment Type — skeleton while loading, error indicator on failure */}
          {isFiltersLoading ? (
            <Skeleton
              variant="rounded"
              height={40}
              sx={{ mb: 2, borderRadius: 1 }}
            />
          ) : (
            <Box
              sx={{
                display: "flex",
                alignItems: "center",
                gap: 0.5,
                mb: 2,
              }}
            >
              <FormControl fullWidth size="small">
                <InputLabel id="deployment-type-label">
                  Deployment Type *
                </InputLabel>
                <Select<string>
                  labelId="deployment-type-label"
                  id="deployment-type"
                  value={form.deploymentTypeKey}
                  label="Deployment Type *"
                  onChange={handleSelectChange}
                  disabled={createDeployment.isPending || isFiltersError}
                >
                  {deploymentTypes.map(({ id, label }) => (
                    <MenuItem key={id} value={id}>
                      {label}
                    </MenuItem>
                  ))}
                </Select>
              </FormControl>
              {isFiltersError && (
                <ErrorIndicator entityName="deployment types" size="small" />
              )}
            </Box>
          )}

          <TextField
            id="deployment-description"
            label="Description *"
            placeholder="Describe this deployment environment..."
            value={form.description}
            onChange={handleTextChange("description")}
            fullWidth
            size="small"
            multiline
            rows={3}
            disabled={createDeployment.isPending}
          />
        </DialogContent>

        <DialogActions
          sx={{ px: 3, pb: 3, pt: 1, justifyContent: "flex-end", gap: 1 }}
        >
          <Button
            variant="outlined"
            onClick={handleClose}
            disabled={createDeployment.isPending}
          >
            Cancel
          </Button>
          {createDeployment.isPending ? (
            <Button
              variant="contained"
              color="primary"
              startIcon={<CircularProgress color="inherit" size={16} />}
              disabled
            >
              Creating...
            </Button>
          ) : (
            <Button
              type="button"
              variant="contained"
              color="primary"
              onClick={handleSubmit}
              disabled={!isValid || isFiltersError}
            >
              Add Deployment
            </Button>
          )}
        </DialogActions>
      </Dialog>
    </>
  );
}
