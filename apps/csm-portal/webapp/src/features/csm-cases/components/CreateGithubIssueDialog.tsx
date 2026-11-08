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
import { useGetGithubIssueRepoOptions } from "@features/csm-cases/api/useGetGithubIssueRepoOptions";

// ---------------------------------------------------------------------------
// Option lists. Every select starts unset ("" → "-- Select --") and omits its
// field from the payload when left unset.
// ---------------------------------------------------------------------------

const UNSET = "";
const SELECT_PLACEHOLDER = "-- Select --";

const PRIORITY_OPTIONS: Array<{ value: string; label: string }> = [
  { value: "P1", label: "P1" },
  { value: "P2", label: "P2" },
  { value: "P3", label: "P3" },
];

const HOTFIX_REASON_OPTIONS: Array<{ value: string; label: string }> = [
  { value: "Customer-requested hotfix", label: "Customer-requested hotfix" },
];

// Cloud-case repositories. Fetched from GET /metadata's
// githubIssueRepoOptions field (useGetGithubIssueRepoOptions) rather than
// hardcoded here — each option
// carries its own real owner/repo, sent as repoOverride to bypass the SN
// product-unit routing (which only covers on-prem/product-unit-mapped cases).

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
  /** Show the repository field only for cloud subscription / cloud
   * evaluation subscription projects — other project types route by
   * product unit on the SN side and have no repo to choose. Conversely,
   * Update Level and Public Git Issue apply only when this is false. */
  showRepoField?: boolean;
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
 * Form for filing an internal GitHub issue from a case (ISSU-020). Subject,
 * Description and Priority are always required (Subject/Description start
 * empty — they are deliberately not prefilled from the case). Priority
 * Justification appears, and is required, only for P1/P2. Hotfix Reason
 * appears, and is required, only when Hotfix Required is on. Reproducing
 * Steps, Update Level and Public Git Issue are optional. Reason is fixed to
 * `default`. Repo selection is offered for cloud cases; when unset the SN side
 * routes by the case's product unit.
 */
export function CreateGithubIssueDialog({
  open,
  submitting,
  error,
  createdIssue,
  defaultUpdateLevel,
  showRepoField,
  onClose,
  onSubmit,
  onOpenConfirm,
}: CreateGithubIssueDialogProps): JSX.Element {
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [reproducingSteps, setReproducingSteps] = useState("");
  const [updateLevel, setUpdateLevel] = useState(defaultUpdateLevel ?? "");
  const [publicIssueUrl, setPublicIssueUrl] = useState("");
  const [priorityLevel, setPriorityLevel] = useState<string>(UNSET);
  const [priorityJustification, setPriorityJustification] = useState("");
  const [repo, setRepo] = useState<string>(UNSET);
  const [hotFix, setHotFix] = useState(false);
  const [hotFixReason, setHotFixReason] = useState<string>(UNSET);
  // Set once the user clicks "Create issue" on the form; holds the built
  // payload until they confirm on the follow-up step below. Filing this issue
  // is a real write to an external GitHub repo, so it gets an explicit
  // confirm step like the case's other hard-to-undo actions (see
  // CaseActionBar's TARGET_CONFIG.confirm).
  const [confirmPayload, setConfirmPayload] =
    useState<BeCreateCaseGithubIssuePayload | null>(null);

  // The parent only mounts this dialog once it's actually opened (see
  // CsmCaseDetailPage.tsx's `githubIssueOpen &&` guard), so this only fires
  // per open, not on every case detail page load. `repoOptions` itself is
  // only ever rendered when `showRepoField` is true.
  const {
    data: repoOptionsData,
    isLoading: repoOptionsLoading,
    isError: repoOptionsError,
  } = useGetGithubIssueRepoOptions();
  const repoOptions = repoOptionsData ?? [];
  // Until this resolves (success or a confirmed-empty catalogue), a cloud
  // case's submission must not be allowed through: handleSubmit only sets
  // repoOverride when a repo is actually selected, and no repo can be
  // selected before repoOptions is populated. Letting canSubmit go true in
  // that window would silently fall back to product-unit routing — which
  // this dialog's own doc comment above says only applies to non-cloud
  // projects — for a case where the engineer never got the chance to choose.
  const repoOptionsUnavailable =
    showRepoField && (repoOptionsLoading || repoOptionsError);
  const repoSelectOptions = repoOptions.map((o) => ({
    value: o.value,
    label: o.displayLabel,
  }));

  const showPriorityJustification =
    priorityLevel === "P1" || priorityLevel === "P2";

  const resetAndClose = () => {
    setTitle("");
    setDescription("");
    setReproducingSteps("");
    setUpdateLevel(defaultUpdateLevel ?? "");
    setPublicIssueUrl("");
    setPriorityLevel(UNSET);
    setPriorityJustification("");
    setRepo(UNSET);
    setHotFix(false);
    setHotFixReason(UNSET);
    setConfirmPayload(null);
    onClose();
  };

  const canSubmit =
    title.trim().length > 0 &&
    description.trim().length > 0 &&
    !!priorityLevel &&
    (!showPriorityJustification || priorityJustification.trim().length > 0) &&
    (!hotFix || !!hotFixReason) &&
    !repoOptionsUnavailable;

  const selectedRepoOption = repoOptions.find((o) => o.value === repo);

  const handleSubmit = () => {
    if (!canSubmit) return;

    const payload: BeCreateCaseGithubIssuePayload = {
      reason: "default",
      title: title.trim(),
      description: description.trim(),
      priorityLevel,
    };
    if (reproducingSteps.trim()) {
      payload.reproducingSteps = reproducingSteps.trim();
    }
    // Only P1/P2 show the justification field; never send a stale value
    // typed before the priority was switched to P3.
    if (showPriorityJustification) {
      payload.priorityJustification = priorityJustification.trim();
    }
    if (updateLevel.trim()) payload.updateLevel = updateLevel.trim();
    if (publicIssueUrl.trim()) payload.publicIssueUrl = publicIssueUrl.trim();
    if (selectedRepoOption) {
      payload.repoOverride = {
        owner: selectedRepoOption.owner,
        repo: selectedRepoOption.repo,
      };
    }
    if (hotFix) {
      payload.hotFixRequired = true;
      payload.hotFixReason = hotFixReason;
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
            helperText="Case number, product and reporter are appended to the issue body automatically."
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
            setPriorityLevel,
            PRIORITY_OPTIONS,
            true,
          )}

          {showPriorityJustification && (
            <TextField
              label="Priority Justification"
              multiline
              minRows={2}
              value={priorityJustification}
              onChange={(e) => setPriorityJustification(e.target.value)}
              fullWidth
              required
              disabled={submitting}
            />
          )}

          <TextField
            label="Update Level"
            value={updateLevel}
            onChange={(e) => setUpdateLevel(e.target.value)}
            disabled={submitting}
            size="small"
            fullWidth
          />

          <TextField
            label="Public Git Issue"
            value={publicIssueUrl}
            onChange={(e) => setPublicIssueUrl(e.target.value)}
            disabled={submitting}
            size="small"
            fullWidth
            placeholder="https://github.com/…"
          />

          <FormControlLabel
            control={
              <Switch
                checked={hotFix}
                onChange={(e) => {
                  setHotFix(e.target.checked);
                  if (!e.target.checked) setHotFixReason(UNSET);
                }}
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
              HOTFIX_REASON_OPTIONS,
              true,
            )}

          {showRepoField &&
            renderSelect(
              "ghi-repo",
              "Choose repository",
              repo,
              setRepo,
              repoSelectOptions,
              false,
              repoOptionsLoading,
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
