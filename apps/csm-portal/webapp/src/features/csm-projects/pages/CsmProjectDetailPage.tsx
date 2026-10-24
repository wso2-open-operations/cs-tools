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
  IconButton,
  Menu,
  MenuItem,
  Skeleton,
  Tab,
  Tabs,
  Typography,
} from "@wso2/oxygen-ui";
import { ArrowLeft, ChevronDown, Pencil, Plus } from "@wso2/oxygen-ui-icons-react";
import { useState, type JSX, type MouseEvent, type ReactNode } from "react";
import { Link as RouterLink, useLocation, useParams } from "react-router";
import { BackendApiError } from "@api/backend/client";
import { usePortalAccess } from "@context/current-user/usePortalAccess";
import { useGetProject } from "@features/csm-projects/api/useGetProject";
import { usePatchProjectClosureState } from "@features/csm-projects/api/usePatchProjectClosureState";
import { usePatchProjectOnboarding } from "@features/csm-projects/api/usePatchProjectOnboarding";
import { useProjectMetadata } from "@features/csm-projects/api/useProjectMetadata";
import ClosureStateChip from "@features/csm-projects/components/ClosureStateChip";
import DeploymentsTab from "@features/csm-projects/components/DeploymentsTab";
import EditProjectClosureStateDialog from "@features/csm-projects/components/EditProjectClosureStateDialog";
import EditProjectOnboardingDialog from "@features/csm-projects/components/EditProjectOnboardingDialog";
import ProjectMetaCell from "@features/csm-projects/components/ProjectMetaCell";
import ProjectOnboardingOwnerValue from "@features/csm-projects/components/ProjectOnboardingOwnerValue";
import ProjectOnboardingTab from "@features/csm-projects/components/ProjectOnboardingTab";
import ProjectContactsTab from "@features/csm-projects/components/ProjectContactsTab";
import { isOnboardingEnabled } from "@features/csm-projects/utils/projectOnboardingEnabled";
import ProjectOnboardingStatusChip from "@features/csm-projects/components/ProjectOnboardingStatusChip";
import WorkItemsTab from "@features/csm-projects/components/WorkItemsTab";
import {
  endDateLabel,
  startDateLabel,
} from "@features/csm-projects/utils/projectLifecycle";
import { canRaiseServiceRequests } from "@features/csm-projects/utils/subscriptionType";
import { useNavTransition } from "@hooks/useNavTransition";
import { useQueryParamTabs } from "@hooks/useSectionTabs";

type ProjectTabId = "overview" | "onboarding" | "deployments" | "contacts" | "workItems";
const PROJECT_TAB_IDS: readonly ProjectTabId[] = [
  "overview",
  "onboarding",
  "deployments",
  "contacts",
  "workItems",
];
// A sub-tab selection only means something under the project tab that owns
// it (currently just Work items' own `?subTab=`) — see the `useQueryParamTabs`
// call below.
const PROJECT_TAB_CHANGE_CLEARS: readonly string[] = ["subTab"];

/**
 * A raw upstream message is only safe to show for a client error (<500) --
 * a 5xx body can leak internal detail and isn't actionable by the user, so
 * it falls back to a generic message instead. Mirrors
 * CsmAccountDetailPage's own `accountTeamsSaveErrorMessage`.
 */
function closureStateSaveErrorMessage(err: unknown): string {
  return err instanceof BackendApiError && err.status < 500 && err.message
    ? err.message
    : "Could not update the project's closure state. Please try again.";
}

/** Same 5xx-safe rule as {@link closureStateSaveErrorMessage}; the backend's
 *  own 4xx text (403 for a caller without the onboarding permission, 400 for a
 *  project it cannot edit or an owner it rejects) is what the user needs. */
function onboardingSaveErrorMessage(err: unknown): string {
  return err instanceof BackendApiError && err.status < 500 && err.message
    ? err.message
    : "Could not update the project's onboarding details. Please try again.";
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

/** "managed_cloud_subscription" -> "Managed Cloud Subscription". */
function formatSubscriptionType(value: string): string {
  return value.replace(/_/g, " ").replace(/\b\w/g, (c) => c.toUpperCase());
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
      className="csm-print-hide"
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
  const {
    canWrite,
    canViewWorkItemsStaffView,
    canUpdateProjectClosureState,
    canEditProjectOnboarding,
  } = usePortalAccess();
  const patchClosureState = usePatchProjectClosureState(id);
  const [editClosureStateOpen, setEditClosureStateOpen] = useState(false);
  const patchOnboarding = usePatchProjectOnboarding(id);
  const [editOnboardingOpen, setEditOnboardingOpen] = useState(false);
  // One dialog for both entry points (overview pencil, Onboarding tab Edit).
  // Drops a stale error from a previous session with the dialog.
  const openEditOnboarding = (): void => {
    patchOnboarding.reset();
    setEditOnboardingOpen(true);
  };
  // Prefer wherever the caller came from (e.g. a case's Overview panel) over
  // the hardcoded projects list, so Back returns to that page instead of
  // skipping past it — same convention as CsmCaseDetailPage's own back path.
  const fromListState = location.state as { from?: string } | undefined;
  const resolvedBackPath = fromListState?.from ?? "/customers/projects";
  const { data, isLoading, isError } = useGetProject(id);
  // Only used to gate the "Create service request" menu item below — fail
  // open (show the item) while metadata is loading or failed, since
  // `canRaiseServiceRequests` (managed-cloud and Cloud Support) already
  // narrows to projects where SR is normally offered; an explicit `false` once loaded
  // is what actually hides it. The create page itself is the authoritative,
  // fail-closed gate (see CreateServiceRequestPage.tsx's hasNoSrReadAccess).
  const projectMetadata = useProjectMetadata(id);
  const hasNoSrReadAccess =
    projectMetadata.data?.features?.hasServiceRequestReadAccess === false;
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
  // Existing display gate: the onboarding cells show only for an
  // onboarding-enabled project. An editor additionally always sees them (see
  // below) so onboarding can be started on a project that has none yet.
  const onboardingEnabled = isOnboardingEnabled(p.onboardingStatus);
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
        <Box sx={{ display: "flex", gap: 1, flexWrap: "wrap", flexShrink: 0 }}>
          {/* Shown to anyone who can open this project. Outlined, not
              contained: the single contained CTA on this page is Create. */}
          <Button variant="outlined" size="small" className="csm-print-hide" disabled>
            SLA Report — Coming soon
          </Button>
          <Button
            variant="outlined"
            size="small"
            className="csm-print-hide"
            component={RouterLink}
            to={`/customers/projects/${p.id}/timelogs-report`}
          >
            Time Report
          </Button>
          <Button
            variant="outlined"
            size="small"
            className="csm-print-hide"
            component={RouterLink}
            to={`/customers/projects/${p.id}/cs-report/${p.id}`}
          >
            CS Report
          </Button>
          {/* File any issue type already scoped to this project — every create
              form below locks the project field, so it can't be filed against
              the wrong one. */}
          {canWrite && (
            <>
            <Button
              variant="contained"
              className="csm-print-hide"
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
              {canRaiseServiceRequests(p.subscriptionType) && !hasNoSrReadAccess && (
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
            </>
          )}
        </Box>
      </Box>

      <Box className="csm-print-hide" sx={{ borderBottom: 1, borderColor: "divider" }}>
        <Tabs value={activeTab} onChange={(_, v) => setActiveTab(v as ProjectTabId)}>
          <Tab value="overview" label="Overview" />
          <Tab value="onboarding" label="Onboarding" />
          <Tab value="deployments" label="Deployments" />
          <Tab value="contacts" label="Project contacts" />
          {/* A caller without the staff view never sees Chats inside this tab
              (see WorkItemsTab's own canViewWorkItemsStaffView gate), so for
              them it's just the case list -- labelled accordingly. */}
          <Tab value="workItems" label={canViewWorkItemsStaffView ? "Work items" : "Cases"} />
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
            <ProjectMetaCell label="Project key">
              <Mono>{p.key}</Mono>
            </ProjectMetaCell>
            <ProjectMetaCell label="State">
              <Box sx={{ display: "flex", alignItems: "center", gap: 0.5 }}>
                <ClosureStateChip closureState={p.closureState} />
                {canUpdateProjectClosureState && (
                  <IconButton
                    size="small"
                    aria-label="Edit closure state"
                    className="csm-print-hide"
                    onClick={() => {
                      // A failed save from a previous session with this
                      // dialog would otherwise still be `isError` here,
                      // showing a stale error the moment the dialog reopens.
                      patchClosureState.reset();
                      setEditClosureStateOpen(true);
                    }}
                  >
                    <Pencil size={14} />
                  </IconButton>
                )}
              </Box>
            </ProjectMetaCell>
            <ProjectMetaCell label="Subscription">
              <Typography variant="body2">{formatSubscriptionType(p.subscriptionType)}</Typography>
            </ProjectMetaCell>
            <ProjectMetaCell label="Account">
              {p.account?.id ? (
                <LinkText to={`/customers/accounts/${p.account.id}`}>
                  {p.account.name || p.account.id}
                </LinkText>
              ) : (
                <Typography variant="body2">—</Typography>
              )}
            </ProjectMetaCell>
            <ProjectMetaCell label="Salesforce ID">
              <Mono>{p.sfId || "—"}</Mono>
            </ProjectMetaCell>
            <ProjectMetaCell label="Updated on">
              <Typography variant="body2">{formatDate(p.updatedOn)}</Typography>
            </ProjectMetaCell>
            <ProjectMetaCell label="Created on">
              <Typography variant="body2">{formatDate(p.createdOn)}</Typography>
            </ProjectMetaCell>
            <ProjectMetaCell label={startDateLabel(p.createdOn, p.startDate)}>
              <Typography variant="body2">{formatDate(p.startDate)}</Typography>
            </ProjectMetaCell>
            <ProjectMetaCell label={endDateLabel(p.endDate)}>
              <Typography variant="body2">{formatDate(p.endDate)}</Typography>
            </ProjectMetaCell>
            {(onboardingEnabled || canEditProjectOnboarding) && (
              <ProjectMetaCell label="Onboarding Status">
                <Box sx={{ display: "flex", alignItems: "center", gap: 0.5 }}>
                  {onboardingEnabled ? (
                    <ProjectOnboardingStatusChip onboardingStatus={p.onboardingStatus} />
                  ) : (
                    <Typography variant="body2">—</Typography>
                  )}
                  {canEditProjectOnboarding && (
                    <IconButton
                      size="small"
                      aria-label="Edit onboarding"
                      className="csm-print-hide"
                      onClick={openEditOnboarding}
                    >
                      <Pencil size={14} />
                    </IconButton>
                  )}
                </Box>
              </ProjectMetaCell>
            )}
            {(onboardingEnabled || canEditProjectOnboarding) &&
              (p.onboardingOwner || canEditProjectOnboarding) && (
                <ProjectMetaCell label="Onboarding Owner">
                  <ProjectOnboardingOwnerValue owner={p.onboardingOwner} />
                </ProjectMetaCell>
              )}
          </Box>
        </Card>
      )}

      {activeTab === "onboarding" && (
        <ProjectOnboardingTab
          project={p}
          canEdit={canEditProjectOnboarding}
          onEdit={openEditOnboarding}
        />
      )}

      {activeTab === "deployments" && <DeploymentsTab projectId={p.id} />}

      {activeTab === "contacts" && <ProjectContactsTab projectId={p.id} />}

      {activeTab === "workItems" && <WorkItemsTab projectId={p.id} />}

      {editOnboardingOpen && (
        <EditProjectOnboardingDialog
          currentStatus={p.onboardingStatus}
          currentOwner={p.onboardingOwner}
          currentDates={p}
          isSaving={patchOnboarding.isPending}
          saveError={
            patchOnboarding.isError
              ? onboardingSaveErrorMessage(patchOnboarding.error)
              : null
          }
          onClose={() => {
            if (!patchOnboarding.isPending) setEditOnboardingOpen(false);
          }}
          onSave={(patch) =>
            // No onError handler: the dialog's own inline alert (saveError
            // above) already surfaces the failure.
            patchOnboarding.mutate(patch, {
              onSuccess: () => setEditOnboardingOpen(false),
            })
          }
        />
      )}

      {editClosureStateOpen && (
        <EditProjectClosureStateDialog
          currentClosureState={p.closureState}
          isSaving={patchClosureState.isPending}
          saveError={
            patchClosureState.isError
              ? closureStateSaveErrorMessage(patchClosureState.error)
              : null
          }
          onClose={() => {
            if (!patchClosureState.isPending) setEditClosureStateOpen(false);
          }}
          onSave={(closureState) =>
            // No onError handler: the dialog's own inline alert (saveError
            // above, driven by patchClosureState.isError/.error) already
            // surfaces the failure.
            patchClosureState.mutate(closureState, {
              onSuccess: () => setEditClosureStateOpen(false),
            })
          }
        />
      )}
    </Box>
  );
}
