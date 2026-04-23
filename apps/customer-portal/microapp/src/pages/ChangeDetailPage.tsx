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

import { useLayoutEffect, useState } from "react";
import { Box, Card, Chip, colors, Grid, pxToRem, Stack, Typography } from "@wso2/oxygen-ui";
import { InfoField, OverlineSlot, StakeholderItem, StickyCommentBar, TimelineEntry } from "@components/features/detail";
import { PriorityChip, StatusChip } from "@components/features/support";
import { ChecklistItem } from "@components/features/chat";
import { SectionCard } from "@components/shared";
import { Timeline } from "@components/ui";
import { useLayout } from "@context/layout";

import { MOCK_ACTIVITY_TIMELINE, MOCK_IMPLEMENTATION_STEPS, MOCK_STAKEHOLDERS } from "@src/mocks/data/change";
import { Calendar, TriangleAlert, User, Users } from "@wso2/oxygen-ui-icons-react";

export default function ChangeDetailPage() {
  const layout = useLayout();
  const [comment, setComment] = useState("");
  const [activities, setActivities] = useState(MOCK_ACTIVITY_TIMELINE);

  const handleSend = () => {
    if (!comment.trim()) return;

    setActivities((prev) => [...prev, { author: "You", description: comment, timestamp: "Just Now" }]);
  };

  const AppBarSlot = () => (
    <Stack direction="row" gap={1.5} mt={1}>
      <StatusChip status="scheduled" size="small" />
      <PriorityChip prefix="Impact" priority="low" size="small" />
      <Chip label="Security Update" size="small" />
    </Stack>
  );

  useLayoutEffect(() => {
    layout.setTitleOverride("Update API Gateway security policies");
    layout.setOverlineSlotOverride(<OverlineSlot type="change" id="CR-1234" />);
    layout.setAppBarSlotsOverride(<AppBarSlot />);

    return () => {
      layout.setTitleOverride(undefined);
      layout.setOverlineSlotOverride(undefined);
      layout.setAppBarSlotsOverride(undefined);
    };
  }, []);

  return (
    <>
      <Stack gap={2} mb={10}>
        <MaintenanceNoticeCard />
        <SectionCard title="Change Information">
          <Grid spacing={1.5} container>
            <Grid size={12}>
              <InfoField
                label="Description"
                value="Update API Gateway security policies to implement new authentication requirements and enhanced rate limiting. This change will improve security posture and prevent potential DDoS attacks."
              />
            </Grid>
            <Grid size={6}>
              <InfoField label="Change Owner" value="Sarah Chen" icon={User} />
            </Grid>
            <Grid size={6}>
              <InfoField label="Requested By" value="Security Team" icon={Users} />
            </Grid>
            <Grid size={6}>
              <InfoField label="Priority" value={<PriorityChip size="small" priority="medium" />} />
            </Grid>
            <Grid size={6}>
              <InfoField label="Estimated Duration" value="1 hour" />
            </Grid>
            <Grid size={6}>
              <InfoField label="Approval Status" value={<StatusChip size="small" status="approved" />} />
            </Grid>
            <Grid size={6}>
              <InfoField label="Approved By" value="Change Advisory Board" />
            </Grid>
          </Grid>
        </SectionCard>
        <SectionCard title="Impact Assessment">
          <Grid spacing={1.5} container>
            <Grid size={12}>
              <InfoField
                label="Affected Services"
                value={
                  <Stack direction="row" gap={1}>
                    <Chip size="small" label="API Gateway" />
                    <Chip size="small" label="Authentication Service" />
                  </Stack>
                }
              />
            </Grid>
            <Grid size={12}>
              <InfoField label="Affected Users" value="None - backend configuration only" />
            </Grid>
            <Grid size={12}>
              <InfoField label="Expected Downtime" value={<ChecklistItem>No downtime expected</ChecklistItem>} />
            </Grid>
            <Grid size={12}>
              <InfoField
                label="Rollback Plan"
                value={
                  <ChecklistItem icon={TriangleAlert} color="warning">
                    Automated rollback to previous policy version available
                  </ChecklistItem>
                }
              />
            </Grid>
          </Grid>
        </SectionCard>
        <SectionCard title="Implementation Steps">
          <Timeline>
            {MOCK_IMPLEMENTATION_STEPS.map((step, index) => (
              <TimelineEntry
                key={index}
                variant="step"
                index={index + 1}
                title={step.title}
                description={step.description}
                timestamp={step.timestamp}
                last={index === MOCK_IMPLEMENTATION_STEPS.length - 1}
              />
            ))}
          </Timeline>
        </SectionCard>
        <SectionCard title="Stakeholders">
          <Stack gap={1.5}>
            {MOCK_STAKEHOLDERS.map((stakeholder, index) => (
              <StakeholderItem key={index} name={stakeholder.name} role={stakeholder.role} />
            ))}
          </Stack>
        </SectionCard>
        <SectionCard title="Activity Timeline">
          <Timeline>
            {activities.map((props, index) => (
              <TimelineEntry key={index} variant="activity" {...props} last={index === activities.length - 1} />
            ))}
          </Timeline>
          <Box
            sx={{
              "& > .MuiStack-root": {
                position: "static !important",
                bottom: "auto !important",
                p: 0,
                m: 0,
              },
            }}
          >
            <StickyCommentBar placeholder="Add Comment" value={comment} onChange={setComment} onSend={handleSend} />
          </Box>
        </SectionCard>
      </Stack>
    </>
  );
}

export function MaintenanceNoticeCard() {
  return (
    <Card
      component={Stack}
      direction="row"
      alignItems="center"
      px={2}
      py={1.5}
      gap={2}
      sx={{ bgcolor: colors.yellow[50] }}
    >
      <Box color="primary.main">
        <Calendar size={pxToRem(26)} />
      </Box>
      <Stack>
        <Typography variant="body1" fontWeight="medium" color="primary">
          Scheduled Maintenance Window
        </Typography>
        <Typography variant="subtitle2" fontWeight="medium" color="text.tertiary">
          Nov 25, 2025 &nbsp; 10:00 PM - 11:00 PM EST
        </Typography>
      </Stack>
    </Card>
  );
}
