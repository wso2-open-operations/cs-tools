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
  Alert,
  Avatar,
  Box,
  Button,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  IconButton,
  InputAdornment,
  Skeleton,
  TextField,
  ToggleButton,
  ToggleButtonGroup,
  Tooltip,
  Typography,
} from "@wso2/oxygen-ui";
import { Search, Users, X } from "@wso2/oxygen-ui-icons-react";
import { type JSX, useMemo, useState } from "react";
import { BackendApiError } from "@api/backend/client";
import { useDebouncedValue } from "@hooks/useDebouncedValue";
import { initialsOf } from "@utils/userClaims";
import { useSearchUsers } from "@features/csm-users/api/useSearchUsers";
import { INTERNAL_USER_ROLES } from "@features/csm-users/types/csmUsers";
import {
  useDeleteCustomDashboardShare,
  useGetCustomDashboardShares,
  usePutCustomDashboardShare,
} from "@features/csm-custom-dashboards/api/useDashboardShares";
import {
  useDeleteCustomDashboardTeamShare,
  useGetCustomDashboardTeamShares,
  usePutCustomDashboardTeamShare,
} from "@features/csm-custom-dashboards/api/useDashboardTeamShares";
import { useTeams } from "@features/csm-dashboard/api/useTeams";
import type { BeTeam } from "@api/backend/types";

interface ShareCustomDashboardDialogProps {
  open: boolean;
  /** The dashboard being shared: one of the caller's own. */
  dashboard: { id: string; name: string };
  /** The signed-in user's email, left out of the search results. */
  currentUserEmail?: string;
  onClose: () => void;
}

/**
 * What to tell the owner when a share or unshare is refused: the backend's
 * message for a refusal they can act on (4xx, e.g. not internal staff), as a
 * sentence; anything else is generic.
 */
function shareErrorMessage(err: unknown, fallback: string): string {
  if (err instanceof BackendApiError && err.status >= 400 && err.status < 500 && err.message) {
    const msg = err.message.charAt(0).toUpperCase() + err.message.slice(1);
    return /[.!?]$/.test(msg) ? msg : `${msg}.`;
  }
  return fallback;
}

type ShareMode = "people" | "teams";

/**
 * Team families in the order the Teams list shows them, with their labels.
 * Each team is its own target: inside CRE ABT, Rigel and Castor are shared
 * with separately.
 */
const TEAM_FAMILIES: { family: string; label: string }[] = [
  { family: "cre-abt", label: "CRE ABT" },
  { family: "cre", label: "CRE" },
  { family: "sre-abt", label: "SRE ABT" },
  { family: "sre", label: "SRE" },
];

/** The readable label of a team family, e.g. "CRE ABT"; "Team" when unknown. */
function familyLabel(family: string | undefined): string {
  return TEAM_FAMILIES.find((f) => f.family === family)?.label ?? "Team";
}

/**
 * The teams that can be chosen: registry teams (they have a family) with a
 * group configured, by family in TEAM_FAMILIES order, then by name. The
 * backend refuses any other team.
 */
function groupShareableTeams(teams: BeTeam[], query: string, sharedKeys: Set<string>): { label: string; teams: BeTeam[] }[] {
  const q = query.toLowerCase();
  const shareable = teams.filter(
    (t) =>
      !!t.family &&
      (!!t.creGroupId || !!t.sreGroupId) &&
      !sharedKeys.has(t.id) &&
      (q === "" || t.name.toLowerCase().includes(q)),
  );
  return TEAM_FAMILIES.map(({ family, label }) => ({
    label,
    teams: shareable.filter((t) => t.family === family).sort((a, b) => a.name.localeCompare(b.name)),
  })).filter((g) => g.teams.length > 0);
}

/**
 * Shares one of the caller's saved dashboards, view only, with people or
 * teams (a People / Teams toggle above the search). People: a search over
 * internal staff (the Assign engineer dialog's search: `POST /users/search`,
 * internal roles, active only), where clicking a person shares with them;
 * below it, who it is shared with, each with a remove button. The caller and
 * people it is already shared with are left out of the results. Teams: the
 * registry teams grouped by family (CRE ABT, CRE, SRE ABT, SRE), filtered by
 * the same box; clicking one shares with it, and whoever is in the team sees
 * the dashboard. "Shared with" lists teams first, then people. A refusal
 * shows in the dialog; the backend is the real check.
 */
export default function ShareCustomDashboardDialog({
  open,
  dashboard,
  currentUserEmail,
  onClose,
}: ShareCustomDashboardDialogProps): JSX.Element {
  const [mode, setMode] = useState<ShareMode>("people");
  const [input, setInput] = useState("");
  const [error, setError] = useState<string | null>(null);
  const shares = useGetCustomDashboardShares(open ? dashboard.id : undefined);
  const share = usePutCustomDashboardShare();
  const unshare = useDeleteCustomDashboardShare();
  const teamShares = useGetCustomDashboardTeamShares(open ? dashboard.id : undefined);
  const shareTeam = usePutCustomDashboardTeamShare();
  const unshareTeam = useDeleteCustomDashboardTeamShare();
  const teams = useTeams(open && mode === "teams");

  const search = useDebouncedValue(input.trim(), 300);
  const results = useSearchUsers(
    {
      filters: {
        ...(search.length > 0 && { searchQuery: search }),
        roleIds: INTERNAL_USER_ROLES,
        active: true,
      },
      pagination: { limit: 8, offset: 0 },
    },
    open && mode === "people",
  );

  const sharedIds = useMemo(() => new Set((shares.data ?? []).map((s) => s.user.id)), [shares.data]);
  const me = currentUserEmail?.toLowerCase();
  const people = useMemo(
    () =>
      (results.data?.users ?? []).filter(
        (u) =>
          !!u.email &&
          u.active !== false &&
          (u.userType ? u.userType === "internal" : true) &&
          u.email.toLowerCase() !== me &&
          !sharedIds.has(u.id),
      ),
    [results.data, me, sharedIds],
  );

  const sharedTeamKeys = useMemo(
    () => new Set((teamShares.data ?? []).map((s) => s.team.key).filter((k) => k !== "")),
    [teamShares.data],
  );
  const teamGroups = useMemo(
    () => groupShareableTeams(teams.data ?? [], input.trim(), sharedTeamKeys),
    [teams.data, input, sharedTeamKeys],
  );

  const busy = share.isPending || unshare.isPending || shareTeam.isPending || unshareTeam.isPending;

  const handleShare = async (userId: string): Promise<void> => {
    setError(null);
    try {
      await share.mutateAsync({ dashboardId: dashboard.id, userId });
    } catch (err) {
      setError(shareErrorMessage(err, "The dashboard couldn't be shared. Try again in a moment."));
    }
  };

  const handleUnshare = async (userId: string): Promise<void> => {
    setError(null);
    try {
      await unshare.mutateAsync({ dashboardId: dashboard.id, userId });
    } catch (err) {
      setError(shareErrorMessage(err, "The dashboard couldn't be unshared. Try again in a moment."));
    }
  };

  const handleShareTeam = async (teamKey: string): Promise<void> => {
    setError(null);
    try {
      await shareTeam.mutateAsync({ dashboardId: dashboard.id, teamKey });
    } catch (err) {
      setError(shareErrorMessage(err, "The dashboard couldn't be shared with the team. Try again in a moment."));
    }
  };

  const handleUnshareTeam = async (teamKey: string): Promise<void> => {
    setError(null);
    try {
      await unshareTeam.mutateAsync({ dashboardId: dashboard.id, teamKey });
    } catch (err) {
      setError(shareErrorMessage(err, "The dashboard couldn't be unshared from the team. Try again in a moment."));
    }
  };

  const handleClose = (): void => {
    setMode("people");
    setInput("");
    setError(null);
    onClose();
  };

  const sharedPeople = shares.data ?? [];
  const sharedTeams = teamShares.data ?? [];
  // The two lists load separately: when only one fails, the other stays on
  // screen (and its shares can still be removed).
  const peopleFailed = shares.isError || shares.data === null;
  const teamsFailed = teamShares.isError || teamShares.data === null;

  return (
    <Dialog open={open} onClose={handleClose} maxWidth="xs" fullWidth>
      <DialogTitle>Share &ldquo;{dashboard.name}&rdquo;</DialogTitle>
      <DialogContent dividers>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
          <Typography variant="body2" color="text.secondary">
            People and teams you share with can view this dashboard in their Shared Dashboards and see your later
            changes. They can&apos;t edit it. A team&apos;s members see it while they are in the team.
          </Typography>

          <ToggleButtonGroup
            exclusive
            size="small"
            color="primary"
            value={mode}
            aria-label="Share with"
            onChange={(_, next: ShareMode | null) => {
              // null when the selected button is pressed again: keep a mode chosen.
              if (next) {
                setMode(next);
                setInput("");
              }
            }}
          >
            <ToggleButton value="people">People</ToggleButton>
            <ToggleButton value="teams">Teams</ToggleButton>
          </ToggleButtonGroup>

          {error && (
            <Alert severity="error" onClose={() => setError(null)}>
              {error}
            </Alert>
          )}

          <TextField
            value={input}
            onChange={(e) => setInput(e.target.value)}
            placeholder={mode === "people" ? "Search people by name or email…" : "Filter teams by name…"}
            size="small"
            fullWidth
            autoFocus
            slotProps={{
              htmlInput: { "aria-label": mode === "people" ? "Search people" : "Filter teams" },
              input: {
                startAdornment: (
                  <InputAdornment position="start">
                    <Search size={16} />
                  </InputAdornment>
                ),
              },
            }}
          />

          {mode === "teams" ? (
            <Box sx={{ minHeight: 120, maxHeight: 280, overflowY: "auto" }} aria-label="Teams to share with">
              {teams.isLoading ? (
                <Box sx={{ display: "flex", flexDirection: "column", gap: 0.5, pt: 0.5 }}>
                  {[0, 1, 2].map((i) => (
                    <Skeleton key={i} variant="rounded" height={32} />
                  ))}
                </Box>
              ) : teams.isError ? (
                <Typography variant="body2" color="error" sx={{ py: 1 }}>
                  Could not load the teams. Try again.
                </Typography>
              ) : teamGroups.length === 0 ? (
                <Typography variant="body2" color="text.secondary" sx={{ py: 1 }}>
                  No matching teams.
                </Typography>
              ) : (
                teamGroups.map((group) => (
                  <Box key={group.label} role="group" aria-label={group.label} sx={{ mb: 0.5 }}>
                    <Typography variant="overline" color="text.secondary" sx={{ px: 1, display: "block", lineHeight: 2 }}>
                      {group.label}
                    </Typography>
                    {group.teams.map((t) => (
                      <Button
                        key={t.id}
                        variant="text"
                        color="inherit"
                        fullWidth
                        disabled={busy}
                        onClick={() => void handleShareTeam(t.id)}
                        aria-label={`Share with team ${t.name}`}
                        sx={{ justifyContent: "flex-start", textTransform: "none", px: 1, py: 0.5, gap: 1.25 }}
                      >
                        <Avatar sx={{ width: 28, height: 28 }}>
                          <Users size={14} />
                        </Avatar>
                        <Typography variant="body2" noWrap>
                          {t.name}
                        </Typography>
                      </Button>
                    ))}
                  </Box>
                ))
              )}
            </Box>
          ) : (
            <Box sx={{ minHeight: 120 }} aria-label="People to share with">
              {results.isFetching ? (
                <Box sx={{ display: "flex", flexDirection: "column", gap: 0.5, pt: 0.5 }}>
                  {[0, 1, 2].map((i) => (
                    <Skeleton key={i} variant="rounded" height={36} />
                  ))}
                </Box>
              ) : results.isError ? (
                <Typography variant="body2" color="error" sx={{ py: 1 }}>
                  Could not search people. Try again.
                </Typography>
              ) : people.length === 0 ? (
                <Typography variant="body2" color="text.secondary" sx={{ py: 1 }}>
                  No matching people.
                </Typography>
              ) : (
                <Box sx={{ display: "flex", flexDirection: "column" }}>
                  {people.map((u) => {
                    const name = u.name.trim() || u.userName;
                    return (
                      <Button
                        key={u.id}
                        variant="text"
                        color="inherit"
                        disabled={busy}
                        onClick={() => void handleShare(u.id)}
                        aria-label={`Share with ${name}`}
                        sx={{ justifyContent: "flex-start", textTransform: "none", px: 1, py: 0.75, gap: 1.25 }}
                      >
                        <Avatar sx={{ width: 28, height: 28, fontSize: "0.75rem" }}>{initialsOf(name)}</Avatar>
                        <Box sx={{ minWidth: 0, textAlign: "left" }}>
                          <Typography variant="body2" sx={{ lineHeight: 1.2 }} noWrap>
                            {name}
                          </Typography>
                          <Typography variant="caption" color="text.secondary" noWrap sx={{ display: "block" }}>
                            {u.email}
                          </Typography>
                        </Box>
                      </Button>
                    );
                  })}
                </Box>
              )}
            </Box>
          )}

          <Typography variant="subtitle2">Shared with</Typography>
          <Box aria-label="Shared with">
            {shares.isLoading || teamShares.isLoading ? (
              <Skeleton variant="rounded" height={36} />
            ) : peopleFailed && teamsFailed ? (
              <Typography variant="body2" color="error">
                Could not load who this dashboard is shared with.
              </Typography>
            ) : !peopleFailed && !teamsFailed && sharedPeople.length === 0 && sharedTeams.length === 0 ? (
              <Typography variant="body2" color="text.secondary">
                Not shared with anyone yet.
              </Typography>
            ) : (
              <>
                {teamsFailed && (
                  <Typography variant="body2" color="error" sx={{ px: 1, py: 0.5 }}>
                    Could not load the teams this dashboard is shared with.
                  </Typography>
                )}
                {sharedTeams.map((s) => (
                  <Box key={s.team.key || s.team.name} sx={{ display: "flex", alignItems: "center", gap: 1.25, px: 1, py: 0.5 }}>
                    <Avatar sx={{ width: 28, height: 28 }}>
                      <Users size={14} />
                    </Avatar>
                    <Box sx={{ minWidth: 0, flex: 1 }}>
                      <Typography variant="body2" sx={{ lineHeight: 1.2 }} noWrap>
                        {s.team.name}
                      </Typography>
                      <Typography variant="caption" color="text.secondary" noWrap sx={{ display: "block" }}>
                        Team · {familyLabel(s.team.family)}
                      </Typography>
                    </Box>
                    {s.team.key && (
                      <Tooltip title="Stop sharing">
                        <span>
                          <IconButton
                            size="small"
                            disabled={busy}
                            aria-label={`Stop sharing with team ${s.team.name}`}
                            onClick={() => void handleUnshareTeam(s.team.key)}
                          >
                            {unshareTeam.isPending && unshareTeam.variables?.teamKey === s.team.key ? (
                              <CircularProgress size={14} color="inherit" />
                            ) : (
                              <X size={16} />
                            )}
                          </IconButton>
                        </span>
                      </Tooltip>
                    )}
                  </Box>
                ))}
                {peopleFailed && (
                  <Typography variant="body2" color="error" sx={{ px: 1, py: 0.5 }}>
                    Could not load the people this dashboard is shared with.
                  </Typography>
                )}
                {sharedPeople.map((s) => {
                  const name = s.user.name || s.user.email;
                  return (
                    <Box key={s.user.id ?? s.user.email} sx={{ display: "flex", alignItems: "center", gap: 1.25, px: 1, py: 0.5 }}>
                      <Avatar sx={{ width: 28, height: 28, fontSize: "0.75rem" }}>{initialsOf(name)}</Avatar>
                      <Box sx={{ minWidth: 0, flex: 1 }}>
                        <Typography variant="body2" sx={{ lineHeight: 1.2 }} noWrap>
                          {name}
                        </Typography>
                        <Typography variant="caption" color="text.secondary" noWrap sx={{ display: "block" }}>
                          {s.user.email}
                        </Typography>
                      </Box>
                      {s.user.id && (
                        <Tooltip title="Stop sharing">
                          <span>
                            <IconButton
                              size="small"
                              disabled={busy}
                              aria-label={`Stop sharing with ${name}`}
                              onClick={() => void handleUnshare(s.user.id as string)}
                            >
                              {unshare.isPending && unshare.variables?.userId === s.user.id ? (
                                <CircularProgress size={14} color="inherit" />
                              ) : (
                                <X size={16} />
                              )}
                            </IconButton>
                          </span>
                        </Tooltip>
                      )}
                    </Box>
                  );
                })}
              </>
            )}
          </Box>
        </Box>
      </DialogContent>
      <DialogActions>
        <Button onClick={handleClose}>Done</Button>
      </DialogActions>
    </Dialog>
  );
}
