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
import { useState, type JSX, type MouseEvent, type ReactNode } from "react";
import { Link as RouterLink, useLocation, useParams } from "react-router";
import UserRefLink from "@components/UserRefLink";
import { useGetProject } from "@features/csm-projects/api/useGetProject";
import ClosureStateChip from "@features/csm-projects/components/ClosureStateChip";
import DeploymentsTab from "@features/csm-projects/components/DeploymentsTab";
import ProjectContactsTab from "@features/csm-projects/components/ProjectContactsTab";
import WorkItemsTab from "@features/csm-projects/components/WorkItemsTab";
import {
  endDateLabel,
  startDateLabel,
} from "@features/csm-projects/utils/projectLifecycle";
import { useNavTransition } from "@hooks/useNavTransition";
import { useQueryParamTabs } from "@hooks/useSectionTabs";

type ProjectTabId = "overview" | "deployments" | "contacts" | "workItems";
const PROJECT_TAB_IDS: readonly ProjectTabId[] = [
  "overview",
  "deployments",
  "contacts",
  "workItems",
];
// A sub-tab selection only means something under the project tab that owns
// it (currently just Work items' own `?subTab=`) — see the `useQueryParamTabs`
// call below.
const PROJECT_TAB_CHANGE_CLEARS: readonly string[] = ["subTab"];

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
  // Kept in the URL (`?tab=`), not local state, so returning here after a
  // create-flow round trip (see `projectPath` below) restores the tab the
  // engineer was actually on, instead of always resetting to Overview. A
  // sub-tab selection (e.g. Work items' own tab strip, `?subTab=`) only means
  // something under the tab that owns it -- `clearParamsOnChange` drops it so
  // switching away doesn't carry a stale sub-tab back in if this tab is
  // revisited.
  const { activeTab, setActiveTab } = useQueryParamTabs<ProjectTabId>(
    PROJECT_TAB_IDS,
    "overview",
    { clearParamsOnChange: PROJECT_TAB_CHANGE_CLEARS },
  );
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
  // Handed to each "Create X" menu item below as router state, so that
  // create page's own Back/Cancel -- and the entity it creates -- return
  // here instead of their hardcoded top-level list (see CsmCaseCreatePage.tsx
  // and its 3 siblings). Includes the current `?tab=`/`&subTab=` query string
  // (not just the bare `/customers/projects/:id` path), so the round trip
  // restores the exact tab -- Work items' own sub-tab included -- rather than
  // resetting to Overview.
  const projectPath = `${location.pathname}${location.search}`;

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
              navigate(`/cases/new?projectId=${encodeURIComponent(p.id)}`, {
                state: { from: projectPath },
              });
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
                  { state: { from: projectPath } },
                );
              }}
            >
              Create service request
            </MenuItem>
          )}
          <MenuItem
            onClick={() => {
              setCreateMenuAnchor(null);
              navigate(`/engagements/new?projectId=${encodeURIComponent(p.id)}`, {
                state: { from: projectPath },
              });
            }}
          >
            Create engagement
          </MenuItem>
          <MenuItem
            onClick={() => {
              setCreateMenuAnchor(null);
              navigate(
                `/security-center/reports/new?projectId=${encodeURIComponent(p.id)}`,
                { state: { from: projectPath } },
              );
            }}
          >
            Create security report
          </MenuItem>
        </Menu>
      </Box>

      <Box sx={{ borderBottom: 1, borderColor: "divider" }}>
        <Tabs value={activeTab} onChange={(_, v) => setActiveTab(v as ProjectTabId)}>
          <Tab value="overview" label="Overview" />
          <Tab value="deployments" label="Deployments" />
          <Tab value="contacts" label="Project contacts" />
          <Tab value="workItems" label="Work items" />
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
            {p.onboardingStatus &&
              p.onboardingStatus !== "Not-Applicable" &&
              p.onboardingOwner && (
                <MetaCell label="Onboarding Owner">
                  <Typography variant="body2">
                    <UserRefLink
                      name={p.onboardingOwner.name}
                      email={p.onboardingOwner.email || undefined}
                      userId={p.onboardingOwner.id}
                    />
                  </Typography>
                </MetaCell>
              )}
          </Box>
        </Card>
      )}

      {activeTab === "deployments" && <DeploymentsTab projectId={p.id} />}

      {activeTab === "contacts" && <ProjectContactsTab projectId={p.id} />}

      {activeTab === "workItems" && <WorkItemsTab projectId={p.id} />}
    </Box>
  );
}
