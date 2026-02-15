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
import { Link } from "react-router-dom";
import { Button, Card, Grid, pxToRem, Stack, Tab, Tabs, Typography, useTheme } from "@wso2/oxygen-ui";
import { MessageSquareQuote } from "@wso2/oxygen-ui-icons-react";
import { MetricWidget } from "@components/features/dashboard";
import { ItemListView, ItemCard, type ItemCardProps } from "@components/features/support";

import { TAB_TITLES } from "@src/mocks/data/support";
import { useSuspenseQuery } from "@tanstack/react-query";
import { cases } from "@src/services/cases";
import { projects } from "@src/services/projects";
import { useProject } from "@context/project";

export default function SupportPage() {
  const theme = useTheme();
  const [tab, setTab] = useState<ItemCardProps["type"]>("case");

  const { projectId } = useProject();
  const project = useSuspenseQuery(projects.all()).data.find((project) => project.id === projectId);
  const { data } = useSuspenseQuery(cases.all(projectId!, { pagination: { limit: 3 } }));

  const metrics = [
    { label: "Open Cases", value: project?.metrics.cases ?? "N/A" },
    { label: "Active Chats", value: project?.metrics.chats ?? "N/A" },
    { label: "Service Requests", value: "N/A" },
    { label: "Change Requests", value: "N/A" },
  ];

  const items = {
    cases: data,
    chat: [],
    service: [],
    change: [],
  };

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
                  Create Support Ticket
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
        <Tab label="Services" value="service" disableRipple />
        <Tab label="Changes" value="change" disableRipple />
      </Tabs>
      <Card component={Stack} p={2} mt={2} gap={0.5}>
        <ItemListView title={TAB_TITLES[tab]} viewAllPath={`/${tab}s/all`}>
          {items["cases"].map((item) => (
            <ItemCard key={item.id} type="case" to="/" {...item} />
          ))}
        </ItemListView>
      </Card>
    </>
  );
}
