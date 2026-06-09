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
  Grid,
  InputLabel,
  MenuItem,
  Select,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { ArrowLeft } from "@wso2/oxygen-ui-icons-react";
import { useMemo, useState, type JSX } from "react";
import { useNavigate } from "react-router";
import { priorityFromSeverity } from "@api/backend/mappers";
import Editor from "@components/rich-text-editor/Editor";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import { useProjectOptions } from "@features/csm-cases/api/useProjectOptions";
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

export default function CsmCaseCreatePage(): JSX.Element {
  const navigate = useNavigate();
  const { showError } = useErrorBanner();

  const [projectId, setProjectId] = useState("");
  const [deploymentId, setDeploymentId] = useState("");
  const [deployedProductId, setDeployedProductId] = useState("");
  const [severity, setSeverity] = useState<Severity | "">("");
  const [issueType, setIssueType] = useState<BeCaseIssueType | "">("");
  const [subject, setSubject] = useState("");
  const [description, setDescription] = useState("");

  const projects = useProjectOptions();
  const deployments = useSearchDeployments(projectId || undefined);
  const deployedProducts = useDeployedProductOptions(deploymentId || undefined);
  const postCase = usePostCsmCase();

  const canSubmit = useMemo(
    () =>
      !!projectId &&
      !!deploymentId &&
      !!deployedProductId &&
      !!severity &&
      !!issueType &&
      subject.trim().length > 0 &&
      !isEmptyHtml(description) &&
      !postCase.isPending,
    [
      projectId,
      deploymentId,
      deployedProductId,
      severity,
      issueType,
      subject,
      description,
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
    if (!canSubmit || !severity || !issueType) return;
    postCase.mutate(
      {
        projectId,
        deploymentId,
        deployedProductId,
        subject: subject.trim(),
        description,
        priority: priorityFromSeverity(severity),
        issueType,
      },
      {
        onSuccess: (created) => navigate(`/cases/${created.id}`),
        onError: () => showError("Could not create the case. Please try again."),
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
        <Grid container spacing={2.5}>
          <Grid size={{ xs: 12, md: 4 }}>
            <FormControl fullWidth size="small" required>
              <InputLabel id="case-project-label">Project</InputLabel>
              <Select
                labelId="case-project-label"
                label="Project"
                value={projectId}
                onChange={(e) => onProjectChange(String(e.target.value))}
                disabled={projects.isLoading}
              >
                {(projects.data ?? []).map((p) => (
                  <MenuItem key={p.id} value={p.id}>
                    {p.name ?? p.id}
                  </MenuItem>
                ))}
              </Select>
            </FormControl>
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
            />
          </Grid>
          <Grid size={{ xs: 12 }}>
            <Typography
              variant="caption"
              color="text.secondary"
              sx={{ display: "block", mb: 0.5 }}
            >
              Description
            </Typography>
            <Editor
              value={description}
              onChange={setDescription}
              placeholder="Describe the issue…"
              minHeight={180}
              maxHeight={420}
              toolbarVariant="full"
              disabled={postCase.isPending}
            />
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
