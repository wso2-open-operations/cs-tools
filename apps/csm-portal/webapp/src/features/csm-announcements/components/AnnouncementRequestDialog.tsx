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

import { useEffect, useMemo, useState, type JSX, type ReactNode } from "react";
import {
  AdapterDateFns,
  Box,
  Button,
  Checkbox,
  Chip,
  DatePickers,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Divider,
  FormControlLabel,
  IconButton,
  Skeleton,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { RefreshCw, X } from "@wso2/oxygen-ui-icons-react";
import { Link } from "react-router";
import { useIdTokenClaims } from "@hooks/useIdTokenClaims";
import { usePortalAccess } from "@context/current-user/usePortalAccess";
import EditorWithSourceToggle from "@components/rich-text-editor/EditorWithSourceToggle";
import {
  formatAbsoluteForUser,
  formatDateTimeLocal,
  isPastDateTime,
  parseDateTimeLocal,
  resolveDisplayTimeZone,
  zonedInputToUtcIso,
} from "@utils/dateTime";
import { sanitizeRichTextHtml } from "@utils/sanitizeHtml";
import {
  DRY_RUN_TAG_LABEL,
  useAnnouncementDryRun,
} from "@features/csm-announcements/api/useAnnouncementDryRun";
import { useGetAnnouncementRequest } from "@features/csm-announcements/api/useGetAnnouncementRequest";
import { useUpdateAnnouncementRequest } from "@features/csm-announcements/api/useUpdateAnnouncementRequest";
import { useRecordAnnouncementRequestDryRun } from "@features/csm-announcements/api/useRecordAnnouncementRequestDryRun";
import { useSubmitAnnouncementRequest } from "@features/csm-announcements/api/useSubmitAnnouncementRequest";
import { useApproveAnnouncementRequest } from "@features/csm-announcements/api/useApproveAnnouncementRequest";
import { useScheduleAnnouncementRequest } from "@features/csm-announcements/api/useScheduleAnnouncementRequest";
import { usePublishAnnouncementRequest } from "@features/csm-announcements/api/usePublishAnnouncementRequest";
import { SECURITY_ANNOUNCEMENT_TAG_LABEL } from "@features/csm-announcements/components/CreateCustomerAnnouncementForm";
import AnnouncementSendProgress, {
  type AnnouncementSendProgressState,
} from "@features/csm-announcements/components/AnnouncementSendProgress";
import PublishConfirmationDialog from "@features/csm-announcements/components/PublishConfirmationDialog";
import { useResolvedAudiencePreview } from "@features/csm-announcements/api/useResolvedAudiencePreview";
import AddUpdateConfirmationDialog from "@features/csm-announcements/components/AddUpdateConfirmationDialog";
import { useCreateAnnouncementRequestUpdate } from "@features/csm-announcements/api/useCreateAnnouncementRequestUpdate";
import { useListAnnouncementRequestUpdates } from "@features/csm-announcements/api/useListAnnouncementRequestUpdates";
import { usePostAnnouncementUpdateComments } from "@features/csm-announcements/api/usePostAnnouncementUpdateComments";
import type { AnnouncementRegistryCaseMember } from "@features/csm-announcements/types/announcementRegistry";

const { DateTimePicker, LocalizationProvider } = DatePickers;

interface AnnouncementRequestDialogProps {
  requestId: string;
  onClose: () => void;
  /**
   * Every member case this request published — only known when this dialog
   * was opened from a batch row in the Announcements tab's registry list
   * (the one place this data exists; see AnnouncementRegistryRow's own doc
   * comment). Opened from the Pending tab instead, this is empty, and the
   * "Delivered to" section below simply doesn't render — same graceful
   * "nothing to show" behavior as a legacy published request with no
   * publishedCaseIds at all.
   */
  caseMembers?: AnnouncementRegistryCaseMember[];
}

const STATE_TITLE: Record<string, string> = {
  draft: "Draft",
  pending_approval: "Pending approval",
  approved: "Approved",
  published: "Published",
};

function DetailField({ label, children }: { label: string; children: ReactNode }): JSX.Element {
  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 0.25, minWidth: 0 }}>
      <Typography variant="caption" color="text.secondary">
        {label}
      </Typography>
      <Box>{children}</Box>
    </Box>
  );
}

function whoWhen(who?: string | null, when?: string | null): string {
  if (!who && !when) return "—";
  const whenText = when ? formatAbsoluteForUser(when) : null;
  if (who && whenText) return `${who} · ${whenText}`;
  return who ?? whenText ?? "—";
}

/** The rich-text editor emits `<p></p>` when empty; check the stripped text. */
function isEmptyHtml(html: string): boolean {
  return html.replace(/<[^>]*>/g, "").replace(/&nbsp;/g, " ").trim().length === 0;
}

/**
 * Detail + action dialog for a not-yet-published announcement request,
 * opened from the registry page's "Pending" tab. One dialog covers every
 * state rather than a separate route per state — the record is small and the
 * state-appropriate actions (Submit / Mark as approved / Publish) belong
 * right next to the content they act on. See the Phase 2 plan's own
 * per-state edit-behavior breakdown, mirrored exactly below:
 *  - draft: content is freely editable; "Submit for approval" itself runs
 *    the dry run (creating the one real case in the fixed test project),
 *    records it, and submits — one action, not a separate "run a dry run
 *    first" step, since the dry-run case *is* what gets shared with the
 *    approver (there's nothing to gain from reviewing it before submitting;
 *    editing afterward while `pending_approval` reverts to draft anyway).
 *  - pending_approval: read-only until "Edit" is explicitly confirmed —
 *    editing reverts the request to draft and clears its dry run, since the
 *    content is out for real review over email and a silent change under
 *    the reviewer isn't safe.
 *  - approved: content stays editable in place with no state reset (a human
 *    already said yes over email) — but the frozen audience snapshot from
 *    submit time is read-only here; Publish sends whatever's currently in
 *    the fields to that exact snapshot.
 *  - published: read-only summary, nothing left to do.
 */
export default function AnnouncementRequestDialog({
  requestId,
  onClose,
  caseMembers = [],
}: AnnouncementRequestDialogProps): JSX.Element {
  const { canWrite } = usePortalAccess();
  const { data: request, isLoading, isError, refetch } = useGetAnnouncementRequest(requestId);
  const update = useUpdateAnnouncementRequest();
  const recordDryRun = useRecordAnnouncementRequestDryRun();
  const submit = useSubmitAnnouncementRequest();
  const approve = useApproveAnnouncementRequest();
  const publish = usePublishAnnouncementRequest(request);
  // Publish is restricted to the request's own creator server-side (an
  // approver's job is only to approve, not to also trigger the real send) —
  // this mirrors that here so the button reflects reality instead of
  // failing with a 403 only after being clicked. claims.userid is the same
  // stable per-account identifier the backend's JWT "userid" claim (and so
  // request.createdBy) is sourced from — see IdTokenClaims's own doc
  // comment for why not `sub`, which is per-session.
  const claims = useIdTokenClaims();
  // useIdTokenClaims briefly returns undefined while it decodes the token
  // asynchronously after mount, even for an already-signed-in user (this
  // dialog is only ever reached signed-in, behind AuthGuard) — without
  // distinguishing that from "loaded, and it's someone else," the Publish
  // button would flash disabled with an incorrect "only X can publish" for
  // the real creator on every open, until the token finishes decoding.
  const claimsReady = claims !== undefined;
  const isRequestCreator = !!request && !!claims?.userid && claims.userid === request.createdBy;
  const [confirmPublishOpen, setConfirmPublishOpen] = useState(false);
  const [confirmGiveUpOpen, setConfirmGiveUpOpen] = useState(false);
  const [givingUp, setGivingUp] = useState(false);

  // Resolves failedProjectIds to their real short keys (e.g. "CUPPTSUB") for
  // the send-progress card's chips below — those ids come straight off the
  // frozen resolvedProjectIds snapshot, which carries no key/name data of
  // its own. Re-resolves whenever the failed set actually changes (a retry
  // narrowing it, a fresh failure widening it), not on every render.
  const failedProjectsPreview = useResolvedAudiencePreview();
  const failedProjectIdsKey = publish.failedProjectIds.join(",");
  useEffect(() => {
    if (publish.failedProjectIds.length > 0) {
      void failedProjectsPreview.resolve(publish.failedProjectIds);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [failedProjectIdsKey]);
  const failedProjectLabel = (projectId: string): string =>
    failedProjectsPreview.projects.find((p) => p.id === projectId)?.key ?? projectId;

  // Resolves the frozen resolvedProjectIds snapshot to real short keys for
  // the Audience box below -- otherwise shown as raw, meaningless UUIDs to
  // whoever's reviewing/approving the request. Same resolve-on-change
  // pattern as failedProjectsPreview above; any id that fails to resolve (a
  // fetch error, or past the 200-project preview cap) just falls back to its
  // raw id rather than blocking the rest of the list.
  const audiencePreview = useResolvedAudiencePreview();
  const resolvedProjectIdsKey = request?.resolvedProjectIds?.join(",") ?? "";
  useEffect(() => {
    if (request?.resolvedProjectIds && request.resolvedProjectIds.length > 0) {
      void audiencePreview.resolve(request.resolvedProjectIds);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [resolvedProjectIdsKey]);
  const audienceProjectLabel = (projectId: string): string =>
    audiencePreview.projects.find((p) => p.id === projectId)?.key ?? projectId;

  // Schedule: an alternative to clicking Publish immediately — pick a
  // future date/time and operations/csm-scheduled-tasks' own sub-cron
  // publishes automatically once it arrives. Purely additive: Publish
  // itself (above) keeps every one of its own guards unchanged and still
  // works at any time, schedule pending or not, as an explicit override.
  const schedule = useScheduleAnnouncementRequest();
  const [schedulePickerOpen, setSchedulePickerOpen] = useState(false);
  const [scheduleInput, setScheduleInput] = useState("");
  const scheduleTimeZone = resolveDisplayTimeZone();
  const scheduleInputDate = parseDateTimeLocal(scheduleInput);
  const scheduleInputIsPast = isPastDateTime(scheduleInputDate);

  // Add-update: composing and posting a follow-up comment to every case a
  // published request created. Restricted to the creator, same as Publish
  // and for the same reason.
  const createUpdate = useCreateAnnouncementRequestUpdate();
  const postUpdateComments = usePostAnnouncementUpdateComments();
  const updatesQuery = useListAnnouncementRequestUpdates(request?.id, request?.state === "published");
  const [updateContent, setUpdateContent] = useState("");
  const [confirmUpdateOpen, setConfirmUpdateOpen] = useState(false);
  // Set once createUpdate has recorded the current updateContent, so a
  // retry after a partial comment-fan-out failure only retries the
  // outstanding comments (postUpdateComments already tracks that itself)
  // without creating a second, duplicate AnnouncementRequestUpdate row for
  // the same text. This is plain component state, not persisted anywhere —
  // closing and reopening this dialog mid-retry loses it, the same
  // accepted trade-off usePublishAnnouncementRequest's own doc comment
  // already documents for Publish ("if the dialog is closed mid-retry,
  // progress made so far is lost"). A closed-then-reopened retry here
  // would re-record a second AnnouncementRequestUpdate row for the same
  // text rather than resuming the first one — a duplicate history entry
  // (visible in "Past updates", not silent data loss), not a persistence
  // layer this slice builds for.
  const [recordedUpdateContent, setRecordedUpdateContent] = useState<string | null>(null);

  useEffect(() => {
    if (postUpdateComments.done && recordedUpdateContent !== null) {
      setUpdateContent("");
      setRecordedUpdateContent(null);
    }
    // Only reacts to the fan-out actually finishing, not every render.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [postUpdateComments.done]);

  const handleConfirmUpdate = async (): Promise<void> => {
    if (!request) return;
    setConfirmUpdateOpen(false);
    if (recordedUpdateContent === null) {
      // A genuinely new update, not a retry of one already recorded —
      // postUpdateComments' own per-case tracking must be cleared first, or
      // it would see every case already "succeeded" from whichever earlier
      // update this same dialog instance already posted and silently skip
      // this one's fan-out entirely (see that hook's own reset() doc comment).
      postUpdateComments.reset();
      try {
        await createUpdate.mutateAsync({ id: request.id, payload: { content: updateContent } });
        setRecordedUpdateContent(updateContent);
      } catch {
        return;
      }
    }
    await postUpdateComments.handlePost(request.publishedCaseIds ?? [], updateContent, request.createdBy);
  };

  const [subject, setSubject] = useState("");
  const [description, setDescription] = useState("");
  const [isSecurityAnnouncement, setIsSecurityAnnouncement] = useState(false);
  const [pendingApprovalEditUnlocked, setPendingApprovalEditUnlocked] = useState(false);
  const [confirmEditOpen, setConfirmEditOpen] = useState(false);

  // Re-sync local editable fields whenever the server's own copy changes —
  // covers both the initial load and a save round-tripping back with the
  // server's canonical value.
  useEffect(() => {
    if (!request) return;
    setSubject(request.subject);
    setDescription(request.description);
    setIsSecurityAnnouncement(request.isSecurityAnnouncement);
    // Deliberately narrowed to the specific fields read above, not the whole
    // `request` object, so this doesn't re-fire (and stomp in-progress local
    // edits) on every refetch that leaves those fields unchanged.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [request?.id, request?.subject, request?.description, request?.isSecurityAnnouncement]);

  // A save that reverts pending_approval to draft moves request.state itself,
  // so the "unlocked" flag no longer applies to whatever state comes next —
  // clear it rather than let a stale unlock leak into a future re-approval.
  useEffect(() => {
    if (request?.state !== "pending_approval") setPendingApprovalEditUnlocked(false);
  }, [request?.state]);

  const dryRunTagLabels = useMemo(
    () =>
      isSecurityAnnouncement
        ? [DRY_RUN_TAG_LABEL, SECURITY_ANNOUNCEMENT_TAG_LABEL]
        : [DRY_RUN_TAG_LABEL],
    [isSecurityAnnouncement],
  );
  const dryRun = useAnnouncementDryRun({
    subject,
    description,
    tagLabels: dryRunTagLabels,
    extraCanRun: request?.state === "draft" && !submit.isPending && !recordDryRun.isPending,
  });

  const isEditable =
    request?.state === "draft" ||
    request?.state === "approved" ||
    (request?.state === "pending_approval" && pendingApprovalEditUnlocked);

  // Both primary actions below (Submit for approval, Publish) act on
  // whatever's already persisted server-side, not on these local fields —
  // `submit` takes no body at all, and `publish` reads `request.subject`/
  // `request.description` (the last fetch), not `subject`/`description`.
  // Without this guard, editing a field and immediately clicking Submit or
  // Publish would silently send the *previous* saved content: the approver
  // (or the real customer-facing case) would never see the edit the sender
  // believes they just made.
  const hasUnsavedChanges =
    isEditable &&
    !!request &&
    (subject.trim() !== request.subject ||
      description !== request.description ||
      isSecurityAnnouncement !== request.isSecurityAnnouncement);

  // publish.failedProjectIds non-empty means some projects already got a
  // real case from an earlier attempt and "Retry failed projects" will send
  // to only the rest — using whatever's currently saved. Editing and saving
  // content in between would send the retried projects different content
  // than the ones that already succeeded, silently splitting one
  // announcement into two different messages with no way to reconcile them
  // afterward. Locking here mirrors the same fix already made for the
  // create forms' own immediate-send retry (PR #1834).
  const contentLockedForRetry = request?.state === "approved" && publish.failedProjectIds.length > 0;

  const handleSaveContent = (): void => {
    if (!request) return;
    update.mutate({
      id: request.id,
      subject: subject.trim(),
      description,
      isSecurityAnnouncement,
    });
  };

  const submittingForApproval = dryRun.runningDryRun || recordDryRun.isPending || submit.isPending;

  // "Submit for approval" runs the dry run, records it, and submits in one
  // action — see this component's own doc comment for why there's no
  // separate "run a dry run first" step here. Each mutation's own `isError`
  // already renders inline below, so a failure partway through just leaves
  // the button re-clickable rather than needing its own error handling here.
  const handleSubmitForApproval = async (): Promise<void> => {
    if (!request || submittingForApproval || hasUnsavedChanges) return;
    const result = await dryRun.handleRunDryRun();
    if (!result) return;
    try {
      await recordDryRun.mutateAsync({ id: request.id, caseId: result.caseId });
      await submit.mutateAsync({ id: request.id });
    } catch {
      /* surfaced inline via recordDryRun.isError / submit.isError below */
    }
  };

  const dryRunLink = request?.dryRunCaseId ? (
    <Link to={`/announcements/${request.dryRunCaseId}`} target="_blank" rel="noopener noreferrer">
      view dry run
    </Link>
  ) : (
    "not yet run"
  );

  // AnnouncementSendProgress expects one coherent tally against the whole
  // frozen audience (request.resolvedProjectIds), not just the current
  // round's own subset — publish.progress (from settleWithConcurrencyLimit's
  // onSettle) only covers whatever's pending *this* round (every project on
  // the first send, just the outstanding ones on a retry), so it's summed
  // with succeededProjectIds carried over from any earlier round. While
  // still in flight, in-progress projects are optimistically counted as
  // succeeded — corrected the moment publishing finishes, at which point
  // failed/failedProjectIds take over as the authoritative count instead.
  //
  // Branches on publish.publishing, not publish.progress: a security-tag
  // retry or the final bookkeeping /publish call both run with publishing
  // still true but progress back to null (see usePublishAnnouncementRequest
  // — neither the tag-retry pass nor the bookkeeping call touches progress
  // at all). Branching on progress alone showed "Announcement sent" during
  // those windows — completed already equalled total from the case-create
  // side — while the button right next to it still said "Publishing…", a
  // visible contradiction. Capping completed just below total whenever
  // publishing is true (regardless of which sub-phase) keeps the title on
  // "Sending announcement…" until the whole call actually finishes.
  const totalResolvedProjects = request?.resolvedProjectIds?.length ?? 0;
  const priorSucceededCount = publish.succeededProjectIds.length;
  const sendProgress: AnnouncementSendProgressState = publish.publishing
    ? {
        total: totalResolvedProjects,
        completed: publish.progress
          ? priorSucceededCount + publish.progress.completed
          : Math.min(priorSucceededCount, Math.max(totalResolvedProjects - 1, 0)),
        succeeded: publish.progress
          ? priorSucceededCount + publish.progress.completed
          : priorSucceededCount,
        failed: 0,
        failedProjectIds: [],
      }
    : {
        total: totalResolvedProjects,
        completed: priorSucceededCount + publish.failedProjectIds.length,
        succeeded: priorSucceededCount,
        failed: publish.failedProjectIds.length,
        failedProjectIds: publish.failedProjectIds,
      };

  return (
    <Dialog open onClose={onClose} maxWidth="sm" fullWidth>
      <DialogTitle sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 1 }}>
        <Typography variant="subtitle1" component="span">
          Announcement request{request ? ` · ${STATE_TITLE[request.state] ?? request.state}` : ""}
        </Typography>
        <IconButton size="small" onClick={onClose} aria-label="Close">
          <X size={16} />
        </IconButton>
      </DialogTitle>
      <DialogContent dividers sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        {isLoading && (
          <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
            <Skeleton variant="text" width="60%" />
            <Skeleton variant="rounded" height={120} />
          </Box>
        )}

        {isError && (
          <Box sx={{ display: "flex", flexDirection: "column", alignItems: "center", gap: 1, py: 3 }}>
            <Typography variant="body2" color="error">
              Could not load this announcement request.
            </Typography>
            <Button
              size="small"
              variant="outlined"
              startIcon={<RefreshCw size={14} />}
              onClick={() => void refetch()}
            >
              Retry
            </Button>
          </Box>
        )}

        {!isLoading && !isError && request && (
          <>
            {isEditable ? (
              <>
                <TextField
                  label="Subject"
                  value={subject}
                  onChange={(e) => setSubject(e.target.value)}
                  fullWidth
                  size="small"
                  disabled={contentLockedForRetry}
                />
                <Box>
                  <Typography variant="caption" color="text.secondary" sx={{ display: "block", mb: 0.5 }}>
                    Description
                  </Typography>
                  <EditorWithSourceToggle
                    value={description}
                    onChange={setDescription}
                    placeholder="Describe the announcement…"
                    minHeight={140}
                    maxHeight={320}
                    toolbarVariant="full"
                    disabled={contentLockedForRetry}
                  />
                </Box>
                {request.kind === "customer" && (
                  <FormControlLabel
                    control={
                      <Checkbox
                        checked={isSecurityAnnouncement}
                        disabled={contentLockedForRetry}
                        onChange={(e) => setIsSecurityAnnouncement(e.target.checked)}
                      />
                    }
                    label="Security announcement"
                  />
                )}
                {contentLockedForRetry && (
                  <Typography variant="caption" color="text.secondary">
                    Subject, description, and the security label are locked while retrying failed
                    projects — this resend must match what the succeeded projects already got.
                  </Typography>
                )}
                <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                  <Button
                    variant="outlined"
                    size="small"
                    onClick={handleSaveContent}
                    disabled={
                      update.isPending ||
                      subject.trim().length === 0 ||
                      contentLockedForRetry ||
                      !canWrite
                    }
                  >
                    {update.isPending ? "Saving…" : "Save changes"}
                  </Button>
                  {update.isError && (
                    <Typography variant="caption" color="error">
                      {update.error instanceof Error ? update.error.message : "Could not save changes."}
                    </Typography>
                  )}
                </Box>
              </>
            ) : (
              <>
                <Box sx={{ display: "flex", alignItems: "center", gap: 0.75 }}>
                  <Typography variant="body1" sx={{ fontWeight: 600 }}>
                    {request.subject || "(no subject)"}
                  </Typography>
                  {request.isSecurityAnnouncement && (
                    <Chip size="small" color="warning" label="Security" sx={{ flexShrink: 0 }} />
                  )}
                </Box>
                <Box
                  sx={{ fontSize: "0.875rem", lineHeight: 1.5, wordBreak: "break-word" }}
                  dangerouslySetInnerHTML={{ __html: sanitizeRichTextHtml(request.description) }}
                />
              </>
            )}

            <Divider />

            <Box
              sx={{
                display: "grid",
                gridTemplateColumns: "repeat(auto-fill, minmax(160px, 1fr))",
                gap: 2,
              }}
            >
              <DetailField label="Kind">
                <Typography variant="body2">{request.kind === "eol" ? "EOL" : "Customer"}</Typography>
              </DetailField>
              <DetailField label="Dry run">
                <Typography variant="body2">{dryRunLink}</Typography>
              </DetailField>
              <DetailField label="Audience">
                <Typography variant="body2">
                  {request.resolvedProjectCount != null
                    ? `${request.resolvedProjectCount} project${request.resolvedProjectCount === 1 ? "" : "s"}`
                    : "not resolved yet"}
                </Typography>
              </DetailField>
              <DetailField label="Created">
                <Typography variant="body2">
                  {whoWhen(request.createdByEmail ?? request.createdBy, request.createdAt)}
                </Typography>
              </DetailField>
              {request.submittedAt && (
                <DetailField label="Submitted">
                  <Typography variant="body2">
                    {whoWhen(request.submittedByEmail ?? request.submittedBy, request.submittedAt)}
                  </Typography>
                </DetailField>
              )}
              {request.approvedAt && (
                <DetailField label="Approved">
                  <Typography variant="body2">
                    {whoWhen(request.approvedByEmail ?? request.approvedBy, request.approvedAt)}
                  </Typography>
                </DetailField>
              )}
              {request.publishedAt && (
                <DetailField label="Published">
                  <Typography variant="body2">
                    {whoWhen(request.publishedByEmail ?? request.publishedBy, request.publishedAt)}
                  </Typography>
                </DetailField>
              )}
            </Box>

            {request.resolvedProjectIds && request.resolvedProjectIds.length > 0 && (
              <Box
                sx={{
                  border: 1,
                  borderColor: "divider",
                  borderRadius: 1,
                  maxHeight: 120,
                  overflowY: "auto",
                  p: 1,
                }}
              >
                <Typography
                  variant="caption"
                  color="text.secondary"
                  sx={{ fontFamily: "monospace", whiteSpace: "pre-wrap" }}
                >
                  {audiencePreview.isLoading
                    ? "Resolving project names…"
                    : request.resolvedProjectIds.map(audienceProjectLabel).join(", ")}
                </Typography>
              </Box>
            )}

            {request.state === "pending_approval" && !pendingApprovalEditUnlocked && (
              <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                <Button
                  variant="contained"
                  size="small"
                  onClick={() => approve.mutate({ id: request.id })}
                  disabled={approve.isPending || !canWrite}
                >
                  {approve.isPending ? "Approving…" : "Mark as approved"}
                </Button>
                <Button
                  variant="text"
                  size="small"
                  onClick={() => setConfirmEditOpen(true)}
                  disabled={!canWrite}
                >
                  Edit
                </Button>
                {approve.isError && (
                  <Typography variant="caption" color="error">
                    {approve.error instanceof Error ? approve.error.message : "Could not approve."}
                  </Typography>
                )}
              </Box>
            )}

            {request.state === "draft" && (
              <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                <Button
                  variant="contained"
                  color="primary"
                  size="small"
                  onClick={() => void handleSubmitForApproval()}
                  disabled={
                    submittingForApproval ||
                    hasUnsavedChanges ||
                    subject.trim().length === 0 ||
                    isEmptyHtml(description) ||
                    !canWrite
                  }
                >
                  {dryRun.runningDryRun
                    ? "Running dry run…"
                    : submittingForApproval
                      ? "Submitting…"
                      : "Submit for approval"}
                </Button>
                <Typography variant="caption" color="text.secondary">
                  {hasUnsavedChanges
                    ? "Save your changes first — Submit sends whatever's currently saved, not what's still unsaved here."
                    : "Creates a real case in the DCPSUB test project to share with your approver."}
                </Typography>
                {(recordDryRun.isError || submit.isError) && (
                  <Typography variant="caption" color="error">
                    {submit.error instanceof Error
                      ? submit.error.message
                      : recordDryRun.error instanceof Error
                        ? recordDryRun.error.message
                        : "Could not submit for approval."}
                  </Typography>
                )}
              </Box>
            )}

            {request.state === "approved" && (
              <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
                <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                  <Button
                    variant="contained"
                    color="primary"
                    size="small"
                    onClick={() =>
                      !hasUnsavedChanges &&
                      claimsReady &&
                      isRequestCreator &&
                      publish.readyToPublish &&
                      setConfirmPublishOpen(true)
                    }
                    disabled={
                      publish.publishing ||
                      hasUnsavedChanges ||
                      !claimsReady ||
                      !isRequestCreator ||
                      !publish.readyToPublish ||
                      !canWrite
                    }
                  >
                    {!claimsReady
                      ? "Publish"
                      : publish.publishing
                        ? "Publishing…"
                        : publish.failedProjectIds.length > 0
                          ? "Retry failed projects"
                          : publish.failedTagProjectIds.length > 0
                            ? "Retry security label"
                            : "Publish"}
                  </Button>
                  {claimsReady &&
                    isRequestCreator &&
                    publish.readyToPublish &&
                    !publish.publishing &&
                    publish.failedProjectIds.length > 0 &&
                    publish.failedTagProjectIds.length === 0 &&
                    publish.succeededProjectIds.length > 0 && (
                      <Button
                        variant="outlined"
                        color="warning"
                        size="small"
                        disabled={hasUnsavedChanges || !canWrite}
                        onClick={() => setConfirmGiveUpOpen(true)}
                      >
                        Publish anyway
                      </Button>
                    )}
                </Box>
                {claimsReady && !isRequestCreator && (
                  <Typography variant="caption" color="text.secondary">
                    Only {request.createdByEmail ?? request.createdBy} can publish this request — approving it
                    doesn't grant that.
                  </Typography>
                )}
                {claimsReady && isRequestCreator && hasUnsavedChanges && (
                  <Typography variant="caption" color="text.secondary">
                    Save your changes first — Publish sends whatever's currently saved, not what's still
                    unsaved here.
                  </Typography>
                )}
                {claimsReady && isRequestCreator && !hasUnsavedChanges && publish.hydratingDeliveries && (
                  <Typography variant="caption" color="text.secondary">
                    Loading previous progress…
                  </Typography>
                )}
                {claimsReady && isRequestCreator && !hasUnsavedChanges && publish.hydrationFailed && (
                  <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                    <Typography variant="caption" color="error">
                      Couldn't load previous progress for this request — Publish is blocked until this
                      loads, so an already-sent project isn't sent a duplicate case.
                    </Typography>
                    <Button
                      size="small"
                      variant="outlined"
                      startIcon={<RefreshCw size={14} />}
                      onClick={publish.retryHydration}
                    >
                      Retry
                    </Button>
                  </Box>
                )}
                {(publish.publishing || priorSucceededCount > 0 || publish.failedProjectIds.length > 0) && (
                  <AnnouncementSendProgress
                    progress={sendProgress}
                    // Once there's an outstanding failure, the succeeded
                    // count is either stale history (reopening a request
                    // with prior progress) or already visible via the
                    // "Retry failed projects" flow itself — only the
                    // currently-failing projects need attention. A fully
                    // successful send (no failures at all) still shows the
                    // reassuring full tally.
                    hideSucceededTally={publish.failedProjectIds.length > 0}
                    projectLabel={failedProjectLabel}
                  />
                )}
                {publish.failedTagProjectIds.length > 0 && (
                  <Typography variant="caption" color="warning.main">
                    Security label couldn't be attached for: {publish.failedTagProjectIds.join(", ")} —
                    retry before this can be published.
                  </Typography>
                )}

                <Divider />

                {request.dueOn && (
                  <Typography variant="caption" color="text.secondary">
                    Due {formatAbsoluteForUser(request.dueOn)}
                  </Typography>
                )}

                {request.scheduledFor ? (
                  <Box sx={{ display: "flex", alignItems: "center", gap: 1, flexWrap: "wrap" }}>
                    <Typography variant="body2">
                      Scheduled to publish on {formatAbsoluteForUser(request.scheduledFor)}
                    </Typography>
                    <Button
                      size="small"
                      variant="text"
                      color="error"
                      disabled={schedule.isPending || !canWrite || !isRequestCreator}
                      onClick={() => schedule.mutate({ id: request.id, scheduledFor: null })}
                    >
                      Cancel schedule
                    </Button>
                  </Box>
                ) : (
                  isRequestCreator &&
                  canWrite &&
                  (schedulePickerOpen ? (
                    <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
                      <LocalizationProvider dateAdapter={AdapterDateFns}>
                        <DateTimePicker
                          label={`Publish at (${scheduleTimeZone})`}
                          value={scheduleInputDate}
                          onChange={(next) =>
                            setScheduleInput(next instanceof Date && !Number.isNaN(next.getTime()) ? formatDateTimeLocal(next) : "")
                          }
                          disabled={schedule.isPending}
                          slotProps={{
                            textField: {
                              fullWidth: true,
                              size: "small",
                              error: !!scheduleInput && scheduleInputIsPast,
                              helperText: scheduleInputIsPast
                                ? "Must be in the future."
                                : `Entered in your timezone (${scheduleTimeZone}); stored as UTC.`,
                            },
                          }}
                        />
                      </LocalizationProvider>
                      <Box sx={{ display: "flex", gap: 1 }}>
                        <Button
                          size="small"
                          variant="contained"
                          disabled={!scheduleInput || scheduleInputIsPast || schedule.isPending}
                          onClick={() => {
                            const iso = zonedInputToUtcIso(scheduleInput, scheduleTimeZone);
                            if (!iso) return;
                            schedule.mutate(
                              { id: request.id, scheduledFor: iso },
                              { onSuccess: () => setSchedulePickerOpen(false) },
                            );
                          }}
                        >
                          {schedule.isPending ? "Scheduling…" : "Confirm schedule"}
                        </Button>
                        <Button
                          size="small"
                          variant="text"
                          disabled={schedule.isPending}
                          onClick={() => {
                            setSchedulePickerOpen(false);
                            setScheduleInput("");
                          }}
                        >
                          Cancel
                        </Button>
                      </Box>
                      {schedule.isError && (
                        <Typography variant="caption" color="error">
                          {schedule.error instanceof Error ? schedule.error.message : "Could not set the schedule."}
                        </Typography>
                      )}
                    </Box>
                  ) : (
                    <Box>
                      <Button size="small" variant="outlined" onClick={() => setSchedulePickerOpen(true)}>
                        Schedule for later…
                      </Button>
                    </Box>
                  ))
                )}
              </Box>
            )}

            {request.state === "published" && caseMembers.length > 0 && (
              <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
                <Divider />
                <Typography variant="subtitle2">
                  Delivered to {caseMembers.length} project{caseMembers.length === 1 ? "" : "s"}
                </Typography>
                {/* A real send can reach ~100 projects, so this is a dense,
                    scrollable list rather than one card per case (the shape
                    "Past updates" below uses, fine there since those are
                    rare and rich-text) — bounded height keeps the dialog
                    itself from growing without limit alongside the list. */}
                <Box
                  sx={{
                    maxHeight: 220,
                    overflowY: "auto",
                    border: 1,
                    borderColor: "divider",
                    borderRadius: 1,
                  }}
                >
                  {caseMembers.map((m) => (
                    <Box
                      key={m.caseId}
                      component={Link}
                      to={`/announcements/${m.caseId}`}
                      sx={{
                        display: "flex",
                        justifyContent: "space-between",
                        gap: 1,
                        px: 1.5,
                        py: 0.75,
                        textDecoration: "none",
                        color: "inherit",
                        borderBottom: 1,
                        borderColor: "divider",
                        "&:last-of-type": { borderBottom: 0 },
                        "&:hover": { bgcolor: "action.hover" },
                      }}
                    >
                      <Typography variant="body2" noWrap>
                        {m.projectName || "—"}
                      </Typography>
                      <Typography variant="body2" color="text.secondary" sx={{ flexShrink: 0 }}>
                        {m.caseNumber || m.wso2CaseId || "—"}
                      </Typography>
                    </Box>
                  ))}
                </Box>
              </Box>
            )}

            {request.state === "published" && (
              <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
                <Divider />
                <Typography variant="subtitle2">Post an update</Typography>
                <Typography variant="body2" color="text.secondary">
                  Appends a dated follow-up as a real comment on every case this announcement created —
                  not a replacement for the original content.
                </Typography>

                {claimsReady && !isRequestCreator && (
                  <Typography variant="caption" color="text.secondary">
                    Only {request.createdByEmail ?? request.createdBy} can post an update to this request.
                  </Typography>
                )}
                {claimsReady && isRequestCreator && (request.publishedCaseIds ?? []).length === 0 && (
                  <Typography variant="caption" color="text.secondary">
                    This request was published before case tracking existed — there's nothing to post an
                    update to.
                  </Typography>
                )}
                {(!claimsReady ||
                  (isRequestCreator && (request.publishedCaseIds ?? []).length > 0)) && (
                  <>
                    <EditorWithSourceToggle value={updateContent} onChange={setUpdateContent} />
                    <Box>
                      <Button
                        variant="outlined"
                        size="small"
                        disabled={
                          !claimsReady ||
                          !isRequestCreator ||
                          updateContent.trim().length === 0 ||
                          createUpdate.isPending ||
                          postUpdateComments.posting ||
                          (request.publishedCaseIds ?? []).length === 0 ||
                          !canWrite
                        }
                        onClick={() => setConfirmUpdateOpen(true)}
                      >
                        {createUpdate.isPending || postUpdateComments.posting ? "Posting…" : "Post update"}
                      </Button>
                    </Box>
                    {createUpdate.isError && (
                      <Typography variant="caption" color="error">
                        Could not record the update. Try again.
                      </Typography>
                    )}
                    {postUpdateComments.failedCaseIds.length > 0 && (
                      <Typography variant="caption" color="warning.main">
                        Couldn&apos;t post to {postUpdateComments.failedCaseIds.length} case
                        {postUpdateComments.failedCaseIds.length === 1 ? "" : "s"} — retry to resend just
                        those.
                      </Typography>
                    )}
                  </>
                )}

                {updatesQuery.data && updatesQuery.data.updates.length > 0 && (
                  <Box sx={{ display: "flex", flexDirection: "column", gap: 1, mt: 1 }}>
                    <Typography variant="subtitle2">Past updates</Typography>
                    {updatesQuery.data.updates.map((u) => (
                      <Box key={u.id} sx={{ border: 1, borderColor: "divider", borderRadius: 1, p: 1.5 }}>
                        <Typography variant="caption" color="text.secondary">
                          {whoWhen(u.createdByEmail ?? u.createdBy, u.createdOn)}
                        </Typography>
                        <Box
                          sx={{ fontSize: "0.875rem", lineHeight: 1.5, wordBreak: "break-word", mt: 0.5 }}
                          dangerouslySetInnerHTML={{ __html: sanitizeRichTextHtml(u.content) }}
                        />
                      </Box>
                    ))}
                  </Box>
                )}
              </Box>
            )}
          </>
        )}
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>Close</Button>
      </DialogActions>

      {confirmEditOpen && (
        <Dialog open onClose={() => setConfirmEditOpen(false)} maxWidth="xs" fullWidth>
          <DialogTitle>Edit this request?</DialogTitle>
          <DialogContent>
            <Typography variant="body2">
              Editing will revert this request to draft and clear its recorded dry run, since the
              content is currently out for review over email. You'll need to run a new dry run
              and submit it again.
            </Typography>
          </DialogContent>
          <DialogActions>
            <Button onClick={() => setConfirmEditOpen(false)}>Cancel</Button>
            <Button
              variant="contained"
              color="warning"
              onClick={() => {
                setPendingApprovalEditUnlocked(true);
                setConfirmEditOpen(false);
              }}
            >
              Continue editing
            </Button>
          </DialogActions>
        </Dialog>
      )}

      {request && (
        <PublishConfirmationDialog
          open={confirmPublishOpen}
          request={request}
          isRetry={publish.failedProjectIds.length > 0 || publish.failedTagProjectIds.length > 0}
          confirming={publish.publishing}
          onCancel={() => setConfirmPublishOpen(false)}
          onConfirm={() => {
            setConfirmPublishOpen(false);
            void publish.handlePublish();
          }}
        />
      )}

      {request && (
        <Dialog open={confirmGiveUpOpen} onClose={givingUp ? undefined : () => setConfirmGiveUpOpen(false)} maxWidth="sm" fullWidth>
          <DialogTitle>Publish without the failed projects?</DialogTitle>
          <DialogContent sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
            <Typography variant="body2">
              This marks the request published using only the {publish.succeededProjectIds.length} project
              {publish.succeededProjectIds.length === 1 ? "" : "s"} that already received a case. The
              following {publish.failedProjectIds.length === 1 ? "project" : "projects"} will be permanently
              skipped — there's no way to send this announcement to {publish.failedProjectIds.length === 1 ? "it" : "them"} afterward:
            </Typography>
            <Typography variant="body2" fontWeight={600} color="error.main">
              {publish.failedProjectIds.map(failedProjectLabel).join(", ")}
            </Typography>
            <Typography variant="body2" color="text.secondary">
              Only use this once you've confirmed the retry genuinely can't succeed — a project that's just
              slow or transiently failing should be retried instead.
            </Typography>
          </DialogContent>
          <DialogActions>
            <Button onClick={() => setConfirmGiveUpOpen(false)} disabled={givingUp}>
              Cancel
            </Button>
            <Button
              variant="contained"
              color="warning"
              disabled={givingUp}
              onClick={async () => {
                setGivingUp(true);
                try {
                  await publish.publishGivingUpOnFailed();
                } finally {
                  setGivingUp(false);
                  setConfirmGiveUpOpen(false);
                }
              }}
            >
              {givingUp ? "Publishing…" : "Publish anyway"}
            </Button>
          </DialogActions>
        </Dialog>
      )}

      {request && (
        <AddUpdateConfirmationDialog
          open={confirmUpdateOpen}
          content={updateContent}
          caseCount={(request.publishedCaseIds ?? []).length}
          confirming={createUpdate.isPending || postUpdateComments.posting}
          onCancel={() => setConfirmUpdateOpen(false)}
          onConfirm={() => void handleConfirmUpdate()}
        />
      )}
    </Dialog>
  );
}
