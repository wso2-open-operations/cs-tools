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
  Checkbox,
  FormControlLabel,
  Grid,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { useMemo, useState, type JSX } from "react";
import type { BeSubscriptionType } from "@api/backend/types";
import EditorWithSourceToggle from "@components/rich-text-editor/EditorWithSourceToggle";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import { DRY_RUN_TAG_LABEL, useAnnouncementDryRun } from "@features/csm-announcements/api/useAnnouncementDryRun";
import { useAnnouncementExcludedProjectKeys } from "@features/csm-announcements/api/useAnnouncementExcludedProjectKeys";
import { useResolveAnnouncementAudience } from "@features/csm-announcements/api/useResolveAnnouncementAudience";
import { useCreateAnnouncementRequest } from "@features/csm-announcements/api/useCreateAnnouncementRequest";
import { useUpdateAnnouncementRequest } from "@features/csm-announcements/api/useUpdateAnnouncementRequest";
import { useRecordAnnouncementRequestDryRun } from "@features/csm-announcements/api/useRecordAnnouncementRequestDryRun";
import { useSubmitAnnouncementRequest } from "@features/csm-announcements/api/useSubmitAnnouncementRequest";
import AudienceScopeControls, {
  type AnnouncementAudienceScope,
} from "@features/csm-announcements/components/AudienceScopeControls";
import ResolvedAudienceList from "@features/csm-announcements/components/ResolvedAudienceList";
import type { CustomerAudienceDefinition } from "@features/csm-announcements/types/announcementRequests";
import { useNavTransition } from "@hooks/useNavTransition";

/**
 * The two default exclusions offered for the "all customer projects" scope,
 * mirroring the ServiceNow flow conditions this replaces (see
 * AudienceScopeControls' own doc comment for the Account Life Cycle caveat).
 */
const CLOUD_SUBSCRIPTION_TYPES: BeSubscriptionType[] = ["cloud_support", "cloud_evaluation_support"];
const CLOSED_STATES: string[] = ["Restricted", "Suspended"];

/**
 * The fixed tag label attached to every case in a security announcement.
 * There is no dedicated "announcement type" field anywhere in the platform
 * yet (confirmed: CreateCaseRequest's announcement branch only validates
 * subject/description) — this reuses the existing free-text case-tag
 * mechanism, which already works end-to-end for announcement cases and
 * already renders as a chip on the case detail page's Details tab with no
 * further changes needed. A fixed, exact string (rather than free typing)
 * is what makes it findable/filterable later despite tags having no closed
 * vocabulary on the backend.
 */
export const SECURITY_ANNOUNCEMENT_TAG_LABEL = "Security Announcement";

/** The rich-text editor emits `<p></p>` when empty; check the stripped text. */
function isEmptyHtml(html: string): boolean {
  return html.replace(/<[^>]*>/g, "").replace(/&nbsp;/g, " ").trim().length === 0;
}

const PENDING_TARGET = "/announcements?tab=pending";

/**
 * The "Create announcement for customers" flow (Option 1 of the two-option
 * announcement create page — see AnnouncementKindSelector). Builds a
 * `kind: "customer"` announcement request (Phase 2's draft/approval
 * workflow) rather than sending immediately: this page only ever produces a
 * brand-new `draft` and, optionally, submits it for approval — every other
 * lifecycle step (further edits, re-approval, publish) happens through
 * {@link AnnouncementRequestDialog} from the registry's "Pending" tab, not
 * here. That's a deliberate boundary (see the Phase 2 plan): this component
 * doesn't need any "resume an existing draft" state machine of its own.
 *
 * Two audience scopes (see AudienceScopeControls): a hand-picked project
 * list, or "all customer projects" resolved from the entity service under a
 * pair of default exclusions. `ResolvedAudienceList` here is purely a
 * pre-submit review aid (matching Phase 1's "resolved recipient list before
 * send" intent) — the actual audience that gets messaged is resolved fresh,
 * server-side, at Submit time from the request's stored `audienceDefinition`
 * (with the mandatory excluded-project-key denylist applied), not from
 * whatever this list happened to show a moment earlier.
 *
 * "Submit for approval" runs the dry run (the shared useAnnouncementDryRun
 * mechanism, also used by the EOL flow — creates one real case in a fixed
 * test project), then creates/updates the draft, records the dry run onto
 * it, and submits — all in one click, not a separate "run a dry run first"
 * step. The real announcement process this replaces shares that exact
 * dry-run case's link with the approver for review, so there's nothing to
 * gain from a distinct preview step before submitting; if the sender wants
 * to change anything afterward, editing a `pending_approval` request (via
 * the dialog) already reverts it to draft for a fresh attempt.
 */
export default function CreateCustomerAnnouncementForm(): JSX.Element {
  const navigate = useNavTransition();
  const { showError } = useErrorBanner();

  const [scope, setScope] = useState<AnnouncementAudienceScope>("specific");
  const [projectIds, setProjectIds] = useState<string[]>([]);
  const [excludeCloudTypes, setExcludeCloudTypes] = useState(true);
  const [excludeClosedStates, setExcludeClosedStates] = useState(true);
  const [subject, setSubject] = useState("");
  const [description, setDescription] = useState("");
  const [isSecurityAnnouncement, setIsSecurityAnnouncement] = useState(false);
  const [savingDraft, setSavingDraft] = useState(false);
  const [submittingForApproval, setSubmittingForApproval] = useState(false);
  const [draftId, setDraftId] = useState<string | null>(null);

  const createDraft = useCreateAnnouncementRequest();
  const updateDraft = useUpdateAnnouncementRequest();
  const recordDryRun = useRecordAnnouncementRequestDryRun();
  const submitRequest = useSubmitAnnouncementRequest();

  const dryRunTagLabels = useMemo(
    () => [DRY_RUN_TAG_LABEL, ...(isSecurityAnnouncement ? [SECURITY_ANNOUNCEMENT_TAG_LABEL] : [])],
    [isSecurityAnnouncement],
  );
  const busy = savingDraft || submittingForApproval;
  const { runningDryRun, canRunDryRun, handleRunDryRun } = useAnnouncementDryRun({
    subject,
    description,
    tagLabels: dryRunTagLabels,
    extraCanRun: !busy,
  });

  const audienceFilters = useMemo(
    () => ({
      excludeSubscriptionTypes: excludeCloudTypes ? CLOUD_SUBSCRIPTION_TYPES : [],
      excludeClosureStates: excludeClosedStates ? CLOSED_STATES : [],
    }),
    [excludeCloudTypes, excludeClosedStates],
  );
  const resolvedAudience = useResolveAnnouncementAudience(scope === "all", audienceFilters);
  const excludedProjectKeysQuery = useAnnouncementExcludedProjectKeys();

  // The scope actually being sent: today's hand-picked list, or every id
  // resolved for "all customer projects" (see AudienceScopeControls' own doc
  // comment on what that scope does and doesn't filter). Just a pre-submit
  // sanity check here — the real audience is resolved again server-side.
  const targetProjectIds = useMemo(
    () => (scope === "all" ? resolvedAudience.projects.map((p) => p.id) : projectIds),
    [scope, resolvedAudience.projects, projectIds],
  );

  const audienceDefinition: CustomerAudienceDefinition = useMemo(
    () =>
      scope === "specific"
        ? { scope: "specific", projectIds }
        : {
            scope: "all",
            excludeClosureStates: excludeClosedStates ? CLOSED_STATES : [],
            excludeSubscriptionTypes: excludeCloudTypes ? CLOUD_SUBSCRIPTION_TYPES : [],
          },
    [scope, projectIds, excludeClosedStates, excludeCloudTypes],
  );

  const canSaveDraft = subject.trim().length > 0 && !isEmptyHtml(description) && !busy;

  const canSubmitForApproval =
    canRunDryRun &&
    targetProjectIds.length > 0 &&
    !(scope === "all" && resolvedAudience.isLoading) &&
    // TanStack Query can retain a previous successful fetch's `data` after a
    // later refetch fails (isLoading goes back to false, but the stale list
    // is still sitting there) — without this check, targetProjectIds would
    // still look populated and a resolution failure could let the sender
    // submit against an audience that's actually out of date.
    !(scope === "all" && resolvedAudience.isError) &&
    !busy;

  const handleSaveDraft = async (): Promise<void> => {
    if (!canSaveDraft) return;
    setSavingDraft(true);
    try {
      if (draftId) {
        await updateDraft.mutateAsync({
          id: draftId,
          subject: subject.trim(),
          description,
          isSecurityAnnouncement,
          audienceDefinition,
        });
      } else {
        const created = await createDraft.mutateAsync({
          kind: "customer",
          subject: subject.trim(),
          description,
          isSecurityAnnouncement,
          audienceDefinition,
        });
        setDraftId(created.id);
      }
      navigate(PENDING_TARGET);
    } catch {
      showError("Could not save this draft. Please try again.");
    } finally {
      setSavingDraft(false);
    }
  };

  const handleSubmitForApproval = async (): Promise<void> => {
    if (!canSubmitForApproval) return;
    setSubmittingForApproval(true);
    try {
      const result = await handleRunDryRun();
      if (!result) return; // useAnnouncementDryRun already surfaced the error

      let id = draftId;
      if (id) {
        await updateDraft.mutateAsync({
          id,
          subject: subject.trim(),
          description,
          isSecurityAnnouncement,
          audienceDefinition,
        });
      } else {
        const created = await createDraft.mutateAsync({
          kind: "customer",
          subject: subject.trim(),
          description,
          isSecurityAnnouncement,
          audienceDefinition,
        });
        id = created.id;
        setDraftId(id);
      }

      await recordDryRun.mutateAsync({ id, caseId: result.caseId });
      await submitRequest.mutateAsync({ id });
      navigate(PENDING_TARGET);
    } catch (error) {
      showError(
        error instanceof Error && error.message.trim()
          ? error.message
          : "Could not submit this request for approval. Please try again.",
      );
    } finally {
      setSubmittingForApproval(false);
    }
  };

  return (
    <Card variant="outlined" sx={{ p: 3 }}>
      <Grid container spacing={2.5}>
        <Grid size={{ xs: 12 }}>
          <AudienceScopeControls
            scope={scope}
            onScopeChange={setScope}
            projectIds={projectIds}
            onProjectIdsChange={setProjectIds}
            excludeCloudTypes={excludeCloudTypes}
            onExcludeCloudTypesChange={setExcludeCloudTypes}
            excludeClosedStates={excludeClosedStates}
            onExcludeClosedStatesChange={setExcludeClosedStates}
            excludedProjectKeys={excludedProjectKeysQuery.data ?? []}
            disabled={busy}
          />
        </Grid>

        {scope === "all" && (
          <Grid size={{ xs: 12 }}>
            <ResolvedAudienceList
              projects={resolvedAudience.projects}
              total={resolvedAudience.total}
              isLoading={resolvedAudience.isLoading}
              isError={resolvedAudience.isError}
            />
          </Grid>
        )}

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
          <FormControlLabel
            control={
              <Checkbox
                size="small"
                checked={isSecurityAnnouncement}
                disabled={busy}
                onChange={(e) => setIsSecurityAnnouncement(e.target.checked)}
              />
            }
            label="This is a security announcement"
          />
          <Typography variant="caption" color="text.secondary" sx={{ display: "block", mt: -0.5 }}>
            Attaches a &quot;{SECURITY_ANNOUNCEMENT_TAG_LABEL}&quot; label to every case this
            creates.
          </Typography>
        </Grid>
        <Grid size={{ xs: 12 }}>
          <Typography
            id="announcement-description-label"
            component="label"
            variant="caption"
            color="text.secondary"
            sx={{ display: "block", mb: 0.5 }}
          >
            Description
          </Typography>
          {/* Editor doesn't accept an `id`, so associate the label by wrapping
              the editor in a labelled group for assistive tech. */}
          <Box role="group" aria-labelledby="announcement-description-label">
            <EditorWithSourceToggle
              value={description}
              onChange={setDescription}
              placeholder="Describe the announcement…"
              minHeight={180}
              maxHeight={420}
              toolbarVariant="full"
              disabled={busy}
            />
          </Box>
        </Grid>
      </Grid>

      <Box
        sx={{
          display: "flex",
          justifyContent: "flex-end",
          alignItems: "center",
          gap: 1.5,
          mt: 2.5,
          pt: 2,
          borderTop: 1,
          borderColor: "divider",
        }}
      >
        <Button variant="outlined" onClick={() => navigate("/announcements")}>
          Cancel
        </Button>
        <Button variant="outlined" onClick={() => void handleSaveDraft()} disabled={!canSaveDraft}>
          {savingDraft ? "Saving…" : "Save as draft"}
        </Button>
        <Button
          variant="contained"
          onClick={() => void handleSubmitForApproval()}
          disabled={!canSubmitForApproval}
        >
          {runningDryRun ? "Running dry run…" : submittingForApproval ? "Submitting…" : "Submit for approval"}
        </Button>
      </Box>
      <Typography variant="caption" color="text.secondary" sx={{ display: "block", textAlign: "right", mt: 0.5 }}>
        Submitting creates a real case in the DCPSUB test project to share with your approver.
      </Typography>
    </Card>
  );
}
