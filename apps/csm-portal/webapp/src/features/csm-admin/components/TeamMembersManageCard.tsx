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

import {
  Alert,
  Box,
  Button,
  Card,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Skeleton,
  TextField,
  Tooltip,
  Typography,
} from "@wso2/oxygen-ui";
import { useMemo, useState, type JSX } from "react";
import { useQueryClient } from "@tanstack/react-query";
import AsyncUserIdSingleSelect from "@features/csm-cases/components/AsyncUserIdSingleSelect";
import { INTERNAL_USER_ROLES } from "@features/csm-users/types/csmUsers";
import { useChangeRotaAccess } from "@features/csm-team-schedule/api/useRotaAccess";
import { useScheduleCatalogue } from "@features/csm-team-schedule/api/useTeamSchedule";
import { useGetPagingChain } from "@features/csm-team-schedule/api/usePagingChain";
import type { ScheduleTeam, ScheduleTeamMember } from "@features/csm-team-schedule/types";
import {
  invalidateTeamRoster,
  useMoveTeamMember,
  useUpdateTeamMember,
  type AlertTier,
  type EditableRole,
} from "@features/csm-admin/api/useManageTeamMember";

interface TeamMembersManageCardProps {
  /** The team's registry key (the `:id` of `/admin/teams/:id`), which is what
   * the team-schedule member routes take. Never the team's UUID. */
  teamKey: string;
  /** Admin only. Renders nothing at all for anyone else. */
  isAdmin: boolean;
}

/**
 * Admin-only "Manage members": add, remove, change role and alert tier, and
 * move a member to another team. All of it goes through the team-schedule
 * member routes, keyed by the team's registry key.
 *
 * This is a deliberate, documented exception to this app's "show the action,
 * let the backend reject it" default, by explicit product decision (same as
 * `TeamLeadCard`): the whole section and every write action is not rendered
 * for a non-admin, who keeps the read-only member list. The backend still
 * enforces it; this gate is not the authorization.
 *
 * Lead-type roles are never created or changed here, so a lead's role is
 * read-only and change/move are disabled for them. Remove stays available: a
 * CSM admin may remove a lead.
 */
export default function TeamMembersManageCard({ teamKey, isAdmin }: TeamMembersManageCardProps): JSX.Element | null {
  if (!isAdmin) return null;
  return <ManageBody teamKey={teamKey} />;
}

const EDITABLE_ROLES: EditableRole[] = ["engineer", "sub_lead"];
const TIERS: AlertTier[] = ["T1", "T2", "T3"];
const ROLE_LABEL: Record<string, string> = { engineer: "Engineer", member: "Engineer", sub_lead: "Sub lead" };
const KEEP = "keep";

/** A role this page may set; `member` is the stored spelling of engineer. */
function editableRoleOf(m: ScheduleTeamMember): EditableRole | null {
  if (m.isLead) return null;
  if (m.role === "engineer" || m.role === "member") return "engineer";
  if (m.role === "sub_lead") return "sub_lead";
  return null;
}

function tierOfRank(rank: number | undefined): AlertTier | "" {
  return rank === 1 ? "T1" : rank === 2 ? "T2" : rank === 3 ? "T3" : "";
}

function ManageBody({ teamKey }: { teamKey: string }): JSX.Element {
  const catalogue = useScheduleCatalogue();
  const team = catalogue.data?.teams.find((t) => t.key === teamKey);
  const chain = useGetPagingChain(team?.family ?? "", !!team);
  const qc = useQueryClient();
  const change = useChangeRotaAccess();
  const update = useUpdateTeamMember(teamKey);
  const move = useMoveTeamMember(teamKey);

  const [notice, setNotice] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [addUserId, setAddUserId] = useState("");
  const [removing, setRemoving] = useState<ScheduleTeamMember | null>(null);
  const [moving, setMoving] = useState<ScheduleTeamMember | null>(null);

  const tierByUser = useMemo(() => {
    const out = new Map<string, AlertTier | "">();
    for (const m of chain.data?.members ?? []) {
      if (m.teamKey === teamKey) out.set(m.userId, tierOfRank(m.responderRank));
    }
    return out;
  }, [chain.data, teamKey]);
  const tiersKnown = !!chain.data;

  const members = useMemo(
    () => [...(team?.members ?? [])].sort((a, b) => Number(b.isLead) - Number(a.isLead) || a.name.localeCompare(b.name)),
    [team],
  );
  const busy = change.isPending || update.isPending || move.isPending;

  const fail = (e: Error, fallback: string): void => {
    setNotice(null);
    setError(e.message || fallback);
  };
  const done = (msg: string): void => {
    setError(null);
    setNotice(msg);
  };

  const add = (): void => {
    if (!addUserId) return;
    change.mutate(
      { kind: "member", teamKey, userId: addUserId, grant: true },
      {
        onSuccess: () => {
          invalidateTeamRoster(qc);
          setAddUserId("");
          done("Member added as an engineer.");
        },
        onError: (e) => fail(e, "The member could not be added."),
      },
    );
  };

  const remove = (m: ScheduleTeamMember): void => {
    change.mutate(
      { kind: "member", teamKey, userId: m.userId, grant: false },
      {
        onSuccess: () => {
          invalidateTeamRoster(qc);
          setRemoving(null);
          done(`${m.name || m.email} was removed from the team.`);
        },
        onError: (e) => {
          setRemoving(null);
          fail(e, "The member could not be removed.");
        },
      },
    );
  };

  const save = (m: ScheduleTeamMember, role: EditableRole | undefined, tier: AlertTier | "" | undefined): void => {
    update.mutate(
      {
        userId: m.userId,
        ...(role ? { role } : {}),
        ...(tier !== undefined ? { alertTier: tier === "" ? null : tier } : {}),
      },
      {
        onSuccess: () => done(`${m.name || m.email} was updated.`),
        onError: (e) => fail(e, "The change could not be saved."),
      },
    );
  };

  const doMove = (m: ScheduleTeamMember, toTeam: ScheduleTeam): void => {
    move.mutate(
      { userId: m.userId, toTeamKey: toTeam.key },
      {
        onSuccess: () => {
          setMoving(null);
          done(`${m.name || m.email} was moved to ${toTeam.name}. Their alert tier was cleared.`);
        },
        onError: (e) => {
          setMoving(null);
          fail(e, "The member could not be moved.");
        },
      },
    );
  };

  const alreadyOnTeam = !!addUserId && members.some((m) => m.userId === addUserId);

  return (
    <Card variant="outlined" sx={{ p: 2 }} component="section" aria-labelledby="manage-members-title">
      <Typography id="manage-members-title" variant="subtitle2">
        Manage members
      </Typography>
      <Typography variant="caption" color="text.secondary">
        Add or remove people, change a member's role and alert tier, or move them to another team.
      </Typography>

      <Box sx={{ mt: 1.5, display: "flex", flexDirection: "column", gap: 1.5 }}>
        {error && (
          <Alert severity="error" onClose={() => setError(null)}>
            {error}
          </Alert>
        )}
        {notice && (
          <Alert severity="success" role="status" onClose={() => setNotice(null)}>
            {notice}
          </Alert>
        )}

        {catalogue.isLoading ? (
          <Skeleton variant="rounded" height={96} aria-label="Loading members" />
        ) : catalogue.isError ? (
          <Typography variant="body2" color="text.secondary">
            Couldn't load the team schedule's members.
          </Typography>
        ) : !team ? (
          <Typography variant="body2" color="text.secondary">
            This team is not on the team schedule, so its members can't be managed here.
          </Typography>
        ) : (
          <>
            <Box sx={{ display: "flex", gap: 1, alignItems: "flex-start" }}>
              <Box sx={{ flex: 1, maxWidth: 420 }}>
                <AsyncUserIdSingleSelect
                  id="manage-team-add-member"
                  label="Add a member"
                  placeholder="Search internal users…"
                  value={addUserId}
                  onChange={setAddUserId}
                  roleIds={INTERNAL_USER_ROLES}
                  active
                />
                {alreadyOnTeam && (
                  <Typography variant="caption" color="text.secondary">
                    Already on this team.
                  </Typography>
                )}
              </Box>
              <Button variant="contained" onClick={add} disabled={busy || !addUserId || alreadyOnTeam} sx={{ mt: 0.5 }}>
                Add member
              </Button>
            </Box>

            {members.length === 0 ? (
              <Typography variant="body2" color="text.secondary">
                No one is on {team.name} yet. Add someone above.
              </Typography>
            ) : (
              <Box component="ul" aria-label={`${team.name} members`} sx={{ listStyle: "none", m: 0, p: 0, display: "flex", flexDirection: "column", gap: 1 }}>
                {members.map((m) => (
                  <MemberRow
                    key={m.userId}
                    member={m}
                    teamName={team.name}
                    currentTier={tierByUser.get(m.userId) ?? ""}
                    tiersKnown={tiersKnown}
                    busy={busy}
                    onSave={save}
                    onRemove={() => setRemoving(m)}
                    onMove={() => setMoving(m)}
                  />
                ))}
              </Box>
            )}
          </>
        )}
      </Box>

      {removing && team && (
        <Dialog open onClose={busy ? undefined : () => setRemoving(null)} maxWidth="xs" fullWidth aria-labelledby="manage-remove-title">
          <DialogTitle id="manage-remove-title">Remove {removing.name || removing.email}?</DialogTitle>
          <DialogContent>
            <Typography variant="body2">
              {removing.name || removing.email} will be taken off {team.name}. This is refused while they have upcoming
              rota turns or hold an alert tier on the team.
              {removing.isLead ? " They are the team's lead." : ""}
            </Typography>
          </DialogContent>
          <DialogActions>
            <Button color="inherit" onClick={() => setRemoving(null)} disabled={busy}>
              Cancel
            </Button>
            <Button variant="contained" color="error" onClick={() => remove(removing)} disabled={busy}>
              {change.isPending ? "Removing…" : "Remove"}
            </Button>
          </DialogActions>
        </Dialog>
      )}

      {moving && team && (
        <MoveDialog
          member={moving}
          from={team}
          targets={(catalogue.data?.teams ?? []).filter((t) => t.key !== teamKey).sort((a, b) => a.name.localeCompare(b.name))}
          busy={busy}
          pending={move.isPending}
          onCancel={() => setMoving(null)}
          onMove={(to) => doMove(moving, to)}
        />
      )}
    </Card>
  );
}

function MemberRow({
  member,
  teamName,
  currentTier,
  tiersKnown,
  busy,
  onSave,
  onRemove,
  onMove,
}: {
  member: ScheduleTeamMember;
  teamName: string;
  currentTier: AlertTier | "";
  tiersKnown: boolean;
  busy: boolean;
  onSave: (m: ScheduleTeamMember, role: EditableRole | undefined, tier: AlertTier | "" | undefined) => void;
  onRemove: () => void;
  onMove: () => void;
}): JSX.Element {
  const who = member.name || member.email;
  const role = editableRoleOf(member);
  const locked = role === null;
  const initialRole: EditableRole = role ?? "engineer";
  // When the tiers could not be read, "unchanged" is the honest starting point.
  const initialTier: AlertTier | "" | typeof KEEP = tiersKnown ? currentTier : KEEP;
  const [roleDraft, setRoleDraft] = useState<EditableRole>(initialRole);
  const [tierDraft, setTierDraft] = useState<AlertTier | "" | typeof KEEP>(initialTier);
  // Re-seed when the server's value changes (after a refetch), adjusting
  // state during render rather than in an effect.
  const [seen, setSeen] = useState(`${initialRole}|${initialTier}`);
  if (seen !== `${initialRole}|${initialTier}`) {
    setSeen(`${initialRole}|${initialTier}`);
    setRoleDraft(initialRole);
    setTierDraft(initialTier);
  }

  const roleChanged = !locked && roleDraft !== initialRole;
  const tierChanged = !locked && tierDraft !== initialTier && tierDraft !== KEEP;
  const lockedWhy = "A lead's role can't be changed here, and a lead can't be moved.";

  const lockWrap = (el: JSX.Element): JSX.Element =>
    locked ? (
      <Tooltip title={lockedWhy}>
        <span>{el}</span>
      </Tooltip>
    ) : (
      el
    );

  return (
    <Box
      component="li"
      sx={{ display: "flex", flexWrap: "wrap", alignItems: "center", gap: 1.5, p: 1, border: 1, borderColor: "divider", borderRadius: 1 }}
    >
      <Box sx={{ flex: "1 1 180px", minWidth: 0 }}>
        <Typography variant="body2" noWrap>
          {who}
        </Typography>
        {member.name && member.email && (
          <Typography variant="caption" color="text.secondary" noWrap>
            {member.email}
          </Typography>
        )}
      </Box>

      {locked ? (
        <Typography variant="body2" sx={{ minWidth: 120 }}>
          Role: {ROLE_LABEL[member.role] ?? member.role.replace(/_/g, " ")} (lead)
        </Typography>
      ) : (
        <TextField
          select
          size="small"
          label="Role"
          value={roleDraft}
          onChange={(e) => setRoleDraft(e.target.value as EditableRole)}
          disabled={busy}
          slotProps={{ select: { native: true } }}
          sx={{ minWidth: 130 }}
          inputProps={{ "aria-label": `Role for ${who}` }}
        >
          {EDITABLE_ROLES.map((r) => (
            <option key={r} value={r}>
              {ROLE_LABEL[r]}
            </option>
          ))}
        </TextField>
      )}

      {locked ? (
        <Typography variant="body2" sx={{ minWidth: 110 }}>
          Alert tier: {tiersKnown ? currentTier || "none" : "unknown"}
        </Typography>
      ) : (
        <TextField
          select
          size="small"
          label="Alert tier"
          value={tierDraft}
          onChange={(e) => setTierDraft(e.target.value as AlertTier | "" | typeof KEEP)}
          disabled={busy}
          slotProps={{ select: { native: true } }}
          sx={{ minWidth: 130 }}
          inputProps={{ "aria-label": `Alert tier for ${who}` }}
          helperText={tiersKnown ? undefined : "Current tier unavailable"}
        >
          {!tiersKnown && <option value={KEEP}>Unchanged</option>}
          <option value="">None</option>
          {TIERS.map((t) => (
            <option key={t} value={t}>
              {t}
            </option>
          ))}
        </TextField>
      )}

      <Box sx={{ display: "flex", gap: 1 }}>
        {lockWrap(
          <Button
            size="small"
            variant="outlined"
            disabled={busy || locked || !(roleChanged || tierChanged)}
            aria-label={`Save changes for ${who}`}
            onClick={() =>
              onSave(member, roleChanged ? roleDraft : undefined, tierChanged ? (tierDraft as AlertTier | "") : undefined)
            }
          >
            Save
          </Button>,
        )}
        {lockWrap(
          <Button
            size="small"
            variant="outlined"
            disabled={busy || locked}
            aria-label={`Move ${who} to another team`}
            onClick={onMove}
          >
            Move
          </Button>,
        )}
        <Button
          size="small"
          variant="outlined"
          color="error"
          disabled={busy}
          aria-label={`Remove ${who} from ${teamName}`}
          onClick={onRemove}
        >
          Remove
        </Button>
      </Box>
    </Box>
  );
}

function MoveDialog({
  member,
  from,
  targets,
  busy,
  pending,
  onCancel,
  onMove,
}: {
  member: ScheduleTeamMember;
  from: ScheduleTeam;
  targets: ScheduleTeam[];
  busy: boolean;
  pending: boolean;
  onCancel: () => void;
  onMove: (to: ScheduleTeam) => void;
}): JSX.Element {
  const [toKey, setToKey] = useState("");
  const to = targets.find((t) => t.key === toKey);
  const who = member.name || member.email;
  return (
    <Dialog open onClose={busy ? undefined : onCancel} maxWidth="xs" fullWidth aria-labelledby="manage-move-title">
      <DialogTitle id="manage-move-title">Move {who}</DialogTitle>
      <DialogContent dividers>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2, pt: 0.5 }}>
          <Alert severity="warning">
            Moving {who} off {from.name} clears their alert tier and drops their rota manager grant on {from.name}. Their
            role is kept. It is refused while they have upcoming turns or an alert tier on {from.name}.
          </Alert>
          <TextField
            select
            size="small"
            label="Move to"
            value={toKey}
            onChange={(e) => setToKey(e.target.value)}
            slotProps={{ select: { native: true } }}
            disabled={busy}
            fullWidth
          >
            <option value="">Choose a team</option>
            {targets.map((t) => (
              <option key={t.key} value={t.key}>
                {t.name} ({t.family})
              </option>
            ))}
          </TextField>
        </Box>
      </DialogContent>
      <DialogActions>
        <Button color="inherit" onClick={onCancel} disabled={busy}>
          Cancel
        </Button>
        <Button variant="contained" color="warning" disabled={busy || !to} onClick={() => to && onMove(to)}>
          {pending ? "Moving…" : "Move member"}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
