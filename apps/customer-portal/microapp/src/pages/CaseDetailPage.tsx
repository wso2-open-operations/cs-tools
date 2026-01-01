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

import { PeopleAlt, Person } from "@mui/icons-material";
import { Card, Grid, Stack, Typography } from "@mui/material";
import { Timeline } from "@mui/lab";
import { InfoField, StickyCommentBar, TimelineEntry } from "@components/features/detail";
import { PriorityChip, StatusChip } from "@components/features/support";
import { useState } from "react";

export default function CaseDetailPage() {
  const [comment, setComment] = useState("");

  const handleSend = () => {};

  return (
    <>
      <Stack gap={2} mb={10}>
        <Card component={Stack} p={1.5} gap={1.5} elevation={0}>
          <Typography variant="h5" fontWeight="medium">
            Case Information
          </Typography>
          <Grid spacing={1.5} container>
            <Grid size={12}>
              <InfoField
                label="Description"
                value="We are experiencing issues with the authentication service. Users are unable to log in, and we are seeing
            error messages related to JWT token expiration."
              />
            </Grid>
            <Grid size={6}>
              <InfoField label="Assignee" value="Support Team" icon={PeopleAlt} />
            </Grid>
            <Grid size={6}>
              <InfoField label="Reporter" value="John Smith" icon={Person} />
            </Grid>
            <Grid size={6}>
              <InfoField label="Category" value="Technical Issue" />
            </Grid>
            <Grid size={6}>
              <InfoField label="Severity" value="Critical" />
            </Grid>
            <Grid size={12}>
              <InfoField label="Affected Service" value="Authentication Service" />
            </Grid>
            <Grid size={6}>
              <InfoField label="Created" value="Nov 18, 2025 10:30 AM" />
            </Grid>
            <Grid size={6}>
              <InfoField label="Last Updated" value="6 hours ago" />
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
            <TimelineEntry variant="activity" author="System" title="created this case" timestamp="2 days ago" />
            <TimelineEntry variant="activity" author="System" title="assigned to Support Team" timestamp="2 days ago" />
            <TimelineEntry
              variant="activity"
              author="System"
              title="assigned to Support Team"
              timestamp="2 days ago"
              comment=""
            />
            <TimelineEntry
              variant="activity"
              author="System"
              title="Hey Hey"
              timestamp="2 days ago"
              comment="I've reviewed your case. Could you please share the error logs from your authentication service?"
              attachment="auth-service-logs.txt"
              last
            />
          </Timeline>
        </Card>
      </Stack>
      <StickyCommentBar placeholder="Add Comment" value={comment} onChange={setComment} onSend={handleSend} />
    </>
  );
}

export function DetailedPageAppBarSlot() {
  return (
    <Stack direction="row" gap={1.5} mt={1}>
      <StatusChip status="in progress" />
      <PriorityChip priority="high" />
    </Stack>
  );
}
