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

import { Suspense, useState } from "react";
import { Link } from "react-router-dom";
import { Button, Card, Grid, pxToRem, Stack, Tab, Tabs, Typography, useTheme } from "@wso2/oxygen-ui";
import { MessageSquareQuote } from "@wso2/oxygen-ui-icons-react";
import { MetricWidget } from "@components/features/dashboard";
import { ItemListView, ItemCard, type ItemCardProps, ItemCardSkeleton } from "@components/features/support";

import { useSuspenseQuery } from "@tanstack/react-query";
import { cases } from "@src/services/cases";
import { projects } from "@src/services/projects";
import { useProject } from "@context/project";
import { ErrorBoundary } from "@components/core";
import { chats } from "../services/chats";

export const TAB_CONFIG = {
  case: { title: "Open Cases", subtitle: "Active support tickets" },
  chat: { title: "Chat History", subtitle: "Recent Novera coversations" },
  service: { title: "Service Requests", subtitle: "Managed cloud service requests" },
  change: { title: "Change Requests", subtitle: "Scheduled and pending changes" },
};

export const ITEM_DETAIL_PATHS: Record<ItemCardProps["type"], (id: string) => string> = {
  case: (id) => `/cases/${id}`,
  chat: (id) => `/chats/${id}`,
  service: (id) => `/services/${id}`,
  change: (id) => `/changes/${id}`,
};

export default function SupportPage() {
  const theme = useTheme();
  const [tab, setTab] = useState<ItemCardProps["type"]>("case");

  const { projectId } = useProject();
  const project = useSuspenseQuery(projects.all()).data.find((project) => project.id === projectId);

  const metrics = [
    { label: "Open Cases", value: project?.metrics.cases ?? "N/A" },
    { label: "Active Chats", value: project?.metrics.chats ?? "N/A" },
    { label: "Service Requests", value: "N/A" },
    { label: "Change Requests", value: "N/A" },
  ];

  return (
    <>
      <Grid spacing={1.5} container>
        {metrics.map((props) => (
          <Grid size={3}>
            <MetricWidget {...props} size="small" base />
          </Grid>
        ))}
        <Grid size={12}>
          <Card
            component={Stack}
            direction="row"
            justifyContent="space-between"
            alignItems="center"
            px={2}
            py={1.5}
            gap={2}
            elevation={0}
          >
            <Stack direction="row" alignItems="center" gap={2}>
              <MessageSquareQuote size={pxToRem(40)} color={theme.palette.primary.main} />
              <Stack>
                <Typography variant="body1" fontWeight="medium" color="primary">
                  Need help with something new?
                </Typography>
                <Typography variant="subtitle2" fontWeight="medium" color="text.tertiary">
                  Chat with Novera or Create Support Case
                </Typography>
              </Stack>
            </Stack>
            <Button
              component={Link}
              to="/chat"
              variant="contained"
              sx={{ textTransform: "initial", flexShrink: 0, height: 40 }}
            >
              Get Help
            </Button>
          </Card>
        </Grid>
      </Grid>
      <Tabs variant="fullWidth" sx={{ mt: 3 }} value={tab} onChange={(_, value) => setTab(value)}>
        <Tab label="Cases" value="case" disableRipple />
        <Tab label="Chats" value="chat" disableRipple />
        <Tab label="Services Requests" value="service" disableRipple />
        <Tab label="Change Requests" value="change" disableRipple />
      </Tabs>
      <Card component={Stack} p={2} mt={2} gap={0.5}>
        <ItemListView title={TAB_CONFIG[tab].title} subtitle={TAB_CONFIG[tab].subtitle} viewAllPath={`/${tab}s/all`}>
          <ErrorBoundary fallback={<ItemsListContentSkeleton tab={tab} />}>
            <Suspense fallback={<ItemsListContentSkeleton tab={tab} />}>
              <ItemsListContent tab={tab} />
            </Suspense>
          </ErrorBoundary>
        </ItemListView>
      </Card>
    </>
  );
}

function ItemsListContent({ tab }: { tab: ItemCardProps["type"] }) {
  const { projectId } = useProject();
  const caseQuery = tab === "case" ? useSuspenseQuery(cases.all(projectId!, { pagination: { limit: 3 } })) : null;
  const chatQuery = tab === "chat" ? useSuspenseQuery(chats.all(projectId!, { pagination: { limit: 3 } })) : null;

  const items = {
    case: caseQuery?.data,
    chat: chatQuery?.data,
    service: [],
    change: [],
  };

  return (
    <>
      {items[tab]?.map((item) => (
        <ItemCard key={item.id} type={tab} to={ITEM_DETAIL_PATHS[tab](item.id)} {...item} />
      ))}
    </>
  );
}

function ItemsListContentSkeleton({ tab }: { tab: ItemCardProps["type"] }) {
  return (
    <>
      {Array.from({ length: 3 }).map((_, index) => (
        <ItemCardSkeleton key={index} />
      ))}
    </>
  );
}
