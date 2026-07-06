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
import { ArrowLeft, Plus } from "@wso2/oxygen-ui-icons-react";
import { useState, type JSX, type ReactNode } from "react";
import { Link as RouterLink, useNavigate, useParams } from "react-router";
import { useGetProject } from "@features/csm-projects/api/useGetProject";
import CsmIssuesView from "@features/csm-cases/components/CsmIssuesView";
import DeploymentsTab from "@features/csm-projects/components/DeploymentsTab";

type ProjectTabId = "overview" | "issues" | "deployments";

function formatDate(value?: string | null): string {
  if (!value) return "—";
  const d = new Date(value);
  return Number.isNaN(d.getTime()) ? value : d.toLocaleDateString();
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
      Back to projects
    </Button>
  );
}

export default function CsmProjectDetailPage(): JSX.Element {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { data, isLoading, isError } = useGetProject(id);
  const [activeTab, setActiveTab] = useState<ProjectTabId>("overview");

  if (isLoading) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
        <Skeleton variant="rectangular" height={32} width={240} />
        <Skeleton variant="rectangular" height={220} />
      </Box>
    );
  }

  if (isError) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        <BackButton onClick={() => navigate("/customers/projects")} />
        <Typography variant="body1" color="error">
          Could not load project {id}.
        </Typography>
      </Box>
    );
  }

  if (!data) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        <BackButton onClick={() => navigate("/customers/projects")} />
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
      <BackButton onClick={() => navigate("/customers/projects")} />

      <Box
        sx={{
          display: "flex",
          gap: 2,
          alignItems: "flex-start",
          justifyContent: "space-between",
          flexWrap: "wrap",
        }}
      >
        <Box sx={{ display: "flex", flexDirection: "column", gap: 1, minWidth: 0 }}>
          <Box sx={{ display: "flex", alignItems: "center", gap: 1.5, flexWrap: "wrap" }}>
            <Typography variant="h5">{p.name}</Typography>
            <Chip
              size="small"
              label={formatSubscriptionType(p.subscriptionType)}
              variant="outlined"
            />
          </Box>
          <Typography variant="body2" color="text.secondary" sx={{ fontFamily: "monospace" }}>
            {p.key}
          </Typography>
        </Box>
        {/* File a case already scoped to this project — the create form locks the
            project field, so it can't be filed against the wrong one. */}
        <Button
          variant="contained"
          startIcon={<Plus size={16} />}
          onClick={() => navigate(`/cases/new?projectId=${encodeURIComponent(p.id)}`)}
          sx={{ flexShrink: 0 }}
        >
          Create case
        </Button>
      </Box>

      <Box sx={{ borderBottom: 1, borderColor: "divider" }}>
        <Tabs value={activeTab} onChange={(_, v) => setActiveTab(v as ProjectTabId)}>
          <Tab value="overview" label="Overview" />
          <Tab value="issues" label="Issues" />
          <Tab value="deployments" label="Deployments" />
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
            <MetaCell label="Account tier">
              <Typography variant="body2">{p.account?.tier || "—"}</Typography>
            </MetaCell>
            <MetaCell label="Start date">
              <Typography variant="body2">{formatDate(p.startDate)}</Typography>
            </MetaCell>
            <MetaCell label="End date">
              <Typography variant="body2">{formatDate(p.endDate)}</Typography>
            </MetaCell>
            <MetaCell label="Salesforce ID">
              <Mono>{p.sfId || "—"}</Mono>
            </MetaCell>
            <MetaCell label="Created">
              <Typography variant="body2">{formatDate(p.createdOn)}</Typography>
            </MetaCell>
            <MetaCell label="Last updated">
              <Typography variant="body2">{formatDate(p.updatedOn)}</Typography>
            </MetaCell>
            <MetaCell label="Project ID">
              <Mono>{p.id}</Mono>
            </MetaCell>
          </Box>
        </Card>
      )}

      {activeTab === "issues" && (
        <CsmIssuesView
          entityNoun="issues"
          lockedFilters={{ projects: [p.id] }}
          hideProjectFilter
        />
      )}

      {activeTab === "deployments" && <DeploymentsTab projectId={p.id} />}
    </Box>
  );
}
