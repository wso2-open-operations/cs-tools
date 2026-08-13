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
  Menu,
  MenuItem,
  Skeleton,
  Tab,
  Tabs,
  Typography,
} from "@wso2/oxygen-ui";
import { ArrowLeft, ChevronDown, Plus } from "@wso2/oxygen-ui-icons-react";
import { type JSX, type MouseEvent, type ReactNode, useState } from "react";
import { Link as RouterLink, useLocation, useParams } from "react-router";
import { useGetProject } from "@features/csm-projects/api/useGetProject";
import ClosureStateChip from "@features/csm-projects/components/ClosureStateChip";
import DeploymentsTab from "@features/csm-projects/components/DeploymentsTab";
import ProjectContactsTab from "@features/csm-projects/components/ProjectContactsTab";
import WorkItemsTab from "@features/csm-projects/components/WorkItemsTab";
import {
  endDateLabel,
  startDateLabel,
} from "@features/csm-projects/utils/projectLifecycle";
import { usePathTabs } from "@hooks/useSectionTabs";
import { useNavTransition } from "@hooks/useNavTransition";

type ProjectTabId = "overview" | "deployments" | "contacts" | "work-items";

// Every known top-level tab id, for `usePathTabs` to validate an incoming URL
// segment against.
const PROJECT_TAB_IDS: readonly ProjectTabId[] = [
  "overview",
  "deployments",
  "contacts",
  "work-items",
];

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

function formatSubscriptionType(value: string): string {
  return value.replace(/_/g, " ");
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

// Real anchor (RouterLink) so the account link is cmd/middle-clickable and
// copyable, with plain left-click staying in-app. Colour is picked per colour
// scheme: brand orange (`primary.main`) fails WCAG AA on a light surface, while
// `primary.dark` fails on the dark surface, so we apply the dark shade only in
// the light scheme and vice versa (matching the case meta band's links).
function LinkText({ to, children }: { to: string; children: ReactNode }): JSX.Element {
  return (
    <Typography
      component={RouterLink}
      to={to}
      variant="body2"
      sx={(t) => ({
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

export default function CsmProjectDetailPage(): JSX.Element {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavTransition();
  const location = useLocation();
  // Prefer wherever the caller came from (e.g. a case's Overview panel) over
  // the hardcoded projects list, so Back returns to that page instead of
  // skipping past it — same convention as CsmCaseDetailPage's own back path.
  const fromListState = location.state as { from?: string } | undefined;
  const resolvedBackPath = fromListState?.from ?? "/customers/projects";
  const { data, isLoading, isError } = useGetProject(id);
  // Tab is a real URL path segment (`/customers/projects/:id/:tab?`) so a
  // link to a specific tab is shareable/bookmarkable. Work items nests a
  // second, independently-tabbed level under its own
  // `/customers/projects/:id/work-items/:tab?` route (see App.tsx), so its
  // active-tab state isn't read from this hook's own `tab` param — that
  // param, under the nested route, actually holds the *sub*-tab segment
  // (consumed by WorkItemsTab's own `usePathTabs` call below). This page
  // instead derives whether Work items is the active top-level tab from the
  // pathname directly, and only defers to `usePathTabs`'s own resolution for
  // the other three tabs.
  const basePath = `/customers/projects/${id}`;
  const { activeTab: pathTab, setActiveTab: setPathTab } =
    usePathTabs<ProjectTabId>(basePath, PROJECT_TAB_IDS, "overview");
  const isWorkItemsRoute =
    location.pathname === `${basePath}/work-items` ||
    location.pathname.startsWith(`${basePath}/work-items/`);
  const activeTab: ProjectTabId = isWorkItemsRoute ? "work-items" : pathTab;
  const [createMenuAnchor, setCreateMenuAnchor] = useState<HTMLElement | null>(
    null,
  );

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
          Could not load project {id}.
        </Typography>
      </Box>
    );
  }

  if (!data) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        <BackButton onClick={() => navigate(resolvedBackPath)} />
        <Typography variant="h5">Project not found</Typography>
        <Typography variant="body2" color="text.secondary">
          No project with id <code>{id}</code>.
        </Typography>
      </Box>
    );
  }

  const p = data;

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2.5 }}>
      <BackButton onClick={() => navigate(resolvedBackPath)} />

      <Box
        sx={{
          display: "flex",
          gap: 2,
          alignItems: "flex-start",
          justifyContent: "space-between",
          flexWrap: "wrap",
        }}
      >
        <Box sx={{ display: "flex", alignItems: "center", gap: 1.5, flexWrap: "wrap", minWidth: 0 }}>
          <Typography variant="h5">{p.name}</Typography>
          <Chip
            size="small"
            label={formatSubscriptionType(p.subscriptionType)}
            variant="outlined"
          />
        </Box>
        {/* File any issue type already scoped to this project — every create
            form below locks the project field, so it can't be filed against
            the wrong one. */}
        <Button
          variant="contained"
          startIcon={<Plus size={16} />}
          endIcon={<ChevronDown size={16} />}
          onClick={(e: MouseEvent<HTMLElement>) => setCreateMenuAnchor(e.currentTarget)}
          sx={{ flexShrink: 0 }}
        >
          Create
        </Button>
        <Menu
          anchorEl={createMenuAnchor}
          open={!!createMenuAnchor}
          onClose={() => setCreateMenuAnchor(null)}
        >
          <MenuItem
            onClick={() => {
              setCreateMenuAnchor(null);
              navigate(`/cases/new?projectId=${encodeURIComponent(p.id)}`);
            }}
          >
            Create case
          </MenuItem>
          {p.subscriptionType === "managed_cloud_subscription" && (
            <MenuItem
              onClick={() => {
                setCreateMenuAnchor(null);
                navigate(
                  `/operations/service-requests/new?projectId=${encodeURIComponent(p.id)}`,
                );
              }}
            >
              Create service request
            </MenuItem>
          )}
          <MenuItem
            onClick={() => {
              setCreateMenuAnchor(null);
              navigate(`/engagements/new?projectId=${encodeURIComponent(p.id)}`);
            }}
          >
            Create engagement
          </MenuItem>
          <MenuItem
            onClick={() => {
              setCreateMenuAnchor(null);
              navigate(
                `/security-center/reports/new?projectId=${encodeURIComponent(p.id)}`,
              );
            }}
          >
            Create security report
          </MenuItem>
        </Menu>
      </Box>

      <Box sx={{ borderBottom: 1, borderColor: "divider" }}>
        <Tabs
          value={activeTab}
          onChange={(_, v) => setPathTab(v as ProjectTabId)}
        >
          <Tab value="overview" label="Overview" />
          <Tab value="deployments" label="Deployments" />
          <Tab value="contacts" label="Project contacts" />
          <Tab value="work-items" label="Work items" />
        </Tabs>
      </Box>

      {activeTab === "overview" && (
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
            <MetaCell label="Project key">
              <Mono>{p.key}</Mono>
            </MetaCell>
            <MetaCell label="State">
              <ClosureStateChip closureState={p.closureState} />
            </MetaCell>
            <MetaCell label="Subscription">
              <Typography variant="body2">{formatSubscriptionType(p.subscriptionType)}</Typography>
            </MetaCell>
            <MetaCell label="Account">
              {p.account?.id ? (
                <LinkText to={`/customers/accounts/${p.account.id}`}>
                  {p.account.name || p.account.id}
                </LinkText>
              ) : (
                <Typography variant="body2">—</Typography>
              )}
            </MetaCell>
            <MetaCell label="Salesforce ID">
              <Mono>{p.sfId || "—"}</Mono>
            </MetaCell>
            <MetaCell label="Updated on">
              <Typography variant="body2">{formatDate(p.updatedOn)}</Typography>
            </MetaCell>
            <MetaCell label="Created on">
              <Typography variant="body2">{formatDate(p.createdOn)}</Typography>
            </MetaCell>
            <MetaCell label={startDateLabel(p.createdOn, p.startDate)}>
              <Typography variant="body2">{formatDate(p.startDate)}</Typography>
            </MetaCell>
            <MetaCell label={endDateLabel(p.endDate)}>
              <Typography variant="body2">{formatDate(p.endDate)}</Typography>
            </MetaCell>
          </Box>
        </Card>
      )}

      {activeTab === "deployments" && <DeploymentsTab projectId={p.id} />}

      {activeTab === "contacts" && <ProjectContactsTab projectId={p.id} />}

      {activeTab === "work-items" && (
        <WorkItemsTab projectId={p.id} basePath={`${basePath}/work-items`} />
      )}
    </Box>
  );
}
