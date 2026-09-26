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
import { useMemo, useState, type JSX } from "react";
import EditorWithSourceToggle from "@components/rich-text-editor/EditorWithSourceToggle";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import { useAnnouncementDryRun, DRY_RUN_TAG_LABEL } from "@features/csm-announcements/api/useAnnouncementDryRun";
import { useResolveProductVersionAudience } from "@features/csm-announcements/api/useResolveProductVersionAudience";
import { useCreateAnnouncementRequest } from "@features/csm-announcements/api/useCreateAnnouncementRequest";
import { useUpdateAnnouncementRequest } from "@features/csm-announcements/api/useUpdateAnnouncementRequest";
import { useRecordAnnouncementRequestDryRun } from "@features/csm-announcements/api/useRecordAnnouncementRequestDryRun";
import { useSubmitAnnouncementRequest } from "@features/csm-announcements/api/useSubmitAnnouncementRequest";
import ResolvedAudienceList from "@features/csm-announcements/components/ResolvedAudienceList";
import type { EolAudienceDefinition } from "@features/csm-announcements/types/announcementRequests";
import { useSearchProducts } from "@features/csm-projects/api/useSearchProducts";
import { useSearchProductVersions } from "@features/csm-projects/api/useSearchProductVersions";
import { useNavTransition } from "@hooks/useNavTransition";
import { formatDateOnlyForDisplay } from "@utils/dateTime";

/** The rich-text editor emits `<p></p>` when empty; check the stripped text. */
function isEmptyHtml(html: string): boolean {
  return html.replace(/<[^>]*>/g, "").replace(/&nbsp;/g, " ").trim().length === 0;
}

const PENDING_TARGET = "/announcements?tab=pending";

const DRY_RUN_TAG_LABELS = [DRY_RUN_TAG_LABEL];

/**
 * The "Product version / EOL announcement" flow (Option 2 of the two-option
 * announcement create page — see AnnouncementKindSelector). Builds a
 * `kind: "eol"` announcement request (Phase 2's draft/approval workflow)
 * rather than sending immediately — see CreateCustomerAnnouncementForm's own
 * doc comment for the shared draft/dry-run/submit design this mirrors; this
 * form only differs in its audience shape (`{productId, productVersionId}`,
 * no scope choice) and in having no security-announcement concept at all.
 *
 * The resolved audience always excludes Restricted/Suspended projects and
 * Cloud Support/Cloud Evaluation Support subscriptions — this is applied
 * unconditionally by the backend (see useResolveProductVersionAudience's own
 * doc comment), not a filter this form offers a toggle for.
 */
export default function CreateEolAnnouncementForm(): JSX.Element {
  const navigate = useNavTransition();
  const { showError } = useErrorBanner();

  const [productId, setProductId] = useState("");
  const [productVersionId, setProductVersionId] = useState("");
  const [subject, setSubject] = useState("");
  const [description, setDescription] = useState("");
  const [savingDraft, setSavingDraft] = useState(false);
  const [submittingForApproval, setSubmittingForApproval] = useState(false);
  const [draftId, setDraftId] = useState<string | null>(null);

  const createDraft = useCreateAnnouncementRequest();
  const updateDraft = useUpdateAnnouncementRequest();
  const recordDryRun = useRecordAnnouncementRequestDryRun();
  const submitRequest = useSubmitAnnouncementRequest();

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

  const busy = savingDraft || submittingForApproval;
  const { runningDryRun, canRunDryRun, handleRunDryRun } = useAnnouncementDryRun({
    subject,
    description,
    tagLabels: DRY_RUN_TAG_LABELS,
    extraCanRun: !busy,
  });

  const audienceDefinition: EolAudienceDefinition = useMemo(
    () => ({ productId, productVersionId }),
    [productId, productVersionId],
  );

  const canSaveDraft =
    !!productId && !!productVersionId && subject.trim().length > 0 && !isEmptyHtml(description) && !busy;

  const canSubmitForApproval =
    canRunDryRun &&
    !!productId &&
    !!productVersionId &&
    !resolvedAudience.isLoading &&
    // See CreateCustomerAnnouncementForm's identical check: a stale
    // successful fetch can leave `total` looking populated after a later
    // refetch fails.
    !resolvedAudience.isError &&
    resolvedAudience.total > 0 &&
    !busy;

  const handleSaveDraft = async (): Promise<void> => {
    if (!canSaveDraft) return;
    setSavingDraft(true);
    try {
      if (draftId) {
        await updateDraft.mutateAsync({ id: draftId, subject: subject.trim(), description, audienceDefinition });
      } else {
        const created = await createDraft.mutateAsync({
          kind: "eol",
          subject: subject.trim(),
          description,
          isSecurityAnnouncement: false,
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

  // "Submit for approval" runs the dry run, then creates/updates the draft,
  // records the dry run onto it, and submits — one action. See
  // CreateCustomerAnnouncementForm's identical handler for the full
  // reasoning (the dry-run case is what actually gets shared for approval,
  // so there's no separate preview step to gain from).
  const handleSubmitForApproval = async (): Promise<void> => {
    if (!canSubmitForApproval) return;
    setSubmittingForApproval(true);
    try {
      const result = await handleRunDryRun();
      if (!result) return; // useAnnouncementDryRun already surfaced the error

      let id = draftId;
      if (id) {
        await updateDraft.mutateAsync({ id, subject: subject.trim(), description, audienceDefinition });
      } else {
        const created = await createDraft.mutateAsync({
          kind: "eol",
          subject: subject.trim(),
          description,
          isSecurityAnnouncement: false,
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
            <FormControl size="small" fullWidth required disabled={busy}>
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

            <FormControl size="small" fullWidth required disabled={!productId || busy}>
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
            disabled={busy}
          />
        </Grid>
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
