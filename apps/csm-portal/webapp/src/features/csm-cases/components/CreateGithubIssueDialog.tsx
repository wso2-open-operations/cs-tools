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
  Alert,
  Box,
  Button,
  Checkbox,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Divider,
  FormControl,
  FormControlLabel,
  InputLabel,
  Link,
  MenuItem,
  Select,
  Switch,
  TextField,
  Tooltip,
  Typography,
} from "@wso2/oxygen-ui";
import { CheckCircle, GitHub } from "@wso2/oxygen-ui-icons-react";
import { useState, type JSX, type ReactNode } from "react";
import type {
  BeCreateCaseGithubIssuePayload,
  BeCreateCaseGithubIssueResponse,
} from "@api/backend/types";
import { useGetProductRepoMapping } from "@features/csm-cases/api/useGetProductRepoMapping";
import githubIssueOptions from "@features/csm-cases/utils/githubIssueOptions.json";

// ---------------------------------------------------------------------------
// Option lists. Every select starts unset ("" → "-- Select --"). An issue is
// filed as a Discussion (Type/Discussion → Type/Incident on GitHub) or a
// Feature Request (Type/Feature); the Type select decides which fields show.
// Priority is sent as the GitHub priority label.
// ---------------------------------------------------------------------------

const UNSET = "";
const SELECT_PLACEHOLDER = "-- Select --";

/** A Discussion: a problem on the case, with priority, hotfix and regression details. */
const GITHUB_DISCUSSION_ISSUE_TYPE = "Type/Discussion";

/** A Feature Request: a short form with a business use case and no urgency fields. */
const GITHUB_FEATURE_ISSUE_TYPE = "Type/Feature";

/** The update level a Feature Request is filed with, whatever the case holds. */
const FEATURE_UPDATE_LEVEL = "-";

const ISSUE_TYPE_OPTIONS: Array<{ value: string; label: string }> = [
  { value: GITHUB_DISCUSSION_ISSUE_TYPE, label: "Discussion" },
  { value: GITHUB_FEATURE_ISSUE_TYPE, label: "Feature Request" },
];

// The priorities of the Internal Issue Handling Policy. Each value is the
// GitHub priority label the backend applies (case_github_issue.go).
const PRIORITY_P0 = "Priority/Immediate(P0)";
const PRIORITY_P1 = "Priority/High(P1)";
const PRIORITY_P2 = "Priority/Medium(P2)";
const PRIORITY_P3 = "Priority/Low(P3)";

/** The policy's priorities; `short` is the level named in the justification prompt. */
const PRIORITY_OPTIONS: Array<{ value: string; label: string; short: string }> = [
  { value: PRIORITY_P0, label: "P0 – Immediate Attention", short: "P0" },
  { value: PRIORITY_P1, label: "P1 – High Priority", short: "P1" },
  { value: PRIORITY_P2, label: "P2 – Medium Priority", short: "P2" },
  { value: PRIORITY_P3, label: "P3 – Low Priority", short: "P3" },
];

/** P0, P1 and P2 carry a Priority Justification. */
const PRIORITIES_NEEDING_JUSTIFICATION = new Set([PRIORITY_P0, PRIORITY_P1, PRIORITY_P2]);

/** P0 and P1 also need CS leadership's review before they are raised. */
const PRIORITIES_NEEDING_APPROVAL = new Set([PRIORITY_P0, PRIORITY_P1]);

/** Why the Reviewed by CS Leadership tick box is there, shown as its tooltip. */
const LEADERSHIP_REVIEW_HINT =
  "CS leadership review is required before raising a P0 or P1 internal issue.";

/** Hides an element visually while keeping it for screen readers. */
const visuallyHidden = {
  position: "absolute",
  width: 1,
  height: 1,
  overflow: "hidden",
  clip: "rect(0 0 0 0)",
  whiteSpace: "nowrap",
} as const;

// The hotfix-reason options live in githubIssueOptions.json. The backend
// refuses any other value, and its copy of the list (case_github_issue.go) is
// checked against that file by TestGitHubIssueChoiceListsMatchTheWebapp, so
// edit both together.
const HOTFIX_REASON_OPTIONS: string[] = githubIssueOptions.hotfixReasons;

const toOptions = (values: string[]): Array<{ value: string; label: string }> =>
  values.map((v) => ({ value: v, label: v }));

/** Distance from a flag row's left edge to its label text: the switch's width
 * less FormControlLabel's -11px margin. Hotfix Reason is indented by this. */
const FLAG_LABEL_INSET = 47;

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
  /** Fired when the confirm step is (re-)entered from "Raise issue…", before
   * any submission happens. The parent should clear a stale `error` here —
   * otherwise going Back from a failed confirm, editing the form, and
   * clicking "Raise issue…" again re-shows the previous attempt's error on
   * a confirm step for a payload that hasn't been submitted yet. */
  onOpenConfirm?: () => void;
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

/**
 * Form for filing an internal GitHub issue from a case (ISSU-020). Type* comes
 * first and starts unset; until one is chosen only Type, Subject and
 * Description show, and Raise stays disabled.
 *
 * Feature Request shows only Subject*, Description*, Business Use Case,
 * Related Git Issue(s) and Migration. Values typed under the other type are
 * kept in state but never sent.
 *
 * Discussion fields, in order: Subject*, Description*, Reproducing Steps,
 * Priority* (P0 to P3, per the Internal Issue Handling Policy), Priority
 * Justification* (free text, P0 to P2) and Reviewed by CS Leadership* (P0/P1
 * only, its reason in a tooltip; the tick only gates Create and is not sent), Update Level, Public Git Issue (both hidden on cloud cases, never required),
 * Related Internal Issue(s), Hotfix Required, Hotfix Reason* (only while
 * Hotfix Required is on), Regression and Migration (on by default for a
 * migration engagement). Subject and Description start empty.
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
  const [issueType, setIssueType] = useState<string>(UNSET);
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [reproducingSteps, setReproducingSteps] = useState("");
  const [priorityLevel, setPriorityLevel] = useState<string>(UNSET);
  const [priorityJustification, setPriorityJustification] = useState("");
  const [leadershipApproved, setLeadershipApproved] = useState(false);
  const [updateLevel, setUpdateLevel] = useState(defaultUpdateLevel ?? "");
  const [publicIssueUrl, setPublicIssueUrl] = useState("");
  const [hotFix, setHotFix] = useState(false);
  const [hotFixReason, setHotFixReason] = useState<string>(UNSET);
  const [regression, setRegression] = useState(false);
  const [migration, setMigration] = useState(defaultMigration ?? false);
  const [relatedInternalIssues, setRelatedInternalIssues] = useState("");
  const [businessUseCase, setBusinessUseCase] = useState("");
  const [relatedGitIssues, setRelatedGitIssues] = useState("");
  // Set once the user clicks "Raise issue…" on the form; holds the built
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

  const isDiscussion = issueType === GITHUB_DISCUSSION_ISSUE_TYPE;
  const isFeature = issueType === GITHUB_FEATURE_ISSUE_TYPE;

  // P0 to P2 carry a justification; only P0 and P1 also need CS leadership's review.
  const needsJustification = PRIORITIES_NEEDING_JUSTIFICATION.has(priorityLevel);
  const needsApproval = PRIORITIES_NEEDING_APPROVAL.has(priorityLevel);
  const priorityShort = PRIORITY_OPTIONS.find((o) => o.value === priorityLevel)?.short;
  // Update Level / Public Git Issue apply to non-cloud projects only.
  const showUpdateLevelAndIssueUrl = !showRepoField;

  /**
   * Sets the priority. The approval tick is cleared, since it was given for the
   * previous priority. The justification text is kept: it is only sent while
   * the new priority shows it.
   */
  const handlePriorityChange = (value: string) => {
    setPriorityLevel(value);
    setLeadershipApproved(false);
  };

  /** Toggles Hotfix Required; switching it off also clears the Hotfix Reason. */
  const handleHotFixChange = (checked: boolean) => {
    setHotFix(checked);
    if (!checked) setHotFixReason(UNSET);
  };

  /** Returns every field to its initial value (empty, or the case's default) and closes the dialog. */
  const resetAndClose = () => {
    setIssueType(UNSET);
    setTitle("");
    setDescription("");
    setReproducingSteps("");
    setPriorityLevel(UNSET);
    setPriorityJustification("");
    setLeadershipApproved(false);
    setUpdateLevel(defaultUpdateLevel ?? "");
    setPublicIssueUrl("");
    setHotFix(false);
    setHotFixReason(UNSET);
    setRegression(false);
    setMigration(defaultMigration ?? false);
    setRelatedInternalIssues("");
    setBusinessUseCase("");
    setRelatedGitIssues("");
    setConfirmPayload(null);
    onClose();
  };

  const typeFieldsValid = isDiscussion
    ? !!priorityLevel &&
      (!needsJustification || priorityJustification.trim().length > 0) &&
      (!needsApproval || leadershipApproved) &&
      (!hotFix || !!hotFixReason)
    : isFeature;

  const canSubmit =
    title.trim().length > 0 &&
    description.trim().length > 0 &&
    typeFieldsValid &&
    !repoOptionsUnavailable &&
    !!selectedRepoOption &&
    !projectStatusPending &&
    !projectStatusFailed;

  /**
   * Builds the payload from the form and opens the confirm step. Optional
   * fields are sent only when filled, and the justification and hotfix reason
   * only while they apply. Nothing is sent until the user confirms.
   */
  const handleSubmit = () => {
    if (!canSubmit || !selectedRepoOption) return;

    if (isFeature) {
      const featurePayload: BeCreateCaseGithubIssuePayload = {
        reason: migration ? "migration" : "default",
        title: title.trim(),
        description: description.trim(),
        issueTypeLabel: GITHUB_FEATURE_ISSUE_TYPE,
        // A feature request is not tied to an update level: the footer reads
        // "Update level : -" instead of "undefined".
        updateLevel: FEATURE_UPDATE_LEVEL,
        repoOverride: {
          owner: selectedRepoOption.owner,
          repo: selectedRepoOption.repo,
        },
      };
      if (businessUseCase.trim()) featurePayload.businessUseCase = businessUseCase.trim();
      if (relatedGitIssues.trim()) featurePayload.relatedGitIssues = relatedGitIssues.trim();
      if (onboardingInProgress) featurePayload.onboardingInProgress = true;
      onOpenConfirm?.();
      setConfirmPayload(featurePayload);
      return;
    }

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
    if (needsJustification) payload.priorityJustification = priorityJustification.trim();
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

  /** Renders a dropdown that starts on the "-- Select --" placeholder. */
  const renderSelect = <V extends string>(
    id: string,
    label: string,
    value: V,
    onChange: (v: V) => void,
    options: Array<{ value: V; label: string }>,
    opts: { required?: boolean } = {},
  ): JSX.Element => (
    <FormControl fullWidth size="small" disabled={submitting} required={opts.required}>
      <InputLabel id={`${id}-label`} shrink>
        {label}
      </InputLabel>
      <Select
        labelId={`${id}-label`}
        label={label}
        value={value}
        displayEmpty
        notched
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

  // Every field in the form shares one size and one label style (label always
  // raised, so the placeholders show before the field is focused).
  const fieldProps = {
    fullWidth: true,
    size: "small",
    disabled: submitting,
    slotProps: { inputLabel: { shrink: true } },
  } as const;

  /** One titled group of related fields. Groups are parted by a divider. */
  const section = (title: string, children: ReactNode, first = false): JSX.Element => (
    <Box component="section" sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      {!first && <Divider />}
      <Typography variant="subtitle2" component="h3">
        {title}
      </Typography>
      {children}
    </Box>
  );

  // Icon and text top-aligned, so a line that wraps on a narrow screen keeps
  // its icon beside the first line.
  const statusLineSx = { display: "flex", alignItems: "flex-start", gap: 1, color: "text.secondary" } as const;

  // Where the issue goes, or why it can't be filed yet. Shown above the form
  // so a blocked form says so before anything is typed into it.
  const destination = projectStatusFailed ? (
    <Alert
      severity="error"
      action={
        <Button color="inherit" size="small" onClick={onRetryProjectStatus} disabled={submitting}>
          Try again
        </Button>
      }
    >
      Could not load this case's project status.
    </Alert>
  ) : projectStatusPending || repoOptionsLoading ? (
    <Box sx={statusLineSx}>
      <CircularProgress size={14} color="inherit" sx={{ flexShrink: 0, mt: 0.375 }} />
      <Typography variant="body2">
        {projectStatusPending
          ? "Waiting for this case's project status…"
          : "Looking up the GitHub repository for this product…"}
      </Typography>
    </Box>
  ) : selectedRepoOption ? (
    <Box sx={statusLineSx}>
      <Box component="span" sx={{ display: "flex", flexShrink: 0, mt: 0.25 }}>
        <GitHub size={16} aria-hidden />
      </Box>
      <Typography variant="body2">
        {`This issue will be raised in: ${selectedRepoOption.owner}/${selectedRepoOption.repo} (${selectedRepoOption.displayLabel})`}
      </Typography>
    </Box>
  ) : (
    <Alert severity="error">No GitHub repository is mapped for this product.</Alert>
  );

  return (
    <Dialog open={open} onClose={resetAndClose} maxWidth="sm" fullWidth>
      <DialogTitle>Raise internal Git issue</DialogTitle>
      <DialogContent dividers sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
          {destination}
          {error && <Alert severity="error">{error}</Alert>}
        </Box>

        {section(
          "Issue",
          <>
            {renderSelect("ghi-type", "Type", issueType, setIssueType, ISSUE_TYPE_OPTIONS, {
              required: true,
            })}
            <TextField
              {...fieldProps}
              label="Subject"
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              required
              placeholder={isFeature ? "Short summary of the feature" : "Short summary of the problem"}
            />
            <TextField
              {...fieldProps}
              label="Description"
              multiline
              minRows={4}
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              required
              placeholder={isFeature ? "What should the feature do?" : "What needs to be fixed?"}
            />
            {isFeature && (
              <TextField
                {...fieldProps}
                label="Business Use Case"
                multiline
                minRows={3}
                value={businessUseCase}
                onChange={(e) => setBusinessUseCase(e.target.value)}
                placeholder="Why the customer needs it and what it enables"
              />
            )}
            {isDiscussion && (
              <TextField
                {...fieldProps}
                label="Reproducing Steps"
                multiline
                minRows={3}
                value={reproducingSteps}
                onChange={(e) => setReproducingSteps(e.target.value)}
                placeholder="Steps that show the problem"
              />
            )}
          </>,
          true,
        )}

        {isDiscussion &&
          section(
          "Urgency",
          <>
            {renderSelect("ghi-priority", "Priority", priorityLevel, handlePriorityChange, PRIORITY_OPTIONS, {
              required: true,
            })}
            {needsJustification && (
              // P0 to P2: the justification, plus for P0 and P1 the review tick,
              // are one sign-off step, so they sit together in their own panel.
              <Box
                sx={{
                  display: "flex",
                  flexDirection: "column",
                  gap: 2,
                  p: 2,
                  // With the tick box, its own 9px padding makes up the rest of the 16px.
                  pb: needsApproval ? 1 : 2,
                  borderRadius: 1,
                  // An outline, not a fill: the theme's inputs are filled, and
                  // a tinted panel would wash them out in the light scheme.
                  border: 1,
                  borderColor: "divider",
                }}
              >
                <TextField
                  {...fieldProps}
                  label="Priority Justification"
                  value={priorityJustification}
                  onChange={(e) => setPriorityJustification(e.target.value)}
                  required
                  placeholder={`Why does this need ${priorityShort} attention from the product team?`}
                />
                {needsApproval && (
                <>
                {/* The tooltip shows only on hover or focus; this copy keeps it as the tick box's description for screen readers. */}
                <Box component="span" id="ghi-leadership-approval-help" sx={visuallyHidden}>
                  {LEADERSHIP_REVIEW_HINT}
                </Box>
                <Tooltip describeChild title={LEADERSHIP_REVIEW_HINT} placement="bottom-start">
                <FormControlLabel
                  sx={(theme) => ({
                    alignSelf: "flex-start",
                    // Same red as the other required-field asterisks.
                    "& .MuiFormControlLabel-asterisk": {
                      color: "error.main",
                      ...theme.applyStyles("dark", { color: "error.light" }),
                    },
                  })}
                  control={
                    <Checkbox
                      checked={leadershipApproved}
                      onChange={(e) => setLeadershipApproved(e.target.checked)}
                      disabled={submitting}
                      slotProps={{ input: { "aria-describedby": "ghi-leadership-approval-help" } }}
                    />
                  }
                  label="Reviewed by CS Leadership"
                  required
                />
                </Tooltip>
                </>
                )}
              </Box>
            )}
          </>,
        )}

        {isDiscussion &&
          section(
          showUpdateLevelAndIssueUrl ? "Version and references" : "References",
          <>
            {showUpdateLevelAndIssueUrl && (
              <Box
                sx={{
                  display: "grid",
                  gridTemplateColumns: { xs: "1fr", sm: "minmax(0, 1fr) minmax(0, 2fr)" },
                  gap: 2,
                }}
              >
                <TextField
                  {...fieldProps}
                  label="Update Level"
                  value={updateLevel}
                  onChange={(e) => setUpdateLevel(e.target.value)}
                />
                <TextField
                  {...fieldProps}
                  label="Public Git Issue or Security Internal JIRA"
                  value={publicIssueUrl}
                  onChange={(e) => setPublicIssueUrl(e.target.value)}
                  placeholder="https://github.com/… or JIRA link"
                />
              </Box>
            )}
            <TextField
              {...fieldProps}
              label="Related Internal Issue(s)"
              multiline
              minRows={2}
              value={relatedInternalIssues}
              onChange={(e) => setRelatedInternalIssues(e.target.value)}
              placeholder="Links or numbers of related internal issues"
            />
          </>,
        )}

        {isFeature &&
          section(
            "References",
            <TextField
              {...fieldProps}
              label="Related Git Issue(s)"
              multiline
              minRows={2}
              value={relatedGitIssues}
              onChange={(e) => setRelatedGitIssues(e.target.value)}
              placeholder="Links or numbers of related Git issues"
            />,
          )}

        {/* Until a type is chosen only Type, Subject and Description show. */}
        {(isDiscussion || isFeature) &&
          section(
          "Flags",
          // The switches carry their own vertical padding; the set is pulled up
          // so it keeps the same 16px rhythm under the heading.
          <Box sx={{ display: "flex", flexDirection: "column", mt: -0.75 }}>
            {isDiscussion && (
            <FormControlLabel
              sx={{ alignSelf: "flex-start" }}
              control={
                <Switch
                  checked={hotFix}
                  onChange={(e) => handleHotFixChange(e.target.checked)}
                  disabled={submitting}
                />
              }
              label="Hotfix Required"
            />
            )}
            {isDiscussion && hotFix && (
              // Indented to the switch labels so it reads as part of Hotfix Required.
              <Box sx={{ pl: `${FLAG_LABEL_INSET}px`, pt: 0.5, pb: 1.5 }}>
                {renderSelect(
                  "ghi-hotfix-reason",
                  "Hotfix Reason",
                  hotFixReason,
                  setHotFixReason,
                  toOptions(HOTFIX_REASON_OPTIONS),
                  { required: true },
                )}
              </Box>
            )}
            {isDiscussion && (
            <FormControlLabel
              sx={{ alignSelf: "flex-start" }}
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
            <FormControlLabel
              sx={{ alignSelf: "flex-start" }}
              control={
                <Switch
                  checked={migration}
                  onChange={(e) => setMigration(e.target.checked)}
                  disabled={submitting}
                />
              }
              label="Migration"
            />
          </Box>,
        )}
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
          Raise issue…
        </Button>
      </DialogActions>

      {/* Filing a GitHub issue is a real write to an external repo, so it
          gets its own explicit confirm step. Once createdIssue is set (the
          submission succeeded), the same dialog switches to a success view
          with a clickable link instead of closing itself. */}
      <Dialog
        open={!!confirmPayload}
        // Before the issue is raised, Escape or an outside click goes back to
        // the form. Once it is raised they act as Done: going back would leave
        // a filled form whose next "Raise issue…" shows this result again.
        onClose={() => (createdIssue ? resetAndClose() : setConfirmPayload(null))}
        maxWidth="xs"
        fullWidth
      >
        {createdIssue ? (
          <>
            <DialogTitle>
              <Box sx={{ display: "flex", alignItems: "center", gap: 1, color: "success.main" }}>
                <CheckCircle size={20} />
                <Box component="span" sx={{ color: "text.primary" }}>
                  Issue raised
                </Box>
              </Box>
            </DialogTitle>
            <DialogContent>
              {createdIssue.issue?.url ? (
                <Typography variant="body2">
                  Raised as{" "}
                  <Link href={createdIssue.issue.url} target="_blank" rel="noopener noreferrer">
                    {createdIssue.issue.repo && createdIssue.issue.number
                      ? `${createdIssue.issue.repo}#${createdIssue.issue.number}`
                      : createdIssue.issue.url}
                  </Link>
                  .
                </Typography>
              ) : (
                <Typography variant="body2">
                  {createdIssue.message ?? "The issue was raised."}
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
            <DialogTitle>Raise this Git issue?</DialogTitle>
            <DialogContent>
              <Typography variant="body2">
                This raises a real issue in{" "}
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
                <Alert severity="error" sx={{ mt: 2 }}>
                  {error}
                </Alert>
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
                Raise issue
              </Button>
            </DialogActions>
          </>
        )}
      </Dialog>
    </Dialog>
  );
}
