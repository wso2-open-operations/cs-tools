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
  Card,
  CircularProgress,
  FormControl,
  FormHelperText,
  Grid,
  InputLabel,
  MenuItem,
  Select,
  Typography,
} from "@wso2/oxygen-ui";
import { ArrowLeft } from "@wso2/oxygen-ui-icons-react";
import { useMemo, useState, type JSX } from "react";
import { useNavigate } from "react-router";
import { BackendApiError } from "@api/backend/client";
import AttachmentsField from "@components/attachments/AttachmentsField";
import {
  POST_CREATE_ATTACHMENTS_MAX_ENCODED_BYTES,
  type EncodedAttachment,
} from "@components/attachments/encodeAttachment";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import AsyncProjectSelect from "@features/csm-cases/components/AsyncProjectSelect";
import { useSearchDeployments } from "@features/csm-cases/api/useSearchDeployments";
import { useDeployedProductOptions } from "@features/csm-cases/api/useDeployedProductOptions";
import { usePostCsmCase } from "@features/csm-cases/api/usePostCsmCase";
import { usePostCsmCaseAttachment } from "@features/csm-cases/api/useCsmCaseAttachments";
import { uploadAttachmentsToCase } from "@features/csm-cases/api/uploadAttachmentsToCase";
import { useEngineerDisplayName } from "@hooks/useEngineerDisplayName";
import { useSearchCatalogs } from "@features/csm-operations/api/useSearchCatalogs";
import { useCatalogItemVariables } from "@features/csm-operations/api/useCatalogItemVariables";
import CatalogVariableFields from "@features/csm-operations/components/CatalogVariableFields";
import {
  encodeVariableValue,
  getFirstEmptyRequiredField,
  getUserEditableVariables,
  isAttachmentField,
} from "@features/csm-operations/utils/catalogVariables";

export default function CreateServiceRequestPage(): JSX.Element {
  const navigate = useNavigate();
  const { showError } = useErrorBanner();

  const [projectId, setProjectId] = useState("");
  const [deploymentId, setDeploymentId] = useState("");
  const [deployedProductId, setDeployedProductId] = useState("");
  const [catalogId, setCatalogId] = useState("");
  const [catalogItemId, setCatalogItemId] = useState("");
  // Variable answers, keyed by variable id.
  const [answers, setAnswers] = useState<Record<string, string>>({});
  const [attachments, setAttachments] = useState<EncodedAttachment[]>([]);

  const deployments = useSearchDeployments(projectId || undefined);
  const deployedProducts = useDeployedProductOptions(deploymentId || undefined);
  const catalogs = useSearchCatalogs(deployedProductId || undefined);
  const variables = useCatalogItemVariables(
    catalogId || undefined,
    catalogItemId || undefined,
  );
  const postCase = usePostCsmCase();
  const postAttachment = usePostCsmCaseAttachment();
  const uploadedBy = useEngineerDisplayName();
  // Spans the whole submit (create + post-create attachment uploads).
  const [submitting, setSubmitting] = useState(false);

  const catalogItems = useMemo(
    () => catalogs.data?.find((c) => c.id === catalogId)?.catalogItems ?? [],
    [catalogs.data, catalogId],
  );

  // ServiceNow returns context/hidden fields mixed in; only user-editable
  // (non-attachment) variables get a rendered input. Attachments go to the
  // shared attachments section below.
  const allVariables = useMemo(() => variables.data ?? [], [variables.data]);
  const renderableVars = useMemo(
    () => getUserEditableVariables(allVariables).filter((v) => !isAttachmentField(v)),
    [allVariables],
  );
  const firstEmptyRequired = useMemo(
    () => getFirstEmptyRequiredField(allVariables, answers),
    [allVariables, answers],
  );


  // A deployed product with no catalogs is the common non-ServiceNow case;
  // tell the engineer rather than leaving an empty dropdown.
  const noCatalogs =
    !!deployedProductId &&
    catalogs.isSuccess &&
    (catalogs.data?.length ?? 0) === 0;

  // Cascade resets: a parent change invalidates every dependent field below it.
  const onProjectChange = (next: string): void => {
    setProjectId(next);
    setDeploymentId("");
    setDeployedProductId("");
    setCatalogId("");
    setCatalogItemId("");
    setAnswers({});
  };
  const onDeploymentChange = (next: string): void => {
    setDeploymentId(next);
    setDeployedProductId("");
    setCatalogId("");
    setCatalogItemId("");
    setAnswers({});
  };
  const onDeployedProductChange = (next: string): void => {
    setDeployedProductId(next);
    setCatalogId("");
    setCatalogItemId("");
    setAnswers({});
  };
  const onCatalogChange = (next: string): void => {
    setCatalogId(next);
    setCatalogItemId("");
    setAnswers({});
  };
  const onCatalogItemChange = (next: string): void => {
    setCatalogItemId(next);
    setAnswers({});
  };

  const hasOptionsError =
    deployments.isError || deployedProducts.isError || catalogs.isError;
  const retryOptions = (): void => {
    if (deployments.isError) void deployments.refetch();
    if (deployedProducts.isError) void deployedProducts.refetch();
    if (catalogs.isError) void catalogs.refetch();
  };

  const canSubmit = useMemo(
    () =>
      !!projectId &&
      !!deploymentId &&
      !!deployedProductId &&
      !!catalogId &&
      !!catalogItemId &&
      !variables.isLoading &&
      !variables.isError &&
      // Hot fix (mirrors the customer portal): all typable variables required.
      firstEmptyRequired === null &&
      !submitting,
    [
      projectId,
      deploymentId,
      deployedProductId,
      catalogId,
      catalogItemId,
      variables.isLoading,
      variables.isError,
      firstEmptyRequired,
      submitting,
    ],
  );

  const handleSubmit = async (): Promise<void> => {
    if (!canSubmit) return;
    // Send user-editable, non-attachment variables, encoded by type, omitting
    // empties. Context/hidden fields are excluded by getUserEditableVariables.
    const variablePayload = getUserEditableVariables(allVariables)
      .filter((v) => !isAttachmentField(v))
      .map((v) => ({ id: v.id, value: encodeVariableValue(v, answers[v.id] ?? "") }))
      .filter((v) => v.value !== "");

    setSubmitting(true);
    try {
      const created = await postCase.mutateAsync({
        type: "service_request",
        projectId,
        deploymentId,
        deployedProductId,
        catalogId,
        catalogItemId,
        variables: variablePayload,
      });
      // The create endpoint doesn't attach files for service requests, so upload
      // them to the new case afterwards. A partial failure still lands the case.
      const failed = await uploadAttachmentsToCase(
        postAttachment.mutateAsync,
        created.id,
        attachments,
        uploadedBy,
      );
      if (failed > 0) {
        showError(
          `The service request was created, but ${failed} attachment${failed === 1 ? "" : "s"} failed to upload. You can add ${failed === 1 ? "it" : "them"} from the case page.`,
        );
      }
      navigate(`/cases/${created.id}`);
    } catch (err) {
      setSubmitting(false);
      // The backend surfaces real validation messages on 4xx; show them.
      const msg =
        err instanceof BackendApiError && err.status < 500 && err.message
          ? err.message
          : "Could not create the service request. Please try again.";
      showError(msg, err);
    }
  };

  return (
    <Box sx={{ width: "100%", px: 3, py: 3 }}>
      <Button
        variant="text"
        startIcon={<ArrowLeft size={16} />}
        onClick={() => navigate("/operations")}
        sx={{ mb: 1 }}
      >
        Back to operations
      </Button>
      <Typography variant="h5" sx={{ mb: 2 }}>
        New service request
      </Typography>

      <Card variant="outlined" sx={{ p: 3 }}>
        {hasOptionsError && (
          <Box
            sx={{
              mb: 2,
              display: "flex",
              alignItems: "center",
              gap: 1.5,
              flexWrap: "wrap",
            }}
          >
            <Typography variant="body2" color="error.main">
              Some dropdown options failed to load.
            </Typography>
            <Button size="small" variant="outlined" onClick={retryOptions}>
              Retry
            </Button>
          </Box>
        )}

        <Grid container spacing={2.5}>
          <Grid size={{ xs: 12, md: 4 }}>
            <AsyncProjectSelect
              value={projectId}
              onChange={onProjectChange}
              required
            />
          </Grid>

          <Grid size={{ xs: 12, md: 4 }}>
            <FormControl fullWidth size="small" required>
              <InputLabel id="sr-deployment-label">Deployment</InputLabel>
              <Select
                labelId="sr-deployment-label"
                label="Deployment"
                value={deploymentId}
                onChange={(e) => onDeploymentChange(String(e.target.value))}
                disabled={!projectId || deployments.isLoading}
              >
                {(deployments.data ?? []).map((d) => (
                  <MenuItem key={d.id} value={d.id}>
                    {d.name ?? d.id}
                  </MenuItem>
                ))}
              </Select>
              {!projectId ? (
                <FormHelperText>Select a project first</FormHelperText>
              ) : deployments.isLoading ? (
                <FormHelperText>Loading deployments…</FormHelperText>
              ) : (deployments.data ?? []).length === 0 ? (
                <FormHelperText>No deployments found for this project.</FormHelperText>
              ) : null}
            </FormControl>
          </Grid>

          <Grid size={{ xs: 12, md: 4 }}>
            <FormControl fullWidth size="small" required>
              <InputLabel id="sr-product-label">Deployed product</InputLabel>
              <Select
                labelId="sr-product-label"
                label="Deployed product"
                value={deployedProductId}
                onChange={(e) => onDeployedProductChange(String(e.target.value))}
                disabled={!deploymentId || deployedProducts.isLoading}
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
              ) : (deployedProducts.data ?? []).length === 0 ? (
                <FormHelperText>No deployed products found for this deployment.</FormHelperText>
              ) : null}
            </FormControl>
          </Grid>

          <Grid size={{ xs: 12, md: 6 }}>
            <FormControl fullWidth size="small" required>
              <InputLabel id="sr-catalog-label">Catalog</InputLabel>
              <Select
                labelId="sr-catalog-label"
                label="Catalog"
                value={catalogId}
                onChange={(e) => onCatalogChange(String(e.target.value))}
                disabled={!deployedProductId || catalogs.isLoading || noCatalogs}
              >
                {(catalogs.data ?? []).map((c) => (
                  <MenuItem key={c.id} value={c.id}>
                    {c.name ?? c.id}
                  </MenuItem>
                ))}
              </Select>
              {!deployedProductId ? (
                <FormHelperText>Select a deployed product first</FormHelperText>
              ) : catalogs.isLoading ? (
                <FormHelperText>Loading catalogs…</FormHelperText>
              ) : noCatalogs ? (
                <FormHelperText>
                  No service catalogs are available for this product.
                </FormHelperText>
              ) : null}
            </FormControl>
          </Grid>

          <Grid size={{ xs: 12, md: 6 }}>
            <FormControl fullWidth size="small" required>
              <InputLabel id="sr-catalog-item-label">Catalog item</InputLabel>
              <Select
                labelId="sr-catalog-item-label"
                label="Catalog item"
                value={catalogItemId}
                onChange={(e) => onCatalogItemChange(String(e.target.value))}
                disabled={!catalogId}
              >
                {catalogItems.map((ci) => (
                  <MenuItem key={ci.id} value={ci.id}>
                    {ci.name ?? ci.id}
                  </MenuItem>
                ))}
              </Select>
              {!catalogId ? (
                <FormHelperText>Select a catalog first</FormHelperText>
              ) : catalogItems.length === 0 ? (
                <FormHelperText>No items found in this catalog.</FormHelperText>
              ) : null}
            </FormControl>
          </Grid>

          {/* Dynamic variable form for the chosen catalog item. */}
          {catalogItemId && (
            <Grid size={{ xs: 12 }}>
              {variables.isLoading ? (
                <Box sx={{ display: "flex", alignItems: "center", gap: 1.5, py: 1 }}>
                  <CircularProgress size={18} />
                  <Typography variant="body2" color="text.secondary">
                    Loading request form…
                  </Typography>
                </Box>
              ) : variables.isError ? (
                <Alert
                  severity="error"
                  action={
                    <Button
                      color="inherit"
                      size="small"
                      onClick={() => void variables.refetch()}
                    >
                      Retry
                    </Button>
                  }
                >
                  Could not load the request form for this catalog item.
                </Alert>
              ) : renderableVars.length === 0 ? (
                <Typography variant="body2" color="text.secondary">
                  This catalog item has no additional fields.
                </Typography>
              ) : (
                <CatalogVariableFields
                  variables={renderableVars}
                  values={answers}
                  onChange={(id, value) =>
                    setAnswers((prev) => ({ ...prev, [id]: value }))
                  }
                />
              )}
            </Grid>
          )}

          {/* Optional supporting attachments. */}
          {catalogItemId && !variables.isLoading && !variables.isError && (
            <Grid size={{ xs: 12 }}>
              <Typography
                variant="caption"
                color="text.secondary"
                sx={{ display: "block", mb: 0.5 }}
              >
                Attachments
              </Typography>
              <AttachmentsField
                attachments={attachments}
                onChange={setAttachments}
                onError={showError}
                maxEncodedBytes={POST_CREATE_ATTACHMENTS_MAX_ENCODED_BYTES}
              />
            </Grid>
          )}
        </Grid>

        <Box
          sx={{
            display: "flex",
            justifyContent: "flex-end",
            alignItems: "center",
            gap: 1.5,
            mt: 2.5,
          }}
        >
          {firstEmptyRequired && !variables.isLoading && (
            <Typography variant="caption" color="text.secondary" sx={{ mr: "auto" }}>
              Required field: {firstEmptyRequired}
            </Typography>
          )}
          <Button variant="outlined" onClick={() => navigate("/operations")}>
            Cancel
          </Button>
          <Button
            variant="contained"
            onClick={() => void handleSubmit()}
            disabled={!canSubmit}
          >
            {submitting ? "Creating…" : "Create service request"}
          </Button>
        </Box>
      </Card>
    </Box>
  );
}
