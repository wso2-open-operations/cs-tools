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

import {
  CalendarMonth,
  Chat,
  ChevronRight,
  Circle,
  ReplayCircleFilled,
  Report,
  Schedule,
  Settings,
  Try,
} from "@mui/icons-material";
import { Box, ButtonBase as Button, Card, Chip, Divider, Grid, Stack, Tab, Tabs, Typography } from "@mui/material";
import { useState } from "react";
import { Link } from "react-router-dom";

interface TabPanelProps {
  children?: React.ReactNode;
  index: number;
  value: number;
}

function TabPanel(props: TabPanelProps) {
  const { children, value, index, ...other } = props;

  return (
    <Box role="tabpanel" hidden={value !== index} id={`tab-${index}`} {...other}>
      {value === index && <>{children}</>}
    </Box>
  );
}

export default function SupportPage() {
  const [value, setValue] = useState(0);

  const handleChange = (event: React.SyntheticEvent, newValue: number) => {
    setValue(newValue);
  };

  return (
    <Box p={2} mb={20}>
      <Grid spacing={1.5} container>
        <Grid size={3}>
          <Card component={Stack} p={1.2} gap={0.5} elevation={0}>
            <Typography variant="h4" fontWeight="bold">
              10
            </Typography>
            <Typography variant="subtitle1" fontWeight="medium" color="text.secondary">
              Open Cases
            </Typography>
          </Card>
        </Grid>
        <Grid size={3}>
          <Card component={Stack} p={1.2} gap={0.5} elevation={0}>
            <Typography variant="h4" fontWeight="bold">
              8
            </Typography>
            <Typography variant="subtitle1" fontWeight="medium" color="text.secondary">
              Active Chats
            </Typography>
          </Card>
        </Grid>
        <Grid size={3}>
          <Card component={Stack} p={1.2} gap={0.5} elevation={0}>
            <Typography variant="h4" fontWeight="bold">
              3
            </Typography>
            <Typography variant="subtitle1" fontWeight="medium" color="text.secondary">
              Service Requests
            </Typography>
          </Card>
        </Grid>
        <Grid size={3}>
          <Card component={Stack} p={1.2} gap={0.5} elevation={0}>
            <Typography variant="h4" fontWeight="bold">
              2
            </Typography>
            <Typography variant="subtitle1" fontWeight="medium" color="text.secondary">
              Change Requests
            </Typography>
          </Card>
        </Grid>
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
      <Tabs
        value={value}
        onChange={handleChange}
        variant="fullWidth"
        sx={(theme) => ({
          mt: 3,

          "& .MuiButtonBase-root": {
            p: 0,
            fontSize: theme.typography.body2,
            color: "text.tertiary",
            fontWeight: "medium",
            textTransform: "revert",
            borderRadius: 8,
          },

          "& .Mui-selected": {
            backgroundColor: "primary.contrastText",
            color: "primary.main",
            fontWeight: "bold",
          },

          "& .MuiTabs-indicator": {
            display: "none",
          },
        })}
      >
        <Tab label="Cases" disableRipple />
        <Tab label="Chats" disableRipple />
        <Tab label="Services" disableRipple />
        <Tab label="Changes" disableRipple />
      </Tabs>
      <Card component={Stack} p={2} mt={2} gap={0.5} elevation={0}>
        <TabPanel value={value} index={0}>
          <Stack direction="row" justifyContent="space-between" pb={1}>
            <Typography variant="h6" fontWeight="bold">
              Open Cases
            </Typography>
            <Button component={Link} to="/cases/all" sx={{ padding: 0 }} disableRipple>
              <Stack direction="row" gap={2}>
                <Typography variant="body2" color="primary" fontWeight="medium">
                  View All
                </Typography>
                <ChevronRight color="primary" />
              </Stack>
            </Button>
          </Stack>
          <Divider />
          <Stack gap={2} pt={1} divider={<Divider />}>
            <CaseCard />
            <CaseCard />
            <CaseCard />
          </Stack>
        </TabPanel>
        <TabPanel value={value} index={1}>
          <Stack direction="row" justifyContent="space-between" pb={1}>
            <Typography variant="h6" fontWeight="bold">
              Chat History
            </Typography>
            <Button component={Link} to="/cases/all" sx={{ padding: 0 }} disableRipple>
              <Stack direction="row" gap={2}>
                <Typography variant="body2" color="primary" fontWeight="medium">
                  View All
                </Typography>
                <ChevronRight color="primary" />
              </Stack>
            </Button>
          </Stack>
          <Divider />
          <Stack gap={2} pt={1} divider={<Divider />}>
            <CaseCard2 />
            <CaseCard2 />
            <CaseCard2 />
          </Stack>
        </TabPanel>
        <TabPanel value={value} index={2}>
          <Stack direction="row" justifyContent="space-between" pb={1}>
            <Typography variant="h6" fontWeight="bold">
              Service Requests
            </Typography>
            <Button component={Link} to="/cases/all" sx={{ padding: 0 }} disableRipple>
              <Stack direction="row" gap={2}>
                <Typography variant="body2" color="primary" fontWeight="medium">
                  View All
                </Typography>
                <ChevronRight color="primary" />
              </Stack>
            </Button>
          </Stack>
          <Divider />
          <Stack gap={2} pt={1} divider={<Divider />}>
            <CaseCard3 />
            <CaseCard3 />
            <CaseCard3 />
          </Stack>
        </TabPanel>
        <TabPanel value={value} index={3}>
          <Stack direction="row" justifyContent="space-between" pb={1}>
            <Typography variant="h6" fontWeight="bold">
              Change Requests
            </Typography>
            <Button component={Link} to="/cases/all" sx={{ padding: 0 }} disableRipple>
              <Stack direction="row" gap={2}>
                <Typography variant="body2" color="primary" fontWeight="medium">
                  View All
                </Typography>
                <ChevronRight color="primary" />
              </Stack>
            </Button>
          </Stack>
          <Divider />
          <Stack gap={2} pt={1} divider={<Divider />}>
            <CaseCard4 />
            <CaseCard4 />
            <CaseCard4 />
          </Stack>
        </TabPanel>
      </Card>
    </Box>
  );
}

function CaseCard() {
  return (
    <Card component={Link} to="/cases/1" elevation={0} sx={{ textDecoration: "none" }}>
      <Stack gap={0.8}>
        <Stack direction="row" justifyContent="space-between" alignItems="center">
          <Stack direction="row" alignItems="center" gap={1}>
            <Report sx={(theme) => ({ color: "text.secondary", fontSize: theme.typography.pxToRem(21) })} />
            <Typography variant="subtitle2" fontWeight="regular" color="text.secondary">
              CASE-2845
            </Typography>
            <Chip size="small" label="High" color="error" />
          </Stack>
          <ChevronRight sx={{ color: "text.tertiary" }} />
        </Stack>
        <Typography variant="body1">Identity Server LDAP connection failures</Typography>
        <Stack direction="row" alignItems="center" gap={1}>
          <Chip size="small" label="In Progress" color="error" sx={{ borderRadius: 1 }} />
          <Circle sx={(theme) => ({ color: "text.tertiary", fontSize: theme.typography.pxToRem(4) })} />
          <Typography variant="subtitle2" fontWeight="regular" color="text.secondary">
            Lithika Damnod
          </Typography>
        </Stack>
        <Stack direction="row" alignItems="center" gap={1} mt={1}>
          <Schedule sx={(theme) => ({ color: "text.tertiary", fontSize: theme.typography.pxToRem(18) })} />
          <Typography
            fontWeight="regular"
            color="text.tertiary"
            sx={(theme) => ({ fontSize: theme.typography.pxToRem(14) })}
          >
            2 Hours Ago
          </Typography>
        </Stack>
      </Stack>
    </Card>
  );
}

function CaseCard2() {
  return (
    <Card component={Link} to="/cases/1" elevation={0} sx={{ textDecoration: "none" }}>
      <Stack gap={0.8}>
        <Stack direction="row" justifyContent="space-between" alignItems="center">
          <Stack direction="row" alignItems="center" gap={1}>
            <Chat sx={(theme) => ({ color: "semantic.portal.accent.blue", fontSize: theme.typography.pxToRem(21) })} />
            <Typography variant="subtitle2" fontWeight="regular" color="text.secondary">
              CH-2845
            </Typography>
            <Chip size="small" label="High" color="error" />
          </Stack>
          <ChevronRight sx={{ color: "text.tertiary" }} />
        </Stack>
        <Typography variant="body1">JWT Token Configuration Help</Typography>
        <Stack direction="row" alignItems="center" gap={1}>
          <Chip size="small" label="In Progress" color="error" sx={{ borderRadius: 1 }} />
          <Circle sx={(theme) => ({ color: "text.tertiary", fontSize: theme.typography.pxToRem(4) })} />
          <Typography variant="subtitle2" fontWeight="regular" color="text.secondary">
            12 messages
          </Typography>
          <Circle sx={(theme) => ({ color: "text.tertiary", fontSize: theme.typography.pxToRem(4) })} />
          <Typography variant="subtitle2" fontWeight="regular" color="text.secondary">
            5 KB
          </Typography>
        </Stack>
        <Stack direction="row" alignItems="center" gap={1} mt={1}>
          <Schedule sx={(theme) => ({ color: "text.tertiary", fontSize: theme.typography.pxToRem(18) })} />
          <Typography
            fontWeight="regular"
            color="text.tertiary"
            sx={(theme) => ({ fontSize: theme.typography.pxToRem(14) })}
          >
            1 day ago
          </Typography>
        </Stack>
      </Stack>
    </Card>
  );
}

function CaseCard3() {
  return (
    <Card component={Link} to="/cases/1" elevation={0} sx={{ textDecoration: "none" }}>
      <Stack gap={0.8}>
        <Stack direction="row" justifyContent="space-between" alignItems="center">
          <Stack direction="row" alignItems="center" gap={1}>
            <Settings
              sx={(theme) => ({ color: "semantic.portal.accent.purple", fontSize: theme.typography.pxToRem(21) })}
            />
            <Typography variant="subtitle2" fontWeight="regular" color="text.secondary">
              SR-2845
            </Typography>
            <Chip size="small" label="High" color="error" />
          </Stack>
          <ChevronRight sx={{ color: "text.tertiary" }} />
        </Stack>
        <Typography variant="body1">JWT Token Configuration Help</Typography>
        <Stack direction="row" alignItems="center" gap={1}>
          <Chip size="small" label="In Progress" color="error" sx={{ borderRadius: 1 }} />
          <Circle sx={(theme) => ({ color: "text.tertiary", fontSize: theme.typography.pxToRem(4) })} />
          <Typography variant="subtitle2" fontWeight="regular" color="text.secondary">
            Infrastructure
          </Typography>
        </Stack>
        <Stack direction="row" alignItems="center" gap={1} mt={1}>
          <Schedule sx={(theme) => ({ color: "text.tertiary", fontSize: theme.typography.pxToRem(18) })} />
          <Typography
            fontWeight="regular"
            color="text.tertiary"
            sx={(theme) => ({ fontSize: theme.typography.pxToRem(14) })}
          >
            1 day ago
          </Typography>
        </Stack>
      </Stack>
    </Card>
  );
}

function CaseCard4() {
  return (
    <Card component={Link} to="/cases/1" elevation={0} sx={{ textDecoration: "none" }}>
      <Stack gap={0.8}>
        <Stack direction="row" justifyContent="space-between" alignItems="center">
          <Stack direction="row" alignItems="center" gap={1}>
            <ReplayCircleFilled
              sx={(theme) => ({ color: "semantic.portal.accent.blue", fontSize: theme.typography.pxToRem(21) })}
            />
            <Typography variant="subtitle2" fontWeight="regular" color="text.secondary">
              CR-2845
            </Typography>
            <Chip size="small" label="Impact: High" color="error" />
          </Stack>
          <ChevronRight sx={{ color: "text.tertiary" }} />
        </Stack>
        <Typography variant="body1">Update API Gateway security policies</Typography>
        <Stack direction="row" alignItems="center" gap={1}>
          <Chip size="small" label="In Progress" color="error" sx={{ borderRadius: 1 }} />
          <Circle sx={(theme) => ({ color: "text.tertiary", fontSize: theme.typography.pxToRem(4) })} />
          <Typography variant="subtitle2" fontWeight="regular" color="text.secondary">
            Security Update
          </Typography>
        </Stack>
        <Stack direction="row" alignItems="center" gap={1}>
          <CalendarMonth sx={(theme) => ({ fontSize: theme.typography.pxToRem(18), color: "text.secondary" })} />
          <Typography variant="subtitle2" fontWeight="regular" color="text.secondary">
            Scheduled: Nov 25,2025
          </Typography>
        </Stack>
        <Stack direction="row" alignItems="center" gap={1} mt={1}>
          <Schedule sx={(theme) => ({ color: "text.tertiary", fontSize: theme.typography.pxToRem(18) })} />
          <Typography
            fontWeight="regular"
            color="text.tertiary"
            sx={(theme) => ({ fontSize: theme.typography.pxToRem(14) })}
          >
            1 day ago
          </Typography>
        </Stack>
      </Stack>
    </Card>
  );
}
