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
// Option lists. Every select starts unset ("" → "-- Select --") and omits its
// field from the payload when left unset. Type is Patch or Discussion. Severity
// is sent as a GitHub priority label only for Discussion.
// ---------------------------------------------------------------------------

const UNSET = "";
const SELECT_PLACEHOLDER = "-- Select --";

type IssueTypeValue = "" | "Type/Patch" | "Type/Discussion";

const TYPE_OPTIONS: Array<{ value: IssueTypeValue; label: string }> = [
  { value: "Type/Patch", label: "Patch" },
  { value: "Type/Discussion", label: "Discussion" },
];

const SEVERITY_OPTIONS: Array<{ value: string; label: string }> = [
  { value: "Priority/Critical", label: "P1 - Critical" },
  { value: "Priority/High", label: "P2 - High" },
  { value: "Priority/Medium", label: "P3 - Medium" },
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
  /** Cloud cases pass true. There is no repository dropdown either way.
   * When true, Update Level and Public Git Issue are hidden. When false,
   * a Patch must fill both. */
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
 * Form for filing an internal GitHub issue from a case (ISSU-020).
 * Subject and Description are always required. Type is Patch or Discussion:
 *   - Discussion: Severity is required. Hotfix Required and Regression are hidden.
 *   - Patch: Severity is hidden. Hotfix Required and Regression are shown. On a
 *     non-cloud case, Update Level and Public Git Issue are required.
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
  defaultTitle,
  defaultDescription,
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
  const [type, setType] = useState<IssueTypeValue>(UNSET);
  const [title, setTitle] = useState(defaultTitle ?? "");
  const [description, setDescription] = useState(defaultDescription ?? "");
  const [updateLevel, setUpdateLevel] = useState(defaultUpdateLevel ?? "");
  const [publicIssueUrl, setPublicIssueUrl] = useState("");
  const [priorityLevel, setPriorityLevel] = useState<string>(UNSET);
  const [hotFix, setHotFix] = useState(false);
  const [regression, setRegression] = useState(false);
  const [migration, setMigration] = useState(false);
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

  // Type drives which fields apply — see the component doc comment above.
  const showSeverity = type === "Type/Discussion";
  const requireSeverity = type === "Type/Discussion";
  const showHotFix = type === "Type/Patch";
  const showRegression = type === "Type/Patch";
  // Update Level / Public Git Issue apply to non-cloud projects only — cloud
  // projects route via the repo field instead (see showRepoField).
  const showUpdateLevelAndIssueUrl = !showRepoField;
  const requireUpdateLevel =
    type === "Type/Patch" && showUpdateLevelAndIssueUrl;
  const requirePublicIssueUrl =
    type === "Type/Patch" && showUpdateLevelAndIssueUrl;

  const resetAndClose = () => {
    setType(UNSET);
    setTitle(defaultTitle ?? "");
    setDescription(defaultDescription ?? "");
    setUpdateLevel(defaultUpdateLevel ?? "");
    setPublicIssueUrl("");
    setPriorityLevel(UNSET);
    setHotFix(false);
    setRegression(false);
    setMigration(false);
    setConfirmPayload(null);
    onClose();
  };

  const canSubmit =
    !!type &&
    title.trim().length > 0 &&
    description.trim().length > 0 &&
    (!requireSeverity || !!priorityLevel) &&
    (!requireUpdateLevel || updateLevel.trim().length > 0) &&
    (!requirePublicIssueUrl || publicIssueUrl.trim().length > 0) &&
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
      issueTypeLabel: type,
      repoOverride: {
        owner: selectedRepoOption.owner,
        repo: selectedRepoOption.repo,
      },
    };
    if (updateLevel.trim()) payload.updateLevel = updateLevel.trim();
    if (publicIssueUrl.trim()) payload.publicIssueUrl = publicIssueUrl.trim();
    if (showSeverity && priorityLevel) payload.priorityLevel = priorityLevel;
    if (onboardingInProgress) payload.onboardingInProgress = true;
    if (showHotFix && hotFix) payload.hotFixRequired = true;
    if (showRegression && regression) payload.regression = true;

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

          {showUpdateLevelAndIssueUrl && (
            <TextField
              label="Update Level"
              value={updateLevel}
              onChange={(e) => setUpdateLevel(e.target.value)}
              disabled={submitting}
              required={requireUpdateLevel}
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
              required={requirePublicIssueUrl}
              size="small"
              fullWidth
              placeholder="https://github.com/… or JIRA link"
            />
          )}

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
                checked={migration}
                onChange={(e) => setMigration(e.target.checked)}
                disabled={submitting}
              />
            }
            label="Migration"
          />

          {showRegression && (
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
          )}

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
