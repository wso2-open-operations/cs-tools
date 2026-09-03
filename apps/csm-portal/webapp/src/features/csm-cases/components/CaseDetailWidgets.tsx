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
  ArrowDownRight,
  ArrowUpRight,
  Bell,
  BellOff,
  Building,
  CheckCircle,
  ClipboardList,
  Clock,
  Download,
  Eye,
  History,
  Link as LinkIcon,
  Paperclip,
  Plus,
  Server,
  Shield,
  Trash2,
  TriangleAlert,
  Upload,
  User,
  Users,
  X,
} from "@wso2/oxygen-ui-icons-react";
import {
  Fragment,
  useCallback,
  useId,
  useMemo,
  useRef,
  type ChangeEvent,
  type JSX,
  type ReactNode,
} from "react";
import { Link as RouterLink } from "react-router";
import { formatBytes } from "@utils/formatBytes";
import DirectoryEntityChip from "@features/csm-admin/components/DirectoryEntityChip";
import { useSearchUsersByName } from "@api/useSearchUsersByName";
import { userLabel } from "@features/csm-operations/utils/incidentFormOptions";
import AttachmentPreviewDialog from "@features/csm-cases/components/AttachmentPreviewDialog";
import { getAttachmentPreviewKind } from "@features/csm-cases/utils/attachmentPreview";
import type {
  CaseAttachment,
  CaseAuditEntry,
  CaseCustomerContext,
  CaseEscalationRecord,
  CaseProductContext,
  CaseRequestVariable,
  CaseTag,
  CaseTimeLogEntry,
} from "@features/csm-cases/types/csmCases";
import { tierColor, tierLabel } from "@features/csm-cases/utils/caseTier";
import { escalationLevelLabel } from "@features/csm-cases/utils/escalationLevel";
import {
  deploymentTypeLabel,
  formatDeploymentDate,
} from "@features/csm-projects/utils/deployments";
import type { ProjectDetails } from "@features/csm-projects/types/csmProjects";
import type { BeDeployment, BeUser } from "@api/backend/types";
import type { UserReference } from "@/types/userReference";
import AsyncEntitySelect from "@components/AsyncEntitySelect";
import EscalationLevelChip from "@components/EscalationLevelChip";
import RelativeTime from "@components/RelativeTime";
import UserRefLink from "@components/UserRefLink";
import RefreshButton from "@components/RefreshButton";

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

// Same link styling/convention as `CaseMetaBand`'s own `LinkText` (not
// exported from there) — a real anchor so account/project names stay
// cmd/middle-clickable and copyable, with plain left-click staying in-app.
function LinkText({ to, children }: { to: string; children: ReactNode }): JSX.Element {
  return (
    <Typography
      component={RouterLink}
      to={to}
      variant="body2"
      sx={(t) => ({
        display: "inline",
        cursor: "pointer",
        textDecoration: "none",
        color: t.palette.primary.dark,
        ...t.applyStyles("dark", { color: t.palette.primary.main }),
        "&:hover": { textDecoration: "underline" },
        "&:focus-visible": {
          outline: "2px solid",
          outlineColor: "primary.main",
          outlineOffset: 2,
          borderRadius: 0.5,
        },
      })}
    >
      {children}
    </Typography>
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
  accountId,
}: {
  ctx: CaseCustomerContext;
  /** The case's project, via `GET /projects/{id}` — carries the subscription
   * type/dates plus a fuller account snapshot than the case-detail payload's
   * embedded `customerContext`. */
  project?: ProjectDetails | null;
  isLoadingProject?: boolean;
  /** The case's account id, when known — links `ctx.accountName` to its
   * detail page (`/customers/accounts/{accountId}`), matching `CaseMetaBand`'s
   * own Account cell. Omit (e.g. an announcement without a resolved account)
   * to fall back to plain text. */
  accountId?: string;
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
          {accountId ? (
            <strong>
              <LinkText to={`/customers/accounts/${accountId}`}>
                {ctx.accountName}
              </LinkText>
            </strong>
          ) : (
            <strong>{ctx.accountName}</strong>
          )}
        </Typography>
      </MetaRow>
      {ctx.technicalOwner && (
        <MetaRow label="Technical Owner">
          <Typography variant="body2">{ctx.technicalOwner}</Typography>
        </MetaRow>
      )}
      {(ctx.creTeam || ctx.sreTeam) && (
        <MetaRow label="CRE / SRE team">
          <Box sx={{ display: "flex", flexWrap: "wrap", gap: 0.5 }}>
            {ctx.creTeam && (
              <DirectoryEntityChip
                id={ctx.creTeam.id}
                name={ctx.creTeam.name}
                routeBase="/admin/teams"
              />
            )}
            {ctx.sreTeam && (
              <DirectoryEntityChip
                id={ctx.sreTeam.id}
                name={ctx.sreTeam.name}
                routeBase="/admin/teams"
              />
            )}
          </Box>
        </MetaRow>
      )}
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
            <Typography variant="body2">
              <LinkText to={`/customers/projects/${project.id}`}>
                {project.name}
              </LinkText>
            </Typography>
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
  onRemove,
  removingId,
}: {
  tags: CaseTag[];
  onAdd?: () => void;
  /** Remove a single tag (`DELETE /cases/{id}/tags/{tagId}`). Omit to hide the per-chip delete affordance. */
  onRemove?: (tag: CaseTag) => void;
  /** Id of the tag whose removal is in flight; disables its chip's delete. */
  removingId?: string | null;
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
              onDelete={onRemove ? () => onRemove(t) : undefined}
              disabled={removingId === t.id}
            />
          ))}
        </Box>
      )}
    </WidgetCard>
  );
}

// ---------------------------------------------------------------------------
// 3b. Escalation
// ---------------------------------------------------------------------------

/**
 * Current escalation level (as a badge) plus a read-only history of past
 * escalate/de-escalate steps. Each of `onEscalate` / `onDeescalate` renders
 * its own clearly-labeled button ("Escalate" / "De-escalate") independently —
 * both can show at once (e.g. EL2, which can go either way), or just one
 * (EL0 has only "Escalate"; EL5 has only "De-escalate"). Two distinctly
 * labeled buttons are unambiguous about which action a click performs, unlike
 * a single button whose label/target silently changes with the case's
 * current level would be.
 *
 * No role/permission gate on either action button — deliberate, matches the
 * backend, which has none either (see `usePostCsmCaseEscalation`'s doc
 * comment). `onEscalate`/`onDeescalate` both open the caller's own confirm
 * dialog (which collects the reason) rather than firing the mutation
 * directly, so this widget never has to know the mutation's pending/error
 * state itself.
 */
export function EscalationWidget({
  currentLevel,
  history,
  isHistoryLoading,
  isHistoryError,
  onEscalate,
  onDeescalate,
  actionDisabledReason,
}: {
  /** Raw escalation-level id ("0"-"5"), or null when the data source doesn't
   * track it (e.g. non-ServiceNow-backed case). */
  currentLevel: string | null;
  history: CaseEscalationRecord[];
  isHistoryLoading?: boolean;
  isHistoryError?: boolean;
  /** Opens the escalate confirm dialog. Omit to hide the button entirely
   * (e.g. already at EL5). */
  onEscalate?: () => void;
  /** Opens the de-escalate confirm dialog. Omit to hide the button entirely
   * (e.g. already at EL0 / unset). */
  onDeescalate?: () => void;
  /** Disables both action buttons with an explanatory tooltip (e.g. "This
   * case is closed — it's read-only.") without hiding them. */
  actionDisabledReason?: string;
}): JSX.Element {
  const level = currentLevel ?? "0";

  return (
    <WidgetCard
      title="Escalation"
      icon={<ArrowUpRight size={16} />}
      action={
        <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
          <EscalationLevelChip level={level} />
          {(onEscalate || onDeescalate) && (
            <Tooltip title={actionDisabledReason ?? ""}>
              <Box component="span" sx={{ display: "flex", gap: 0.5 }}>
                {onDeescalate && (
                  <Button
                    size="small"
                    variant="outlined"
                    onClick={onDeescalate}
                    disabled={!!actionDisabledReason}
                  >
                    De-escalate
                  </Button>
                )}
                {onEscalate && (
                  <Button
                    size="small"
                    variant="outlined"
                    onClick={onEscalate}
                    disabled={!!actionDisabledReason}
                  >
                    Escalate
                  </Button>
                )}
              </Box>
            </Tooltip>
          )}
        </Box>
      }
    >
      {isHistoryLoading ? (
        <Typography variant="body2" color="text.secondary">
          Loading escalation history…
        </Typography>
      ) : isHistoryError ? (
        <Typography variant="body2" color="error">
          Could not load the escalation history.
        </Typography>
      ) : history.length === 0 ? (
        <Typography variant="body2" color="text.secondary">
          No escalations on this case.
        </Typography>
      ) : (
        <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
          {history.map((h) => {
            const isEscalate =
              Number(h.currentLevel) > Number(h.previousLevel);
            return (
              <Box
                key={h.id}
                sx={{
                  display: "flex",
                  flexDirection: "column",
                  gap: 0.25,
                  p: 0.75,
                  borderRadius: 1,
                  border: 1,
                  borderColor: "divider",
                }}
              >
                <Box sx={{ display: "flex", alignItems: "center", gap: 0.75 }}>
                  {isEscalate ? (
                    <ArrowUpRight size={14} />
                  ) : (
                    <ArrowDownRight size={14} />
                  )}
                  <Typography variant="body2">
                    {`${escalationLevelLabel(h.previousLevel, true)} → ${escalationLevelLabel(h.currentLevel, true)}`}
                  </Typography>
                </Box>
                <Typography variant="caption" color="text.secondary">
                  {h.createdBy} · <RelativeTime iso={h.createdOn} />
                </Typography>
                {h.reason && (
                  <Typography
                    variant="body2"
                    sx={{ mt: 0.25, overflowWrap: "anywhere" }}
                  >
                    {h.reason}
                  </Typography>
                )}
              </Box>
            );
          })}
        </Box>
      )}
    </WidgetCard>
  );
}

// ---------------------------------------------------------------------------
// 3c. Watchers
// ---------------------------------------------------------------------------

/**
 * One entry on a record's watch list, in the shape both the case and the
 * incident detail read models expose. `id` is the platform user UUID, which
 * is also what the write side is keyed by — see {@link WatchersWidget}.
 */
export interface WatchListMember {
  id: string;
  name: string;
  email?: string;
  /** True for the signed-in engineer; renders a "(you)" suffix. */
  isMe?: boolean;
  /** Canonical user reference, when the read model carries one. */
  user?: UserReference;
}

/**
 * The single place the per-record-type watch-list rules live, so the two
 * detail pages rendering {@link WatchersWidget} can't drift apart or carry
 * their own copy of the "may the last watcher go?" flag.
 *
 * `minWatchers` is the asymmetry between the two. The incident update request
 * declares its watch list as an *optional* list, so an explicitly empty one
 * survives the round trip and clears the list. The case update request
 * declares a plain list, which makes an empty one indistinguishable from an
 * absent field, and the backend then rejects the whole request as changing
 * nothing. Removing a case's only watcher is therefore not expressible at
 * all, so the control is blocked rather than fired at a request already known
 * to fail. Lifting it needs a deliberate change to the case update contract.
 */
const WATCH_LIST_RULES = {
  case: {
    noun: "case",
    minWatchers: 1,
    minWatchersReason: "A case must keep at least one watcher.",
  },
  incident: {
    noun: "incident",
    minWatchers: 0,
    minWatchersReason: "",
  },
} as const;

/** Record types that have an editable watch list. */
export type WatchedEntityKind = keyof typeof WATCH_LIST_RULES;

/**
 * Watch list for a case or an incident, with add/remove.
 *
 * Neither backend has an add-one/remove-one endpoint: both take the **whole**
 * watch list as user UUIDs and replace what is stored. Sending only the user
 * that changed silently wipes everyone else. So this widget — not its callers
 * — computes the replacement list in {@link addWatcher}/{@link removeWatcher}
 * and hands the finished array to `onReplace`; a page can only forward it.
 *
 * `entityKind` selects the rules in {@link WATCH_LIST_RULES}; nothing else
 * about this component varies by record type.
 */
export function WatchersWidget({
  entityKind,
  watchers,
  onReplace,
  isSaving,
  onRefresh,
  isRefreshing,
  refreshedAt,
  currentUserId,
  autoWatchingReason,
}: {
  /** Which record's watch list this is. Drives the copy and the rules. */
  entityKind: WatchedEntityKind;
  watchers: WatchListMember[];
  /**
   * Persist `nextWatcherIds` as the record's complete new watch list. Already
   * the full replacement list, never a delta. Omit to render read-only.
   */
  onReplace?: (nextWatcherIds: string[], action: "add" | "remove") => void;
  /** True while a watch-list write is in flight; blocks add and remove so a
   * double-click can't fire two conflicting replacements. */
  isSaving?: boolean;
  /** Re-runs the detail query the watch list comes from. Omit to hide the
   * refresh control. */
  onRefresh?: () => void;
  isRefreshing?: boolean;
  refreshedAt?: number;
  /**
   * Platform UUID of the signed-in engineer. Drives the self-subscribe
   * Follow/Unfollow control: without it there is no id to add on Follow, so
   * the button is omitted entirely rather than rendered disabled.
   */
  currentUserId?: string;
  /**
   * Non-empty when the signed-in engineer is on this watch list only because
   * of an automatic, role-based add (e.g. they're the record's assigned
   * engineer) rather than having chosen to self-subscribe. The widget has no
   * visibility into role assignment, so the caller supplies this; when set,
   * Unfollow is blocked with this as the reason, same treatment as
   * {@link removalBlockedReason} above.
   */
  autoWatchingReason?: string;
}): JSX.Element {
  const rules = WATCH_LIST_RULES[entityKind];
  const reasonId = useId();
  const followReasonId = useId();
  const watcherIds = useMemo(() => watchers.map((w) => w.id), [watchers]);
  // Keyed on the UUID, not `isMe`: both page callers derive `isMe` from an
  // email match, which is unreliable when email data is missing, whereas
  // `currentUserId` is the same UUID the watch list itself is keyed by.
  const isFollowing = !!currentUserId && watcherIds.includes(currentUserId);

  // Below the floor the record type allows, removal isn't expressible at all
  // (see WATCH_LIST_RULES), so the control is blocked with the reason rather
  // than firing a request that is known to be rejected.
  const belowFloorAfterRemoval = watchers.length <= rules.minWatchers;
  const removalBlockedReason = belowFloorAfterRemoval ? rules.minWatchersReason : "";
  const unfollowBlockedReason = autoWatchingReason || removalBlockedReason;

  const addWatcher = useCallback(
    (userId: string) => {
      if (!onReplace || !userId || isSaving) return;
      // Already watching: nothing to write, and resending the same list would
      // burn a request for no change.
      if (watcherIds.includes(userId)) return;
      onReplace([...watcherIds, userId], "add");
    },
    [onReplace, isSaving, watcherIds],
  );

  const removeWatcher = useCallback(
    (watcher: WatchListMember) => {
      if (!onReplace || isSaving || belowFloorAfterRemoval) return;
      onReplace(
        watcherIds.filter((id) => id !== watcher.id),
        "remove",
      );
    },
    [onReplace, isSaving, belowFloorAfterRemoval, watcherIds],
  );

  // Self-subscribe: the same add/remove path as the per-watcher controls
  // below, just always targeting the signed-in engineer's own id rather than
  // a picked-from-search or a listed watcher.
  const onFollowClick = useCallback(() => {
    if (currentUserId) addWatcher(currentUserId);
  }, [addWatcher, currentUserId]);
  const onUnfollowClick = useCallback(() => {
    if (!currentUserId || unfollowBlockedReason) return;
    if (!onReplace || isSaving) return;
    onReplace(
      watcherIds.filter((id) => id !== currentUserId),
      "remove",
    );
  }, [currentUserId, unfollowBlockedReason, onReplace, isSaving, watcherIds]);

  return (
    <WidgetCard
      title="Watchers"
      icon={<Users size={16} />}
      action={
        onRefresh && (
          <RefreshButton
            onRefresh={onRefresh}
            isFetching={!!isRefreshing}
            updatedAt={refreshedAt}
            label="Refresh watchers"
          />
        )
      }
    >
      {onReplace && currentUserId && (
        <Box sx={{ mb: 1.5 }}>
          {isFollowing ? (
            <Tooltip title={unfollowBlockedReason}>
              {/* aria-disabled, not disabled, so the reason stays reachable
                  via aria-describedby — same reasoning as the per-watcher
                  remove control below. */}
              <span>
                <Button
                  size="small"
                  variant="outlined"
                  startIcon={<BellOff size={14} />}
                  aria-disabled={!!unfollowBlockedReason || isSaving || undefined}
                  aria-describedby={unfollowBlockedReason ? followReasonId : undefined}
                  onClick={onUnfollowClick}
                  sx={{ opacity: unfollowBlockedReason ? 0.6 : 1 }}
                >
                  {`Unfollow ${rules.noun} updates`}
                </Button>
              </span>
            </Tooltip>
          ) : (
            <Button
              size="small"
              variant="outlined"
              startIcon={<Bell size={14} />}
              disabled={isSaving}
              onClick={onFollowClick}
            >
              {`Follow ${rules.noun} updates`}
            </Button>
          )}
          {unfollowBlockedReason && (
            <Box
              component="span"
              id={followReasonId}
              sx={{
                position: "absolute",
                width: 1,
                height: 1,
                overflow: "hidden",
                clip: "rect(0 0 0 0)",
                whiteSpace: "nowrap",
              }}
            >
              {unfollowBlockedReason}
            </Box>
          )}
        </Box>
      )}
      {watchers.length === 0 ? (
        <Typography variant="body2" color="text.secondary">
          {`No one is watching this ${rules.noun}.`}
        </Typography>
      ) : (
        <Box
          component="ul"
          aria-label="Watchers"
          sx={{ listStyle: "none", m: 0, p: 0, display: "flex", flexDirection: "column" }}
        >
          {watchers.map((w) => {
            const label = w.isMe ? `${w.name} (you)` : w.name;
            return (
              <Box
                component="li"
                key={w.id}
                sx={{
                  display: "flex",
                  alignItems: "center",
                  gap: 1,
                  py: 0.5,
                  minWidth: 0,
                  borderBottom: 1,
                  borderColor: "divider",
                  "&:last-of-type": { borderBottom: 0 },
                }}
              >
                <User size={14} />
                <Typography
                  variant="body2"
                  component="span"
                  sx={{ flex: 1, minWidth: 0, overflowWrap: "anywhere" }}
                >
                  <UserRefLink
                    name={label}
                    email={w.user?.email || w.email}
                    userId={w.user?.id}
                  />
                </Typography>
                {onReplace && (
                  <Tooltip title={removalBlockedReason}>
                    {/* aria-disabled, not disabled: a disabled button drops out
                        of the tab order, taking the reason for its own
                        disabling with it. This stays focusable, keeps its
                        tooltip, and points at that reason via
                        aria-describedby. */}
                    <IconButton
                      size="small"
                      aria-label={`Remove ${label} from the watch list`}
                      aria-disabled={belowFloorAfterRemoval || isSaving || undefined}
                      aria-describedby={belowFloorAfterRemoval ? reasonId : undefined}
                      onClick={() => removeWatcher(w)}
                      sx={{
                        color: "text.secondary",
                        opacity: belowFloorAfterRemoval || isSaving ? 0.5 : 1,
                      }}
                    >
                      <X size={14} />
                    </IconButton>
                  </Tooltip>
                )}
              </Box>
            );
          })}
        </Box>
      )}
      {onReplace && (
        <Box sx={{ mt: 1.5 }}>
          <AsyncEntitySelect<BeUser>
            id={`${entityKind}-watchers-add`}
            label="Add a watcher"
            placeholder="Search people…"
            // Held at "" so the field clears itself after each pick and reads
            // as an "add another" control rather than a current selection.
            value=""
            onChange={addWatcher}
            disabled={isSaving}
            useSearch={useSearchUsersByName}
            // useSearchUsersByName drops any user without an id, so every
            // option here is guaranteed to have one.
            getId={(u) => u.id!}
            getLabel={userLabel}
            excludeIds={watcherIds}
            helperText={`Notified on updates to this ${rules.noun}.`}
          />
        </Box>
      )}
      {belowFloorAfterRemoval && onReplace && watchers.length > 0 && (
        <Box
          component="span"
          id={reasonId}
          sx={{
            position: "absolute",
            width: 1,
            height: 1,
            overflow: "hidden",
            clip: "rect(0 0 0 0)",
            whiteSpace: "nowrap",
          }}
        >
          {removalBlockedReason}
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
  preview,
  onRefresh,
  isRefreshing,
  refreshedAt,
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
  /**
   * Inline attachment preview. All three fields are required together —
   * fetching content, tracking which attachment is open, and closing the
   * dialog are one feature, not three independent knobs — so omit the whole
   * object to hide the per-row Preview affordance entirely (e.g. in contexts
   * without network access, such as tests/storybook) rather than supplying
   * only some of the fields.
   */
  preview?: {
    /** Fetch an attachment's raw bytes for inline preview. */
    onGetPreviewContent: (attachment: CaseAttachment) => Promise<Blob>;
    /**
     * Attachment currently shown in the preview dialog, lifted to the parent
     * page so it can be reset on case-to-case navigation (this widget stays
     * mounted while the page's `caseId` route param changes).
     */
    previewTarget: CaseAttachment | null;
    onPreviewTargetChange: (attachment: CaseAttachment | null) => void;
  };
  /** Re-runs the attachments list query. Omit to hide the refresh control. */
  onRefresh?: () => void;
  isRefreshing?: boolean;
  refreshedAt?: number;
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
    <>
      <WidgetCard
        title={`Attachments (${sorted.length})`}
        icon={<Paperclip size={16} />}
        action={
          <Box sx={{ display: "flex", alignItems: "center", gap: 0.5 }}>
            {onRefresh && (
              <RefreshButton
                onRefresh={onRefresh}
                isFetching={!!isRefreshing}
                updatedAt={refreshedAt}
                label="Refresh attachments"
              />
            )}
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
                    <UserRefLink
                      name={a.uploadedBy}
                      email={a.uploadedByUser?.email || a.uploadedByEmail}
                      userId={a.uploadedByUser?.id}
                    />{" "}
                    · <RelativeTime iso={a.uploadedAt} />
                  </Typography>
                </Box>
                {preview &&
                  getAttachmentPreviewKind(a.contentType) && (
                    <Button
                      size="small"
                      variant="outlined"
                      startIcon={<Eye size={14} />}
                      onClick={() => preview.onPreviewTargetChange(a)}
                      aria-label={`Preview ${a.filename}`}
                      sx={{ flexShrink: 0 }}
                    >
                      Preview
                    </Button>
                  )}
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
      {preview && (
        <AttachmentPreviewDialog
          attachment={preview.previewTarget}
          onClose={() => preview.onPreviewTargetChange(null)}
          fetchContent={preview.onGetPreviewContent}
        />
      )}
    </>
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
// 8. Service-request "Request details"
// ---------------------------------------------------------------------------

/** Placeholder for a value the backing data source did not give us. */
const NO_VALUE = "\u2014";

/**
 * The catalog, catalog item, and the answers the requester gave to that
 * item's questions — the payload a service request is actually made of, and
 * which the engineer working it otherwise cannot see anywhere in the portal.
 *
 * Renders even when `variables` is empty: an SR with no captured answers is a
 * real upstream data problem, and hiding the card would make it invisible.
 * An answer that is present but blank renders as an em dash, so "asked and
 * left blank" stays distinguishable from "never asked".
 */
export function RequestDetailsWidget({
  catalog,
  catalogItem,
  variables,
}: {
  catalog?: { id: string; name: string };
  catalogItem?: { id: string; name: string };
  /** In the backing data source's display order — never re-sorted here. */
  variables?: CaseRequestVariable[];
}): JSX.Element {
  const answers = variables ?? [];
  return (
    <WidgetCard title="Request details" icon={<ClipboardList size={16} />}>
      <MetaRow label="Catalog">
        <Typography variant="body2">{catalog?.name || NO_VALUE}</Typography>
      </MetaRow>
      <MetaRow label="Catalog item">
        <Typography variant="body2">{catalogItem?.name || NO_VALUE}</Typography>
      </MetaRow>
      {answers.length === 0 ? (
        <Typography variant="body2" color="text.secondary" sx={{ mt: 1.5 }}>
          No request details captured.
        </Typography>
      ) : (
        <Box
          component="dl"
          sx={{
            display: "grid",
            // Single column on narrow viewports so a long question and its
            // answer stack instead of squeezing; two columns from sm up.
            gridTemplateColumns: {
              xs: "minmax(0, 1fr)",
              sm: "minmax(0, 12rem) minmax(0, 1fr)",
            },
            columnGap: 1.5,
            rowGap: 0.75,
            alignItems: "baseline",
            m: 0,
            mt: 1.5,
            pt: 1.5,
            borderTop: 1,
            borderColor: "divider",
          }}
        >
          {answers.map((v, i) => (
            // The data source guarantees no id per answer and question text
            // is not guaranteed unique, so the index is part of the key.
            <Fragment key={`${v.name}-${i}`}>
              <Typography
                component="dt"
                variant="caption"
                color="text.secondary"
                sx={{ minWidth: 0, overflowWrap: "anywhere" }}
              >
                {v.name}
              </Typography>
              <Typography
                component="dd"
                variant="body2"
                color={v.value ? "text.primary" : "text.secondary"}
                sx={{
                  m: 0,
                  minWidth: 0,
                  // Long single-token answers (paths, ids, URLs) must wrap
                  // rather than push a horizontal scrollbar onto the page.
                  overflowWrap: "anywhere",
                  whiteSpace: "pre-wrap",
                }}
              >
                {v.value || NO_VALUE}
              </Typography>
            </Fragment>
          ))}
        </Box>
      )}
    </WidgetCard>
  );
}

// ---------------------------------------------------------------------------
// Re-export: silence "Activity" reference vs `Activity` rename in lucide.
// ---------------------------------------------------------------------------
