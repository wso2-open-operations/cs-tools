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

import { Attachment, AttachmentOutlined, CircleOutlined, PeopleAlt, Person, Send } from "@mui/icons-material";
import { Box, Card, Chip, Grid, IconButton, Stack, InputBase as TextField, Typography } from "@mui/material";
import { useParams } from "react-router-dom";

import Timeline from "@mui/lab/Timeline";
import TimelineItem from "@mui/lab/TimelineItem";
import TimelineSeparator from "@mui/lab/TimelineSeparator";
import TimelineConnector from "@mui/lab/TimelineConnector";
import TimelineContent from "@mui/lab/TimelineContent";

export default function DetailedPage({ type }: { type: string }) {
  const { id } = useParams();

  return (
    <>
      <Stack p={2} mb={20} gap={2}>
        <Card component={Stack} p={1.5} gap={1.5} elevation={0}>
          <Typography variant="h5" fontWeight="medium">
            Case Information
          </Typography>
          <Stack gap={0.5}>
            <Typography variant="subtitle2" fontWeight="regular" color="text.secondary">
              Description
            </Typography>
            <Typography variant="body2" lineHeight={1.6}>
              We are experiencing issues with the authentication service. Users are unable to log in, and we are seeing
              error messages related to JWT token expiration.
            </Typography>
          </Stack>
          <Grid spacing={1.5} container>
            <Grid size={6}>
              <Stack gap={0.5}>
                <Typography variant="subtitle2" fontWeight="regular" color="text.secondary">
                  Assignee
                </Typography>
                <Stack direction="row" alignItems="center" gap={1}>
                  <PeopleAlt sx={(theme) => ({ fontSize: theme.typography.pxToRem(20) })} />
                  <Typography variant="body2">Support Team</Typography>
                </Stack>
              </Stack>
            </Grid>
            <Grid size={6}>
              <Stack gap={0.5}>
                <Typography variant="subtitle2" fontWeight="regular" color="text.secondary">
                  Reporter
                </Typography>
                <Stack direction="row" alignItems="center" gap={1}>
                  <Person sx={(theme) => ({ fontSize: theme.typography.pxToRem(20) })} />
                  <Typography variant="body2">John Smith</Typography>
                </Stack>
              </Stack>
            </Grid>
            <Grid size={6}>
              <Stack gap={0.5}>
                <Typography variant="subtitle2" fontWeight="regular" color="text.secondary">
                  Category
                </Typography>
                <Stack direction="row" alignItems="center" gap={1}>
                  <Typography variant="body2">Technical Issue</Typography>
                </Stack>
              </Stack>
            </Grid>
            <Grid size={6}>
              <Stack gap={0.5}>
                <Typography variant="subtitle2" fontWeight="regular" color="text.secondary">
                  Severity
                </Typography>
                <Stack direction="row" alignItems="center" gap={1}>
                  <Typography variant="body2">Critical</Typography>
                </Stack>
              </Stack>
            </Grid>
            <Grid size={6}>
              <Stack gap={0.5}>
                <Typography variant="subtitle2" fontWeight="regular" color="text.secondary">
                  Affected Service
                </Typography>
                <Stack direction="row" alignItems="center" gap={1}>
                  <Typography variant="body2">Authentication Service</Typography>
                </Stack>
              </Stack>
            </Grid>
            <Grid size={6} />
            <Grid size={6}>
              <Stack gap={0.5}>
                <Typography variant="subtitle2" fontWeight="regular" color="text.secondary">
                  Created
                </Typography>
                <Stack direction="row" alignItems="center" gap={1}>
                  <Typography variant="body2">Nov 18, 2025 10:30 AM</Typography>
                </Stack>
              </Stack>
            </Grid>
            <Grid size={6}>
              <Stack gap={0.5}>
                <Typography variant="subtitle2" fontWeight="regular" color="text.secondary">
                  Last Updated
                </Typography>
                <Stack direction="row" alignItems="center" gap={1}>
                  <Typography variant="body2">6 hours ago</Typography>
                </Stack>
              </Stack>
            </Grid>
          </Grid>
        </Card>
        <Card component={Stack} p={1.5} gap={1.5} elevation={0}>
          <Typography variant="h5" fontWeight="medium">
            Activity Timeline
          </Typography>

          <Timeline
            position="right"
            sx={{
              p: 0,
              [`& .MuiTimelineItem-root:before`]: {
                flex: 0,
                padding: 0,
              },
            }}
          >
            <TimelineItem sx={{ minHeight: "auto" }}>
              <TimelineSeparator>
                <CircleOutlined sx={{ color: "text.tertiary", fontSize: 20 }} />
                <TimelineConnector />
              </TimelineSeparator>
              <TimelineContent sx={{ p: 0, ml: 1.5, mb: 2 }}>
                <Stack direction="row" justifyContent="space-between" gap={1}>
                  <Stack direction="row" gap={0.5}>
                    <Typography variant="body2">
                      <Box component="span" fontWeight="bold">
                        System
                      </Box>{" "}
                      created this case
                    </Typography>
                    <Typography variant="body2" color="text.secondary"></Typography>
                  </Stack>
                  <Typography variant="subtitle2" fontWeight="regular" color="text.disabled" flexShrink={0}>
                    2 days ago
                  </Typography>
                </Stack>
              </TimelineContent>
            </TimelineItem>
            <TimelineItem sx={{ minHeight: "auto" }}>
              <TimelineSeparator>
                <CircleOutlined sx={{ color: "text.tertiary", fontSize: 20 }} />
                <TimelineConnector />
              </TimelineSeparator>
              <TimelineContent sx={{ p: 0, ml: 1.5, mb: 2 }}>
                <Stack direction="row" justifyContent="space-between" gap={1}>
                  <Stack direction="row" gap={0.5}>
                    <Typography variant="body2">
                      <Box component="span" fontWeight="bold">
                        System
                      </Box>{" "}
                      assigned to Support Team
                    </Typography>
                    <Typography variant="body2" color="text.secondary"></Typography>
                  </Stack>
                  <Typography variant="subtitle2" fontWeight="regular" color="text.disabled" flexShrink={0}>
                    2 days ago
                  </Typography>
                </Stack>
              </TimelineContent>
            </TimelineItem>
            <TimelineItem sx={{ minHeight: "auto" }}>
              <TimelineSeparator>
                <CircleOutlined sx={{ color: "text.tertiary", fontSize: 20 }} />
              </TimelineSeparator>
              <TimelineContent component={Stack} gap={1.5} sx={{ p: 0, ml: 1.5, mb: 2 }}>
                <Stack direction="row" justifyContent="space-between" gap={1}>
                  <Stack direction="row" gap={0.5}>
                    <Typography variant="body2">
                      <Box component="span" fontWeight="bold">
                        System
                      </Box>{" "}
                      assigned to Support Team
                    </Typography>
                    <Typography variant="body2" color="text.secondary"></Typography>
                  </Stack>
                  <Typography variant="subtitle2" fontWeight="regular" color="text.disabled" flexShrink={0}>
                    2 days ago
                  </Typography>
                </Stack>
                <Card variant="outlined" sx={{ p: 1.5, bgcolor: "action.hover", border: "none" }}>
                  <Typography variant="body2">
                    I've reviewed your case. Could you please share the error logs from your authentication service?
                  </Typography>
                  <Stack direction="row" alignItems="center" pt={1} gap={1}>
                    <AttachmentOutlined
                      sx={(theme) => ({ fontSize: theme.typography.pxToRem(20), color: "text.secondary" })}
                    />
                    <Typography variant="subtitle1" color="text.secondary">
                      auth-service-logs.txt
                    </Typography>
                  </Stack>
                </Card>
              </TimelineContent>
            </TimelineItem>
          </Timeline>
        </Card>
      </Stack>
      <Stack
        position="fixed"
        width="100%"
        p={2}
        bottom={100}
        gap={4}
        justifyContent="space-between"
        bgcolor="background.paper"
      >
        <Stack direction="row" gap={2}>
          <TextField placeholder="Add comment" fullWidth sx={{ alignSelf: "center" }} />
          <IconButton color="primary">
            <Send sx={(theme) => ({ color: "text.tertiary", fontSize: theme.typography.pxToRem(21) })} />
          </IconButton>
        </Stack>
      </Stack>
    </>
  );
}

export function DetailedPageAppBarSlot() {
  return (
    <Stack direction="row" gap={1.5} mt={1}>
      <Chip label="In Progress" color="error" sx={{ borderRadius: 1 }} />
      <Chip label="High" color="error" />
    </Stack>
  );
}
