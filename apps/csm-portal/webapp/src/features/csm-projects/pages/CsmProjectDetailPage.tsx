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

import { Box, Button, Card, Chip, Tab, Tabs, Typography } from "@wso2/oxygen-ui";
import { ArrowLeft, ArrowRight } from "@wso2/oxygen-ui-icons-react";
import { useEffect, useMemo, useState, type JSX, type ReactNode } from "react";
import { useNavigate, useParams } from "react-router";
import CasesList from "@features/csm-cases/components/CasesList";
import { useGetCsmCases } from "@features/csm-cases/api/useGetCsmCases";
import { casesHref } from "@features/csm-cases/utils/casesFiltersUrl";
import { getMockCsmProjectById } from "@features/csm-projects/api/mocks/projectsMocks";
import { useRecordRecentView } from "@features/csm-recent/hooks/useRecentViews";
import RelativeTime from "@components/RelativeTime";
import type {
  CsmProjectRow,
  CsmProjectStatus,
  CsmProjectTier,
} from "@features/csm-projects/types/csmProjects";

type ProjectTab = "overview" | "cases";

function MetaCell({
  label,
  children,
}: {
  label: string;
  children: ReactNode;
}): JSX.Element {
  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 0.25, minWidth: 0 }}>
      <Typography variant="caption" color="text.secondary">
        {label}
      </Typography>
      <Box>{children}</Box>
    </Box>
  );
}

function TierChip({ tier }: { tier: CsmProjectTier }): JSX.Element {
  const color: "primary" | "warning" | "default" =
    tier === "Platinum" ? "primary" : tier === "Gold" ? "warning" : "default";
  return <Chip size="small" variant="outlined" label={tier} color={color} />;
}

function StatusChip({ status }: { status: CsmProjectStatus }): JSX.Element {
  const color: "success" | "warning" | "default" =
    status === "Active"
      ? "success"
      : status === "Onboarding"
        ? "warning"
        : "default";
  return <Chip size="small" variant="outlined" label={status} color={color} />;
}

/**
 * CSM-native project detail page at `/projects/:projectId`.
 *
 * Sits **outside** ProjectGuard on purpose: ProjectGuard hits the
 * customer-portal backend which rejects mock project ids (path-pattern
 * validation). This page reads from the CSM mocks so the demo flow stays
 * intact. The nested legacy routes (`/projects/:id/dashboard`, etc.) remain
 * under ProjectGuard for when real backend data lands.
 */
export default function CsmProjectDetailPage(): JSX.Element {
  const { projectId } = useParams<{ projectId: string }>();
  const navigate = useNavigate();
  const [activeTab, setActiveTab] = useState<ProjectTab>("overview");

  const project: CsmProjectRow | undefined = useMemo(
    () => (projectId ? getMockCsmProjectById(projectId) : undefined),
    [projectId],
  );

  const recordView = useRecordRecentView();
  useEffect(() => {
    if (!project) return;
    recordView({
      kind: "project",
      id: project.id,
      title: project.name,
      subtitle: `${project.customer} · ${project.productType}`,
      href: `/projects/${project.id}`,
    });
  }, [project, recordView]);

  // Pull all-customer cases so we can filter to this project regardless of
  // the engineer's ABT scope.
  const {
    data: casesData,
    isLoading: isCasesLoading,
  } = useGetCsmCases("all_customers");

  const projectCases = useMemo(() => {
    if (!projectId) return [];
    const rows = casesData?.cases ?? [];
    return rows
      .filter((c) => c.projectId === projectId)
      .slice()
      .sort((a, b) => {
        const aClosed = a.state === "closed" ? 1 : 0;
        const bClosed = b.state === "closed" ? 1 : 0;
        if (aClosed !== bClosed) return aClosed - bClosed;
        return a.minutesToBreach - b.minutesToBreach;
      });
  }, [casesData?.cases, projectId]);

  if (!project) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        <Button
          variant="text"
          size="small"
          startIcon={<ArrowLeft size={16} />}
          onClick={() => navigate("/projects")}
          sx={{ alignSelf: "flex-start" }}
        >
          Back to projects
        </Button>
        <Typography variant="h5">Project not found</Typography>
        <Typography variant="body2" color="text.secondary">
          No project with id <code>{projectId}</code> in the current mock dataset.
        </Typography>
      </Box>
    );
  }

  const breachedCount = projectCases.filter(
    (c) => c.minutesToBreach < 0 && c.state !== "closed",
  ).length;

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      <Button
        variant="text"
        size="small"
        startIcon={<ArrowLeft size={16} />}
        onClick={() => navigate("/projects")}
        sx={{ alignSelf: "flex-start" }}
      >
        Back to projects
      </Button>

      <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
        <Box
          sx={{
            display: "flex",
            alignItems: "center",
            gap: 1.5,
            flexWrap: "wrap",
          }}
        >
          <Typography variant="h5">{project.name}</Typography>
          <TierChip tier={project.tier} />
          <StatusChip status={project.status} />
          {breachedCount > 0 && (
            <Chip size="small" color="error" label={`${breachedCount} breached`} />
          )}
        </Box>
        <Typography variant="body2" color="text.secondary">
          <Box
            component="span"
            sx={{
              textDecoration: "underline",
              cursor: "pointer",
              color: "primary.main",
            }}
            onClick={() => navigate(`/accounts/${project.accountId}`)}
          >
            {project.customer}
          </Box>{" "}
          · {project.productType}
        </Typography>
      </Box>

      <Box sx={{ borderBottom: 1, borderColor: "divider" }}>
        <Tabs
          value={activeTab}
          onChange={(_, value) => setActiveTab(value as ProjectTab)}
        >
          <Tab value="overview" label="Overview" />
          <Tab
            value="cases"
            label={`Cases${
              projectCases.length > 0 ? ` (${projectCases.length})` : ""
            }`}
          />
        </Tabs>
      </Box>

      {activeTab === "overview" && (
        <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2 }}>
          <Typography variant="subtitle2">Summary</Typography>
          <Box
            sx={{
              display: "grid",
              gridTemplateColumns: {
                xs: "1fr 1fr",
                md: "repeat(5, minmax(0, 1fr))",
              },
              gap: 2,
            }}
          >
            <MetaCell label="Customer">
              <Typography variant="body2">{project.customer}</Typography>
            </MetaCell>
            <MetaCell label="Product">
              <Typography variant="body2">{project.productType}</Typography>
            </MetaCell>
            <MetaCell label="Update level">
              <Typography variant="body2">{project.updateLevel}</Typography>
            </MetaCell>
            <MetaCell label="Open cases">
              <Typography variant="h6">{project.openCaseCount}</Typography>
            </MetaCell>
            <MetaCell label="Last activity">
              <Typography variant="body2">
                <RelativeTime iso={project.lastActivityAt} />
              </Typography>
            </MetaCell>
          </Box>
        </Card>
      )}

      {activeTab === "cases" && (
        <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              justifyContent: "space-between",
              gap: 2,
              flexWrap: "wrap",
            }}
          >
            <Typography variant="caption" color="text.secondary">
              {projectCases.length} case
              {projectCases.length === 1 ? "" : "s"} for {project.name}.
            </Typography>
            <Button
              size="small"
              variant="outlined"
              endIcon={<ArrowRight size={16} />}
              onClick={() => navigate(casesHref({ search: project.name }))}
            >
              View in cases list
            </Button>
          </Box>
          <CasesList cases={projectCases} isLoading={isCasesLoading} />
        </Box>
      )}
    </Box>
  );
}
