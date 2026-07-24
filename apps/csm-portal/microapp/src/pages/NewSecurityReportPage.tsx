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

import { useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import {
  Button,
  FormControl,
  FormHelperText,
  InputLabel,
  MenuItem,
  Select,
  Stack,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { useMutation, useQuery } from "@tanstack/react-query";
import { cases } from "@src/services/cases";
import { deployments } from "@src/services/deployments";
import type { Project, SecurityReportCreatePayloadDto } from "@src/types";
import { toRawBase64, type PendingAttachment } from "@utils/attachments";
import { AttachmentsField } from "@components/support/AttachmentsField";
import { ProjectSelect } from "@components/support/ProjectSelect";

const SUBJECT_MAX_LENGTH = 200;

// POST /cases caps the whole body at 10 MiB (backend's maxCaseBodyBytes) — unlike NewCasePage's
// attachments (uploaded separately, one request per file after the case exists), a security
// report's attachments are embedded directly in this same create request alongside subject and
// description, so the combined size has to stay under the same cap the backend enforces.
const MAX_BODY_BYTES = 10 * 1024 * 1024;

/** Today as YYYY-MM-DD, for the auto-generated report subject. */
function todayStamp(): string {
  const d = new Date();
  const mm = String(d.getMonth() + 1).padStart(2, "0");
  const dd = String(d.getDate()).padStart(2, "0");
  return `${d.getFullYear()}-${mm}-${dd}`;
}

// Ports apps/csm-portal/webapp/src/features/csm-security-center/pages/CreateSecurityReportPage.tsx.
// Deviations from the webapp, called out where they matter: description is a plain multiline field
// rather than the rich-text editor (same call NewCasePage already made), and there's no
// cloud-support-project special case for the deployment picker — the webapp's own reference page
// doesn't have one either, unlike NewCasePage's "case" flow.
export default function NewSecurityReportPage() {
  const navigate = useNavigate();

  const [project, setProject] = useState<Project | null>(null);
  const [deploymentId, setDeploymentId] = useState("");
  const [deployedProductId, setDeployedProductId] = useState("");
  const [subject, setSubject] = useState("");
  // Once the user edits the subject we stop auto-regenerating it, so a later product reselect
  // doesn't clobber their text.
  const [subjectEdited, setSubjectEdited] = useState(false);
  const [description, setDescription] = useState("");
  const [attachments, setAttachments] = useState<PendingAttachment[]>([]);
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);

  const deploymentsQuery = useQuery(deployments.byProject(project?.id ?? ""));
  const productsQuery = useQuery(deployments.productsByDeployment(deploymentId));

  const createSecurityReport = useMutation({ mutationFn: cases.create });

  // The exact object handleSubmit sends — reused for the size check below so the two can never
  // drift apart.
  const buildPayload = (): SecurityReportCreatePayloadDto => ({
    type: "security_report_analysis",
    projectId: project?.id ?? "",
    deploymentId,
    deployedProductId,
    subject: subject.trim(),
    description,
    attachments: attachments.map((a) => ({ name: a.name, file: toRawBase64(a.file) })),
  });

  // Measures the real wire size of the actual payload (JSON structure, field names, IDs, filenames,
  // and all) rather than approximating from subject/description bytes plus a flat overhead buffer —
  // that approximation undercounts the per-attachment JSON overhead (braces, commas, quoted
  // filenames), which could let a submission with several small attachments slip past the check
  // and still 413 against the backend's real cap.
  const payloadBytes = useMemo(
    () => new TextEncoder().encode(JSON.stringify(buildPayload())).length,
    // eslint-disable-next-line react-hooks/exhaustive-deps -- buildPayload is a plain closure over these same primitives, not memoized itself
    [project, deploymentId, deployedProductId, subject, description, attachments],
  );
  const overLimit = payloadBytes > MAX_BODY_BYTES;

  // Auto-generate "{Deployment} - {Product} - {date}" once both names are loaded, matching the
  // webapp. Event-driven (fired from the product-change handler) rather than a setState effect, so
  // it stays editable and doesn't fight the subjectEdited guard.
  const regenerateSubject = (depId: string, prodId: string): void => {
    if (subjectEdited) return;
    const depName = deploymentsQuery.data?.find((d) => d.id === depId)?.name;
    const prodLabel = productsQuery.data?.find((p) => p.id === prodId)?.label;
    if (depName && prodLabel) {
      setSubject(`${depName} - ${prodLabel} - ${todayStamp()}`.slice(0, SUBJECT_MAX_LENGTH));
    }
  };

  const handleProjectChange = (next: Project | null) => {
    setProject(next);
    setDeploymentId("");
    setDeployedProductId("");
  };

  const handleDeploymentChange = (next: string) => {
    setDeploymentId(next);
    setDeployedProductId("");
  };

  const handleProductChange = (next: string) => {
    setDeployedProductId(next);
    regenerateSubject(deploymentId, next);
  };

  const canSubmit =
    !!project &&
    !!deploymentId &&
    !!deployedProductId &&
    subject.trim().length > 0 &&
    description.trim().length > 0 &&
    attachments.length > 0 &&
    !overLimit &&
    !createSecurityReport.isPending &&
    !isSubmitting;

  const handleSubmit = async () => {
    if (!canSubmit || !project) return;
    setSubmitError(null);
    setIsSubmitting(true);

    try {
      const created = await createSecurityReport.mutateAsync(buildPayload());

      navigate(`/cases/${created.id}`);
    } catch {
      setSubmitError("Could not create the security report. Please try again.");
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <Stack gap={2}>
      <Typography variant="h6">New Security Report</Typography>

      <Stack gap={2}>
        <ProjectSelect value={project} onChange={handleProjectChange} disabled={createSecurityReport.isPending} />

        <FormControl size="small" fullWidth disabled={!project || createSecurityReport.isPending}>
          <InputLabel id="new-sr-deployment-label">Deployment</InputLabel>
          <Select
            labelId="new-sr-deployment-label"
            label="Deployment"
            value={deploymentId}
            onChange={(event) => handleDeploymentChange(event.target.value)}
          >
            {(deploymentsQuery.data ?? []).map((d) => (
              <MenuItem key={d.id} value={d.id}>
                {d.name}
              </MenuItem>
            ))}
          </Select>
          {!project && <FormHelperText>Select a project first</FormHelperText>}
        </FormControl>

        <FormControl size="small" fullWidth disabled={!deploymentId || createSecurityReport.isPending}>
          <InputLabel id="new-sr-product-label">Deployed Product</InputLabel>
          <Select
            labelId="new-sr-product-label"
            label="Deployed Product"
            value={deployedProductId}
            onChange={(event) => handleProductChange(event.target.value)}
          >
            {(productsQuery.data ?? []).map((p) => (
              <MenuItem key={p.id} value={p.id}>
                {p.label}
              </MenuItem>
            ))}
          </Select>
          {!deploymentId && <FormHelperText>Select a deployment first</FormHelperText>}
        </FormControl>

        <TextField
          label="Subject"
          value={subject}
          onChange={(event) => {
            setSubjectEdited(true);
            setSubject(event.target.value.slice(0, SUBJECT_MAX_LENGTH));
          }}
          size="small"
          fullWidth
          disabled={createSecurityReport.isPending}
          helperText={
            subject.length >= 160
              ? `${subject.length}/${SUBJECT_MAX_LENGTH}`
              : "Auto-generated from deployment and product; edit if needed."
          }
        />

        <TextField
          label="Description"
          value={description}
          onChange={(event) => setDescription(event.target.value)}
          size="small"
          fullWidth
          multiline
          minRows={5}
          disabled={createSecurityReport.isPending}
        />

        <AttachmentsField
          attachments={attachments}
          onChange={setAttachments}
          disabled={createSecurityReport.isPending}
          label="Attachments (required)"
        />
        {overLimit && (
          <Typography variant="body2" color="error.main">
            Attachments are too large for a single security report. Remove one or reduce their size.
          </Typography>
        )}

        {submitError && (
          <Typography variant="body2" color="error.main">
            {submitError}
          </Typography>
        )}

        <Stack direction="row" gap={1} justifyContent="flex-end">
          <Button onClick={() => navigate(-1)} disabled={createSecurityReport.isPending}>
            Cancel
          </Button>
          <Button variant="contained" disabled={!canSubmit} onClick={() => void handleSubmit()}>
            {isSubmitting ? "Creating…" : "Create Security Report"}
          </Button>
        </Stack>
      </Stack>
    </Stack>
  );
}
