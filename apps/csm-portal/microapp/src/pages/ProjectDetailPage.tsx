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

import { useState } from "react";
import { useParams, Link } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { Card, Chip, Divider, Skeleton, Stack, Tab, Tabs, Typography } from "@wso2/oxygen-ui";
import { projects } from "@src/services/projects";
import { formatDateOnly, formatEnumLabel } from "@utils/customers";
import { MetaRow, MetaValue } from "@components/customers/MetaRow";
import { ProjectIssuesTab } from "@components/customers/ProjectIssuesTab";
import { DeploymentsTab } from "@components/customers/DeploymentsTab";
import { ErrorState } from "@components/support/ErrorState";

type ProjectTabId = "overview" | "issues" | "deployments";

const TABS: { id: ProjectTabId; label: string }[] = [
  { id: "overview", label: "Overview" },
  { id: "issues", label: "Issues" },
  { id: "deployments", label: "Deployments" },
];

export default function ProjectDetailPage() {
  const { id } = useParams<{ id: string }>();
  const { data, isLoading, isError, refetch } = useQuery(projects.get(id ?? ""));
  const [activeTab, setActiveTab] = useState<ProjectTabId>("overview");

  if (isLoading) {
    return (
      <Stack gap={2}>
        <Skeleton variant="rounded" height={28} width="60%" />
        <Skeleton variant="rounded" height={260} />
      </Stack>
    );
  }

  if (isError || !data) {
    return <ErrorState onRetry={() => void refetch()} />;
  }

  const p = data;

  return (
    <Stack gap={2}>
      <Stack gap={0.5}>
        <Stack direction="row" alignItems="center" gap={1} flexWrap="wrap">
          <Typography variant="h6">{p.name}</Typography>
          <Chip size="small" variant="outlined" label={formatEnumLabel(p.subscriptionType)} />
        </Stack>
        <Typography variant="caption" color="text.secondary" sx={{ fontFamily: "monospace" }}>
          {p.key ?? "—"}
        </Typography>
      </Stack>

      <Tabs variant="scrollable" value={activeTab} onChange={(_, value: ProjectTabId) => setActiveTab(value)}>
        {TABS.map((tab) => (
          <Tab key={tab.id} label={tab.label} value={tab.id} disableRipple />
        ))}
      </Tabs>

      {activeTab === "overview" && (
        <Card variant="outlined" sx={{ p: 2 }}>
          <Stack gap={1.5}>
            <MetaRow label="Project key">
              <MetaValue mono>{p.key ?? "—"}</MetaValue>
            </MetaRow>
            <Divider />
            <MetaRow label="Subscription">
              <MetaValue>{formatEnumLabel(p.subscriptionType)}</MetaValue>
            </MetaRow>
            <Divider />
            <MetaRow label="Account">
              {p.account ? (
                <Typography
                  component={Link}
                  to={`/more/customers/accounts/${p.account.id}`}
                  variant="body2"
                  align="right"
                  sx={{ color: "primary.main", textDecoration: "none" }}
                >
                  {p.account.name || p.account.id}
                </Typography>
              ) : (
                <MetaValue>—</MetaValue>
              )}
            </MetaRow>
            <Divider />
            <MetaRow label="Account tier">
              <MetaValue sx={{ textTransform: "capitalize" }}>{p.account?.tier || "—"}</MetaValue>
            </MetaRow>
            <Divider />
            <MetaRow label="Start date">
              <MetaValue>{formatDateOnly(p.startDate)}</MetaValue>
            </MetaRow>
            <Divider />
            <MetaRow label="End date">
              <MetaValue>{formatDateOnly(p.endDate)}</MetaValue>
            </MetaRow>
            <Divider />
            <MetaRow label="Salesforce ID">
              <MetaValue mono>{p.sfId || "—"}</MetaValue>
            </MetaRow>
            <Divider />
            <MetaRow label="Created">
              <MetaValue>{formatDateOnly(p.createdOn)}</MetaValue>
            </MetaRow>
            <Divider />
            <MetaRow label="Last updated">
              <MetaValue>{formatDateOnly(p.updatedOn)}</MetaValue>
            </MetaRow>
            <Divider />
            <MetaRow label="Project ID">
              <MetaValue mono>{p.id}</MetaValue>
            </MetaRow>
          </Stack>
        </Card>
      )}

      {activeTab === "issues" && <ProjectIssuesTab projectId={p.id} />}

      {activeTab === "deployments" && <DeploymentsTab projectId={p.id} />}
    </Stack>
  );
}
