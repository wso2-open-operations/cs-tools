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
import { ArrowLeft } from "@wso2/oxygen-ui-icons-react";
import { useEffect, useMemo, useRef, useState, type JSX } from "react";
import { useLocation, useSearchParams } from "react-router";
import { priorityFromSeverity } from "@api/backend/mappers";
import { formatBytes } from "@utils/formatBytes";
import Editor from "@components/rich-text-editor/Editor";
import RequiredMark from "@components/RequiredMark";
import AttachmentsField from "@components/attachments/AttachmentsField";
import {
  POST_CREATE_ATTACHMENTS_MAX_ENCODED_BYTES,
  type EncodedAttachment,
} from "@components/attachments/encodeAttachment";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import { useNewCaseDraft } from "@context/new-case-draft/NewCaseDraftContext";
import ProjectSelectionField from "@features/csm-cases/components/ProjectSelectionField";
import { useGetProject } from "@features/csm-projects/api/useGetProject";
import { isCloudSupportSubscription } from "@features/csm-projects/utils/subscriptionType";
import { useSearchDeployments } from "@features/csm-cases/api/useSearchDeployments";
import { useDeployedProductOptions } from "@features/csm-cases/api/useDeployedProductOptions";
import { usePostCsmCase } from "@features/csm-cases/api/usePostCsmCase";
import { usePostCsmCaseAttachment } from "@features/csm-cases/api/useCsmCaseAttachments";
import { uploadAttachmentsToCase } from "@features/csm-cases/api/uploadAttachmentsToCase";
import { useEngineerDisplayName } from "@hooks/useEngineerDisplayName";
import { SEVERITY_LABEL } from "@features/csm-dashboard/utils/abtDashboard";
import type { Severity } from "@features/csm-dashboard/types/abtDashboard";
import type { BeCaseIssueType } from "@api/backend/types";
import { ALL_ISSUE_TYPES, ISSUE_TYPE_LABEL } from "@features/csm-cases/utils/caseIssueType";
import type { CreateRelatedCaseNavState } from "@features/csm-cases/types/csmCases";
import { useNavTransition } from "@hooks/useNavTransition";
import CaseWatchListSection from "@features/csm-cases/components/CaseWatchListSection";
import { useSearchProjectContacts } from "@features/csm-projects/api/useSearchProjectContacts";
import {
  NO_WATCH_LIST_EDITS,
  eligibleContactWatchers,
  resolveWatchList,
  type WatchListEdits,
} from "@features/csm-cases/utils/createCaseWatchList";

const SEVERITIES: Severity[] = ["S0", "S1", "S2", "S3", "S4"];

const ISSUE_TYPES: { value: BeCaseIssueType; label: string }[] = ALL_ISSUE_TYPES.map((value) => ({
  value,
  label: ISSUE_TYPE_LABEL[value],
}));

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
  const navigate = useNavTransition();
  const { showError } = useErrorBanner();

  // When the form is opened from a project's page (`/cases/new?projectId=…`),
  // the project is fixed and shown read-only: the engineer can't accidentally
  // file the case against the wrong project. The id seeds the form's project
  // state once and the picker is replaced by a locked field. Opened without the
  // param (the cases-list "New case" entry), the searchable picker is shown.
  const [searchParams] = useSearchParams();
  // Set when opened from a closed case's "Create related case" action, which
  // navigates here with router state (not query params) so the whole case
  // context — project, deployment, product, severity, issue type, subject —
  // carries over as editable starting values without a query-string round
  // trip or a full page load. See CsmCaseDetailPage.tsx's
  // `create_related_case` handler.
  const location = useLocation();
  const relatedCaseState = location.state as CreateRelatedCaseNavState | undefined;

  // Set when opened from a project's page Create menu (`/cases/new?projectId=…
  // `, state: { from: "/customers/projects/:id" }), so Back/Cancel return
  // there instead of the hardcoded cases list, and the newly created case's
  // own Back button (reading this same convention) returns there too.
  const backState = location.state as { from?: string } | undefined;
  const backTarget = backState?.from ?? "/cases";

  const lockedProjectId =
    searchParams.get("projectId") ?? relatedCaseState?.projectId ?? "";
  // The backend already validated eligibility (closed + within its 60-day
  // window) before offering "Create related case", so this page just carries
  // the id through on submit.
  const relatedCaseId = relatedCaseState?.relatedCaseId;
  const relatedCaseNumber = relatedCaseState?.relatedCaseNumber;

  // The draft outlives this page (it lives above the routes), so switching to
  // an open case tab and back does not lose what was typed. It is restored only
  // into a form opened the same way it was started (same locked project and
  // related case); a different entry starts fresh rather than inheriting a
  // stale draft. Cleared on a successful submit.
  const draftStore = useNewCaseDraft();
  const entryKey = `${lockedProjectId}|${relatedCaseId ?? ""}`;
  const [restored] = useState(() => {
    const draft = draftStore.get();
    return draft?.entryKey === entryKey ? draft : undefined;
  });

  const [projectId, setProjectId] = useState(restored?.projectId ?? lockedProjectId);
  const [deploymentId, setDeploymentId] = useState(
    restored?.deploymentId ?? relatedCaseState?.deploymentId ?? "",
  );
  const [deployedProductId, setDeployedProductId] = useState(
    restored?.deployedProductId ?? relatedCaseState?.deployedProductId ?? "",
  );
  const [severity, setSeverity] = useState<Severity | "">(
    restored?.severity ?? relatedCaseState?.severity ?? "",
  );
  const [issueType, setIssueType] = useState<BeCaseIssueType | "">(
    restored?.issueType ?? relatedCaseState?.issueType ?? "",
  );
  const [subject, setSubject] = useState(restored?.subject ?? relatedCaseState?.subject ?? "");
  const [description, setDescription] = useState(restored?.description ?? "");
  const [attachments, setAttachments] = useState<EncodedAttachment[]>(
    restored?.attachments ?? [],
  );

  const deployments = useSearchDeployments(projectId || undefined);

  // Watch list: pre-filled from the project's registered contacts, then edited.
  // Only the engineer's edits are state; the list itself is derived, so
  // contacts that load late still pre-fill and nothing the engineer removed or
  // added is ever overwritten. Edits belong to one project and reset with it.
  const projectContacts = useSearchProjectContacts(projectId || undefined);
  const [watchEdits, setWatchEdits] = useState<WatchListEdits>(
    restored?.watchEdits ?? NO_WATCH_LIST_EDITS,
  );
  const watchPrefill = useMemo(
    () => eligibleContactWatchers(projectContacts.data),
    [projectContacts.data],
  );
  const watchers = useMemo(
    () => resolveWatchList(watchPrefill, watchEdits),
    [watchPrefill, watchEdits],
  );

  // Details for the selected project (locked or picked) — the subscription
  // type drives the cloud-project deployment behaviour below. (The display
  // name for the locked/confirmed field is resolved by ProjectSelectionField
  // itself, off the same cached query.)
  const selectedProject = useGetProject(projectId || undefined);

  // Cloud-support projects file against the single primary-production
  // deployment, so the form hides the deployment picker and derives it
  // automatically (mirrors the customer portal). Other subscriptions keep the
  // normal deployment → product cascade.
  const isCloudProject = isCloudSupportSubscription(
    selectedProject.data?.subscriptionType,
  );
  // S0 is reserved for Managed Cloud — the highest severity is meant to page
  // WSO2's own on-call for a subscription WSO2 directly operates, not one a
  // customer only gets support for.
  const isManagedCloud =
    selectedProject.data?.subscriptionType === "managed_cloud_subscription";
  // Clears a stale S0 pick if the project changes to a non-Managed-Cloud one
  // after it was selected (the dropdown itself blocks picking it fresh, but
  // can't stop an already-selected value from becoming invalid underneath).
  // Adjusted during render (React's recommended pattern for this) rather
  // than in an effect, which would call setState synchronously post-commit.
  // Gated on the fetch having settled — while a new project's data is still
  // loading, `selectedProject.data` (and so isManagedCloud) is transiently
  // empty/false, which would otherwise wipe a valid S0 when moving between
  // two Managed Cloud projects.
  const [prevManagedCloud, setPrevManagedCloud] = useState(isManagedCloud);
  if (!selectedProject.isFetching && isManagedCloud !== prevManagedCloud) {
    setPrevManagedCloud(isManagedCloud);
    if (severity === "S0" && !isManagedCloud) setSeverity("");
  }
  const primaryProductionDeployments = useMemo(
    () => (deployments.data ?? []).filter((d) => d.type === "primary_production"),
    [deployments.data],
  );
  // For a cloud project the deployment is derived (the lone primary-production
  // one), not user-picked; everything downstream keys off this effective id.
  const effectiveDeploymentId = isCloudProject
    ? (primaryProductionDeployments[0]?.id ?? "")
    : deploymentId;

  const deployedProducts = useDeployedProductOptions(
    effectiveDeploymentId || undefined,
  );
  const postCase = usePostCsmCase();
  const postAttachment = usePostCsmCaseAttachment();
  const uploadedBy = useEngineerDisplayName();
  // Spans the whole submit (create + post-create attachment uploads).
  const [submitting, setSubmitting] = useState(false);

  // Mirror the form into the draft store on every change. `submitted` stops it
  // writing the draft back after a successful create has cleared it.
  const submitted = useRef(false);
  useEffect(() => {
    if (submitted.current) return;
    draftStore.set({
      entryKey,
      projectId,
      deploymentId,
      deployedProductId,
      severity,
      issueType,
      subject,
      description,
      attachments,
      watchEdits,
    });
  }, [
    draftStore,
    entryKey,
    projectId,
    deploymentId,
    deployedProductId,
    severity,
    issueType,
    subject,
    description,
    attachments,
    watchEdits,
  ]);

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
  // Attachments are uploaded after the case is created (separate requests), so
  // they don't count against the create-body budget — only the per-file cap in
  // AttachmentsField applies.
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
      !!effectiveDeploymentId &&
      !!deployedProductId &&
      !!severity &&
      !!issueType &&
      subject.trim().length > 0 &&
      !isEmptyHtml(description) &&
      !descriptionOverLimit &&
      // Hold submit until the project's contacts load so the pre-filled
      // watchers are not silently dropped; a failed load does not block.
      !projectContacts.isLoading &&
      !submitting,
    [
      projectId,
      effectiveDeploymentId,
      deployedProductId,
      severity,
      issueType,
      subject,
      description,
      descriptionOverLimit,
      projectContacts.isLoading,
      submitting,
    ],
  );

  // Cascade resets: a parent change invalidates its children.
  const onProjectChange = (next: string): void => {
    setProjectId(next);
    setWatchEdits(NO_WATCH_LIST_EDITS);
    setDeploymentId("");
    setDeployedProductId("");
  };
  const onDeploymentChange = (next: string): void => {
    setDeploymentId(next);
    setDeployedProductId("");
  };

  const handleSubmit = async (): Promise<void> => {
    if (!canSubmit || !severity || !issueType || descriptionOverLimit) return;
    setSubmitting(true);
    try {
      const created = await postCase.mutateAsync({
        type: "case",
        projectId,
        deploymentId: effectiveDeploymentId,
        deployedProductId,
        subject: subject.trim(),
        description,
        severity: priorityFromSeverity(severity),
        issueType: issueType,
        relatedCaseId,
        ...(watchers.length > 0 && { watchList: watchers.map((w) => w.email) }),
      });
      submitted.current = true;
      draftStore.clear();
      // The create endpoint doesn't attach files for standard cases, so upload
      // them to the new case afterwards. A partial failure still lands the case.
      const failed = await uploadAttachmentsToCase(
        postAttachment.mutateAsync,
        created.id,
        attachments,
        uploadedBy,
      );
      if (failed > 0) {
        showError(
          `The case was created, but ${failed} attachment${failed === 1 ? "" : "s"} failed to upload. You can add ${failed === 1 ? "it" : "them"} from the case page.`,
        );
      }
      navigate(`/cases/${created.id}`, { state: { from: backTarget } });
    } catch (err) {
      setSubmitting(false);
      showError("Could not create the case. Please try again.", err);
    }
  };

  return (
    <Box sx={{ width: "100%", px: 3, py: 3 }}>
      <Button
        variant="text"
        startIcon={<ArrowLeft size={16} />}
        onClick={() => navigate(backTarget)}
        sx={{ mb: 1 }}
      >
        Back
      </Button>
      <Typography variant="h5" sx={{ mb: relatedCaseId ? 0.5 : 2 }}>
        New case
      </Typography>
      {relatedCaseId && (
        <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
          Related to {relatedCaseNumber ?? "the closed case"} — its id is carried through automatically.
        </Typography>
      )}

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
            <ProjectSelectionField
              value={projectId}
              onChange={onProjectChange}
              lockedProjectId={lockedProjectId}
              required
            />
          </Grid>

          {/* Cloud-support projects file against the single primary-production
              deployment, auto-selected above — so the picker is hidden. */}
          {!isCloudProject && (
            <Grid size={{ xs: 12, md: 4 }}>
              <FormControl fullWidth size="small" required>
                <InputLabel
                  id="case-deployment-label"
                  shrink={deploymentId !== ""}
                  sx={{ top: "0px !important" }}
                >
                  Deployment
                </InputLabel>
                <Select
                  MenuProps={{
                    // Anchored under the field, not over it. MUI's default aligns the
                    // selected item with the input, which lifts the list on top of the
                    // control and clips the first option behind it. Same MenuProps
                    // CsmUsersPage already carries, for the same reason.
                    anchorOrigin: { vertical: "bottom", horizontal: "left" },
                    transformOrigin: { vertical: "top", horizontal: "left" },
                    slotProps: { paper: { sx: { maxHeight: 320 } } },
                  }}
                  labelId="case-deployment-label"
                  label="Deployment"
                  value={deploymentId}
                  notched={deploymentId !== ""}
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
                ) : (deployments.data ?? []).length === 0 ? (
                  <FormHelperText>No deployments found for this project.</FormHelperText>
                ) : null}
              </FormControl>
            </Grid>
          )}

          <Grid size={{ xs: 12, md: 4 }}>
            <FormControl fullWidth size="small" required>
              <InputLabel
                id="case-product-label"
                shrink={deployedProductId !== ""}
                sx={{ top: "0px !important" }}
              >
                Deployed product
              </InputLabel>
              <Select
                MenuProps={{
                  // Anchored under the field, not over it. MUI's default aligns the
                  // selected item with the input, which lifts the list on top of the
                  // control and clips the first option behind it. Same MenuProps
                  // CsmUsersPage already carries, for the same reason.
                  anchorOrigin: { vertical: "bottom", horizontal: "left" },
                  transformOrigin: { vertical: "top", horizontal: "left" },
                  slotProps: { paper: { sx: { maxHeight: 320 } } },
                }}
                labelId="case-product-label"
                label="Deployed product"
                value={deployedProductId}
                notched={deployedProductId !== ""}
                onChange={(e) => setDeployedProductId(String(e.target.value))}
                disabled={!effectiveDeploymentId || deployedProducts.isLoading}
              >
                {(deployedProducts.data ?? []).map((dp) => (
                  <MenuItem key={dp.id} value={dp.id}>
                    {dp.label}
                  </MenuItem>
                ))}
              </Select>
              {!effectiveDeploymentId ? (
                <FormHelperText>
                  {isCloudProject
                    ? !deployments.isLoading &&
                      primaryProductionDeployments.length === 0
                      ? "No primary production deployment for this project."
                      : "Loading products…"
                    : "Select a deployment first"}
                </FormHelperText>
              ) : deployedProducts.isLoading ? (
                <FormHelperText>Loading products…</FormHelperText>
              ) : (deployedProducts.data ?? []).length === 0 ? (
                <FormHelperText>No deployed products found for this deployment.</FormHelperText>
              ) : null}
            </FormControl>
          </Grid>

          <Grid size={{ xs: 12, sm: 6, md: 3 }}>
            <FormControl fullWidth size="small" required>
              <InputLabel
                id="case-severity-label"
                shrink={severity !== ""}
                sx={{ top: "0px !important" }}
              >
                Severity
              </InputLabel>
              <Select
                MenuProps={{
                  // Anchored under the field, not over it. MUI's default aligns the
                  // selected item with the input, which lifts the list on top of the
                  // control and clips the first option behind it. Same MenuProps
                  // CsmUsersPage already carries, for the same reason.
                  anchorOrigin: { vertical: "bottom", horizontal: "left" },
                  transformOrigin: { vertical: "top", horizontal: "left" },
                  slotProps: { paper: { sx: { maxHeight: 320 } } },
                }}
                labelId="case-severity-label"
                label="Severity"
                value={severity}
                notched={severity !== ""}
                onChange={(e) => setSeverity(e.target.value as Severity)}
              >
                {SEVERITIES.map((s) => (
                  <MenuItem key={s} value={s} disabled={s === "S0" && !isManagedCloud}>
                    {s} · {SEVERITY_LABEL[s]}
                    {s === "S0" && !isManagedCloud ? " (Managed Cloud only)" : ""}
                  </MenuItem>
                ))}
              </Select>
            </FormControl>
          </Grid>

          <Grid size={{ xs: 12, sm: 6, md: 3 }}>
            <FormControl fullWidth size="small" required>
              <InputLabel
                id="case-issue-type-label"
                shrink={issueType !== ""}
                sx={{ top: "0px !important" }}
              >
                Issue type
              </InputLabel>
              <Select
                MenuProps={{
                  // Anchored under the field, not over it. MUI's default aligns the
                  // selected item with the input, which lifts the list on top of the
                  // control and clips the first option behind it. Same MenuProps
                  // CsmUsersPage already carries, for the same reason.
                  anchorOrigin: { vertical: "bottom", horizontal: "left" },
                  transformOrigin: { vertical: "top", horizontal: "left" },
                  slotProps: { paper: { sx: { maxHeight: 320 } } },
                }}
                labelId="case-issue-type-label"
                label="Issue type"
                value={issueType}
                notched={issueType !== ""}
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
              <RequiredMark />
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
                disabled={submitting}
                ariaRequired
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

          <Grid size={{ xs: 12 }}>
            <CaseWatchListSection
              projectId={projectId}
              watchers={watchers}
              edits={watchEdits}
              onEditsChange={setWatchEdits}
              contacts={{
                isLoading: projectContacts.isLoading,
                isError: projectContacts.isError,
                refetch: () => void projectContacts.refetch(),
              }}
              noEligibleContacts={projectContacts.isSuccess && watchPrefill.length === 0}
              disabled={submitting}
            />
          </Grid>

          <Grid size={{ xs: 12 }}>
            <Typography
              variant="caption"
              color="text.secondary"
              sx={{ display: "block", mb: 0.5 }}
            >
              Attachments
            </Typography>
            <AttachmentsField
              attachments={attachments}
              onChange={setAttachments}
              onError={showError}
              maxEncodedBytes={POST_CREATE_ATTACHMENTS_MAX_ENCODED_BYTES}
            />
          </Grid>
        </Grid>

        <Box sx={{ display: "flex", justifyContent: "flex-end", gap: 1.5, mt: 2.5 }}>
          <Button
            variant="outlined"
            onClick={() => {
              // An explicit discard: drop the draft too (flag first, so the
              // mirroring effect cannot write it back) or it would reappear
              // the next time the form opens.
              submitted.current = true;
              draftStore.clear();
              navigate(backTarget);
            }}
          >
            Cancel
          </Button>
          <Button
            variant="contained"
            onClick={() => void handleSubmit()}
            disabled={!canSubmit}
          >
            {submitting ? "Creating…" : "Create case"}
          </Button>
        </Box>
      </Card>
    </Box>
  );
}
