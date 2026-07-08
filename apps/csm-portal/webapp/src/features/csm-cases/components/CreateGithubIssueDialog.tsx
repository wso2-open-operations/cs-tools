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
  Accordion,
  AccordionDetails,
  AccordionSummary,
  Box,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControl,
  FormControlLabel,
  InputLabel,
  Link,
  MenuItem,
  Select,
  Switch,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { CheckCircle, ChevronDown } from "@wso2/oxygen-ui-icons-react";
import { useState, type JSX } from "react";
import type {
  BeCreateCaseGithubIssuePayload,
  BeCreateCaseGithubIssueResponse,
} from "@api/backend/types";

// ---------------------------------------------------------------------------
// Option lists. Every select starts unset ("" → "-- Select --") and omits its
// field from the payload when left unset. Values mirror the legacy SN "Open Git
// Issue" form: Type is a GitHub label string, priority is only meaningful for
// incidents (the SN side applies it as a label only when Type is Incident).
// ---------------------------------------------------------------------------

const UNSET = "";
const SELECT_PLACEHOLDER = "-- Select --";

type IssueTypeValue = "" | "Type/Query" | "Type/Incident" | "Type/Patch";

const TYPE_OPTIONS: Array<{ value: IssueTypeValue; label: string }> = [
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

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export interface CreateGithubIssueDialogProps {
  open: boolean;
  submitting: boolean;
  /** Backend error message to surface inline (cleared by the parent on retry). */
  error: string | null;
  /** Set once the current submission has succeeded — the dialog shows a
   * "created" screen with a clickable link instead of closing immediately.
   * The parent clears this (along with `error`) when the dialog is dismissed. */
  createdIssue?: BeCreateCaseGithubIssueResponse | null;
  /** Prefill for the update-level field, taken from the case's product context. */
  defaultUpdateLevel?: string;
  /** Prefill for the Subject field, taken from the case's subject. */
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
 * legacy ServiceNow "Open Git Issue" form. Subject + Description are always
 * required; Type is required too and drives which other fields are shown/
 * required (per review on #1085):
 *   - Query: Severity and Hotfix Required are hidden.
 *   - Incident: Severity is required; Hotfix Required is hidden.
 *   - Patch: Severity is hidden; Update Level, Public Git Issue, and Hotfix
 *     Required are all required.
 * Reason is fixed to `default` (the migration / R&D-ticket variants were
 * separate SN actions). Repo selection is offered for cloud cases; when unset
 * the SN side routes by the case's product unit.
 */
export function CreateGithubIssueDialog({
  open,
  submitting,
  error,
  createdIssue,
  defaultUpdateLevel,
  defaultTitle,
  defaultDescription,
  onClose,
  onSubmit,
}: CreateGithubIssueDialogProps): JSX.Element {
  const [type, setType] = useState<IssueTypeValue>(UNSET);
  const [title, setTitle] = useState(defaultTitle ?? "");
  const [description, setDescription] = useState(defaultDescription ?? "");
  const [updateLevel, setUpdateLevel] = useState(defaultUpdateLevel ?? "");
  const [publicIssueUrl, setPublicIssueUrl] = useState("");
  const [priorityLevel, setPriorityLevel] = useState<string>(UNSET);
  const [repo, setRepo] = useState<string>(UNSET);
  const [hotFix, setHotFix] = useState(false);
  const [regression, setRegression] = useState(false);
  // Set once the user clicks "Create issue" on the form; holds the built
  // payload until they confirm on the follow-up step below. Filing this issue
  // is a real write to an external GitHub repo, so it gets an explicit
  // confirm step like the case's other hard-to-undo actions (see
  // CaseActionBar's TARGET_CONFIG.confirm).
  const [confirmPayload, setConfirmPayload] =
    useState<BeCreateCaseGithubIssuePayload | null>(null);

  // Type drives which fields apply — see the component doc comment above.
  const showSeverity = type === "Type/Incident";
  const requireSeverity = type === "Type/Incident";
  const showHotFix = type === "Type/Patch";
  const requireUpdateLevel = type === "Type/Patch";
  const requirePublicIssueUrl = type === "Type/Patch";

  const resetAndClose = () => {
    setType(UNSET);
    setTitle(defaultTitle ?? "");
    setDescription(defaultDescription ?? "");
    setUpdateLevel(defaultUpdateLevel ?? "");
    setPublicIssueUrl("");
    setPriorityLevel(UNSET);
    setRepo(UNSET);
    setHotFix(false);
    setRegression(false);
    setConfirmPayload(null);
    onClose();
  };

  const canSubmit =
    !!type &&
    title.trim().length > 0 &&
    description.trim().length > 0 &&
    (!requireSeverity || !!priorityLevel) &&
    (!requireUpdateLevel || updateLevel.trim().length > 0) &&
    (!requirePublicIssueUrl || publicIssueUrl.trim().length > 0);

  const handleSubmit = () => {
    if (!canSubmit) return;

    const payload: BeCreateCaseGithubIssuePayload = {
      reason: "default",
      title: title.trim(),
      description: description.trim(),
      issueTypeLabel: type,
    };
    if (updateLevel.trim()) payload.updateLevel = updateLevel.trim();
    if (publicIssueUrl.trim()) payload.publicIssueUrl = publicIssueUrl.trim();
    // Priority only carries meaning for incidents on the SN side; send it
    // whenever the user picked one and let the SN side decide to apply it.
    if (priorityLevel) payload.priorityLevel = priorityLevel;
    if (repo) payload.repoOverride = { owner: REPO_OWNER, repo };
    if (showHotFix && hotFix) payload.hotFixRequired = true;
    if (regression) payload.regression = true;

    setConfirmPayload(payload);
  };

  const repoLabel = REPO_OPTIONS.find((o) => o.value === repo)?.label;

  // Shared renderer for a "-- Select --" dropdown.
  const renderSelect = <V extends string>(
    id: string,
    label: string,
    value: V,
    onChange: (v: V) => void,
    options: Array<{ value: V; label: string }>,
    required?: boolean,
  ): JSX.Element => (
    <FormControl fullWidth size="small" disabled={submitting} required={required}>
      <InputLabel id={`${id}-label`} shrink>
        {label}
      </InputLabel>
      <Select
        labelId={`${id}-label`}
        label={label}
        value={value}
        displayEmpty
        onChange={(e) => onChange(e.target.value as V)}
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

          {renderSelect("ghi-type", "Type", type, setType, TYPE_OPTIONS, true)}

          <TextField
            label="Subject"
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

          {showSeverity &&
            renderSelect(
              "ghi-severity",
              "Severity",
              priorityLevel,
              setPriorityLevel,
              SEVERITY_OPTIONS,
              requireSeverity,
            )}

          <TextField
            label="Update Level"
            value={updateLevel}
            onChange={(e) => setUpdateLevel(e.target.value)}
            disabled={submitting}
            required={requireUpdateLevel}
            size="small"
            fullWidth
          />

          <TextField
            label="Public Git Issue or Security Internal JIRA"
            value={publicIssueUrl}
            onChange={(e) => setPublicIssueUrl(e.target.value)}
            disabled={submitting}
            required={requirePublicIssueUrl}
            size="small"
            fullWidth
            placeholder="https://github.com/… or JIRA link"
          />

          {showHotFix && (
            <FormControlLabel
              control={
                <Switch
                  checked={hotFix}
                  onChange={(e) => setHotFix(e.target.checked)}
                  disabled={submitting}
                />
              }
              label="Hotfix Required"
            />
          )}

          <FormControlLabel
            control={
              <Switch
                checked={regression}
                onChange={(e) => setRegression(e.target.checked)}
                disabled={submitting}
              />
            }
            label="Regression"
          />

          {/* Only the repo override stays optional across every type and is
              rarely needed (the SN side routes by product unit when unset),
              so it's the only field still tucked away by default. */}
          <Accordion disableGutters sx={{ "&:before": { display: "none" } }}>
            <AccordionSummary expandIcon={<ChevronDown size={16} />}>
              <Typography variant="body2" color="text.secondary">
                More options (optional)
              </Typography>
            </AccordionSummary>
            <AccordionDetails
              sx={{ display: "flex", flexDirection: "column", gap: 2 }}
            >
              {renderSelect(
                "ghi-repo",
                "Choose repository (only for cloud cases)",
                repo,
                setRepo,
                REPO_OPTIONS,
              )}
            </AccordionDetails>
          </Accordion>
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

      {/* Filing a GitHub issue is a real write to an external repo, so it
          gets its own explicit confirm step. Once createdIssue is set (the
          submission succeeded), the same dialog switches to a success view
          with a clickable link instead of closing itself. */}
      <Dialog
        open={!!confirmPayload}
        onClose={() => setConfirmPayload(null)}
        maxWidth="xs"
        fullWidth
      >
        {createdIssue ? (
          <>
            <DialogTitle>
              <Box sx={{ display: "flex", alignItems: "center", gap: 1, color: "success.main" }}>
                <CheckCircle size={20} />
                <Box component="span" sx={{ color: "text.primary" }}>
                  Issue created
                </Box>
              </Box>
            </DialogTitle>
            <DialogContent>
              {createdIssue.issue?.url ? (
                <Typography variant="body2">
                  Filed as{" "}
                  <Link href={createdIssue.issue.url} target="_blank" rel="noopener noreferrer">
                    {createdIssue.issue.repo && createdIssue.issue.number
                      ? `${createdIssue.issue.repo}#${createdIssue.issue.number}`
                      : createdIssue.issue.url}
                  </Link>
                  .
                </Typography>
              ) : (
                <Typography variant="body2">
                  {createdIssue.message ?? "The issue was created."}
                </Typography>
              )}
            </DialogContent>
            <DialogActions>
              <Button variant="contained" onClick={resetAndClose}>
                Done
              </Button>
            </DialogActions>
          </>
        ) : (
          <>
            <DialogTitle>File this GitHub issue?</DialogTitle>
            <DialogContent>
              <Typography variant="body2">
                This files a real issue in{" "}
                {repoLabel
                  ? `wso2-enterprise/${repo} (${repoLabel})`
                  : "a WSO2 product repository, routed automatically by the case's product"}
                . Make sure no sensitive information is included.
              </Typography>
              {/* Errors surface here too, not just on the form step behind
                  this dialog — otherwise a failed submit leaves the user
                  looking at this confirm step with no visible reason why it
                  stopped. */}
              {error && (
                <Typography variant="body2" color="error" sx={{ mt: 1.5 }}>
                  {error}
                </Typography>
              )}
            </DialogContent>
            <DialogActions>
              <Button onClick={() => setConfirmPayload(null)} disabled={submitting}>
                Back
              </Button>
              <Button
                variant="contained"
                color="warning"
                disabled={submitting}
                loading={submitting}
                onClick={() => {
                  if (confirmPayload) onSubmit(confirmPayload);
                }}
              >
                File issue
              </Button>
            </DialogActions>
          </>
        )}
      </Dialog>
    </Dialog>
  );
}
