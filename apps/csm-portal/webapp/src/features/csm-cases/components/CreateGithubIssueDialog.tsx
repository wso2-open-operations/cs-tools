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
  InputLabel,
  MenuItem,
  Select,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { useState, type JSX } from "react";
import type { BeCreateCaseGithubIssuePayload } from "@api/backend/types";

// ---------------------------------------------------------------------------
// Option lists. Every select starts unset ("" → "-- Select --") and omits its
// field from the payload when left unset. Values mirror the legacy SN "Open Git
// Issue" form: Type is a GitHub label string, priority is only meaningful for
// incidents (the SN side applies it as a label only when Type is Incident).
// ---------------------------------------------------------------------------

const UNSET = "";
const SELECT_PLACEHOLDER = "-- Select --";

const TYPE_OPTIONS: Array<{ value: string; label: string }> = [
  { value: "Type/Query", label: "Query" },
  { value: "Type/Incident", label: "Incident" },
  { value: "Type/Patch", label: "Patch" },
];

const SEVERITY_OPTIONS: Array<{ value: string; label: string }> = [
  { value: "P1", label: "P1 - Critical" },
  { value: "P2", label: "P2 - High" },
  { value: "P3", label: "P3 - Medium" },
];

// Cloud-case repositories. owner is fixed to wso2-enterprise; the value is the
// repo. Sent as repoOverride to bypass the SN product-unit routing (which only
// covers on-prem/product-unit-mapped cases).
const REPO_OWNER = "wso2-enterprise";
const REPO_OPTIONS: Array<{ value: string; label: string }> = [
  { value: "asgardeo-product", label: "Asgardeo" },
  { value: "choreo", label: "WSO2 Developer Platform (Choreo)" },
  { value: "wso2-apim-internal", label: "Bijira / API Manager" },
  { value: "wso2-integration-internal", label: "Devant / Integration" },
];

const YES_NO_OPTIONS: Array<{ value: string; label: string }> = [
  { value: "yes", label: "Yes" },
  { value: "no", label: "No" },
];

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export interface CreateGithubIssueDialogProps {
  open: boolean;
  submitting: boolean;
  /** Backend error message to surface inline (cleared by the parent on retry). */
  error: string | null;
  /** Prefill for the update-level field, taken from the case's product context. */
  defaultUpdateLevel?: string;
  /** Prefill for the Summary field, taken from the case's subject. */
  defaultTitle?: string;
  /** Prefill for the Description field, taken from the case's description. */
  defaultDescription?: string;
  onClose: () => void;
  /** Body for `POST /cases/{id}/github-issues` (caseId is added by the caller). */
  onSubmit: (payload: BeCreateCaseGithubIssuePayload) => void;
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

/**
 * Form for filing an internal GitHub issue from a case (ISSU-020). Mirrors the
 * legacy ServiceNow "Open Git Issue" form: Summary + Description are required;
 * everything else is optional. Reason is fixed to `default` (the migration /
 * R&D-ticket variants were separate SN actions). Repo selection is offered for
 * cloud cases; when unset the SN side routes by the case's product unit.
 */
export function CreateGithubIssueDialog({
  open,
  submitting,
  error,
  defaultUpdateLevel,
  defaultTitle,
  defaultDescription,
  onClose,
  onSubmit,
}: CreateGithubIssueDialogProps): JSX.Element {
  const [title, setTitle] = useState(defaultTitle ?? "");
  const [description, setDescription] = useState(defaultDescription ?? "");
  const [updateLevel, setUpdateLevel] = useState(defaultUpdateLevel ?? "");
  const [publicIssueUrl, setPublicIssueUrl] = useState("");
  const [issueTypeLabel, setIssueTypeLabel] = useState<string>(UNSET);
  const [priorityLevel, setPriorityLevel] = useState<string>(UNSET);
  const [repo, setRepo] = useState<string>(UNSET);
  const [hotFix, setHotFix] = useState<string>(UNSET);
  const [regression, setRegression] = useState<string>(UNSET);

  const resetAndClose = () => {
    setTitle(defaultTitle ?? "");
    setDescription(defaultDescription ?? "");
    setUpdateLevel(defaultUpdateLevel ?? "");
    setPublicIssueUrl("");
    setIssueTypeLabel(UNSET);
    setPriorityLevel(UNSET);
    setRepo(UNSET);
    setHotFix(UNSET);
    setRegression(UNSET);
    onClose();
  };

  const canSubmit =
    title.trim().length > 0 && description.trim().length > 0;

  const handleSubmit = () => {
    if (!canSubmit) return;

    const payload: BeCreateCaseGithubIssuePayload = {
      reason: "default",
      title: title.trim(),
      description: description.trim(),
    };
    if (updateLevel.trim()) payload.updateLevel = updateLevel.trim();
    if (publicIssueUrl.trim()) payload.publicIssueUrl = publicIssueUrl.trim();
    if (issueTypeLabel) payload.issueTypeLabel = issueTypeLabel;
    // Priority only carries meaning for incidents on the SN side; send it
    // whenever the user picked one and let the SN side decide to apply it.
    if (priorityLevel) payload.priorityLevel = priorityLevel;
    if (repo) payload.repoOverride = { owner: REPO_OWNER, repo };
    if (hotFix === "yes") payload.hotFixRequired = true;
    if (regression === "yes") payload.regression = true;

    onSubmit(payload);
  };

  // Shared renderer for a "-- Select --" dropdown.
  const renderSelect = (
    id: string,
    label: string,
    value: string,
    onChange: (v: string) => void,
    options: Array<{ value: string; label: string }>,
  ): JSX.Element => (
    <FormControl fullWidth size="small" disabled={submitting}>
      <InputLabel id={`${id}-label`} shrink>
        {label}
      </InputLabel>
      <Select
        labelId={`${id}-label`}
        label={label}
        value={value}
        displayEmpty
        onChange={(e) => onChange(String(e.target.value))}
      >
        <MenuItem value={UNSET}>
          <Typography component="span" color="text.secondary">
            {SELECT_PLACEHOLDER}
          </Typography>
        </MenuItem>
        {options.map((o) => (
          <MenuItem key={o.value} value={o.value}>
            {o.label}
          </MenuItem>
        ))}
      </Select>
    </FormControl>
  );

  return (
    <Dialog open={open} onClose={resetAndClose} maxWidth="sm" fullWidth>
      <DialogTitle>Open Git issue</DialogTitle>
      <DialogContent>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2, pt: 0.5 }}>
          {error && (
            <Typography variant="body2" color="error">
              {error}
            </Typography>
          )}

          <TextField
            label="Summary"
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            fullWidth
            required
            disabled={submitting}
            placeholder="Short summary of the problem"
          />

          <TextField
            label="Description"
            multiline
            minRows={4}
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            fullWidth
            required
            disabled={submitting}
            placeholder="What needs to be fixed?"
            helperText="Case number, product and reporter are appended to the issue body automatically."
          />

          <TextField
            label="Update Level"
            value={updateLevel}
            onChange={(e) => setUpdateLevel(e.target.value)}
            disabled={submitting}
            size="small"
            fullWidth
          />

          <TextField
            label="Public Git Issue or Security Internal JIRA"
            value={publicIssueUrl}
            onChange={(e) => setPublicIssueUrl(e.target.value)}
            disabled={submitting}
            size="small"
            fullWidth
            placeholder="https://github.com/… or JIRA link"
          />

          {renderSelect("ghi-type", "Type", issueTypeLabel, setIssueTypeLabel, TYPE_OPTIONS)}

          {renderSelect("ghi-severity", "Severity", priorityLevel, setPriorityLevel, SEVERITY_OPTIONS)}

          {renderSelect(
            "ghi-repo",
            "Choose repository (only for cloud cases)",
            repo,
            setRepo,
            REPO_OPTIONS,
          )}

          {renderSelect("ghi-hotfix", "Hotfix Required", hotFix, setHotFix, YES_NO_OPTIONS)}

          {renderSelect("ghi-regression", "Regression", regression, setRegression, YES_NO_OPTIONS)}
        </Box>
      </DialogContent>
      <DialogActions>
        <Button color="inherit" onClick={resetAndClose} disabled={submitting}>
          Cancel
        </Button>
        <Button
          variant="contained"
          onClick={handleSubmit}
          disabled={!canSubmit || submitting}
          loading={submitting}
        >
          Create issue
        </Button>
      </DialogActions>
    </Dialog>
  );
}
