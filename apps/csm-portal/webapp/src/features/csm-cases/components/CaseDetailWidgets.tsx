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
  ExternalLink,
  History,
  Link as LinkIcon,
  Mail,
  MapPin,
  Paperclip,
  Plus,
  Server,
  Shield,
  TriangleAlert,
  User,
  Users,
} from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import { Link as RouterLink } from "react-router";
import { initialsOf } from "@utils/userClaims";
import { formatBytes } from "@utils/formatBytes";
import type {
  CaseAttachment,
  CaseAuditEntry,
  CaseCustomerContext,
  CaseLinkedItem,
  CaseProductContext,
  CaseSlaClock,
  CaseTag,
  CaseTimeLogEntry,
  CaseWatcher,
  DeploymentCategory,
} from "@features/csm-cases/types/csmCases";
import { tierColor, tierLabel } from "@features/csm-cases/utils/caseTier";
import {
  SLA_CLOCK_LABEL,
  formatTimeToBreach,
} from "@features/csm-dashboard/utils/abtDashboard";
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
// 3. SLA timeline
// ---------------------------------------------------------------------------

function clockProgress(c: CaseSlaClock): number {
  if (c.state === "met") return 100;
  if (c.state === "breached") return 100;
  const elapsed = c.targetMinutes - Math.max(0, c.minutesToBreach);
  if (c.targetMinutes <= 0) return 0;
  return Math.min(100, Math.max(0, (elapsed / c.targetMinutes) * 100));
}

const CLOCK_COLOR: Record<
  CaseSlaClock["state"],
  "primary" | "warning" | "error" | "success"
> = {
  running: "primary",
  paused: "warning",
  met: "success",
  breached: "error",
};

export function SlaTimelineWidget({
  clocks,
}: {
  clocks: CaseSlaClock[];
}): JSX.Element {
  return (
    <WidgetCard title="SLA timeline" icon={<Clock size={16} />}>
      <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
        {clocks.map((c) => {
          const stateText =
            c.state === "met"
              ? "Met"
              : c.state === "breached"
                ? formatTimeToBreach(c.minutesToBreach)
                : c.state === "paused"
                  ? "Paused"
                  : formatTimeToBreach(c.minutesToBreach);
          return (
            <Box
              key={c.clockType}
              sx={{ display: "flex", flexDirection: "column", gap: 0.5 }}
            >
              <Box
                sx={{
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "space-between",
                  gap: 1,
                }}
              >
                <Typography variant="body2">
                  {SLA_CLOCK_LABEL[c.clockType]}
                </Typography>
                <Typography
                  variant="caption"
                  color={
                    c.state === "breached"
                      ? "error.main"
                      : c.state === "met"
                        ? "success.main"
                        : c.state === "paused"
                          ? "warning.main"
                          : "text.secondary"
                  }
                >
                  {stateText}
                </Typography>
              </Box>
              <LinearProgress
                variant="determinate"
                value={clockProgress(c)}
                color={CLOCK_COLOR[c.state]}
                sx={{ height: 6, borderRadius: 1 }}
              />
              <Typography variant="caption" color="text.secondary">
                Target: {formatMinutes(c.targetMinutes)}
              </Typography>
            </Box>
          );
        })}
      </Box>
    </WidgetCard>
  );
}

function formatMinutes(m: number): string {
  if (m < 60) return `${m}m`;
  const h = m / 60;
  if (h < 24) return `${h}h`;
  return `${Math.round(h / 24)}d`;
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
  onDownloadAll,
  onDownload,
}: {
  attachments: CaseAttachment[];
  onDownloadAll?: () => void;
  onDownload?: (attachment: CaseAttachment) => void;
}): JSX.Element {
  const sorted = [...attachments].sort(
    (a, b) =>
      new Date(b.uploadedAt).getTime() - new Date(a.uploadedAt).getTime(),
  );
  return (
    <WidgetCard
      title={`Attachments (${sorted.length})`}
      icon={<Paperclip size={16} />}
      action={
        <Button
          size="small"
          variant="text"
          startIcon={<Download size={14} />}
          onClick={onDownloadAll}
          disabled={sorted.length === 0}
        >
          Download all
        </Button>
      }
    >
      {sorted.length === 0 ? (
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
              }}
            >
              <Paperclip size={16} />
              <Box sx={{ flex: 1, minWidth: 0 }}>
                <Typography variant="body2" noWrap sx={{ fontWeight: 500 }}>
                  {a.filename}
                </Typography>
                <Typography variant="caption" color="text.secondary" noWrap>
                  {formatBytes(a.size)} · {a.contentType} · uploaded by{" "}
                  {a.uploadedBy} · <RelativeTime iso={a.uploadedAt} />
                </Typography>
              </Box>
              <Tooltip title="Download">
                <IconButton
                  size="small"
                  aria-label={`Download ${a.filename}`}
                  onClick={() => onDownload?.(a)}
                >
                  <Download size={16} />
                </IconButton>
              </Tooltip>
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
