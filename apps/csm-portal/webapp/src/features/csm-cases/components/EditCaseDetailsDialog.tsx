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
  Typography,
} from "@wso2/oxygen-ui";
import { CheckCircle, XCircle } from "@wso2/oxygen-ui-icons-react";
import { useMemo, useState, type JSX } from "react";
import Editor from "@components/rich-text-editor/Editor";
import { useSearchDeployments } from "@features/csm-cases/api/useSearchDeployments";
import { useDeployedProductOptions } from "@features/csm-cases/api/useDeployedProductOptions";

/** One field this dialog can edit and submit independently. */
export type EditableCaseField =
  | "subject"
  | "description"
  | "deploymentId"
  | "deployedProductId";

/** Result of attempting to save a single field's PATCH. */
export interface FieldSaveResult {
  field: EditableCaseField;
  ok: boolean;
  error?: string;
}

export interface EditCaseDetailsDialogProps {
  projectId: string;
  currentSubject: string;
  /** Current description as HTML (the Lexical editor's on-the-wire shape). */
  currentDescriptionHtml: string;
  currentDeploymentId?: string;
  currentDeployedProductId?: string;
  isSaving: boolean;
  onClose: () => void;
  /**
   * Submits every changed field as its own sequential `PATCH /cases/{id}`
   * call (the backend accepts exactly one field per call — see
   * `BeCaseUpdatePayload`) and resolves with a per-field result once all
   * attempts have settled, so a partial failure (e.g. subject saves but
   * deployment doesn't) is reported field-by-field rather than swallowed.
   */
  onSubmit: (changes: {
    subject?: string;
    description?: string;
    deploymentId?: string;
    deployedProductId?: string;
  }) => Promise<FieldSaveResult[]>;
}

const FIELD_LABEL: Record<EditableCaseField, string> = {
  subject: "Subject",
  description: "Description",
  deploymentId: "Deployment",
  deployedProductId: "Deployed product",
};

/**
 * Edit a case's subject, description, deployment, and deployed product in one
 * form. The backend's `PATCH /cases/{id}` contract accepts exactly one field
 * per call, so submitting fires one sequential PATCH per changed field (see
 * {@link EditCaseDetailsDialogProps.onSubmit}) rather than one combined
 * write. Deployment/deployed-product reuse the same cascading-select pattern
 * as case create (`useSearchDeployments` / `useDeployedProductOptions`),
 * scoped to the case's own project — only the deployment/product within that
 * project can be picked, not a different project.
 */
export default function EditCaseDetailsDialog({
  projectId,
  currentSubject,
  currentDescriptionHtml,
  currentDeploymentId,
  currentDeployedProductId,
  isSaving,
  onClose,
  onSubmit,
}: EditCaseDetailsDialogProps): JSX.Element {
  const [subject, setSubject] = useState(currentSubject);
  const [description, setDescription] = useState(currentDescriptionHtml);
  const [deploymentId, setDeploymentId] = useState(currentDeploymentId ?? "");
  const [deployedProductId, setDeployedProductId] = useState(
    currentDeployedProductId ?? "",
  );
  const [results, setResults] = useState<FieldSaveResult[] | null>(null);

  const deployments = useSearchDeployments(projectId || undefined);
  const deployedProducts = useDeployedProductOptions(deploymentId || undefined);

  const onDeploymentChange = (next: string): void => {
    setDeploymentId(next);
    // Cascade reset: a new deployment invalidates the previously picked
    // deployed product, same as case create.
    setDeployedProductId("");
  };

  const changes = useMemo(() => {
    const next: {
      subject?: string;
      description?: string;
      deploymentId?: string;
      deployedProductId?: string;
    } = {};
    if (subject.trim() && subject !== currentSubject) next.subject = subject;
    if (description !== currentDescriptionHtml) next.description = description;
    if (deploymentId && deploymentId !== currentDeploymentId)
      next.deploymentId = deploymentId;
    if (deployedProductId && deployedProductId !== currentDeployedProductId)
      next.deployedProductId = deployedProductId;
    return next;
  }, [
    subject,
    currentSubject,
    description,
    currentDescriptionHtml,
    deploymentId,
    currentDeploymentId,
    deployedProductId,
    currentDeployedProductId,
  ]);

  const hasChanges = Object.keys(changes).length > 0;
  const allSaved = !!results && results.every((r) => r.ok);

  const handleSubmit = (): void => {
    if (!hasChanges) return;
    setResults(null);
    void onSubmit(changes).then(setResults);
  };

  return (
    <Dialog open onClose={onClose} maxWidth="sm" fullWidth>
      <DialogTitle>Edit case details</DialogTitle>
      <DialogContent dividers>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2, pt: 0.5 }}>
          {results && (
            <Box sx={{ display: "flex", flexDirection: "column", gap: 0.5 }}>
              {results.map((r) => (
                <Box
                  key={r.field}
                  sx={{ display: "flex", alignItems: "center", gap: 1 }}
                >
                  <Box
                    sx={{
                      display: "flex",
                      color: r.ok ? "success.main" : "error.main",
                    }}
                  >
                    {r.ok ? <CheckCircle size={16} /> : <XCircle size={16} />}
                  </Box>
                  <Typography variant="body2">
                    {FIELD_LABEL[r.field]}
                    {r.ok ? " saved." : `: ${r.error ?? "could not be saved."}`}
                  </Typography>
                </Box>
              ))}
            </Box>
          )}

          <TextField
            label="Subject"
            size="small"
            fullWidth
            value={subject}
            onChange={(e) => setSubject(e.target.value.slice(0, 200))}
            disabled={isSaving}
          />

          <Box>
            <Typography
              id="edit-case-description-label"
              component="label"
              variant="caption"
              color="text.secondary"
              sx={{ display: "block", mb: 0.5 }}
            >
              Description
            </Typography>
            <Box role="group" aria-labelledby="edit-case-description-label">
              <Editor
                value={description}
                onChange={setDescription}
                placeholder="Describe the issue…"
                minHeight={140}
                maxHeight={320}
                toolbarVariant="full"
                disabled={isSaving}
              />
            </Box>
          </Box>

          <FormControl fullWidth size="small">
            <InputLabel id="edit-case-deployment-label">Deployment</InputLabel>
            <Select
              labelId="edit-case-deployment-label"
              label="Deployment"
              value={deploymentId}
              onChange={(e) => onDeploymentChange(String(e.target.value))}
              disabled={isSaving || deployments.isLoading}
            >
              {(deployments.data ?? []).map((d) => (
                <MenuItem key={d.id} value={d.id}>
                  {d.name ?? d.id}
                </MenuItem>
              ))}
            </Select>
            {deployments.isLoading && (
              <FormHelperText>Loading deployments…</FormHelperText>
            )}
          </FormControl>

          <FormControl fullWidth size="small">
            <InputLabel id="edit-case-product-label">Deployed product</InputLabel>
            <Select
              labelId="edit-case-product-label"
              label="Deployed product"
              value={deployedProductId}
              onChange={(e) => setDeployedProductId(String(e.target.value))}
              disabled={isSaving || !deploymentId || deployedProducts.isLoading}
            >
              {(deployedProducts.data ?? []).map((dp) => (
                <MenuItem key={dp.id} value={dp.id}>
                  {dp.label}
                </MenuItem>
              ))}
            </Select>
            {!deploymentId ? (
              <FormHelperText>Select a deployment first</FormHelperText>
            ) : deployedProducts.isLoading ? (
              <FormHelperText>Loading products…</FormHelperText>
            ) : null}
          </FormControl>
        </Box>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose} disabled={isSaving}>
          {allSaved ? "Close" : "Cancel"}
        </Button>
        <Button
          variant="contained"
          disabled={!hasChanges || isSaving || allSaved}
          loading={isSaving}
          onClick={handleSubmit}
        >
          Save changes
        </Button>
      </DialogActions>
    </Dialog>
  );
}
