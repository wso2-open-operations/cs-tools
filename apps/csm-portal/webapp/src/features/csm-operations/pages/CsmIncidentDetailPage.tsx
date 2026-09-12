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

import { Box, Button, Card, Chip, Skeleton, Tab, Tabs, Typography } from "@wso2/oxygen-ui";
import {
  Activity,
  ArrowLeft,
  Eye,
  FileText,
  Link as LinkIcon,
  MessageSquarePlus,
  Paperclip,
  Pencil,
} from "@wso2/oxygen-ui-icons-react";
import {
  type JSX,
  type ReactNode,
  useCallback,
  useEffect,
  useMemo,
  useState,
} from "react";
import { useLocation } from "react-router";
import { formatBackendTimestampForDisplay } from "@utils/dateTime";
import { BackendApiError } from "@api/backend/client";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import { useCurrentUser } from "@context/current-user/CurrentUserContext";
import { useEngineerDisplayName } from "@hooks/useEngineerDisplayName";
import { useIdTokenClaims } from "@hooks/useIdTokenClaims";
import { useRecordRecentView } from "@features/csm-recent/hooks/useRecentViews";
import { useGetIncident } from "@features/csm-operations/api/useGetIncident";
import { usePatchIncident } from "@features/csm-operations/api/usePatchIncident";
import {
  useGetCsmIncidentComments,
  usePostCsmIncidentComment,
} from "@features/csm-operations/api/useCsmIncidentComments";
import { useGetCsmIncidentActivities } from "@features/csm-operations/api/useCsmIncidentActivities";
import EditIncidentDialog from "@features/csm-operations/components/EditIncidentDialog";
import EntityRefLink from "@features/csm-operations/components/EntityRefLink";
import IncidentActionBar from "@features/csm-operations/components/IncidentActionBar";
import IncidentResolutionDialog from "@features/csm-operations/components/IncidentResolutionDialog";
import {
  incidentCommentGateReason,
  incidentPriorityColor,
  incidentPriorityLabel,
  incidentStateColor,
  incidentStateLabel,
} from "@features/csm-operations/utils/incidents";
import CaseActivitiesFeed from "@features/csm-cases/components/CaseActivitiesFeed";
import CsmCaseCommentInput from "@features/csm-cases/components/CsmCaseCommentInput";
import {
  AttachmentsWidget,
  WatchersWidget,
} from "@features/csm-cases/components/CaseDetailWidgets";
import {
  useGetCsmCaseAttachments,
  usePostCsmCaseAttachment,
  useDownloadCsmCaseAttachment,
  useGetCsmCaseAttachmentPreviewSource,
} from "@features/csm-cases/api/useCsmCaseAttachments";
import type { CaseAttachment } from "@features/csm-cases/types/csmCases";
import type {
  BeEntityRef,
  BeIncidentDetail,
  BeIncidentState,
  BeUpdateIncidentPayload,
} from "@api/backend/types";
import { useNavTransition } from "@hooks/useNavTransition";
import { useNormalizedIdParam } from "@hooks/useNormalizedIdParam";
import { useQueryParamTabs } from "@hooks/useSectionTabs";
import { useCaseRouteOverride } from "@context/case-tabs/CaseRouteOverrideContext";
import { useReportCaseTabMeta } from "@features/case-tabs/hooks/useReportCaseTabMeta";
import { useReportCaseTabDraft } from "@features/case-tabs/hooks/useReportCaseTabDraft";

const OPERATIONS_INCIDENTS_PATH = "/operations/incidents";

/**
 * A single confirmed-live upstream limitation of `PATCH /incidents/{id}`
 * (entity-service/ServiceNow, not this BFF or the FE): `state: RESOLVED`/
 * `CLOSED` 500s without a resolution, fixed by having
 * `EditIncidentDialog`/`IncidentResolutionDialog` collect
 * `resolutionCode`/`resolutionNotes` (write-only fields, no read-side model
 * — see `BeUpdateIncidentPayload`) once the target state is one of those two.
 * `additionalComments` (and, defensively, `workNotes` — same ServiceNow
 * journal-field shape, not independently confirmed) is the dangerous one:
 * the PATCH returns 200, but the response's own echoed value comes back
 * `null` even though we just set it — a silent no-op dressed as success.
 * `checkSilentlyDroppedNotes` exists so this doesn't slip through: it
 * catches a 200 that didn't actually persist what it claims to and treats
 * it like the failure it is, rather than closing the dialog on a false
 * positive. The Edit dialog no longer has UI to set either field (that's
 * the Activities tab's job now), so this only matters if a future patch
 * path resends them.
 */
function checkSilentlyDroppedNotes(patch: BeUpdateIncidentPayload, saved: BeIncidentDetail): string[] {
  const dropped: string[] = [];
  if ("additionalComments" in patch && (saved.additionalComments ?? null) !== patch.additionalComments) {
    dropped.push("Additional comments");
  }
  if ("workNotes" in patch && (saved.workNotes ?? null) !== patch.workNotes) {
    dropped.push("Internal work note");
  }
  return dropped;
}

function formatDateTime(value?: string | null): string {
  return (
    formatBackendTimestampForDisplay(value, {
      dateStyle: "medium",
      timeStyle: "short",
    }) ?? "—"
  );
}

function MetaCell({ label, children }: { label: string; children: ReactNode }): JSX.Element {
  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 0.25, minWidth: 0 }}>
      <Typography
        variant="caption"
        color="text.secondary"
        sx={{ textTransform: "uppercase", letterSpacing: 0.4 }}
      >
        {label}
      </Typography>
      <Box sx={{ minWidth: 0 }}>{children}</Box>
    </Box>
  );
}

function RefText({ value }: { value?: BeEntityRef | null }): JSX.Element {
  return <Typography variant="body2">{value?.name || "—"}</Typography>;
}

type IncidentTabId = "activities" | "details" | "related" | "watchers" | "attachments";

const TAB_DEFS: Array<{ id: IncidentTabId; label: string; icon: JSX.Element }> = [
  { id: "activities", label: "Activities", icon: <Activity size={16} /> },
  { id: "details", label: "Details", icon: <FileText size={16} /> },
  { id: "related", label: "Related", icon: <LinkIcon size={16} /> },
  { id: "watchers", label: "Watchers", icon: <Eye size={16} /> },
  { id: "attachments", label: "Attachments", icon: <Paperclip size={16} /> },
];
const INCIDENT_TAB_IDS: readonly IncidentTabId[] = TAB_DEFS.map((t) => t.id);

/**
 * Detail for a single incident (`GET /incidents/{id}`), tabbed to match
 * `CsmCaseDetailPage`'s structural pattern — Activities / Details / Related /
 * Watchers / Attachments. Incidents have no call-requests, time-tracking, or
 * tasks concept in this platform, so those case tabs are intentionally not
 * carried over. State transitions (`PATCH /incidents/{id} { state }`) via
 * `IncidentActionBar`; `RESOLVED`/`CLOSED` route through
 * `IncidentResolutionDialog` first, since ServiceNow requires a resolution
 * code/notes for those two (see `checkSilentlyDroppedNotes`'s doc comment
 * for the related, already-handled `additionalComments`/`workNotes` quirk).
 */
export default function CsmIncidentDetailPage(): JSX.Element {
  // Real router hooks — called unconditionally regardless of `routeOverride`
  // below (rules of hooks), but their VALUES are only actually used when
  // this instance isn't part of an open in-app tab. See the identical
  // pattern (and its own longer doc comment) at the top of
  // `CsmCaseDetailPage`, which this mirrors: this page can be mounted
  // several times at once (one per open tab, kept alive in the background —
  // see `CaseTabIsolatedRouter`), while there is only ever one real matched
  // route/location for the app as a whole.
  const routedId = useNormalizedIdParam("id");
  const routedNavigate = useNavTransition();
  const routedLocationState = useLocation().state;
  const routeOverride = useCaseRouteOverride();
  const id = routeOverride?.caseId ?? routedId;
  const navigate = routeOverride?.navigate ?? routedNavigate;
  // Prefer the list URL the row link captured (if any) so "back" returns to
  // the exact view the engineer came from, falling back to the bare tab path
  // for a bookmarked or directly-linked incident.
  const backState = (routeOverride ? routeOverride.state : routedLocationState) as
    | { from?: string }
    | undefined;
  const backTarget = backState?.from ?? OPERATIONS_INCIDENTS_PATH;
  const { data, isLoading, isError } = useGetIncident(id);
  // The incident number as the short chip label (matching `CsmCaseDetailPage`'s
  // own `caseNumber`-only report); incidents have no separate project-scoped
  // id the way cases do, so the tooltip's `internalId` reuses the same
  // number, with the subject alongside it.
  useReportCaseTabMeta(id, {
    label: data?.number ?? undefined,
    internalId: data?.number ?? undefined,
    subject: data?.subject ?? undefined,
  });
  const { showError } = useErrorBanner();
  const patchIncident = usePatchIncident();
  const [editOpen, setEditOpen] = useState(false);
  // Kept in the URL (`?tab=`), not local state, so a shared/bookmarked link
  // to a specific tab (e.g. Watchers) survives a refresh.
  const { activeTab, setActiveTab } = useQueryParamTabs<IncidentTabId>(
    INCIDENT_TAB_IDS,
    "activities",
  );
  const [resolutionTarget, setResolutionTarget] = useState<
    Extract<BeIncidentState, "RESOLVED" | "CLOSED"> | null
  >(null);
  const engineerName = useEngineerDisplayName();
  // Signed-in engineer's platform UUID (self-subscribe writes) and email
  // (matching a watch-list entry to "me"), same pair the case detail page
  // uses for its own watch list.
  const { user: currentUser } = useCurrentUser();
  const currentUserEmail = useIdTokenClaims()?.email;

  const { data: comments } = useGetCsmIncidentComments(id);
  const { data: activityAudit } = useGetCsmIncidentActivities(id);
  const postComment = usePostCsmIncidentComment();
  const { data: attachments } = useGetCsmCaseAttachments(id, "incident");
  const postAttachment = usePostCsmCaseAttachment();
  const downloadAttachment = useDownloadCsmCaseAttachment();
  const getAttachmentPreviewContent = useGetCsmCaseAttachmentPreviewSource();
  const [composerOpen, setComposerOpen] = useState(false);
  // Reports composerOpen up to the in-app case-tabs layer, purely so closing
  // this incident's tab from the tab strip can confirm first — see
  // CsmCaseDetailPage's identical call, and the hook's own doc comment for
  // what this signal does and doesn't guarantee. Missing here was itself a
  // bug: this tab's `hasDraft` never became true, so its close-confirm
  // never fired for an unsent reply.
  useReportCaseTabDraft(id, composerOpen);
  // Shared between the Activities feed and the Attachments tab, same as
  // CsmCaseDetailPage — one attachment previewed at a time regardless of
  // which surface opened it.
  const [previewTarget, setPreviewTarget] = useState<CaseAttachment | null>(null);

  const attachmentList = useMemo(() => attachments ?? [], [attachments]);
  // The read model's entries already carry the platform user UUID the write
  // side is keyed by, so the widget can compute a replacement list straight
  // from what is on screen.
  const watchList = useMemo(() => {
    const myEmail = currentUserEmail?.toLowerCase();
    return (data?.watchList ?? []).map((w) => {
      const email = w.email || undefined;
      return {
        id: w.id,
        name: w.name || w.email,
        email,
        isMe: !!email && !!myEmail && email.toLowerCase() === myEmail,
      };
    });
  }, [data?.watchList, currentUserEmail]);

  const recordView = useRecordRecentView();
  useEffect(() => {
    if (!data?.id) return;
    recordView({
      kind: "incident",
      id: data.id,
      title:
        [data.number, data.subject].filter((s): s is string => !!s?.trim()).join(" · ") ||
        "(no subject)",
      subtitle: data.assignedTo?.name,
      href: `/operations/incidents/${data.id}`,
    });
  }, [data, recordView]);

  const onUploadAttachment = useCallback(
    (file: File) => {
      if (!id) return;
      postAttachment.mutate({
        caseId: id,
        file,
        uploadedBy: engineerName,
        referenceType: "incident",
      });
    },
    [id, engineerName, postAttachment],
  );

  const onDownloadAttachment = useCallback(
    (attachment: (typeof attachmentList)[number]) => {
      void downloadAttachment(attachment).catch((err) =>
        showError(`Could not download ${attachment.filename}.`, err),
      );
    },
    [downloadAttachment, showError],
  );

  /**
   * Persist a new watch list. There is no add-one/remove-one endpoint:
   * `PATCH /incidents/{id}` takes the whole `watchList` as user UUIDs and
   * replaces what is stored, so `WatchersWidget` computes the full
   * replacement list (for a removal as much as an addition) and this only
   * forwards it. An explicitly empty list is meaningful here — it clears the
   * watch list — which is why the widget allows removing an incident's last
   * watcher.
   */
  const onReplaceWatchers = useCallback(
    (nextWatcherIds: string[]) => {
      if (!id) return;
      patchIncident.mutate(
        { id, patch: { watchList: nextWatcherIds } },
        {
          onError: (err) => {
            // The watch-list 400s name the offending value (an unknown or
            // malformed user id), which is worth surfacing verbatim.
            const msg =
              err instanceof BackendApiError && err.status < 500 && err.message
                ? err.message
                : "Could not update the watch list. Please try again.";
            showError(msg, err);
          },
        },
      );
    },
    [id, patchIncident, showError],
  );

  /**
   * Dispatch a state transition from `IncidentActionBar`. `RESOLVED`/`CLOSED`
   * need the resolution dialog first (ServiceNow requires those fields — see
   * the file-level doc comment); every other target PATCHes directly, same
   * split of responsibility as `CaseActionBar` + `CsmCaseDetailPage.onAction`.
   */
  const onIncidentAction = useCallback(
    (target: BeIncidentState) => {
      if (!id) return;
      if (target === "RESOLVED" || target === "CLOSED") {
        setResolutionTarget(target);
        return;
      }
      patchIncident.mutate(
        { id, patch: { state: target } },
        {
          onError: (err) => {
            const msg =
              err instanceof BackendApiError && err.status < 500 && err.message
                ? err.message
                : "Could not update the incident's state. Please try again.";
            showError(msg, err);
          },
        },
      );
    },
    [id, patchIncident, showError],
  );

  const onResolutionSubmit = useCallback(
    (fields: { resolutionCode: string; resolutionNotes: string }) => {
      if (!id || !resolutionTarget) return;
      patchIncident.mutate(
        { id, patch: { state: resolutionTarget, ...fields } },
        {
          onSuccess: () => setResolutionTarget(null),
          onError: (err) => {
            const msg =
              err instanceof BackendApiError && err.status < 500 && err.message
                ? err.message
                : "Could not update the incident's state. Please try again.";
            showError(msg, err);
          },
        },
      );
    },
    [id, patchIncident, resolutionTarget, showError],
  );

  const back = (): void => {
    navigate(backTarget);
  };

  const BackButton = (
    <Button
      variant="text"
      size="small"
      startIcon={<ArrowLeft size={16} />}
      onClick={back}
      sx={{ alignSelf: "flex-start" }}
    >
      Back
    </Button>
  );

  if (isLoading) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
        <Skeleton variant="rounded" height={32} width={240} />
        <Skeleton variant="rounded" height={260} />
      </Box>
    );
  }

  if (isError) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        {BackButton}
        <Typography variant="body1" color="error">
          Could not load incident {id}.
        </Typography>
      </Box>
    );
  }

  if (!data) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        {BackButton}
        <Typography variant="h5">Incident not found</Typography>
        <Typography variant="body2" color="text.secondary">
          No incident with id <code>{id}</code>.
        </Typography>
      </Box>
    );
  }

  const incident = data;
  const hasLinks = !!(incident.parent || incident.changeRequest || incident.problem || incident.causedBy);
  const hasLinkedServiceRequests =
    !!incident.linkedServiceRequests && incident.linkedServiceRequests.length > 0;

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2.5 }}>
      {BackButton}

      <Box
        sx={{
          display: "flex",
          gap: 2,
          alignItems: "flex-start",
          flexWrap: { xs: "wrap", md: "nowrap" },
          justifyContent: "space-between",
        }}
      >
        <Box
          sx={{
            display: "flex",
            flexDirection: "column",
            gap: 1,
            flex: 1,
            minWidth: 0,
          }}
        >
          <Typography
            variant="h6"
            sx={{
              fontFamily: "monospace",
              fontWeight: 700,
              letterSpacing: 0.2,
              lineHeight: 1.2,
            }}
          >
            {incident.number || incident.id}
          </Typography>
          <Box sx={{ display: "flex", alignItems: "center", gap: 1, flexWrap: "wrap" }}>
            {incident.state && (
              <Chip
                size="small"
                color={incidentStateColor(incident.state)}
                label={incidentStateLabel(incident.state)}
              />
            )}
            {incident.priority && (
              <Chip
                size="small"
                variant="outlined"
                color={incidentPriorityColor(incident.priority)}
                label={incidentPriorityLabel(incident.priority)}
              />
            )}
          </Box>
          <Typography variant="h5">{incident.subject || "Incident"}</Typography>
        </Box>
        <Box sx={{ flexShrink: 0, alignSelf: { xs: "stretch", md: "flex-start" } }}>
          <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
            <IncidentActionBar
              incident={incident}
              isPending={patchIncident.isPending}
              onAction={onIncidentAction}
            />
            <Button
              variant="outlined"
              size="small"
              startIcon={<Pencil size={14} />}
              onClick={() => setEditOpen(true)}
            >
              Edit
            </Button>
          </Box>
        </Box>
      </Box>

      <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2 }}>
        <Typography variant="subtitle2">Overview</Typography>
        <Box
          sx={{
            display: "grid",
            gap: 2,
            gridTemplateColumns: {
              xs: "1fr",
              sm: "repeat(2, minmax(0, 1fr))",
              md: "repeat(3, minmax(0, 1fr))",
            },
          }}
        >
          {(
            [
              { label: "Caller", render: () => <RefText value={incident.caller} /> },
              { label: "Assignment group", render: () => <RefText value={incident.assignmentGroup} /> },
              { label: "Assigned to", render: () => <RefText value={incident.assignedTo} /> },
              { label: "Opened", render: () => <Typography variant="body2">{formatDateTime(incident.openedOn)}</Typography> },
              { label: "Created by", render: () => <Typography variant="body2">{incident.createdBy || "—"}</Typography> },
              { label: "Last updated", render: () => <Typography variant="body2">{formatDateTime(incident.updatedOn)}</Typography> },
            ] satisfies Array<{ label: string; render: () => JSX.Element }>
          ).map((field) => (
            <MetaCell key={field.label} label={field.label}>
              {field.render()}
            </MetaCell>
          ))}
        </Box>
      </Card>

      <Box sx={{ borderBottom: 1, borderColor: "divider" }}>
        <Tabs
          value={activeTab}
          onChange={(_, v) => setActiveTab(v as IncidentTabId)}
          variant="scrollable"
          scrollButtons="auto"
        >
          {TAB_DEFS.map((t) => {
            const count =
              t.id === "watchers"
                ? watchList.length
                : t.id === "attachments"
                  ? attachmentList.length
                  : undefined;
            return (
              <Tab
                key={t.id}
                value={t.id}
                icon={t.icon}
                iconPosition="start"
                label={count ? `${t.label} (${count})` : t.label}
                sx={{ minHeight: 44, textTransform: "none" }}
              />
            );
          })}
        </Tabs>
      </Box>

      {activeTab === "activities" && (
        <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2 }}>
          {composerOpen ? (
            <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
              <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
                <Typography variant="subtitle2">Reply</Typography>
                <Button
                  size="small"
                  variant="text"
                  color="inherit"
                  onClick={() => setComposerOpen(false)}
                >
                  Cancel
                </Button>
              </Box>
              <CsmCaseCommentInput
                disabled={!id}
                publicCommentDisabledReason={incidentCommentGateReason(incident.state)}
                autoFocus
                onSubmit={async (bodyHtml, internal, commentAttachments) => {
                  if (!id) return;
                  const hasText =
                    bodyHtml.replace(/<[^>]*>/g, "").replace(/&nbsp;/g, " ").trim().length > 0;
                  if (hasText) {
                    await postComment.mutateAsync({
                      incidentId: id,
                      bodyHtml,
                      internal,
                    });
                  }
                  for (const { file, name } of commentAttachments) {
                    await postAttachment.mutateAsync({
                      caseId: id,
                      file,
                      name,
                      uploadedBy: engineerName,
                      referenceType: "incident",
                    });
                  }
                  setComposerOpen(false);
                }}
              />
            </Box>
          ) : (
            <Button
              fullWidth
              variant="outlined"
              color="inherit"
              startIcon={<MessageSquarePlus size={18} />}
              onClick={() => setComposerOpen(true)}
              sx={{ justifyContent: "flex-start", textTransform: "none", py: 1.5, px: 2 }}
            >
              Add a comment…
            </Button>
          )}
          <CaseActivitiesFeed
            comments={comments ?? []}
            audit={activityAudit ?? []}
            attachments={attachmentList}
            onDownloadAttachment={onDownloadAttachment}
            preview={{
              onGetPreviewContent: getAttachmentPreviewContent,
              previewTarget,
              onPreviewTargetChange: setPreviewTarget,
            }}
          />
        </Card>
      )}

      {activeTab === "details" && (
        <Box
          sx={{
            display: "grid",
            gap: 2,
            gridTemplateColumns: {
              xs: "1fr",
              md: "repeat(2, minmax(0, 1fr))",
            },
            alignItems: "start",
          }}
        >
          <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 1.5 }}>
            <Typography variant="subtitle2">Classification</Typography>
            <Box sx={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 2 }}>
              <MetaCell label="Category">
                <Typography variant="body2">{incident.category || "—"}</Typography>
              </MetaCell>
              <MetaCell label="Subcategory">
                <Typography variant="body2">{incident.subcategory || "—"}</Typography>
              </MetaCell>
              <MetaCell label="Contact type">
                <Typography variant="body2">{incident.contactType || "—"}</Typography>
              </MetaCell>
              <MetaCell label="Impact">
                <Typography variant="body2">{incident.impact || "—"}</Typography>
              </MetaCell>
              <MetaCell label="Urgency">
                <Typography variant="body2">{incident.urgency || "—"}</Typography>
              </MetaCell>
              <MetaCell label="Created">
                <Typography variant="body2">{formatDateTime(incident.createdOn)}</Typography>
              </MetaCell>
            </Box>
          </Card>

          <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 1.5 }}>
            <Typography variant="subtitle2">Service &amp; configuration</Typography>
            <Box sx={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 2 }}>
              <MetaCell label="Service"><RefText value={incident.service} /></MetaCell>
              <MetaCell label="Service offering"><RefText value={incident.serviceOffering} /></MetaCell>
              <MetaCell label="Configuration item"><RefText value={incident.configurationItem} /></MetaCell>
            </Box>
          </Card>
        </Box>
      )}

      {activeTab === "related" && (
        <Box
          sx={{
            display: "grid",
            gap: 2,
            gridTemplateColumns: {
              xs: "1fr",
              md: "repeat(2, minmax(0, 1fr))",
            },
            alignItems: "start",
          }}
        >
          {hasLinks ? (
            <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2 }}>
              <Typography variant="subtitle2">Linked records</Typography>
              <Box
                sx={{
                  display: "grid",
                  gap: 2,
                  gridTemplateColumns: "repeat(2, minmax(0, 1fr))",
                }}
              >
                <MetaCell label="Parent incident">
                  <EntityRefLink value={incident.parent} routeBase="/operations/incidents" />
                </MetaCell>
                <MetaCell label="Change request">
                  <EntityRefLink value={incident.changeRequest} routeBase="/operations/change-requests" />
                </MetaCell>
                <MetaCell label="Problem">
                  <EntityRefLink value={incident.problem} routeBase="/operations/problems" />
                </MetaCell>
                {/* "Caused by" has no confirmed target record type (could be a
                    change request, a problem, or something else) — same caveat
                    as Problem.originCase — so it's left as plain text rather
                    than guessing a route. */}
                <MetaCell label="Caused by"><RefText value={incident.causedBy} /></MetaCell>
              </Box>
            </Card>
          ) : (
            <Typography variant="body2" color="text.secondary">
              No linked records for this incident.
            </Typography>
          )}

          {hasLinkedServiceRequests && (
            <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 1.5 }}>
              <Typography variant="subtitle2">Linked service requests</Typography>
              <Box sx={{ display: "flex", flexWrap: "wrap", gap: 1 }}>
                {incident.linkedServiceRequests?.map((sr) => (
                  <Chip
                    key={sr.id}
                    size="small"
                    variant="outlined"
                    clickable
                    label={`${sr.number} — ${sr.name}`}
                    onClick={() => navigate(`/cases/${encodeURIComponent(sr.id)}`)}
                    sx={{ fontWeight: 600 }}
                  />
                ))}
              </Box>
            </Card>
          )}
        </Box>
      )}

      {activeTab === "watchers" && (
        <WatchersWidget
          entityKind="incident"
          watchers={watchList}
          onReplace={onReplaceWatchers}
          isSaving={patchIncident.isPending}
          currentUserId={currentUser?.id}
          // No `autoWatchingReason`: the incident read model's `assignedTo`
          // carries only id/name (no email), so — unlike the case page —
          // there's no reliable signal here for "watching only because
          // auto-assigned" without a new fetch. Unfollow stays available.
        />
      )}

      {activeTab === "attachments" && (
        <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2 }}>
          <AttachmentsWidget
            attachments={attachmentList}
            uploading={postAttachment.isPending}
            uploadError={
              postAttachment.isError
                ? (postAttachment.error?.message ?? "Could not upload the attachment.")
                : null
            }
            onUpload={onUploadAttachment}
            onDownload={onDownloadAttachment}
            preview={{
              onGetPreviewContent: getAttachmentPreviewContent,
              previewTarget,
              onPreviewTargetChange: setPreviewTarget,
            }}
          />
        </Card>
      )}

      {editOpen && (
        <EditIncidentDialog
          incident={incident}
          isSaving={patchIncident.isPending}
          onClose={() => {
            if (!patchIncident.isPending) setEditOpen(false);
          }}
          onSave={(patch) =>
            patchIncident.mutate(
              { id: incident.id as string, patch },
              {
                onSuccess: (data) => {
                  const droppedFields = checkSilentlyDroppedNotes(patch, data.incident);
                  if (droppedFields.length > 0) {
                    showError(
                      `${droppedFields.join(" and ")} didn't save — the request was accepted but that ` +
                        "change wasn't actually persisted. Any other fields in this edit were saved; please retry the note separately.",
                    );
                    return;
                  }
                  setEditOpen(false);
                },
                onError: (err) => {
                  // Real validation messages (e.g. an invalid UUID in one of the
                  // linking fields) are worth surfacing, same as CreateIncidentPage.
                  const msg =
                    err instanceof BackendApiError && err.status < 500 && err.message
                      ? err.message
                      : "Could not update the incident. Please try again.";
                  showError(msg, err);
                },
              },
            )
          }
        />
      )}

      {resolutionTarget && (
        <IncidentResolutionDialog
          target={resolutionTarget}
          isSubmitting={patchIncident.isPending}
          onClose={() => {
            if (!patchIncident.isPending) setResolutionTarget(null);
          }}
          onSubmit={onResolutionSubmit}
        />
      )}
    </Box>
  );
}
