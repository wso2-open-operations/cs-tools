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
  Chip,
  CircularProgress,
  IconButton,
  LinearProgress,
  Tooltip,
  Typography,
} from "@wso2/oxygen-ui";
import {
  Activity,
  ArrowUpRight,
  Building,
  CheckCircle,
  Clock,
  Download,
  History,
  Link as LinkIcon,
  MapPin,
  Paperclip,
  Plus,
  Server,
  Shield,
  Trash2,
  TriangleAlert,
  Upload,
  User,
  Users,
} from "@wso2/oxygen-ui-icons-react";
import { useRef, type ChangeEvent, type JSX } from "react";
import { formatBytes } from "@utils/formatBytes";
import type {
  CaseAttachment,
  CaseAuditEntry,
  CaseCustomerContext,
  CaseProductContext,
  CaseTag,
  CaseTimeLogEntry,
} from "@features/csm-cases/types/csmCases";
import { tierColor, tierLabel } from "@features/csm-cases/utils/caseTier";
import {
  deploymentTypeLabel,
  formatDeploymentDate,
} from "@features/csm-projects/utils/deployments";
import type { ProjectDetails } from "@features/csm-projects/types/csmProjects";
import type { BeDeployment } from "@api/backend/types";
import RelativeTime from "@components/RelativeTime";

// ---------------------------------------------------------------------------
// Shared widget shell
// ---------------------------------------------------------------------------

interface WidgetCardProps {
  title: string;
  icon?: JSX.Element;
  action?: JSX.Element;
  children: React.ReactNode;
  /** Greys out the whole card and explains why via a tooltip on the title —
   * for a widget whose backing feature isn't wired up yet. */
  disabledReason?: string;
}

function WidgetCard({
  title,
  icon,
  action,
  children,
  disabledReason,
}: WidgetCardProps): JSX.Element {
  return (
    <Card
      variant="outlined"
      sx={{ p: 2, opacity: disabledReason ? 0.6 : 1 }}
    >
      <Box
        sx={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          gap: 1,
          mb: 1.25,
        }}
      >
        <Tooltip title={disabledReason ?? ""}>
          <Box sx={{ display: "flex", alignItems: "center", gap: 0.75 }}>
            {icon}
            <Typography variant="subtitle2">{title}</Typography>
          </Box>
        </Tooltip>
        {action}
      </Box>
      <Box sx={disabledReason ? { pointerEvents: "none" } : undefined}>
        {children}
      </Box>
    </Card>
  );
}

function MetaRow({
  label,
  children,
}: {
  label: string;
  children: React.ReactNode;
}): JSX.Element {
  return (
    <Box
      sx={{
        display: "flex",
        gap: 1.5,
        py: 0.5,
        borderTop: 1,
        borderColor: "divider",
        "&:first-of-type": { borderTop: 0, pt: 0 },
      }}
    >
      <Typography
        variant="caption"
        color="text.secondary"
        sx={{ minWidth: 96, pt: 0.25 }}
      >
        {label}
      </Typography>
      <Box sx={{ flex: 1, minWidth: 0, fontSize: "0.875rem" }}>{children}</Box>
    </Box>
  );
}

// ---------------------------------------------------------------------------
// 1. Customer / Account context
// ---------------------------------------------------------------------------

/** "cloud_support" -> "cloud support". Matches the plain formatter already
 * used for the same enum on the project detail page. */
function formatSubscriptionType(value: string): string {
  return value.replace(/_/g, " ");
}

export function CustomerContextWidget({
  ctx,
  project,
  isLoadingProject,
}: {
  ctx: CaseCustomerContext;
  /** The case's project, via `GET /projects/{id}` — carries the subscription
   * type/dates plus a fuller account snapshot than the case-detail payload's
   * embedded `customerContext`. */
  project?: ProjectDetails | null;
  isLoadingProject?: boolean;
}): JSX.Element {
  return (
    <WidgetCard
      title="Customer"
      icon={<Building size={16} />}
      action={
        <Chip
          size="small"
          label={tierLabel(ctx.tier)}
          color={tierColor(ctx.tier)}
        />
      }
    >
      <MetaRow label="Account">
        <Typography variant="body2">
          <strong>{ctx.accountName}</strong>
        </Typography>
      </MetaRow>
      <MetaRow label="Account Manager">
        <Typography variant="body2">{ctx.accountManager}</Typography>
      </MetaRow>
      {ctx.technicalOwner && (
        <MetaRow label="Technical Owner">
          <Typography variant="body2">{ctx.technicalOwner}</Typography>
        </MetaRow>
      )}
      <MetaRow label="Region">
        <Typography
          variant="body2"
          sx={{ display: "flex", alignItems: "center", gap: 0.5 }}
        >
          <MapPin size={12} />
          {ctx.region}
        </Typography>
      </MetaRow>
      {isLoadingProject && (
        <MetaRow label="Project">
          <Typography variant="body2" color="text.secondary">
            Loading…
          </Typography>
        </MetaRow>
      )}
      {project && (
        <>
          <MetaRow label="Project name">
            <Typography variant="body2">{project.name}</Typography>
          </MetaRow>
          <MetaRow label="Project key">
            <Typography variant="body2" sx={{ fontFamily: "monospace" }}>
              {project.key}
            </Typography>
          </MetaRow>
          <MetaRow label="Subscription type">
            <Typography variant="body2" sx={{ textTransform: "capitalize" }}>
              {formatSubscriptionType(project.subscriptionType)}
            </Typography>
          </MetaRow>
          <MetaRow label="Subscription period">
            <Typography variant="body2">
              {formatDeploymentDate(project.startDate)} –{" "}
              {formatDeploymentDate(project.endDate)}
            </Typography>
          </MetaRow>
          {/* No subscription-status field exists on the project record today
              (only start/end dates) — omitted rather than inferring one. */}
        </>
      )}
    </WidgetCard>
  );
}

// ---------------------------------------------------------------------------
// 2. Product / environment context
// ---------------------------------------------------------------------------

export function ProductContextWidget({
  ctx,
  liveDeployment,
  isLoadingLiveDeployment,
}: {
  ctx: CaseProductContext;
  /** The case's deployment as returned by `POST /deployments/search`
   * (looked up by `ctx.deploymentId`) — the live name/type, rather than the
   * snapshot embedded in the case-detail payload at creation time. */
  liveDeployment?: BeDeployment | null;
  isLoadingLiveDeployment?: boolean;
}): JSX.Element {
  const deploymentName = liveDeployment?.name ?? ctx.deployment;
  const categoryLabel = liveDeployment
    ? deploymentTypeLabel(liveDeployment.type)
    : ctx.deploymentCategory
      ? deploymentTypeLabel(ctx.deploymentCategory)
      : null;
  return (
    <WidgetCard title="Deployment info" icon={<Server size={16} />}>
      <MetaRow label="Deployment">
        <Typography variant="body2">
          <strong>{isLoadingLiveDeployment ? "Loading…" : deploymentName}</strong>
        </Typography>
      </MetaRow>
      {!isLoadingLiveDeployment && categoryLabel && (
        <MetaRow label="Type">
          <Typography variant="body2">{categoryLabel}</Typography>
        </MetaRow>
      )}
      <MetaRow label="Product">
        <Typography variant="body2">
          <strong>{ctx.product}</strong>
        </Typography>
      </MetaRow>
      {ctx.updateLevel && (
        <MetaRow label="Update level">
          <Typography variant="body2">
            <code style={{ fontSize: "0.8rem" }}>{ctx.updateLevel}</code>
          </Typography>
        </MetaRow>
      )}
      {ctx.region && (
        <MetaRow label="Region">
          <Typography variant="body2">{ctx.region}</Typography>
        </MetaRow>
      )}
    </WidgetCard>
  );
}

// ---------------------------------------------------------------------------
// 3. Tags
// ---------------------------------------------------------------------------

export function TagsWidget({
  tags,
  onAdd,
}: {
  tags: CaseTag[];
  onAdd?: () => void;
}): JSX.Element {
  return (
    <WidgetCard
      title="Tags"
      icon={<Shield size={16} />}
      action={
        <Button
          size="small"
          variant="text"
          startIcon={<Plus size={14} />}
          onClick={onAdd}
        >
          Tag
        </Button>
      }
    >
      {tags.length === 0 ? (
        <Typography variant="body2" color="text.secondary">
          No tags applied.
        </Typography>
      ) : (
        <Box sx={{ display: "flex", flexWrap: "wrap", gap: 0.5 }}>
          {tags.map((t) => (
            <Chip
              key={t.id}
              size="small"
              label={t.label}
              color={t.color ?? "default"}
              variant="outlined"
            />
          ))}
        </Box>
      )}
    </WidgetCard>
  );
}

// ---------------------------------------------------------------------------
// 4. Time logs
// ---------------------------------------------------------------------------

export function TimeLogsWidget({
  logs,
  onAdd,
}: {
  logs: CaseTimeLogEntry[];
  onAdd?: () => void;
}): JSX.Element {
  const totalHours = logs.reduce((sum, l) => sum + l.hours, 0);
  return (
    <WidgetCard
      title="Time tracked"
      icon={<Clock size={16} />}
      action={
        <Button
          size="small"
          variant="text"
          startIcon={<Plus size={14} />}
          onClick={onAdd}
        >
          Log time
        </Button>
      }
    >
      <Box
        sx={{
          display: "flex",
          alignItems: "baseline",
          justifyContent: "space-between",
          mb: 1,
        }}
      >
        <Typography variant="h6" sx={{ lineHeight: 1 }}>
          {totalHours.toFixed(2)}h
        </Typography>
        <Typography variant="caption" color="text.secondary">
          Across {logs.length} {logs.length === 1 ? "entry" : "entries"}
        </Typography>
      </Box>
      <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
        {logs.slice(0, 3).map((l) => (
          <Box
            key={l.id}
            sx={{
              display: "flex",
              flexDirection: "column",
              p: 0.75,
              borderRadius: 1,
              border: 1,
              borderColor: "divider",
            }}
          >
            <Box
              sx={{
                display: "flex",
                alignItems: "center",
                justifyContent: "space-between",
                gap: 1,
              }}
            >
              <Typography variant="body2">{l.engineer}</Typography>
              <Chip size="small" variant="outlined" label={`${l.hours}h`} />
            </Box>
            <Typography variant="caption" color="text.secondary">
              {l.note} · <RelativeTime iso={l.date} />
            </Typography>
          </Box>
        ))}
      </Box>
    </WidgetCard>
  );
}

// ---------------------------------------------------------------------------
// 4b. Attachments (all files on the case, newest first)
// ---------------------------------------------------------------------------

/**
 * Lists every attachment on the case in descending order of upload time, with
 * a "Download all" affordance. Downloads are surfaced through the parent's
 * action handler (mock) until the BE exposes attachment download URLs.
 */
export function AttachmentsWidget({
  attachments,
  loading = false,
  error = false,
  onRetry,
  uploading = false,
  uploadError,
  onUpload,
  onDownloadAll,
  onDownload,
  onDelete,
  deletingId,
}: {
  attachments: CaseAttachment[];
  /** List query is loading. */
  loading?: boolean;
  /** List query failed. */
  error?: boolean;
  onRetry?: () => void;
  /** An upload is in flight. */
  uploading?: boolean;
  /** Message shown when the last upload failed (size, network, 413, …). */
  uploadError?: string | null;
  onUpload?: (file: File) => void;
  onDownloadAll?: () => void;
  onDownload?: (attachment: CaseAttachment) => void;
  /** Delete an attachment. Omit to hide the per-row delete affordance. */
  onDelete?: (attachment: CaseAttachment) => void;
  /** Id of the attachment whose delete is in flight; disables its row actions. */
  deletingId?: string | null;
}): JSX.Element {
  const fileInputRef = useRef<HTMLInputElement>(null);
  const sorted = [...attachments].sort(
    (a, b) =>
      new Date(b.uploadedAt).getTime() - new Date(a.uploadedAt).getTime(),
  );

  const pickFile = (): void => fileInputRef.current?.click();
  const onFileChange = (e: ChangeEvent<HTMLInputElement>): void => {
    const file = e.target.files?.[0];
    if (file) onUpload?.(file);
    // Reset so re-selecting the same file still fires onChange.
    if (fileInputRef.current) fileInputRef.current.value = "";
  };

  return (
    <WidgetCard
      title={`Attachments (${sorted.length})`}
      icon={<Paperclip size={16} />}
      action={
        <Box sx={{ display: "flex", alignItems: "center", gap: 0.5 }}>
          {onUpload && (
            <Button
              size="small"
              variant="text"
              startIcon={<Upload size={14} />}
              onClick={pickFile}
              disabled={uploading}
            >
              {uploading ? "Uploading…" : "Upload"}
            </Button>
          )}
          <Button
            size="small"
            variant="text"
            startIcon={<Download size={14} />}
            onClick={onDownloadAll}
            disabled={sorted.length === 0}
          >
            Download all
          </Button>
        </Box>
      }
    >
      {onUpload && (
        <input
          ref={fileInputRef}
          type="file"
          hidden
          onChange={onFileChange}
          aria-hidden
        />
      )}
      {uploading && <LinearProgress sx={{ mb: 1 }} />}
      {uploadError && (
        <Typography variant="body2" color="error" sx={{ mb: 1 }}>
          {uploadError}
        </Typography>
      )}
      {loading ? (
        <Typography variant="body2" color="text.secondary">
          Loading attachments…
        </Typography>
      ) : error ? (
        <Box sx={{ display: "flex", alignItems: "center", gap: 1.5, flexWrap: "wrap" }}>
          <Typography variant="body2" color="error">
            Could not load attachments.
          </Typography>
          {onRetry && (
            <Button size="small" variant="outlined" onClick={onRetry}>
              Retry
            </Button>
          )}
        </Box>
      ) : sorted.length === 0 ? (
        <Typography variant="body2" color="text.secondary">
          No attachments on this case.
        </Typography>
      ) : (
        <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
          {sorted.map((a) => (
            <Box
              key={a.id}
              sx={{
                display: "flex",
                alignItems: "center",
                gap: 1,
                p: 1,
                borderRadius: 1,
                border: 1,
                borderColor: "divider",
                transition: "background-color 120ms, border-color 120ms",
                "&:hover": {
                  borderColor: "primary.main",
                  backgroundColor: "action.hover",
                },
              }}
            >
              <Paperclip size={16} />
              <Box sx={{ flex: 1, minWidth: 0 }}>
                {onDownload ? (
                  <Typography
                    component="button"
                    variant="body2"
                    noWrap
                    onClick={() => onDownload(a)}
                    title={`Download ${a.filename}`}
                    sx={{
                      display: "block",
                      width: "100%",
                      textAlign: "left",
                      p: 0,
                      border: 0,
                      bgcolor: "transparent",
                      cursor: "pointer",
                      fontWeight: 600,
                      color: "primary.main",
                      "&:hover": { textDecoration: "underline" },
                    }}
                  >
                    {a.filename}
                  </Typography>
                ) : (
                  <Typography variant="body2" noWrap sx={{ fontWeight: 500 }}>
                    {a.filename}
                  </Typography>
                )}
                <Typography variant="caption" color="text.secondary" noWrap>
                  {formatBytes(a.size)} · {a.contentType} · uploaded by{" "}
                  {a.uploadedBy} · <RelativeTime iso={a.uploadedAt} />
                </Typography>
              </Box>
              <Button
                size="small"
                variant="outlined"
                startIcon={<Download size={14} />}
                onClick={() => onDownload?.(a)}
                aria-label={`Download ${a.filename}`}
                sx={{ flexShrink: 0 }}
              >
                Download
              </Button>
              {onDelete && (
                <IconButton
                  size="small"
                  color="error"
                  onClick={() => onDelete(a)}
                  disabled={deletingId === a.id}
                  aria-label={`Delete ${a.filename}`}
                  sx={{ flexShrink: 0 }}
                >
                  {deletingId === a.id ? (
                    <CircularProgress size={16} color="inherit" />
                  ) : (
                    <Trash2 size={16} />
                  )}
                </IconButton>
              )}
            </Box>
          ))}
        </Box>
      )}
    </WidgetCard>
  );
}

// ---------------------------------------------------------------------------
// 5. Audit timeline (lifecycle events, distinct from comments)
// ---------------------------------------------------------------------------

const AUDIT_ICON: Record<CaseAuditEntry["kind"], JSX.Element> = {
  state_change: <CheckCircle size={14} />,
  assignee_change: <User size={14} />,
  severity_change: <TriangleAlert size={14} />,
  linked: <LinkIcon size={14} />,
  escalated: <ArrowUpRight size={14} />,
  watcher_added: <Users size={14} />,
  comment_added: <Activity size={14} />,
  attachment_added: <Activity size={14} />,
  sla_breached: <TriangleAlert size={14} />,
  created: <Plus size={14} />,
  field_change: <Activity size={14} />,
};

export function AuditTimelineWidget({
  entries,
}: {
  entries: CaseAuditEntry[];
}): JSX.Element {
  return (
    <WidgetCard title="Lifecycle history" icon={<History size={16} />}>
      <Box sx={{ display: "flex", flexDirection: "column" }}>
        {entries.length === 0 ? (
          <Typography variant="body2" color="text.secondary">
            No lifecycle events yet.
          </Typography>
        ) : (
          entries
            .slice()
            .reverse()
            .map((e, idx, arr) => (
              <Box
                key={e.id}
                sx={{
                  display: "flex",
                  gap: 1,
                  position: "relative",
                  pb: idx === arr.length - 1 ? 0 : 1.25,
                }}
              >
                <Box
                  sx={{
                    width: 22,
                    flexShrink: 0,
                    display: "flex",
                    flexDirection: "column",
                    alignItems: "center",
                    color: e.kind === "sla_breached" ? "error.main" : "text.secondary",
                  }}
                >
                  <Box
                    sx={{
                      mt: 0.25,
                      p: 0.25,
                      borderRadius: "50%",
                      border: 1,
                      borderColor: e.kind === "sla_breached" ? "error.main" : "divider",
                      backgroundColor: "background.paper",
                    }}
                  >
                    {AUDIT_ICON[e.kind]}
                  </Box>
                  {idx !== arr.length - 1 && (
                    <Box
                      sx={{
                        width: 1,
                        flex: 1,
                        borderLeft: 1,
                        borderColor: "divider",
                        my: 0.25,
                      }}
                    />
                  )}
                </Box>
                <Box sx={{ flex: 1, minWidth: 0, pb: 0.25 }}>
                  <Typography variant="body2">{e.description}</Typography>
                  <Typography variant="caption" color="text.secondary">
                    {e.actor} · <RelativeTime iso={e.createdAt} />
                  </Typography>
                </Box>
              </Box>
            ))
        )}
      </Box>
    </WidgetCard>
  );
}

// ---------------------------------------------------------------------------
// Re-export: silence "Activity" reference vs `Activity` rename in lucide.
// ---------------------------------------------------------------------------
