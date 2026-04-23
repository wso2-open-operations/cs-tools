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

import { MOCK_METRICS, MOCK_ITEMS, TAB_TITLES } from "@src/mocks/data/support";

export default function SupportPage() {
  const theme = useTheme();
  const [tab, setTab] = useState<ItemCardProps["type"]>("case");

  return (
    <>
      <Grid spacing={1.5} container>
        {MOCK_METRICS.map((props, index) => (
          <Grid key={index} size={3}>
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
      <Card component={Stack} p={2} mt={2} gap={0.5} elevation={0}>
        <ItemListView title={TAB_TITLES[tab]} viewAllPath={`/${tab}s/all`}>
          {MOCK_ITEMS[tab].map((item, index) => (
            <ItemCard key={index} {...item} />
          ))}
        </ItemListView>
      </Card>
    </>
  );
}
