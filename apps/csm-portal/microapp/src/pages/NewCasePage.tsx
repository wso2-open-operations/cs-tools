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
import { attachments as attachmentsService } from "@src/services/attachments";
import { isCloudSupportProject, type CaseIssueType, type CaseSeverity, type Project } from "@src/types";
import { Logger } from "@utils/logger";
import { AttachmentsField } from "@components/support/AttachmentsField";
import { ProjectSelect } from "@components/support/ProjectSelect";
import { ALL_SEVERITIES, ISSUE_TYPE_LABELS, SEVERITY_LABELS } from "@components/support/config";
import type { PendingAttachment } from "@utils/attachments";

const ISSUE_TYPES: CaseIssueType[] = [
  "total_outage",
  "partial_outage",
  "performance_degradation",
  "error",
  "security_or_compliance",
  "question",
];

const SUBJECT_MAX_LENGTH = 200;

// Ports apps/csm-portal/webapp/src/features/csm-cases/pages/CsmCaseCreatePage.tsx (the "case"
// type only — service_request/security_report_analysis creation aren't ported, see the New Case
// research notes). Deviations from the webapp, called out where they matter: description is a
// plain multiline field rather than the rich-text editor, and there's no locked-project
// (?projectId=) entry point since this mobile app has no project pages to link from.
export default function NewCasePage() {
  const navigate = useNavigate();

  const [project, setProject] = useState<Project | null>(null);
  const [deploymentId, setDeploymentId] = useState("");
  const [deployedProductId, setDeployedProductId] = useState("");
  const [severity, setSeverity] = useState<CaseSeverity | "">("");
  const [issueType, setIssueType] = useState<CaseIssueType | "">("");
  const [subject, setSubject] = useState("");
  const [description, setDescription] = useState("");
  const [attachments, setAttachments] = useState<PendingAttachment[]>([]);
  const [submitError, setSubmitError] = useState<string | null>(null);
  // createCase.isPending alone only covers the case-creation call — it flips back to false while
  // the attachment uploads below are still in flight, which would let the button re-enable and
  // fire a duplicate submission. Tracks the whole handleSubmit lifecycle instead.
  const [isSubmitting, setIsSubmitting] = useState(false);

  const isCloudProject = project ? isCloudSupportProject(project) : false;

  const deploymentsQuery = useQuery(deployments.byProject(project?.id ?? ""));
  const primaryProductionDeployments = useMemo(
    () => (deploymentsQuery.data ?? []).filter((d) => d.type === "primary_production"),
    [deploymentsQuery.data],
  );
  // Cloud-support projects have exactly one deployment; hide the picker and derive it, same as
  // the webapp's cloud-project special case.
  const effectiveDeploymentId = isCloudProject ? (primaryProductionDeployments[0]?.id ?? "") : deploymentId;

  const productsQuery = useQuery(deployments.productsByDeployment(effectiveDeploymentId));

  const createCase = useMutation({ mutationFn: cases.create });
  const uploadAttachment = useMutation({ mutationFn: attachmentsService.create });

  const handleProjectChange = (next: Project | null) => {
    setProject(next);
    setDeploymentId("");
    setDeployedProductId("");
  };

  const handleDeploymentChange = (next: string) => {
    setDeploymentId(next);
    setDeployedProductId("");
  };

  const canSubmit =
    !!project &&
    !!effectiveDeploymentId &&
    !!deployedProductId &&
    !!severity &&
    !!issueType &&
    subject.trim().length > 0 &&
    description.trim().length > 0 &&
    !createCase.isPending &&
    !isSubmitting;

  const handleSubmit = async () => {
    if (!canSubmit || !project || !severity || !issueType) return;
    setSubmitError(null);
    setIsSubmitting(true);

    try {
      const created = await createCase.mutateAsync({
        type: "case",
        projectId: project.id,
        deploymentId: effectiveDeploymentId,
        deployedProductId,
        subject: subject.trim(),
        description,
        severity,
        issueType,
      });

      // Attachments upload separately after the case exists, same as the webapp — a partial
      // failure here still lands the case rather than blocking on it.
      const results = await Promise.allSettled(
        attachments.map((attachment) =>
          uploadAttachment.mutateAsync({
            referenceId: created.id,
            referenceType: "case",
            name: attachment.name,
            type: attachment.type,
            file: attachment.file,
          }),
        ),
      );
      const failedCount = results.filter((r) => r.status === "rejected").length;
      if (failedCount > 0) {
        Logger.warn(`${failedCount} attachment(s) failed to upload to case ${created.id}`);
      }

      navigate(`/cases/${created.id}`);
    } catch {
      setSubmitError("Could not create the case. Please try again.");
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <Stack gap={2}>
      <Typography variant="h6">New Case</Typography>

      <Stack gap={2}>
        <ProjectSelect value={project} onChange={handleProjectChange} disabled={createCase.isPending} />

        {!isCloudProject && (
          <FormControl size="small" fullWidth disabled={!project || createCase.isPending}>
            <InputLabel id="new-case-deployment-label">Deployment</InputLabel>
            <Select
              labelId="new-case-deployment-label"
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
        )}

        <FormControl size="small" fullWidth disabled={!effectiveDeploymentId || createCase.isPending}>
          <InputLabel id="new-case-product-label">Deployed Product</InputLabel>
          <Select
            labelId="new-case-product-label"
            label="Deployed Product"
            value={deployedProductId}
            onChange={(event) => setDeployedProductId(event.target.value)}
          >
            {(productsQuery.data ?? []).map((p) => (
              <MenuItem key={p.id} value={p.id}>
                {p.label}
              </MenuItem>
            ))}
          </Select>
          {!effectiveDeploymentId && <FormHelperText>Select a deployment first</FormHelperText>}
        </FormControl>

        <FormControl size="small" fullWidth disabled={createCase.isPending}>
          <InputLabel id="new-case-severity-label">Severity</InputLabel>
          <Select
            labelId="new-case-severity-label"
            label="Severity"
            value={severity}
            onChange={(event) => setSeverity(event.target.value as CaseSeverity)}
          >
            {ALL_SEVERITIES.map((s) => (
              <MenuItem key={s} value={s}>
                {SEVERITY_LABELS[s]}
              </MenuItem>
            ))}
          </Select>
        </FormControl>

        <FormControl size="small" fullWidth disabled={createCase.isPending}>
          <InputLabel id="new-case-issue-type-label">Issue Type</InputLabel>
          <Select
            labelId="new-case-issue-type-label"
            label="Issue Type"
            value={issueType}
            onChange={(event) => setIssueType(event.target.value as CaseIssueType)}
          >
            {ISSUE_TYPES.map((t) => (
              <MenuItem key={t} value={t}>
                {ISSUE_TYPE_LABELS[t]}
              </MenuItem>
            ))}
          </Select>
        </FormControl>

        <TextField
          label="Subject"
          value={subject}
          onChange={(event) => setSubject(event.target.value.slice(0, SUBJECT_MAX_LENGTH))}
          size="small"
          fullWidth
          disabled={createCase.isPending}
          helperText={subject.length >= 160 ? `${subject.length}/${SUBJECT_MAX_LENGTH}` : undefined}
        />

        <TextField
          label="Description"
          value={description}
          onChange={(event) => setDescription(event.target.value)}
          size="small"
          fullWidth
          multiline
          minRows={5}
          disabled={createCase.isPending}
        />

        <AttachmentsField attachments={attachments} onChange={setAttachments} disabled={createCase.isPending} />

        {submitError && (
          <Typography variant="body2" color="error.main">
            {submitError}
          </Typography>
        )}

        <Stack direction="row" gap={1} justifyContent="flex-end">
          <Button onClick={() => navigate(-1)} disabled={createCase.isPending}>
            Cancel
          </Button>
          <Button variant="contained" disabled={!canSubmit} onClick={() => void handleSubmit()}>
            {isSubmitting ? "Creating…" : "Create Case"}
          </Button>
        </Stack>
      </Stack>
    </Stack>
  );
}
