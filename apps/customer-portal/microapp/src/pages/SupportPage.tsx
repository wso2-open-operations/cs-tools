// Copyright (c) 2025 WSO2 LLC. (https://www.wso2.com).
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

import { Try } from "@mui/icons-material";
import { ButtonBase as Button, Card, Grid, Stack, styled, Tab, Tabs as MuiTabs, Typography } from "@mui/material";
import { useState } from "react";
import { Link } from "react-router-dom";
import { MetricWidget } from "@components/features/dashboard";
import { ItemListView, ItemCard, type ItemCardProps } from "@components/features/support";

import { MOCK_METRICS, MOCK_ITEMS } from "@src/mocks/data/support";

export default function SupportPage() {
  const [tab, setTab] = useState<ItemCardProps["type"]>("case");

  return (
    <>
      <Grid spacing={1.5} container>
        {MOCK_METRICS.map((props) => (
          <Grid size={3}>
            <MetricWidget {...props} size="small" base />
          </Grid>
        ))}
        <Grid size={12}>
          <Card component={Stack} direction="row" alignItems="center" px={2} py={1.5} gap={2} elevation={0}>
            <Try fontSize="large" color="primary" />
            <Stack>
              <Typography variant="body1" fontWeight="medium" color="primary">
                Create Support Ticket
              </Typography>
              <Typography variant="subtitle2" fontWeight="medium" color="text.tertiary">
                Chat with Novera or Create Support Case
              </Typography>
            </Stack>
            <Button
              component={Link}
              to="/chat"
              variant="contained"
              sx={{ fontWeight: "bold", flexShrink: 0, height: 40 }}
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
        <ItemListView title="Open Cases" viewAllPath={`/${tab}s/all`}>
          {MOCK_ITEMS[tab].map((item, index) => (
            <ItemCard key={index} {...item} />
          ))}
        </ItemListView>
      </Card>
    </>
  );
}

const Tabs = styled(MuiTabs)(({ theme }) => ({
  "& .MuiButtonBase-root": {
    fontSize: theme.typography.body2,
    color: theme.palette.text.secondary,
    fontWeight: "medium",
    textTransform: "revert",
  },

  "& .Mui-selected": {
    backgroundColor: theme.palette.primary.contrastText,
    color: theme.palette.primary.main,
    fontWeight: "bold",
    borderRadius: 999,
  },

  "& .MuiTabs-indicator": {
    display: "none",
  },
}));
