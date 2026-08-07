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
  Box,
  Button,
  Card,
  Chip,
  Skeleton,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Typography,
} from "@wso2/oxygen-ui";
import { ArrowLeft } from "@wso2/oxygen-ui-icons-react";
import { useMemo, type JSX, type ReactNode } from "react";
import { Link as RouterLink, useLocation, useParams } from "react-router";
import QueryErrorState from "@components/QueryErrorState";
import { useNavTransition } from "@hooks/useNavTransition";
import { formatBackendTimestampForDisplay } from "@utils/dateTime";
import { useGetUserById } from "@features/csm-users/api/useGetUserById";
import { useSearchRoles } from "@features/csm-admin/api/useSearchRoles";
import DirectoryEntityChip from "@features/csm-admin/components/DirectoryEntityChip";
import { BE_MAX_PAGE_LIMIT } from "@constants/apiConstants";
import {
  INTERNAL_USER_ROLES,
  type NormalizedUserDetail,
  type UserProjectAccess,
} from "@features/csm-users/types/csmUsers";

function formatDateTime(value?: string | null): string {
  return (
    formatBackendTimestampForDisplay(value, {
      dateStyle: "medium",
      timeStyle: "short",
    }) ?? "—"
  );
}

function BackButton({ onClick }: { onClick: () => void }): JSX.Element {
  return (
    <Button
      variant="text"
      size="small"
      startIcon={<ArrowLeft size={16} />}
      onClick={onClick}
      sx={{ alignSelf: "flex-start" }}
    >
      Back
    </Button>
  );
}

function MetaCell({
  label,
  children,
}: {
  label: string;
  children: ReactNode;
}): JSX.Element {
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

/**
 * True when `user` is internal (WSO2 staff): either a direct
 * `userType === "internal"`, or (on a backend response predating `userType`)
 * `roles` containing one of {@link INTERNAL_USER_ROLES}. Every other
 * `userType` value (`external`, `customer`, `system`) is treated alike as
 * "not internal" — the two data sources don't agree on the exact label, so
 * branching on `=== "external"` alone would silently miss the postgres
 * source's `customer`/`system` users.
 */
function isInternalUser(user: NormalizedUserDetail): boolean {
  if (user.userType) return user.userType === "internal";
  return (user.roles ?? []).some((r) =>
    (INTERNAL_USER_ROLES as string[]).includes(r),
  );
}

type ChipColor = "success" | "warning" | "error" | "default";

interface ProjectAccessStatus {
  label: string;
  color: ChipColor;
  reason?: string;
}

/**
 * A project row's access status, folding case-access and registration state
 * into the single "Access" column a support engineer scans: a project with no
 * linked contact record is the fundamental failure ("No access", with the
 * reason called out inline); one that's linked but hasn't completed
 * registration yet reads "Invited" rather than "Has access", since the
 * person hasn't actually logged in to see it.
 */
function deriveProjectAccessStatus(pa: UserProjectAccess): ProjectAccessStatus {
  if (!pa.grantsCaseAccess) {
    return {
      label: "No access",
      color: "error",
      reason: "No contact record is linked to this project for this user.",
    };
  }
  if ((pa.registrationState ?? "").toLowerCase() === "invited") {
    return { label: "Invited", color: "warning" };
  }
  return { label: "Has access", color: "success" };
}

/** One row of {@link MembershipRow} rendered as a chip cluster: a role, group,
 * or team the profile's user belongs to, plus enough to link it to its
 * directory page (see `DirectoryEntityChip`). */
interface MembershipRow {
  key: string;
  id: string;
  label: string;
  routeBase: string;
  color?: "default" | "primary";
}

/**
 * An inline, wrapping cluster of membership chips — deliberately not a
 * bordered table-in-a-card: each chip already reads as a distinct item, so a
 * per-row box around it added visual weight without adding information.
 * Rendered even when `rows` is empty — "no memberships" is itself an answer
 * worth showing rather than hiding the section.
 */
function ChipCluster({
  rows,
  emptyMessage,
}: {
  rows: MembershipRow[];
  emptyMessage: string;
}): JSX.Element {
  if (rows.length === 0) {
    return (
      <Typography variant="body2" color="text.secondary">
        {emptyMessage}
      </Typography>
    );
  }
  return (
    <Box sx={{ display: "flex", flexWrap: "wrap", gap: 1 }}>
      {rows.map((row) => (
        <DirectoryEntityChip
          key={row.key}
          id={row.id}
          name={row.label}
          routeBase={row.routeBase}
          color={row.color}
        />
      ))}
    </Box>
  );
}

/** This user's team chips, rendered inline in the Overview card — most users
 * belong to at most one team, so a full card of its own was mostly dead
 * space; here it reads as a normal profile attribute. */
function TeamMetaCell({ user }: { user: NormalizedUserDetail }): JSX.Element {
  const rows: MembershipRow[] = (user.teams ?? []).map((t) => ({
    key: t.id,
    id: t.id,
    label: t.family ? `${t.name} (${t.family})` : t.name,
    routeBase: "/admin/teams",
    color: "primary",
  }));
  return (
    <MetaCell label="Team">
      <ChipCluster rows={rows} emptyMessage="Unassigned" />
    </MetaCell>
  );
}

/**
 * Roles and (for internal users) groups as two side-by-side chip clusters in
 * one card, rather than three separate cards — a user rarely has enough
 * groups to justify a card of its own, and putting roles and groups next to
 * each other reads as "what can this person do" at a glance.
 */
function PermissionsCard({ user }: { user: NormalizedUserDetail }): JSX.Element {
  const { data: rolesData } = useSearchRoles({ pagination: { limit: BE_MAX_PAGE_LIMIT } });
  const roleNameById = useMemo(
    () => new Map((rolesData?.roles ?? []).map((r) => [r.id, r.name])),
    [rolesData],
  );

  const roleRows: MembershipRow[] = (user.roles ?? []).map((r) => ({
    key: r,
    id: r,
    label: roleNameById.get(r) ?? r,
    routeBase: "/admin/roles",
    color: (INTERNAL_USER_ROLES as string[]).includes(r) ? "primary" : "default",
  }));

  const internal = isInternalUser(user);
  const groupRows: MembershipRow[] = (user.groups ?? []).map((g) => ({
    key: g.id,
    id: g.id,
    label: g.name,
    routeBase: "/admin/groups",
  }));

  return (
    <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2 }}>
      <Typography variant="subtitle2">Permissions & assignments</Typography>
      <Box sx={{ display: "flex", flexWrap: "wrap", gap: 3 }}>
        <Box sx={{ flex: "1 1 260px", minWidth: 220, display: "flex", flexDirection: "column", gap: 1 }}>
          <Typography variant="body2" sx={{ fontWeight: 600 }}>
            Platform roles ({roleRows.length})
          </Typography>
          <ChipCluster rows={roleRows} emptyMessage="No roles assigned." />
        </Box>
        {internal && (
          <Box sx={{ flex: "1 1 260px", minWidth: 220, display: "flex", flexDirection: "column", gap: 1 }}>
            <Typography variant="body2" sx={{ fontWeight: 600 }}>
              User groups ({groupRows.length})
            </Typography>
            <ChipCluster rows={groupRows} emptyMessage="No group memberships." />
          </Box>
        )}
      </Box>
    </Card>
  );
}

/**
 * An external user's per-project access, as a data table matching the
 * platform's other project tables (see `ProjectContactsTab`): project name
 * linked to its detail page, the project's short key, the roles this contact
 * carries on it, and a single Access column that's the verdict a support
 * engineer is looking for — with the reason called out inline whenever a
 * project doesn't grant case access.
 */
function AccessibleProjectsCard({ user }: { user: NormalizedUserDetail }): JSX.Element {
  const access = user.projectAccess ?? [];
  const blockedCount = access.filter((pa) => !pa.grantsCaseAccess).length;

  return (
    <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2 }}>
      <Typography variant="subtitle2">Accessible projects</Typography>

      {user.active === false && (
        <Alert severity="error" variant="outlined">
          This user's account is inactive — they can't access any project's cases,
          regardless of the per-project rows below.
        </Alert>
      )}

      {access.length === 0 ? (
        <Typography variant="body2" color="text.secondary">
          No project access records found for this user.
        </Typography>
      ) : (
        <>
          {blockedCount > 0 && (
            <Alert severity="warning" variant="outlined">
              Blocked on {blockedCount} of {access.length} project
              {access.length === 1 ? "" : "s"} — see the reason under each row below.
            </Alert>
          )}
          <TableContainer sx={{ border: 1, borderColor: "divider", borderRadius: 1 }}>
            <Table size="small">
              <TableHead>
                <TableRow>
                  <TableCell>Project name</TableCell>
                  <TableCell>Project key</TableCell>
                  <TableCell>Project roles</TableCell>
                  <TableCell>Access</TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {access.map((pa) => {
                  const status = deriveProjectAccessStatus(pa);
                  return (
                    <TableRow key={pa.projectId} hover>
                      <TableCell>
                        <Typography
                          component={RouterLink}
                          to={`/customers/projects/${pa.projectId}`}
                          variant="body2"
                          sx={(t) => ({
                            fontWeight: 600,
                            textDecoration: "none",
                            color: t.palette.primary.dark,
                            ...t.applyStyles("dark", { color: t.palette.primary.main }),
                            "&:hover": { textDecoration: "underline" },
                          })}
                        >
                          {pa.projectName}
                        </Typography>
                      </TableCell>
                      <TableCell>
                        {pa.projectKey ? (
                          <Typography
                            component="code"
                            variant="caption"
                            color="text.secondary"
                            sx={{
                              px: 0.75,
                              py: 0.25,
                              bgcolor: "action.hover",
                              borderRadius: 0.5,
                            }}
                          >
                            {pa.projectKey}
                          </Typography>
                        ) : (
                          "—"
                        )}
                      </TableCell>
                      <TableCell>
                        {pa.roles && pa.roles.length > 0
                          ? pa.roles.map((r) => (
                              <Chip
                                key={r}
                                size="small"
                                label={r}
                                variant="outlined"
                                sx={{ mr: 0.5, mb: 0.5 }}
                              />
                            ))
                          : "—"}
                      </TableCell>
                      <TableCell>
                        <Chip size="small" label={status.label} color={status.color} variant="outlined" />
                        {status.reason && (
                          <Typography
                            variant="caption"
                            color="error.main"
                            component="div"
                            sx={{ mt: 0.25 }}
                          >
                            {status.reason}
                          </Typography>
                        )}
                      </TableCell>
                    </TableRow>
                  );
                })}
              </TableBody>
            </Table>
          </TableContainer>
        </>
      )}
    </Card>
  );
}

/**
 * A person's profile page, reachable by clicking any user reference in the
 * portal (case creator, assignee, watchers, comment authors, attachment
 * uploaders) once its id is known or resolved (see `UserRefLink` /
 * `useResolvedUserId` — most actor fields carry only an email, resolved to an
 * id through a cached lookup before the link ever appears).
 *
 * Renders everything `GET /users/{id}` returns: name, email, timezone, phone,
 * team (internal users only), roles, created/updated times, plus — split by
 * `userType` — an internal user's group memberships or an external user's
 * per-project access (with the reason surfaced whenever a project doesn't
 * grant case access, per `UserProjectAccess.grantsCaseAccess`).
 */
export default function UserProfilePage(): JSX.Element {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavTransition();
  // This page is reachable from arbitrary contexts (any user reference —
  // case creator/assignee, comment author, dashboard widget row), so there's
  // no single canonical "list" to fall back to the way other detail pages
  // have. Prefer the URL the row link captured (if any) so "back" returns to
  // the exact view the engineer came from; browser history otherwise, same
  // as before this carried no `from` state at all. `parentState` is
  // whatever state that captured list page was itself carrying (e.g.
  // `{ from: "/dashboard" }`) — forwarded back onto it below so a
  // dashboard → list → here → Back round trip restores the list's own Back
  // button instead of silently dropping it. Ignored (harmlessly) when
  // `backTarget` falls back to the numeric `-1` history pop.
  const backState = useLocation().state as
    | { from?: string; parentState?: unknown }
    | undefined;
  const backTarget = backState?.from ?? -1;
  const backNavState = backState?.parentState ?? undefined;

  const { data: user, isLoading, isError, error } = useGetUserById(id);

  if (isLoading) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
        <Skeleton variant="rounded" height={32} width={240} />
        <Skeleton variant="rounded" height={220} />
      </Box>
    );
  }

  if (isError) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        <BackButton onClick={() => navigate(backTarget, { state: backNavState })} />
        <QueryErrorState
          message={error instanceof Error && error.message.trim() ? error.message : "Failed to load user."}
          error={error}
        />
      </Box>
    );
  }

  if (!user) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        <BackButton onClick={() => navigate(backTarget, { state: backNavState })} />
        <Typography variant="h5">User not found</Typography>
        <Typography variant="body2" color="text.secondary">
          No user with id <code>{id}</code>.
        </Typography>
      </Box>
    );
  }

  const internal = isInternalUser(user);

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2.5 }}>
      <BackButton onClick={() => navigate(backTarget, { state: backNavState })} />

      <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
        <Box sx={{ display: "flex", alignItems: "center", gap: 1.5, flexWrap: "wrap" }}>
          <Typography variant="h5">{user.name || user.userName || "—"}</Typography>
          <Chip
            size="small"
            label={internal ? "Internal" : "Customer"}
            color={internal ? "primary" : "default"}
            variant="outlined"
          />
          {user.active !== undefined && (
            <Chip
              size="small"
              label={user.active ? "Active" : "Inactive"}
              color={user.active ? "success" : "default"}
              variant="outlined"
            />
          )}
        </Box>
        <Typography variant="body2" color="text.secondary">
          {user.email}
        </Typography>
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
          <MetaCell label="Username">
            <Typography variant="body2">{user.userName}</Typography>
          </MetaCell>
          <MetaCell label="Email">
            <Typography variant="body2" sx={{ wordBreak: "break-all" }}>
              {user.email}
            </Typography>
          </MetaCell>
          {internal && <TeamMetaCell user={user} />}
          <MetaCell label="Timezone">
            <Typography variant="body2">{user.timezone ?? "Not set"}</Typography>
          </MetaCell>
          {internal && (
            <MetaCell label="Phone">
              <Typography variant="body2">{user.phone ?? "Not set"}</Typography>
            </MetaCell>
          )}
          <MetaCell label="Created on">
            <Typography variant="body2">{formatDateTime(user.createdOn)}</Typography>
          </MetaCell>
          <MetaCell label="Updated on">
            <Typography variant="body2">{formatDateTime(user.updatedOn)}</Typography>
          </MetaCell>
        </Box>
      </Card>

      <PermissionsCard user={user} />

      {!internal && <AccessibleProjectsCard user={user} />}
    </Box>
  );
}
