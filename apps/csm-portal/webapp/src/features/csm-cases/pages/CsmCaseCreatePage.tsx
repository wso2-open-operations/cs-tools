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
import { ArrowLeft, Lock } from "@wso2/oxygen-ui-icons-react";
import { useMemo, useState, type JSX } from "react";
import { useNavigate, useSearchParams } from "react-router";
import { priorityFromSeverity } from "@api/backend/mappers";
import { formatBytes } from "@utils/formatBytes";
import Editor from "@components/rich-text-editor/Editor";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import AsyncProjectSelect from "@features/csm-cases/components/AsyncProjectSelect";
import { useGetProject } from "@features/csm-projects/api/useGetProject";
import { useSearchDeployments } from "@features/csm-cases/api/useSearchDeployments";
import { useDeployedProductOptions } from "@features/csm-cases/api/useDeployedProductOptions";
import { usePostCsmCase } from "@features/csm-cases/api/usePostCsmCase";
import { SEVERITY_LABEL } from "@features/csm-dashboard/utils/abtDashboard";
import type { Severity } from "@features/csm-dashboard/types/abtDashboard";
import type { BeCaseIssueType } from "@api/backend/types";

const SEVERITIES: Severity[] = ["S0", "S1", "S2", "S3", "S4"];

const ISSUE_TYPES: { value: BeCaseIssueType; label: string }[] = [
  { value: "total_outage", label: "Total outage" },
  { value: "partial_outage", label: "Partial outage" },
  { value: "performance_degradation", label: "Performance degradation" },
  { value: "error", label: "Error" },
  { value: "security_or_compliance", label: "Security / compliance" },
  { value: "question", label: "Question" },
];

/** The rich-text editor emits `<p></p>` when empty; check the stripped text. */
function isEmptyHtml(html: string): boolean {
  return html.replace(/<[^>]*>/g, "").replace(/&nbsp;/g, " ").trim().length === 0;
}

// Cap the case-create body, which carries the description HTML (with base64
// inline images). FOLLOW-UP: the CSM backend currently caps POST /cases at
// 1 MiB (maxRequestBodyBytes); this 10 MiB FE cap assumes the endpoint is
// raised to match the comment endpoint (maxCommentBodyBytes = 10 MiB). Until
// the BE matches, it still returns 413 past 1 MiB.
const MAX_DESCRIPTION_BODY_BYTES = 10 * 1024 * 1024;
// Reserve headroom for the other create fields (ids, subject) + JSON envelope
// so the FE blocks before the BE rejects.
const MAX_DESCRIPTION_CONTENT_BYTES = MAX_DESCRIPTION_BODY_BYTES - 4 * 1024;

export default function CsmCaseCreatePage(): JSX.Element {
  const navigate = useNavigate();
  const { showError } = useErrorBanner();

  // When the form is opened from a project's page (`/cases/new?projectId=…`),
  // the project is fixed and shown read-only: the engineer can't accidentally
  // file the case against the wrong project. The id seeds the form's project
  // state once and the picker is replaced by a locked field. Opened without the
  // param (the cases-list "New case" entry), the searchable picker is shown.
  const [searchParams] = useSearchParams();
  const lockedProjectId = searchParams.get("projectId") ?? "";
  const isProjectLocked = !!lockedProjectId;

  const [projectId, setProjectId] = useState(lockedProjectId);
  const [deploymentId, setDeploymentId] = useState("");
  const [deployedProductId, setDeployedProductId] = useState("");
  const [severity, setSeverity] = useState<Severity | "">("");
  const [issueType, setIssueType] = useState<BeCaseIssueType | "">("");
  const [subject, setSubject] = useState("");
  const [description, setDescription] = useState("");

  const deployments = useSearchDeployments(projectId || undefined);
  const deployedProducts = useDeployedProductOptions(deploymentId || undefined);
  const postCase = usePostCsmCase();

  // Resolve the locked project's display name so the read-only field shows the
  // name (not the raw id). Only fetched when the form is project-scoped.
  const lockedProject = useGetProject(isProjectLocked ? lockedProjectId : undefined);
  const lockedProjectLabel = lockedProject.data?.name
    ? lockedProject.data.name
    : lockedProject.isLoading
      ? "Loading project…"
      : lockedProjectId;

  // When a selector query fails the form becomes non-completable, so surface it
  // with an explicit retry rather than leaving an empty dropdown. (Projects are
  // searched on demand by AsyncProjectSelect, which reports its own state.)
  const hasOptionsError = deployments.isError || deployedProducts.isError;
  const retryOptions = (): void => {
    if (deployments.isError) void deployments.refetch();
    if (deployedProducts.isError) void deployedProducts.refetch();
  };

  // UTF-8 byte size of the description; the BE caps the whole create body, so
  // mirror it here to fail fast with a clear message instead of a 413.
  const descriptionBytes = useMemo(
    () => new TextEncoder().encode(description).length,
    [description],
  );
  const descriptionOverLimit = descriptionBytes > MAX_DESCRIPTION_CONTENT_BYTES;
  const descriptionError = descriptionOverLimit
    ? `The case description is too large (${formatBytes(
        descriptionBytes,
      )}). Maximum is ${formatBytes(
        MAX_DESCRIPTION_BODY_BYTES,
      )} — reduce the size or the number of inline images and try again.`
    : null;

  const canSubmit = useMemo(
    () =>
      !!projectId &&
      !!deploymentId &&
      !!deployedProductId &&
      !!severity &&
      !!issueType &&
      subject.trim().length > 0 &&
      !isEmptyHtml(description) &&
      !descriptionOverLimit &&
      !postCase.isPending,
    [
      projectId,
      deploymentId,
      deployedProductId,
      severity,
      issueType,
      subject,
      description,
      descriptionOverLimit,
      postCase.isPending,
    ],
  );

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

  const handleSubmit = (): void => {
    if (!canSubmit || !severity || !issueType || descriptionOverLimit) return;
    postCase.mutate(
      {
        type: "case",
        projectId,
        deploymentId,
        deployedProductId,
        subject: subject.trim(),
        description,
        severity: priorityFromSeverity(severity),
        issueType: issueType,
      },
      {
        onSuccess: (created) => navigate(`/cases/${created.id}`),
        onError: (err) =>
          showError("Could not create the case. Please try again.", err),
      },
    );
  };

  return (
    <Box sx={{ width: "100%", px: 3, py: 3 }}>
      <Button
        variant="text"
        startIcon={<ArrowLeft size={16} />}
        onClick={() => navigate("/cases")}
        sx={{ mb: 1 }}
      >
        Back to cases
      </Button>
      <Typography variant="h5" sx={{ mb: 2 }}>
        New case
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
            {isProjectLocked ? (
              <TextField
                fullWidth
                size="small"
                label="Project"
                required
                value={lockedProjectLabel}
                slotProps={{
                  input: {
                    readOnly: true,
                    endAdornment: (
                      <Lock size={16} aria-hidden style={{ opacity: 0.6 }} />
                    ),
                  },
                  htmlInput: { "aria-readonly": true },
                }}
                helperText="Locked to the project you opened this from. To file against another project, open that project first."
              />
            ) : (
              <AsyncProjectSelect
                value={projectId}
                onChange={onProjectChange}
                required
              />
            )}
          </Grid>

          <Grid size={{ xs: 12, md: 4 }}>
            <FormControl fullWidth size="small" required>
              <InputLabel id="case-deployment-label">Deployment</InputLabel>
              <Select
                labelId="case-deployment-label"
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
              ) : null}
            </FormControl>
          </Grid>

          <Grid size={{ xs: 12, md: 4 }}>
            <FormControl fullWidth size="small" required>
              <InputLabel id="case-product-label">Deployed product</InputLabel>
              <Select
                labelId="case-product-label"
                label="Deployed product"
                value={deployedProductId}
                onChange={(e) => setDeployedProductId(String(e.target.value))}
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
              ) : null}
            </FormControl>
          </Grid>

          <Grid size={{ xs: 12, sm: 6, md: 3 }}>
            <FormControl fullWidth size="small" required>
              <InputLabel id="case-severity-label">Severity</InputLabel>
              <Select
                labelId="case-severity-label"
                label="Severity"
                value={severity}
                onChange={(e) => setSeverity(e.target.value as Severity)}
              >
                {SEVERITIES.map((s) => (
                  <MenuItem key={s} value={s}>
                    {s} · {SEVERITY_LABEL[s]}
                  </MenuItem>
                ))}
              </Select>
            </FormControl>
          </Grid>

          <Grid size={{ xs: 12, sm: 6, md: 3 }}>
            <FormControl fullWidth size="small" required>
              <InputLabel id="case-issue-type-label">Issue type</InputLabel>
              <Select
                labelId="case-issue-type-label"
                label="Issue type"
                value={issueType}
                onChange={(e) => setIssueType(e.target.value as BeCaseIssueType)}
              >
                {ISSUE_TYPES.map((it) => (
                  <MenuItem key={it.value} value={it.value}>
                    {it.label}
                  </MenuItem>
                ))}
              </Select>
            </FormControl>
          </Grid>

          <Grid size={{ xs: 12 }}>
            <TextField
              label="Subject"
              size="small"
              fullWidth
              required
              value={subject}
              onChange={(e) => setSubject(e.target.value.slice(0, 200))}
              helperText={
                subject.length >= 160 ? `${subject.length}/200` : undefined
              }
            />
          </Grid>
          <Grid size={{ xs: 12 }}>
            <Typography
              id="case-description-label"
              component="label"
              variant="caption"
              color="text.secondary"
              sx={{ display: "block", mb: 0.5 }}
            >
              Description
            </Typography>
            {/* Editor doesn't accept an `id`, so associate the label by wrapping
                the editor in a labelled group for assistive tech. */}
            <Box role="group" aria-labelledby="case-description-label">
              <Editor
                value={description}
                onChange={setDescription}
                placeholder="Describe the issue…"
                minHeight={180}
                maxHeight={420}
                toolbarVariant="full"
                disabled={postCase.isPending}
              />
            </Box>
            {descriptionError && (
              <Typography
                variant="caption"
                color="error"
                sx={{ display: "block", mt: 0.5 }}
              >
                {descriptionError}
              </Typography>
            )}
          </Grid>
        </Grid>

        <Box sx={{ display: "flex", justifyContent: "flex-end", gap: 1.5, mt: 2.5 }}>
          <Button variant="outlined" onClick={() => navigate("/cases")}>
            Cancel
          </Button>
          <Button
            variant="contained"
            onClick={handleSubmit}
            disabled={!canSubmit}
          >
            {postCase.isPending ? "Creating…" : "Create case"}
          </Button>
        </Box>
      </Card>
    </Box>
  );
}
