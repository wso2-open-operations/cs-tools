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
  FormControlLabel,
  InputLabel,
  Link,
  MenuItem,
  Select,
  Switch,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { CheckCircle } from "@wso2/oxygen-ui-icons-react";
import { useState, type JSX } from "react";
import type {
  BeCreateCaseGithubIssuePayload,
  BeCreateCaseGithubIssueResponse,
} from "@api/backend/types";
import { useGetProductRepoMapping } from "@features/csm-cases/api/useGetProductRepoMapping";

// ---------------------------------------------------------------------------
// Option lists. Every select starts unset ("" → "-- Select --"). Every issue
// is filed as a Discussion (Type/Discussion → Type/Incident on GitHub), so
// there is no type selector. Priority is sent as the GitHub priority label.
// ---------------------------------------------------------------------------

const UNSET = "";
const SELECT_PLACEHOLDER = "-- Select --";

/** Every Git issue raised from a case is a Discussion. */
const GITHUB_DISCUSSION_ISSUE_TYPE = "Type/Discussion";

const PRIORITY_P1 = "Priority/Critical(P1)";
const PRIORITY_P2 = "Priority/High(P2)";
const PRIORITY_P3 = "Priority/Medium(P3)";

const PRIORITY_OPTIONS: Array<{ value: string; label: string }> = [
  { value: PRIORITY_P1, label: "P1 - Critical" },
  { value: PRIORITY_P2, label: "P2 - High" },
  { value: PRIORITY_P3, label: "P3 - Medium" },
];

/** Justifications offered per priority. P3 needs none, so it has no entry. */
const PRIORITY_JUSTIFICATION_OPTIONS: Record<string, string[]> = {
  [PRIORITY_P1]: [
    "Immediate Product Team Attention",
    "Production Outage / Severe Degradation",
    "Security, Privacy, or Legal Exposure",
    "Go-Live / Deployment Blocker",
    "Regression from Recent Release",
  ],
  [PRIORITY_P2]: [
    "Product Bug (ETA-Based Fix)",
    "Customer Business Escalation",
    "Major Operational / Dev Blocker",
    "Project / Production Deadline",
  ],
};

const HOTFIX_REASON_OPTIONS: string[] = [
  "Customer-Requested Hotfix",
  "Critical Production Bug / Downtime",
  "Defect Blocking Operations / Development",
  "Unable to Upgrade to Latest U2",
];

const toOptions = (values: string[]): Array<{ value: string; label: string }> =>
  values.map((v) => ({ value: v, label: v }));

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
  /** Initial state of the Migration switch: true for a migration engagement. */
  defaultMigration?: boolean;
  /** Cloud cases pass true. There is no repository dropdown either way.
   * When true, Update Level and Public Git Issue are hidden. */
  showRepoField?: boolean;
  /** Deployed product name on the case. Matched to the catalogue. */
  productName?: string;
  /** True when the case's project onboarding status is In-Progress. */
  onboardingInProgress?: boolean;
  /** True while a linked project's status is still loading. Projectless cases leave this unset. */
  projectStatusPending?: boolean;
  /** True when a linked project's lookup failed. Filing stays blocked until it resolves. */
  projectStatusFailed?: boolean;
  /** Retries the project lookup after projectStatusFailed. */
  onRetryProjectStatus?: () => void;
  onClose: () => void;
  /** Body for `POST /cases/{id}/github-issues` (caseId is added by the caller). */
  onSubmit: (payload: BeCreateCaseGithubIssuePayload) => void;
  /** Fired when the confirm step is (re-)entered from "Create issue", before
   * any submission happens. The parent should clear a stale `error` here —
   * otherwise going Back from a failed confirm, editing the form, and
   * clicking "Create issue" again re-shows the previous attempt's error on
   * a confirm step for a payload that hasn't been submitted yet. */
  onOpenConfirm?: () => void;
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

/**
 * Form for filing an internal GitHub issue from a case (ISSU-020). Every issue
 * is a Discussion. Fields, in order: Subject*, Description*, Reproducing Steps,
 * Priority*, Priority Justification* (P1/P2 only, options per priority),
 * Update Level, Public Git Issue (both hidden on cloud cases, never required),
 * Hotfix Required, Hotfix Reason* (only while Hotfix Required is on),
 * Regression, Migration (on by default for a migration engagement) and
 * Related Internal Issue(s). Subject and Description start empty.
 * Migration sends reason "migration"; otherwise reason is "default".
 * The repository comes from GET /products/github-repo for the case product.
 * Submit stays disabled until that lookup returns a mapping.
 */
export function CreateGithubIssueDialog({
  open,
  submitting,
  error,
  createdIssue,
  defaultUpdateLevel,
  defaultMigration,
  showRepoField,
  productName,
  onboardingInProgress,
  projectStatusPending,
  projectStatusFailed,
  onRetryProjectStatus,
  onClose,
  onSubmit,
  onOpenConfirm,
}: CreateGithubIssueDialogProps): JSX.Element {
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [reproducingSteps, setReproducingSteps] = useState("");
  const [priorityLevel, setPriorityLevel] = useState<string>(UNSET);
  const [priorityJustification, setPriorityJustification] = useState<string>(UNSET);
  const [updateLevel, setUpdateLevel] = useState(defaultUpdateLevel ?? "");
  const [publicIssueUrl, setPublicIssueUrl] = useState("");
  const [hotFix, setHotFix] = useState(false);
  const [hotFixReason, setHotFixReason] = useState<string>(UNSET);
  const [regression, setRegression] = useState(false);
  const [migration, setMigration] = useState(defaultMigration ?? false);
  const [relatedInternalIssues, setRelatedInternalIssues] = useState("");
  // Set once the user clicks "Create issue" on the form; holds the built
  // payload until they confirm on the follow-up step below. Filing this issue
  // is a real write to an external GitHub repo, so it gets an explicit
  // confirm step like the case's other hard-to-undo actions (see
  // CaseActionBar's TARGET_CONFIG.confirm).
  const [confirmPayload, setConfirmPayload] =
    useState<BeCreateCaseGithubIssuePayload | null>(null);

  // The parent only mounts this dialog once it's actually opened (see
  // CsmCaseDetailPage.tsx's `githubIssueOpen &&` guard), so this only fires
  // per open, not on every case detail page load.
  const {
    data: productRepo,
    isLoading: repoOptionsLoading,
    isError: repoOptionsError,
  } = useGetProductRepoMapping(productName);
  const repoOptionsUnavailable = repoOptionsLoading || repoOptionsError;
  const selectedRepoOption = productRepo
    ? {
        owner: productRepo.owner,
        repo: productRepo.repository,
        displayLabel: productRepo.productName,
      }
    : undefined;

  // Only P1 and P2 carry a justification; the options depend on which.
  const justificationOptions = PRIORITY_JUSTIFICATION_OPTIONS[priorityLevel];
  const showJustification = !!justificationOptions;
  // Update Level / Public Git Issue apply to non-cloud projects only.
  const showUpdateLevelAndIssueUrl = !showRepoField;

  const handlePriorityChange = (value: string) => {
    setPriorityLevel(value);
    // A justification picked for another priority is not valid for this one.
    setPriorityJustification(UNSET);
  };

  const handleHotFixChange = (checked: boolean) => {
    setHotFix(checked);
    if (!checked) setHotFixReason(UNSET);
  };

  const resetAndClose = () => {
    setTitle("");
    setDescription("");
    setReproducingSteps("");
    setPriorityLevel(UNSET);
    setPriorityJustification(UNSET);
    setUpdateLevel(defaultUpdateLevel ?? "");
    setPublicIssueUrl("");
    setHotFix(false);
    setHotFixReason(UNSET);
    setRegression(false);
    setMigration(defaultMigration ?? false);
    setRelatedInternalIssues("");
    setConfirmPayload(null);
    onClose();
  };

  const canSubmit =
    title.trim().length > 0 &&
    description.trim().length > 0 &&
    !!priorityLevel &&
    (!showJustification || !!priorityJustification) &&
    (!hotFix || !!hotFixReason) &&
    !repoOptionsUnavailable &&
    !!selectedRepoOption &&
    !projectStatusPending &&
    !projectStatusFailed;

  const handleSubmit = () => {
    if (!canSubmit || !selectedRepoOption) return;

    const payload: BeCreateCaseGithubIssuePayload = {
      reason: migration ? "migration" : "default",
      title: title.trim(),
      description: description.trim(),
      issueTypeLabel: GITHUB_DISCUSSION_ISSUE_TYPE,
      priorityLevel,
      repoOverride: {
        owner: selectedRepoOption.owner,
        repo: selectedRepoOption.repo,
      },
    };
    if (reproducingSteps.trim()) payload.reproducingSteps = reproducingSteps.trim();
    if (showJustification && priorityJustification) {
      payload.priorityJustification = priorityJustification;
    }
    // Sent even when hidden on a cloud case: the prefilled update level still
    // belongs in the issue footer.
    if (updateLevel.trim()) payload.updateLevel = updateLevel.trim();
    if (showUpdateLevelAndIssueUrl && publicIssueUrl.trim()) {
      payload.publicIssueUrl = publicIssueUrl.trim();
    }
    if (onboardingInProgress) payload.onboardingInProgress = true;
    if (hotFix) {
      payload.hotFixRequired = true;
      payload.hotFixReason = hotFixReason;
    }
    if (regression) payload.regression = true;
    if (relatedInternalIssues.trim()) {
      payload.relatedInternalIssues = relatedInternalIssues.trim();
    }

    onOpenConfirm?.();
    setConfirmPayload(payload);
  };

  // Shared renderer for a "-- Select --" dropdown.
  const renderSelect = <V extends string>(
    id: string,
    label: string,
    value: V,
    onChange: (v: V) => void,
    options: Array<{ value: V; label: string }>,
    required?: boolean,
    extraDisabled?: boolean,
  ): JSX.Element => (
    <FormControl
      fullWidth
      size="small"
      disabled={submitting || extraDisabled}
      required={required}
    >
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
            helperText="Case number, product and your email (as Opened By) are appended to the issue body automatically."
          />

          <TextField
            label="Reproducing Steps"
            multiline
            minRows={3}
            value={reproducingSteps}
            onChange={(e) => setReproducingSteps(e.target.value)}
            fullWidth
            disabled={submitting}
          />

          {renderSelect(
            "ghi-priority",
            "Priority",
            priorityLevel,
            handlePriorityChange,
            PRIORITY_OPTIONS,
            true,
          )}

          {justificationOptions &&
            renderSelect(
              "ghi-priority-justification",
              "Priority Justification",
              priorityJustification,
              setPriorityJustification,
              toOptions(justificationOptions),
              true,
            )}

          {showUpdateLevelAndIssueUrl && (
            <TextField
              label="Update Level"
              value={updateLevel}
              onChange={(e) => setUpdateLevel(e.target.value)}
              disabled={submitting}
              size="small"
              fullWidth
            />
          )}

          {showUpdateLevelAndIssueUrl && (
            <TextField
              label="Public Git Issue or Security Internal JIRA"
              value={publicIssueUrl}
              onChange={(e) => setPublicIssueUrl(e.target.value)}
              disabled={submitting}
              size="small"
              fullWidth
              placeholder="https://github.com/… or JIRA link"
            />
          )}

          <FormControlLabel
            control={
              <Switch
                checked={hotFix}
                onChange={(e) => handleHotFixChange(e.target.checked)}
                disabled={submitting}
              />
            }
            label="Hotfix Required"
          />

          {hotFix &&
            renderSelect(
              "ghi-hotfix-reason",
              "Hotfix Reason",
              hotFixReason,
              setHotFixReason,
              toOptions(HOTFIX_REASON_OPTIONS),
              true,
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

          <FormControlLabel
            control={
              <Switch
                checked={migration}
                onChange={(e) => setMigration(e.target.checked)}
                disabled={submitting}
              />
            }
            label="Migration"
          />

          <TextField
            label="Related Internal Issue(s)"
            multiline
            minRows={2}
            value={relatedInternalIssues}
            onChange={(e) => setRelatedInternalIssues(e.target.value)}
            fullWidth
            disabled={submitting}
            placeholder="Links or numbers of related internal issues"
          />

          {projectStatusFailed ? (
            <Box>
              <Typography variant="body2" color="error">
                Could not load this case's project status.
              </Typography>
              <Button size="small" onClick={onRetryProjectStatus} disabled={submitting}>
                Try again
              </Button>
            </Box>
          ) : (
            <Typography variant="body2" color={selectedRepoOption && !projectStatusPending ? "text.secondary" : "error"}>
              {projectStatusPending
                ? "Waiting for this case's project status…"
                : repoOptionsLoading
                  ? "Looking up the GitHub repository for this product…"
                  : selectedRepoOption
                    ? `This issue will be created in: ${selectedRepoOption.owner}/${selectedRepoOption.repo} (${selectedRepoOption.displayLabel})`
                    : "No GitHub repository is mapped for this product."}
            </Typography>
          )}
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
                {selectedRepoOption
                  ? `${selectedRepoOption.owner}/${selectedRepoOption.repo} (${selectedRepoOption.displayLabel})`
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
