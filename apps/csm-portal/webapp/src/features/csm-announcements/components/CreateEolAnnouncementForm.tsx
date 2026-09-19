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
  CircularProgress,
  FormControl,
  FormHelperText,
  Grid,
  InputLabel,
  MenuItem,
  Select,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { useCallback, useMemo, useState, type JSX } from "react";
import EditorWithSourceToggle from "@components/rich-text-editor/EditorWithSourceToggle";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import { useAnnouncementDryRun, DRY_RUN_TAG_LABEL } from "@features/csm-announcements/api/useAnnouncementDryRun";
import { useResolveProductVersionAudience } from "@features/csm-announcements/api/useResolveProductVersionAudience";
import AnnouncementDryRunCard from "@features/csm-announcements/components/AnnouncementDryRunCard";
import AnnouncementSendProgress, {
  type AnnouncementSendProgressState,
} from "@features/csm-announcements/components/AnnouncementSendProgress";
import ResolvedAudienceList from "@features/csm-announcements/components/ResolvedAudienceList";
import {
  ANNOUNCEMENT_CASE_CREATE_CONCURRENCY_LIMIT,
  settleWithConcurrencyLimit,
} from "@features/csm-announcements/utils/settleWithConcurrencyLimit";
import { usePostCsmCase } from "@features/csm-cases/api/usePostCsmCase";
import { useSearchProducts } from "@features/csm-projects/api/useSearchProducts";
import { useSearchProductVersions } from "@features/csm-projects/api/useSearchProductVersions";
import { useNavTransition } from "@hooks/useNavTransition";
import { formatDateOnlyForDisplay } from "@utils/dateTime";

/** The rich-text editor emits `<p></p>` when empty; check the stripped text. */
function isEmptyHtml(html: string): boolean {
  return html.replace(/<[^>]*>/g, "").replace(/&nbsp;/g, " ").trim().length === 0;
}

const BACK_TARGET = "/announcements";

const DRY_RUN_TAG_LABELS = [DRY_RUN_TAG_LABEL];

/**
 * The "Product version / EOL announcement" flow (Option 2 of the two-option
 * announcement create page — see AnnouncementKindSelector). Sends to exactly
 * the customers running a chosen product version: no other audience choice
 * exists here, unlike Option 1's scope picker.
 *
 * The resolved audience always excludes Restricted/Suspended projects and
 * Cloud Support/Cloud Evaluation Support subscriptions — this is applied
 * unconditionally by the backend (see useResolveProductVersionAudience's own
 * doc comment), not a filter this form offers a toggle for, mirroring the
 * real ServiceNow flow this replaces ("DRY RUN - Create [EOL] Product
 * Announcements"), whose own first step excludes them the same way with no
 * way to opt out.
 *
 * Unlike Option 1, there is no security-announcement label here — an
 * EOL/product-version notice is a general announcement category, not a
 * security one.
 *
 * "Dry run" reuses the exact same mechanism and Card as Option 1 (see
 * useAnnouncementDryRun/AnnouncementDryRunCard) — same fixed test project,
 * same "Dry Run" tag, same prominence ahead of Subject/Description not
 * being required for it to run: only the product/version choice, since the
 * dry-run case's content is independent of which real projects a send would
 * reach.
 */
export default function CreateEolAnnouncementForm(): JSX.Element {
  const navigate = useNavTransition();
  const { showError } = useErrorBanner();

  const [productId, setProductId] = useState("");
  const [productVersionId, setProductVersionId] = useState("");
  const [subject, setSubject] = useState("");
  const [description, setDescription] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [sendProgress, setSendProgress] = useState<AnnouncementSendProgressState | null>(null);

  const postCase = usePostCsmCase();

  const { data: products, isLoading: productsLoading } = useSearchProducts();
  const { data: versions, isLoading: versionsLoading } = useSearchProductVersions(
    productId || undefined,
  );

  const handleProductChange = (nextProductId: string): void => {
    setProductId(nextProductId);
    setProductVersionId("");
  };

  const resolvedAudience = useResolveProductVersionAudience(
    productId || undefined,
    productVersionId || undefined,
  );

  // id → short key ("CUPPTSUB"), so a failed project can be shown by
  // something readable instead of its raw id — same idea as the
  // customer-announcement form's own projectLabel.
  const projectKeyById = useMemo(() => {
    const m = new Map<string, string>();
    resolvedAudience.projects.forEach((p) => {
      if (p.key) m.set(p.id, p.key);
    });
    return m;
  }, [resolvedAudience.projects]);
  const projectLabel = useCallback(
    (projectId: string) => projectKeyById.get(projectId) ?? projectId,
    [projectKeyById],
  );

  // The full resolved audience for the current product/version.
  const targetProjectIds = useMemo(
    () => resolvedAudience.projects.map((p) => p.id),
    [resolvedAudience.projects],
  );

  // When set, a prior submit left these (and only these) projects without
  // an announcement — the next submit targets just this narrower list
  // instead of the full audience again, so a retry after a partial failure
  // can't create a second, duplicate case for a project that already
  // succeeded (there's no idempotency key on the create call). Reset back
  // to "submit the full audience" the moment the resolved audience itself
  // changes (a different product/version) — same reasoning as the
  // customer-announcement form's own retryProjectIds.
  const [retryProjectIds, setRetryProjectIds] = useState<string[] | null>(null);
  // Adjusting state during render (React's own recommended pattern for
  // "reset state when a derived value changes") rather than an effect — see
  // the customer-announcement form's own retryBaselineKey for why this
  // compares by content (JSON.stringify of a *sorted* copy) rather than
  // array reference: resolvedAudience.projects falls back to a fresh `[]`
  // on every render while unresolved, so targetProjectIds is never
  // referentially stable, and a refetch of the same audience isn't
  // guaranteed to return the same project order either.
  const targetProjectIdsKey = JSON.stringify([...targetProjectIds].sort());
  const [retryBaselineKey, setRetryBaselineKey] = useState(targetProjectIdsKey);
  if (targetProjectIdsKey !== retryBaselineKey) {
    setRetryBaselineKey(targetProjectIdsKey);
    setRetryProjectIds(null);
    // The previous batch's own outcome no longer describes this audience —
    // leaving it up would show stale failed-project chips/counts as if they
    // applied to the newly selected product/version.
    setSendProgress(null);
  }
  const submitProjectIds = retryProjectIds ?? targetProjectIds;

  const { runningDryRun, dryRunResult, canRunDryRun, handleRunDryRun } = useAnnouncementDryRun({
    subject,
    description,
    tagLabels: DRY_RUN_TAG_LABELS,
    extraCanRun: !submitting,
  });

  const canSubmit = useMemo(
    () =>
      !!productId &&
      !!productVersionId &&
      !resolvedAudience.isLoading &&
      // TanStack Query can retain a previous successful fetch's data after a
      // later refetch fails, so `total` alone can't distinguish "resolved,
      // zero recipients" from "resolution just failed but is still showing
      // yesterday's count" — see the customer-announcement form's own
      // canSubmit for the identical reasoning.
      !resolvedAudience.isError &&
      submitProjectIds.length > 0 &&
      subject.trim().length > 0 &&
      !isEmptyHtml(description) &&
      !submitting,
    [
      productId,
      productVersionId,
      resolvedAudience.isLoading,
      resolvedAudience.isError,
      submitProjectIds,
      subject,
      description,
      submitting,
    ],
  );

  const handleSubmit = async (): Promise<void> => {
    if (!canSubmit) return;
    setSubmitting(true);

    const trimmedSubject = subject.trim();
    setSendProgress({
      total: submitProjectIds.length,
      completed: 0,
      succeeded: 0,
      failed: 0,
      failedProjectIds: [],
    });
    const results = await settleWithConcurrencyLimit(
      submitProjectIds,
      ANNOUNCEMENT_CASE_CREATE_CONCURRENCY_LIMIT,
      (projectId) =>
        postCase.mutateAsync({
          type: "announcement",
          projectId,
          subject: trimmedSubject,
          description,
        }),
      (result, projectId) => {
        setSendProgress((prev) =>
          prev && {
            ...prev,
            completed: prev.completed + 1,
            succeeded: prev.succeeded + (result.status === "fulfilled" ? 1 : 0),
            failed: prev.failed + (result.status === "rejected" ? 1 : 0),
            failedProjectIds:
              result.status === "rejected"
                ? [...prev.failedProjectIds, projectId]
                : prev.failedProjectIds,
          },
        );
      },
    );
    setSubmitting(false);

    const failedProjectIds = submitProjectIds.filter((_, i) => results[i].status === "rejected");

    if (failedProjectIds.length === 0) {
      setRetryProjectIds(null);
      navigate(BACK_TARGET);
      return;
    }

    // The next submit must target only these projects, not the full
    // audience again — see retryProjectIds' own doc comment above.
    setRetryProjectIds(failedProjectIds);

    const succeededCount = submitProjectIds.length - failedProjectIds.length;
    showError(
      succeededCount > 0
        ? `The announcement was created for ${succeededCount} of ${submitProjectIds.length} project${
            submitProjectIds.length === 1 ? "" : "s"
          }, but failed for project${failedProjectIds.length === 1 ? "" : "s"} ${failedProjectIds
            .map(projectLabel)
            .join(", ")} — click "Retry" to resend to just the failed project${
            failedProjectIds.length === 1 ? "" : "s"
          }.`
        : "Could not create the announcement. Please try again.",
    );
    // Stay on this page instead of navigating away — at least one customer
    // project never got the announcement, so the failed-project chips on
    // the progress card above need to stay visible for the sender to act
    // on, same reasoning as the customer-announcement form's own handleSubmit.
  };

  const selectedVersion = (versions ?? []).find((v) => v.id === productVersionId);
  const eolDateLabel = selectedVersion
    ? (formatDateOnlyForDisplay(selectedVersion.supportEolDate) ??
      (formatDateOnlyForDisplay(selectedVersion.earliestPossibleSupportEolDate)
        ? `Est. ${formatDateOnlyForDisplay(selectedVersion.earliestPossibleSupportEolDate)}`
        : null))
    : null;

  return (
    <Card variant="outlined" sx={{ p: 3 }}>
      <Grid container spacing={2.5}>
        <Grid size={{ xs: 12 }}>
          <Typography variant="subtitle2" sx={{ mb: 1 }}>
            Affected product version
          </Typography>
          <Box sx={{ display: "flex", gap: 1.5 }}>
            <FormControl size="small" fullWidth required disabled={submitting}>
              <InputLabel id="eol-product-label" shrink={productId !== ""} sx={{ top: "0px !important" }}>
                Product
              </InputLabel>
              <Select
                labelId="eol-product-label"
                label="Product"
                value={productId}
                onChange={(e) => handleProductChange(e.target.value as string)}
                notched={productId !== ""}
                startAdornment={
                  productsLoading ? <CircularProgress size={16} sx={{ mr: 1 }} /> : null
                }
              >
                {(products ?? []).map((p) => (
                  <MenuItem key={p.id} value={p.id}>
                    {p.name ?? p.id}
                  </MenuItem>
                ))}
              </Select>
            </FormControl>

            <FormControl size="small" fullWidth required disabled={!productId || submitting}>
              <InputLabel id="eol-version-label" shrink={productVersionId !== ""} sx={{ top: "0px !important" }}>
                Version
              </InputLabel>
              <Select
                labelId="eol-version-label"
                label="Version"
                value={productVersionId}
                onChange={(e) => setProductVersionId(e.target.value as string)}
                notched={productVersionId !== ""}
                startAdornment={
                  versionsLoading ? <CircularProgress size={16} sx={{ mr: 1 }} /> : null
                }
              >
                {(versions ?? []).map((v) => (
                  <MenuItem key={v.id} value={v.id}>
                    {v.version ?? v.id}
                  </MenuItem>
                ))}
              </Select>
              {!productId && <FormHelperText>Select a product first.</FormHelperText>}
              {productId !== "" && !versionsLoading && (versions ?? []).length === 0 && (
                <FormHelperText>No versions available for this product.</FormHelperText>
              )}
            </FormControl>
          </Box>
          <Typography variant="caption" color="text.secondary" sx={{ display: "block", mt: 1 }}>
            {eolDateLabel ? `End of support: ${eolDateLabel}. ` : ""}
            Always excludes Restricted or Suspended projects, and Cloud Support / Cloud Evaluation
            Support subscriptions — this can&apos;t be turned off.
          </Typography>
        </Grid>

        {productId !== "" && productVersionId !== "" && (
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
            helperText={subject.length >= 160 ? `${subject.length}/200` : undefined}
            disabled={submitting || !!retryProjectIds}
          />
        </Grid>
        {retryProjectIds && (
          <Grid size={{ xs: 12 }}>
            <Typography variant="caption" color="text.secondary">
              Subject and description are locked while retrying failed projects — this resend
              must match what the succeeded projects already got. Change the product/version
              above to start a new send instead.
            </Typography>
          </Grid>
        )}
        <Grid size={{ xs: 12 }}>
          <Typography
            id="eol-announcement-description-label"
            component="label"
            variant="caption"
            color="text.secondary"
            sx={{ display: "block", mb: 0.5 }}
          >
            Description
          </Typography>
          {/* Editor doesn't accept an `id`, so associate the label by wrapping
              the editor in a labelled group for assistive tech. */}
          <Box role="group" aria-labelledby="eol-announcement-description-label">
            <EditorWithSourceToggle
              value={description}
              onChange={setDescription}
              placeholder="Describe the announcement…"
              minHeight={180}
              maxHeight={420}
              toolbarVariant="full"
              disabled={submitting || !!retryProjectIds}
            />
          </Box>
        </Grid>
      </Grid>

      <AnnouncementDryRunCard
        runningDryRun={runningDryRun}
        dryRunResult={dryRunResult}
        canRunDryRun={canRunDryRun}
        onRunDryRun={() => void handleRunDryRun()}
      />

      {sendProgress && (
        <AnnouncementSendProgress progress={sendProgress} projectLabel={projectLabel} />
      )}

      <Box
        sx={{
          display: "flex",
          justifyContent: "flex-end",
          gap: 1.5,
          mt: 2.5,
          pt: 2,
          borderTop: 1,
          borderColor: "divider",
        }}
      >
        <Button variant="outlined" onClick={() => navigate(BACK_TARGET)} disabled={submitting}>
          Cancel
        </Button>
        <Button variant="contained" onClick={() => void handleSubmit()} disabled={!canSubmit}>
          {submitting
            ? "Creating…"
            : retryProjectIds
              ? `Retry ${retryProjectIds.length} failed project${retryProjectIds.length === 1 ? "" : "s"}`
              : "Create announcement"}
        </Button>
      </Box>
    </Card>
  );
}
