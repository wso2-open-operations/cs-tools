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
  Tab,
  Tabs,
  Typography,
} from "@wso2/oxygen-ui";
import {
  Activity,
  ArrowLeft,
  CheckCircle,
  ExternalLink,
  Layers,
  Link as LinkIcon,
  ListChecks,
  Megaphone,
  Package,
  Plus,
} from "@wso2/oxygen-ui-icons-react";
import { useState, type JSX } from "react";
import { useNavigate, useParams } from "react-router";
import RelativeTime from "@components/RelativeTime";
import { useSuccessBanner } from "@context/success-banner/SuccessBannerContext";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import EngagementActionBar from "@features/csm-engagements/components/EngagementActionBar";
import EngagementActivityFeed from "@features/csm-engagements/components/EngagementActivityFeed";
import EngagementOverviewPanel from "@features/csm-engagements/components/EngagementOverviewPanel";
import EngagementStagesTimeline from "@features/csm-engagements/components/EngagementStagesTimeline";
import EngagementTasksPanel from "@features/csm-engagements/components/EngagementTasksPanel";
import EngagementDeliverablesPanel from "@features/csm-engagements/components/EngagementDeliverablesPanel";
import EngagementStatusUpdatesPanel from "@features/csm-engagements/components/EngagementStatusUpdatesPanel";
import { useGetCsmEngagementDetail } from "@features/csm-engagements/api/useGetCsmEngagementDetail";
import {
  useGetCsmEngagementComments,
  usePostCsmEngagementComment,
} from "@features/csm-engagements/api/useCsmEngagementComments";
import { usePatchCsmEngagement } from "@features/csm-engagements/api/usePatchCsmEngagement";
import {
  ENGAGEMENT_HEALTH_COLOR,
  ENGAGEMENT_HEALTH_LABEL,
  ENGAGEMENT_STAGE_LABEL,
  ENGAGEMENT_STATE_COLOR,
  ENGAGEMENT_STATE_LABEL,
  ENGAGEMENT_TYPE_LABEL,
  formatDateOnly,
} from "@features/csm-engagements/utils/engagements";
import type { CsmEngagementLifecycleAction } from "@features/csm-engagements/types/csmEngagements";

type DetailTab =
  | "overview"
  | "stages"
  | "tasks"
  | "deliverables"
  | "updates"
  | "activity"
  | "related";

const TAB_DEFS: Array<{ id: DetailTab; label: string; icon: JSX.Element }> = [
  { id: "overview", label: "Overview", icon: <ListChecks size={16} /> },
  { id: "activity", label: "Activity", icon: <Activity size={16} /> },
  { id: "stages", label: "Stages", icon: <Layers size={16} /> },
  { id: "tasks", label: "Tasks", icon: <CheckCircle size={16} /> },
  { id: "deliverables", label: "Deliverables", icon: <Package size={16} /> },
  { id: "updates", label: "Status updates", icon: <Megaphone size={16} /> },
  { id: "related", label: "Related", icon: <LinkIcon size={16} /> },
];

const LIFECYCLE_TOAST: Record<CsmEngagementLifecycleAction, string> = {
  approve_request: "Engagement approved and started.",
  start_work: "Work started.",
  put_on_hold: "Engagement put on hold.",
  resume_work: "Work resumed.",
  complete_engagement: "Engagement marked complete.",
  cancel_engagement: "Engagement cancelled.",
  reopen: "Engagement reopened.",
};

// Current engineer name placeholder — wire to useGetUserDetails when available.
const CURRENT_ENGINEER_NAME = "Sajith Ekanayaka";

export default function CsmEngagementDetailPage(): JSX.Element {
  const { engagementId } = useParams<{ engagementId: string }>();
  const navigate = useNavigate();
  const { data, isLoading, isError } = useGetCsmEngagementDetail(engagementId);
  const { data: comments } = useGetCsmEngagementComments(engagementId);
  const postComment = usePostCsmEngagementComment();
  const patch = usePatchCsmEngagement();
  const { showSuccess } = useSuccessBanner();
  const { showError } = useErrorBanner();
  const [tab, setTab] = useState<DetailTab>("overview");

  if (isLoading) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
        <Skeleton variant="rectangular" height={32} width={240} />
        <Skeleton variant="rectangular" height={120} />
        <Skeleton variant="rectangular" height={240} />
      </Box>
    );
  }

  if (isError || !data) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        <Button
          variant="text"
          size="small"
          startIcon={<ArrowLeft size={16} />}
          onClick={() => navigate("/engagements")}
          sx={{ alignSelf: "flex-start" }}
        >
          Back to engagements
        </Button>
        <Typography variant="h5">Engagement not found</Typography>
        <Typography variant="body2" color="text.secondary">
          No engagement with id <code>{engagementId}</code>.
        </Typography>
      </Box>
    );
  }

  const e = data;

  const onLifecycle = async (action: CsmEngagementLifecycleAction): Promise<void> => {
    try {
      await patch.mutateAsync({ engagementId: e.id, action });
      showSuccess(LIFECYCLE_TOAST[action]);
    } catch (err) {
      showError(err instanceof Error ? err.message : "Could not update engagement.");
    }
  };

  const onToggleWatch = async (): Promise<void> => {
    try {
      await patch.mutateAsync({ engagementId: e.id, isWatching: !e.isWatching });
      showSuccess(e.isWatching ? "Stopped watching." : "Now watching this engagement.");
    } catch (err) {
      showError(err instanceof Error ? err.message : "Could not toggle watch.");
    }
  };

  const onPostComment = async (bodyHtml: string, internal: boolean): Promise<void> => {
    try {
      await postComment.mutateAsync({
        engagementId: e.id,
        authorName: CURRENT_ENGINEER_NAME,
        bodyHtml,
        internal,
      });
    } catch (err) {
      showError(err instanceof Error ? err.message : "Could not post comment.");
    }
  };

  const onCreateRelatedCase = (): void => {
    // Drop into the case creation flow with the engagement reference preset.
    // The case form reads `state.fromEngagement` when present.
    navigate("/cases/new", {
      state: {
        fromEngagement: {
          id: e.id,
          reference: e.reference,
          name: e.name,
          projectId: e.projectId,
          projectName: e.projectName,
          customer: e.customer,
          accountId: e.accountId,
        },
      },
    });
  };

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2.5 }}>
      <Button
        variant="text"
        size="small"
        startIcon={<ArrowLeft size={16} />}
        onClick={() => navigate("/engagements")}
        sx={{ alignSelf: "flex-start" }}
      >
        Back to engagements
      </Button>

      <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
        <Box sx={{ display: "flex", alignItems: "center", gap: 1.5, flexWrap: "wrap" }}>
          <Typography variant="overline" color="text.secondary">
            {e.reference}
          </Typography>
          <Chip size="small" label={ENGAGEMENT_TYPE_LABEL[e.type]} variant="outlined" />
          <Chip
            size="small"
            label={ENGAGEMENT_STATE_LABEL[e.state]}
            variant="outlined"
            color={ENGAGEMENT_STATE_COLOR[e.state]}
          />
          <Chip size="small" label={ENGAGEMENT_STAGE_LABEL[e.stage]} variant="outlined" />
          {e.health && (
            <Chip
              size="small"
              color={ENGAGEMENT_HEALTH_COLOR[e.health]}
              label={ENGAGEMENT_HEALTH_LABEL[e.health]}
            />
          )}
          <Typography variant="caption" color="text.secondary">
            {formatDateOnly(e.plannedStartDate)} → {formatDateOnly(e.plannedEndDate)}
          </Typography>
        </Box>
        <Typography variant="h5">{e.name}</Typography>
        <Typography variant="body2" color="text.secondary">
          {e.customer} · {e.projectName} · Owner: {e.ownerName} · Updated{" "}
          <RelativeTime iso={e.updatedAt} />
        </Typography>
      </Box>

      <EngagementActionBar
        engagement={e}
        onLifecycle={(action) => void onLifecycle(action)}
        onToggleWatch={() => void onToggleWatch()}
        onPostStatusUpdate={() =>
          showSuccess("Status update editor (mock) — coming soon.")
        }
        isMutating={patch.isPending}
      />

      <Box sx={{ borderBottom: 1, borderColor: "divider" }}>
        <Tabs
          value={tab}
          onChange={(_, v) => setTab(v as DetailTab)}
          variant="scrollable"
          scrollButtons="auto"
        >
          {TAB_DEFS.map((t) => (
            <Tab
              key={t.id}
              value={t.id}
              icon={t.icon}
              iconPosition="start"
              label={t.label}
              sx={{ minHeight: 44, textTransform: "none" }}
            />
          ))}
        </Tabs>
      </Box>

      {tab === "overview" && <EngagementOverviewPanel engagement={e} />}

      {tab === "activity" && (
        <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2 }}>
          <EngagementActivityFeed
            comments={comments ?? []}
            audit={e.audit}
            attachments={e.attachments}
            onPostComment={onPostComment}
            isPosting={postComment.isPending}
          />
        </Card>
      )}

      {tab === "stages" && <EngagementStagesTimeline engagement={e} />}
      {tab === "tasks" && <EngagementTasksPanel tasks={e.tasks} />}
      {tab === "deliverables" && <EngagementDeliverablesPanel deliverables={e.deliverables} />}
      {tab === "updates" && <EngagementStatusUpdatesPanel updates={e.statusUpdates} />}

      {tab === "related" && (
        <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2 }}>
          <Box sx={{ display: "flex", alignItems: "center", gap: 1.5, flexWrap: "wrap" }}>
            <Typography variant="subtitle2" sx={{ flex: 1 }}>
              Related cases
            </Typography>
            <Button
              size="small"
              variant="outlined"
              startIcon={<Plus size={16} />}
              onClick={onCreateRelatedCase}
            >
              Create related case
            </Button>
          </Box>
          {e.linkedCases.length === 0 ? (
            <Typography variant="body2" color="text.secondary">
              No cases are linked to this engagement yet. Use{" "}
              <strong>Create related case</strong> when a support case or product fix is
              needed to deliver against this engagement.
            </Typography>
          ) : (
            e.linkedCases.map((c) => (
              <Box
                key={c.id}
                sx={{
                  display: "flex",
                  alignItems: "center",
                  gap: 1.5,
                  py: 1,
                  borderBottom: 1,
                  borderColor: "divider",
                  "&:last-of-type": { borderBottom: 0 },
                }}
              >
                <Chip size="small" variant="outlined" label={c.severity} />
                <Box sx={{ flex: 1, minWidth: 0 }}>
                  <Typography variant="body2">
                    <code style={{ fontSize: "0.8rem", marginRight: 6 }}>
                      {c.caseNumber}
                    </code>
                    {c.subject}
                  </Typography>
                </Box>
                <Chip size="small" variant="outlined" label={c.state} />
                <Button
                  size="small"
                  variant="text"
                  endIcon={<ExternalLink size={14} />}
                  onClick={() => navigate(c.href)}
                >
                  Open
                </Button>
              </Box>
            ))
          )}
        </Card>
      )}
    </Box>
  );
}
