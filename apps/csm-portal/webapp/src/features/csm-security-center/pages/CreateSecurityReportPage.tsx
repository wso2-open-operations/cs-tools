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
  Card,
  FormControl,
  FormHelperText,
  Grid,
  InputLabel,
  MenuItem,
  Select,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { ArrowLeft } from "@wso2/oxygen-ui-icons-react";
import { useMemo, useState, type JSX } from "react";
import { useLocation, useSearchParams } from "react-router";

import { BackendApiError } from "@api/backend/client";
import Editor from "@components/rich-text-editor/Editor";
import AttachmentsField from "@components/attachments/AttachmentsField";
import {
  POST_CREATE_ATTACHMENTS_MAX_ENCODED_BYTES,
  totalEncodedBytes,
  type EncodedAttachment,
} from "@components/attachments/encodeAttachment";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import ProjectSelectionField from "@features/csm-cases/components/ProjectSelectionField";
import { useSearchDeployments } from "@features/csm-cases/api/useSearchDeployments";
import { useDeployedProductOptions } from "@features/csm-cases/api/useDeployedProductOptions";
import { usePostCsmCase } from "@features/csm-cases/api/usePostCsmCase";
import { useNavTransition } from "@hooks/useNavTransition";

/** The rich-text editor emits `<p></p>` when empty; check the stripped text. */
function isEmptyHtml(html: string): boolean {
  return html.replace(/<[^>]*>/g, "").replace(/&nbsp;/g, " ").trim().length === 0;
}

/** Today as YYYY-MM-DD, for the auto-generated report title. */
function todayStamp(): string {
  const d = new Date();
  const mm = String(d.getMonth() + 1).padStart(2, "0");
  const dd = String(d.getDate()).padStart(2, "0");
  return `${d.getFullYear()}-${mm}-${dd}`;
}

export default function CreateSecurityReportPage(): JSX.Element {
  const navigate = useNavTransition();
  const { showError } = useErrorBanner();

  // When opened from a project's page (`/security-center/reports/new?projectId=…`),
  // the project is fixed and shown read-only, mirroring CsmCaseCreatePage's
  // `?projectId=` lock — the engineer can't accidentally file against the
  // wrong project. Opened without the param (the Security Center list entry),
  // the searchable picker is shown.
  const [searchParams] = useSearchParams();
  const lockedProjectId = searchParams.get("projectId") ?? "";

  // Set when opened from a project's page Create menu (state: { from:
  // "/customers/projects/:id" }), so Back/Cancel return there instead of the
  // hardcoded Security Center list, and the newly created report's own Back
  // button (reading this same convention) returns there too.
  const backState = useLocation().state as { from?: string } | undefined;
  const backTarget = backState?.from ?? "/security-center";

  const [projectId, setProjectId] = useState(lockedProjectId);
  const [deploymentId, setDeploymentId] = useState("");
  const [deployedProductId, setDeployedProductId] = useState("");
  const [subject, setSubject] = useState("");
  // Once the engineer edits the subject we stop auto-regenerating it, so a later
  // product reselect doesn't clobber their text.
  const [subjectEdited, setSubjectEdited] = useState(false);
  const [description, setDescription] = useState("");
  const [attachments, setAttachments] = useState<EncodedAttachment[]>([]);

  const deployments = useSearchDeployments(projectId || undefined);
  const deployedProducts = useDeployedProductOptions(deploymentId || undefined);
  const postCase = usePostCsmCase();
  const [submitting, setSubmitting] = useState(false);

  const overLimit = totalEncodedBytes(attachments) > POST_CREATE_ATTACHMENTS_MAX_ENCODED_BYTES;

  // Auto-generate the report title as "{Deployment} - {Product} - {date}" when
  // the product is chosen (both names are loaded by then), matching the customer
  // portal. Event-driven so it stays editable and avoids a setState effect.
  const regenerateSubject = (depId: string, prodId: string): void => {
    if (subjectEdited) return;
    const depName = deployments.data?.find((d) => d.id === depId)?.name;
    const prodLabel = deployedProducts.data?.find((dp) => dp.id === prodId)?.label;
    if (depName && prodLabel) {
      setSubject(`${depName} - ${prodLabel} - ${todayStamp()}`.slice(0, 200));
    }
  };

  // Cascade resets: a parent change invalidates its children.
  const onProjectChange = (next: string): void => {
    setProjectId(next);
    setDeploymentId("");
    setDeployedProductId("");
  };
  const onDeploymentChange = (next: string): void => {
    setDeploymentId(next);
    setDeployedProductId("");
  };
  const onDeployedProductChange = (next: string): void => {
    setDeployedProductId(next);
    regenerateSubject(deploymentId, next);
  };

  const hasOptionsError = deployments.isError || deployedProducts.isError;
  const retryOptions = (): void => {
    if (deployments.isError) void deployments.refetch();
    if (deployedProducts.isError) void deployedProducts.refetch();
  };

  // At least one attachment is required here: the backing service validates
  // the create request atomically for this case type, so an attachment added
  // only after creation would never satisfy it.
  const canSubmit = useMemo(
    () =>
      !!projectId &&
      !!deploymentId &&
      !!deployedProductId &&
      subject.trim().length > 0 &&
      !isEmptyHtml(description) &&
      attachments.length > 0 &&
      !overLimit &&
      !submitting,
    [
      projectId,
      deploymentId,
      deployedProductId,
      subject,
      description,
      attachments,
      overLimit,
      submitting,
    ],
  );

  const handleSubmit = async (): Promise<void> => {
    if (!canSubmit) return;
    setSubmitting(true);
    try {
      const created = await postCase.mutateAsync({
        type: "security_report_analysis",
        projectId,
        deploymentId,
        deployedProductId,
        subject: subject.trim(),
        description,
        attachments: attachments.map((a) => ({ name: a.name, file: a.file })),
      });
      navigate(`/security-center/security-reports/${created.id}`, {
        state: { from: backTarget },
      });
    } catch (err) {
      setSubmitting(false);
      // The backend surfaces real validation messages on 4xx; show them.
      const msg =
        err instanceof BackendApiError && err.status < 500 && err.message
          ? err.message
          : "Could not create the security report. Please try again.";
      showError(msg, err);
    }
  };

  return (
    <Box sx={{ width: "100%", px: 3, py: 3 }}>
      <Button
        variant="text"
        startIcon={<ArrowLeft size={16} />}
        onClick={() => navigate(backTarget)}
        sx={{ mb: 1 }}
      >
        Back
      </Button>
      <Typography variant="h5" sx={{ mb: 2 }}>
        New security report
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
            <ProjectSelectionField
              value={projectId}
              onChange={onProjectChange}
              lockedProjectId={lockedProjectId}
              required
            />
          </Grid>

          <Grid size={{ xs: 12, md: 4 }}>
            <FormControl fullWidth size="small" required>
              <InputLabel
                id="sra-deployment-label"
                shrink={deploymentId !== ""}
                sx={{ top: "0px !important" }}
              >
                Deployment
              </InputLabel>
              <Select
                labelId="sra-deployment-label"
                label="Deployment"
                value={deploymentId}
                onChange={(e) => onDeploymentChange(String(e.target.value))}
                disabled={!projectId || deployments.isLoading}
                notched={deploymentId !== ""}
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
              <InputLabel
                id="sra-product-label"
                shrink={deployedProductId !== ""}
                sx={{ top: "0px !important" }}
              >
                Deployed product
              </InputLabel>
              <Select
                labelId="sra-product-label"
                label="Deployed product"
                value={deployedProductId}
                onChange={(e) => onDeployedProductChange(String(e.target.value))}
                disabled={!deploymentId || deployedProducts.isLoading}
                notched={deployedProductId !== ""}
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

          <Grid size={{ xs: 12 }}>
            <TextField
              label="Subject"
              size="small"
              fullWidth
              required
              value={subject}
              onChange={(e) => {
                setSubjectEdited(true);
                setSubject(e.target.value.slice(0, 200));
              }}
              helperText={
                subject.length >= 160
                  ? `${subject.length}/200`
                  : "Auto-generated from deployment and product; edit if needed."
              }
            />
          </Grid>

          <Grid size={{ xs: 12 }}>
            <Typography
              id="sra-description-label"
              component="label"
              variant="caption"
              color="text.secondary"
              sx={{ display: "block", mb: 0.5 }}
            >
              Description *
            </Typography>
            <Box role="group" aria-labelledby="sra-description-label">
              <Editor
                value={description}
                onChange={setDescription}
                placeholder="Describe the security report and what needs analysis…"
                minHeight={150}
                maxHeight="300px"
                toolbarVariant="full"
                disabled={submitting}
              />
            </Box>
          </Grid>

          <Grid size={{ xs: 12 }}>
            <Typography
              variant="caption"
              color="text.secondary"
              sx={{ display: "block", mb: 0.5 }}
            >
              Attachments *
            </Typography>
            <AttachmentsField
              attachments={attachments}
              onChange={setAttachments}
              onError={showError}
              maxEncodedBytes={POST_CREATE_ATTACHMENTS_MAX_ENCODED_BYTES}
              required
              disabled={submitting}
            />
          </Grid>
        </Grid>

        <Box sx={{ display: "flex", justifyContent: "flex-end", gap: 1.5, mt: 2.5 }}>
          <Button variant="outlined" onClick={() => navigate(backTarget)}>
            Cancel
          </Button>
          <Button
            variant="contained"
            onClick={() => void handleSubmit()}
            disabled={!canSubmit}
          >
            {submitting ? "Creating…" : "Create security report"}
          </Button>
        </Box>
      </Card>
    </Box>
  );
}
