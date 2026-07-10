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
  Avatar,
  Box,
  Button,
  Card,
  Chip,
  CircularProgress,
  IconButton,
  LinearProgress,
  Typography,
} from "@wso2/oxygen-ui";
import {
  Activity,
  ArrowUpRight,
  Building,
  CheckCircle,
  Clock,
  Download,
  ExternalLink,
  History,
  Link as LinkIcon,
  Mail,
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
import { Link as RouterLink } from "react-router";
import { initialsOf } from "@utils/userClaims";
import { formatBytes } from "@utils/formatBytes";
import type {
  CaseAttachment,
  CaseAuditEntry,
  CaseCustomerContext,
  CaseLinkedItem,
  CaseProductContext,
  CaseTag,
  CaseTimeLogEntry,
  CaseWatcher,
  DeploymentCategory,
} from "@features/csm-cases/types/csmCases";
import { tierColor, tierLabel } from "@features/csm-cases/utils/caseTier";
import RelativeTime from "@components/RelativeTime";

// ---------------------------------------------------------------------------
// Shared widget shell
// ---------------------------------------------------------------------------

interface WidgetCardProps {
  title: string;
  icon?: JSX.Element;
  action?: JSX.Element;
  children: React.ReactNode;
}

function WidgetCard({ title, icon, action, children }: WidgetCardProps): JSX.Element {
  return (
    <Card variant="outlined" sx={{ p: 2 }}>
      <Box
        sx={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          gap: 1,
          mb: 1.25,
        }}
      >
        <Box sx={{ display: "flex", alignItems: "center", gap: 0.75 }}>
          {icon}
          <Typography variant="subtitle2">{title}</Typography>
        </Box>
        {action}
      </Box>
      {children}
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

export function CustomerContextWidget({
  ctx,
}: {
  ctx: CaseCustomerContext;
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
      <MetaRow label="Primary contact">
        <Box sx={{ display: "flex", flexDirection: "column" }}>
          <Typography variant="body2">{ctx.primaryContact}</Typography>
          <Typography
            variant="caption"
            color="text.secondary"
            sx={{ display: "flex", alignItems: "center", gap: 0.5 }}
          >
            <Mail size={12} />
            {ctx.primaryContactEmail}
          </Typography>
        </Box>
      </MetaRow>
      <MetaRow label="Region">
        <Typography
          variant="body2"
          sx={{ display: "flex", alignItems: "center", gap: 0.5 }}
        >
          <MapPin size={12} />
          {ctx.region}
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
      <MetaRow label="Open cases">
        <Chip size="small" variant="outlined" label={`${ctx.openCases} open`} />
      </MetaRow>
    </WidgetCard>
  );
}

// ---------------------------------------------------------------------------
// 2. Product / environment context
// ---------------------------------------------------------------------------

const ENV_COLOR: Record<
  CaseProductContext["environment"],
  "default" | "info" | "warning" | "error"
> = {
  dev: "default",
  qa: "info",
  staging: "warning",
  prod: "error",
};

const DEPLOYMENT_CATEGORY_LABEL: Record<DeploymentCategory, string> = {
  primary_production: "Primary Production",
  staging: "Staging",
  qa: "QA",
  stress: "Stress",
  uat: "UAT",
  development: "Development",
};

export function ProductContextWidget({
  ctx,
}: {
  ctx: CaseProductContext;
}): JSX.Element {
  const categoryLabel = ctx.deploymentCategory
    ? DEPLOYMENT_CATEGORY_LABEL[ctx.deploymentCategory]
    : null;
  return (
    <WidgetCard title="Deployment info" icon={<Server size={16} />}>
      <MetaRow label="Deployment">
        <Box
          sx={{ display: "flex", alignItems: "center", gap: 1, flexWrap: "wrap" }}
        >
          <Typography variant="body2">
            <strong>{ctx.deployment}</strong>
          </Typography>
          {categoryLabel && (
            <Chip
              size="small"
              variant="outlined"
              label={categoryLabel}
              color={ENV_COLOR[ctx.environment]}
            />
          )}
        </Box>
      </MetaRow>
      <MetaRow label="Product">
        <Typography variant="body2">
          <strong>{ctx.product}</strong>
        </Typography>
      </MetaRow>
      <MetaRow label="Version">
        <Typography variant="body2">{ctx.version}</Typography>
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
// 4. Watchers
// ---------------------------------------------------------------------------

const ROLE_LABEL: Record<CaseWatcher["role"], string> = {
  wso2_engineer: "WSO2",
  customer_contact: "Customer",
  manager: "Manager",
};

export function WatchersWidget({
  watchers,
  onAdd,
}: {
  watchers: CaseWatcher[];
  onAdd?: () => void;
}): JSX.Element {
  return (
    <WidgetCard
      title="Watchers"
      icon={<Users size={16} />}
      action={
        <Button
          size="small"
          variant="text"
          startIcon={<Plus size={14} />}
          onClick={onAdd}
        >
          Add
        </Button>
      }
    >
      <Box sx={{ display: "flex", flexDirection: "column", gap: 0.5 }}>
        {watchers.map((w) => (
          <Box
            key={w.id}
            sx={{
              display: "flex",
              alignItems: "center",
              gap: 1,
              py: 0.5,
            }}
          >
            <Avatar sx={{ width: 28, height: 28, fontSize: "0.75rem" }}>
              {initialsOf(w.name)}
            </Avatar>
            <Box sx={{ flex: 1, minWidth: 0 }}>
              <Typography variant="body2" sx={{ lineHeight: 1.2 }}>
                {w.name} {w.isMe && <em>(you)</em>}
              </Typography>
              <Typography variant="caption" color="text.secondary">
                {ROLE_LABEL[w.role]}
              </Typography>
            </Box>
          </Box>
        ))}
      </Box>
    </WidgetCard>
  );
}

// ---------------------------------------------------------------------------
// 5. Linked items
// ---------------------------------------------------------------------------

const LINKED_ICON: Record<CaseLinkedItem["kind"], JSX.Element> = {
  case: <LinkIcon size={14} />,
  incident: <TriangleAlert size={14} />,
  escalation: <ArrowUpRight size={14} />,
  kb: <Shield size={14} />,
  cr: <CheckCircle size={14} />,
  sr: <CheckCircle size={14} />,
};

const LINKED_LABEL: Record<CaseLinkedItem["kind"], string> = {
  case: "Case",
  incident: "Incident",
  escalation: "Escalation",
  kb: "KB",
  cr: "CR",
  sr: "SR",
};

export function LinkedItemsWidget({
  items,
  onLink,
}: {
  items: CaseLinkedItem[];
  onLink?: () => void;
}): JSX.Element {
  return (
    <WidgetCard
      title="Linked items"
      icon={<LinkIcon size={16} />}
      action={
        <Button
          size="small"
          variant="text"
          startIcon={<Plus size={14} />}
          onClick={onLink}
        >
          Link
        </Button>
      }
    >
      {items.length === 0 ? (
        <Typography variant="body2" color="text.secondary">
          Nothing linked yet.
        </Typography>
      ) : (
        <Box sx={{ display: "flex", flexDirection: "column", gap: 0.75 }}>
          {items.map((item) => {
            const inner = (
              <Box
                sx={{
                  display: "flex",
                  alignItems: "center",
                  gap: 1,
                  p: 0.75,
                  borderRadius: 1,
                  border: 1,
                  borderColor: "divider",
                  "&:hover": item.href
                    ? { backgroundColor: "action.hover", cursor: "pointer" }
                    : undefined,
                }}
              >
                {LINKED_ICON[item.kind]}
                <Box sx={{ flex: 1, minWidth: 0 }}>
                  <Typography
                    variant="caption"
                    color="text.secondary"
                    sx={{ display: "block", lineHeight: 1.1 }}
                  >
                    {LINKED_LABEL[item.kind]} · {item.reference} · {item.state}
                  </Typography>
                  <Typography
                    variant="body2"
                    sx={{
                      lineHeight: 1.2,
                      overflow: "hidden",
                      textOverflow: "ellipsis",
                      whiteSpace: "nowrap",
                    }}
                  >
                    {item.title}
                  </Typography>
                </Box>
                {item.href && <ExternalLink size={14} />}
              </Box>
            );
            return item.href ? (
              <RouterLink
                key={item.id}
                to={item.href}
                style={{ textDecoration: "none", color: "inherit" }}
              >
                {inner}
              </RouterLink>
            ) : (
              <Box key={item.id}>{inner}</Box>
            );
          })}
        </Box>
      )}
    </WidgetCard>
  );
}

// ---------------------------------------------------------------------------
// 6. Tags
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
// 7. Time logs
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
// 7b. Attachments (all files on the case, newest first)
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
// 8. Audit timeline (lifecycle events, distinct from comments)
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
