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
import { type JSX, type ReactNode } from "react";
import { Link as RouterLink, useLocation, useParams } from "react-router";
import { useAccountProjects } from "@features/csm-accounts/api/useAccountProjects";
import { useGetAccount } from "@features/csm-accounts/api/useGetAccount";
import {
  getDeactivationState,
  resolveAccountTier,
} from "@features/csm-accounts/types/csmAccounts";
import DirectoryEntityChip from "@features/csm-admin/components/DirectoryEntityChip";
import QueryErrorState from "@components/QueryErrorState";
import { useNavTransition } from "@hooks/useNavTransition";

function formatSubscriptionType(value: string): string {
  return value.replace(/_/g, " ");
}

function formatDate(value?: string | null): string {
  if (!value) return "—";
  const d = new Date(value);
  return Number.isNaN(d.getTime())
    ? value
    : d.toLocaleDateString("en-US", {
        year: "numeric",
        month: "short",
        day: "numeric",
      });
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

function Mono({ children }: { children: ReactNode }): JSX.Element {
  return (
    <Typography variant="body2" sx={{ fontFamily: "monospace", wordBreak: "break-all" }}>
      {children}
    </Typography>
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

/**
 * Projects belonging to this account, server-side filtered via
 * `POST /projects/search`'s `accountId` filter (see `useAccountProjects`).
 */
function ProjectsSection({ accountId }: { accountId: string }): JSX.Element {
  const { data, isLoading, isError, error } = useAccountProjects(accountId);
  const projects = data?.projects ?? [];

  return (
    <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2 }}>
      <Typography variant="subtitle2">Projects</Typography>
      <TableContainer sx={{ border: 1, borderColor: "divider", borderRadius: 1 }}>
        <Table size="small" sx={{ "& .MuiTableCell-root": { borderColor: "divider" } }}>
          <TableHead>
            <TableRow sx={{ bgcolor: "action.hover" }}>
              <TableCell>Name</TableCell>
              <TableCell>Project key</TableCell>
              <TableCell>Subscription</TableCell>
              <TableCell>End date</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {isLoading ? (
              Array.from({ length: 3 }).map((_, i) => (
                <TableRow key={i}>
                  <TableCell><Skeleton variant="rounded" width="70%" height={18} /></TableCell>
                  <TableCell><Skeleton variant="rounded" width="50%" height={18} /></TableCell>
                  <TableCell><Skeleton variant="rounded" width={90} height={22} /></TableCell>
                  <TableCell><Skeleton variant="rounded" width={80} height={18} /></TableCell>
                </TableRow>
              ))
            ) : isError ? (
              <TableRow>
                <TableCell colSpan={4} align="center">
                  <QueryErrorState
                    message={error instanceof Error && error.message.trim() ? error.message : "Failed to load projects."}
                    error={error}
                  />
                </TableCell>
              </TableRow>
            ) : projects.length === 0 ? (
              <TableRow>
                <TableCell colSpan={4} align="center" sx={{ py: 4 }}>
                  <Typography variant="body2" color="text.secondary">
                    No projects found for this account.
                  </Typography>
                </TableCell>
              </TableRow>
            ) : (
              projects.map((p) => (
                <TableRow key={p.id} hover>
                  <TableCell sx={{ maxWidth: 320 }}>
                    <Typography
                      component={RouterLink}
                      to={`/customers/projects/${p.id}`}
                      variant="body2"
                      noWrap
                      title={p.name}
                      sx={(t) => ({
                        display: "block",
                        textDecoration: "none",
                        color: t.palette.primary.dark,
                        ...t.applyStyles("dark", { color: t.palette.primary.main }),
                        "&:hover": { textDecoration: "underline" },
                      })}
                    >
                      {p.name}
                    </Typography>
                  </TableCell>
                  <TableCell>{p.key}</TableCell>
                  <TableCell>
                    <Chip
                      size="small"
                      label={formatSubscriptionType(p.subscriptionType)}
                      variant="outlined"
                    />
                  </TableCell>
                  <TableCell>{formatDate(p.endDate)}</TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </TableContainer>
    </Card>
  );
}

export default function CsmAccountDetailPage(): JSX.Element {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavTransition();
  const location = useLocation();
  // Prefer wherever the caller came from (e.g. a case's Overview panel) over
  // the hardcoded accounts list, so Back returns to that page instead of
  // skipping past it — same convention as CsmCaseDetailPage's own back path.
  const fromListState = location.state as { from?: string } | undefined;
  const resolvedBackPath = fromListState?.from ?? "/customers/accounts";
  const { data, isLoading, isError } = useGetAccount(id);

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
        <BackButton onClick={() => navigate(resolvedBackPath)} />
        <Typography variant="body1" color="error">
          Could not load account {id}.
        </Typography>
      </Box>
    );
  }

  if (!data) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        <BackButton onClick={() => navigate(resolvedBackPath)} />
        <Typography variant="h5">Account not found</Typography>
        <Typography variant="body2" color="text.secondary">
          No account with id <code>{id}</code>.
        </Typography>
      </Box>
    );
  }

  const a = data;
  const tier = resolveAccountTier(a);
  const deactivationState = getDeactivationState(a.deactivationDate);

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2.5 }}>
      <BackButton onClick={() => navigate(resolvedBackPath)} />

      <Box sx={{ display: "flex", alignItems: "center", gap: 1.5, flexWrap: "wrap" }}>
        <Typography variant="h5">{a.name}</Typography>
        {tier && (
          <Chip
            size="small"
            label={tier}
            color={tier === "enterprise" ? "primary" : "default"}
            variant="outlined"
          />
        )}
        {deactivationState === "past" && (
          <Chip size="small" label="Deactivated" color="default" variant="outlined" />
        )}
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
          <MetaCell label="Tier">
            <Typography variant="body2" sx={{ textTransform: "capitalize" }}>
              {tier ?? "—"}
            </Typography>
          </MetaCell>
          <MetaCell label="Region">
            <Typography variant="body2">{a.region ?? "—"}</Typography>
          </MetaCell>
          <MetaCell label="Salesforce ID">
            <Mono>{a.sfId || "—"}</Mono>
          </MetaCell>
          <MetaCell label="Account Manager">
            <Typography variant="body2">{a.accountManager?.name ?? "—"}</Typography>
          </MetaCell>
          <MetaCell label="Renewal Account Manager">
            <Typography variant="body2">{a.renewalAccountManager?.name ?? "—"}</Typography>
          </MetaCell>
          <MetaCell label="Technical Owner">
            <Typography variant="body2">{a.technicalOwner?.name ?? "—"}</Typography>
          </MetaCell>
          {(a.creTeam || a.sreTeam) && (
            <MetaCell label="CRE / SRE team">
              <Box sx={{ display: "flex", flexWrap: "wrap", gap: 0.5 }}>
                {a.creTeam && (
                  <DirectoryEntityChip
                    id={a.creTeam.id}
                    name={a.creTeam.name}
                    routeBase="/admin/teams"
                  />
                )}
                {a.sreTeam && (
                  <DirectoryEntityChip
                    id={a.sreTeam.id}
                    name={a.sreTeam.name}
                    routeBase="/admin/teams"
                  />
                )}
              </Box>
            </MetaCell>
          )}
          <MetaCell label="Activated on">
            <Typography variant="body2">{formatDate(a.activationDate)}</Typography>
          </MetaCell>
          {deactivationState !== "none" && (
            <MetaCell label={deactivationState === "future" ? "Deactivates on" : "Deactivated on"}>
              <Typography variant="body2">{formatDate(a.deactivationDate)}</Typography>
            </MetaCell>
          )}
          <MetaCell label="AI Chat Assistant (Novera)">
            <Chip
              size="small"
              variant="outlined"
              color={a.hasAgent ? "success" : "default"}
              label={a.hasAgent ? "Enabled" : "Disabled"}
            />
          </MetaCell>
          <MetaCell label="Smart KB Suggestions">
            <Chip
              size="small"
              variant="outlined"
              color={a.hasKbReferences ? "success" : "default"}
              label={a.hasKbReferences ? "Enabled" : "Disabled"}
            />
          </MetaCell>
          <MetaCell label="Created on">
            <Typography variant="body2">{formatDate(a.createdOn)}</Typography>
          </MetaCell>
          <MetaCell label="Updated on">
            <Typography variant="body2">{formatDate(a.updatedOn)}</Typography>
          </MetaCell>
        </Box>
      </Card>

      <ProjectsSection accountId={a.id} />
    </Box>
  );
}
