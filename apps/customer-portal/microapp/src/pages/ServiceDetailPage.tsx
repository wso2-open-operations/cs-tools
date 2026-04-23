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
import { Chip, Grid, Stack } from "@wso2/oxygen-ui";
import { User, Users } from "@wso2/oxygen-ui-icons-react";
import { Timeline } from "@mui/lab";
import { Comment, InfoField, OverlineSlot, StickyCommentBar, TimelineEntry } from "@components/features/detail";
import { PriorityChip, StatusChip } from "@components/features/support";
import { ChecklistItem } from "@components/features/chat";
import { useLayout } from "@context/layout";

import { MOCK_REQUIREMENTS, MOCK_TIMELINE_DATA, MOCK_UPDATES } from "@src/mocks/data/service";
import { SectionCard } from "@components/shared";

export default function ServiceDetailPage() {
  const layout = useLayout();
  const [comment, setComment] = useState("");
  const [updates, setUpdates] = useState(MOCK_UPDATES);

  const handleSend = () => {
    if (!comment.trim()) return;
    setUpdates((prev) => [...prev, { author: "You", timestamp: "Just Now", content: comment }]);
  };

  const AppBarSlot = () => (
    <Stack direction="row" gap={1.5} mt={1}>
      <StatusChip status="in progress" size="small" />
      <PriorityChip priority="high" size="small" />
      <Chip label="Environment Setup" size="small" />
    </Stack>
  );

  useLayoutEffect(() => {
    layout.setTitleOverride("Enable additional API Manager environment");
    layout.setOverlineSlotOverride(<OverlineSlot type="service" id="SR-1234" />);
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
        <SectionCard title="Request Information">
          <Grid spacing={1.5} container>
            <Grid size={12}>
              <InfoField
                label="Description"
                value="We need to enable an additional staging environment for API Manager to support our development team's testing requirements. This environment should mirror our production setup with the same configuration and policies."
              />
            </Grid>
            <Grid size={6}>
              <InfoField label="Requested By" value="John Smith" icon={User} />
            </Grid>
            <Grid size={6}>
              <InfoField label="Assigned To" value="DevOps Team" icon={Users} />
            </Grid>
            <Grid size={6}>
              <InfoField label="Created" value="Nov 18, 2025 10:30 AM" />
            </Grid>
            <Grid size={6}>
              <InfoField label="Target Completion" value="Nov 20, 2025" />
            </Grid>
            <Grid size={6}>
              <InfoField label="Estimated Effort" value="4-6 hours" />
            </Grid>
            <Grid size={6}>
              <InfoField label="Approval Status" value={<StatusChip size="small" status="approved" />} />
            </Grid>
            <Grid size={6}>
              <InfoField label="Approved By" value="Manager - Jane Doe" />
            </Grid>
          </Grid>
        </SectionCard>
        <SectionCard title="Requirements">
          <Stack gap={0.5}>
            {MOCK_REQUIREMENTS.map((text, index) => (
              <ChecklistItem key={index} variant="checkbox">
                {text}
              </ChecklistItem>
            ))}
          </Stack>
        </SectionCard>
        <SectionCard title="Progress Timeline">
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
            {MOCK_TIMELINE_DATA.map((step, index) => (
              <TimelineEntry
                key={index}
                variant="progress"
                status={step.status}
                title={step.title}
                description={step.description}
                timestamp={step.timestamp}
                last={index === MOCK_TIMELINE_DATA.length - 1}
              />
            ))}
          </Timeline>
        </SectionCard>
        <SectionCard title="Updates">
          <Stack gap={2} pt={1}>
            {updates.map(({ author, timestamp, content }, index) => (
              <Comment author={author} timestamp={timestamp} key={index}>
                {content}
              </Comment>
            ))}
          </Stack>
        </SectionCard>
      </Stack>
      <StickyCommentBar placeholder="Add Comment" value={comment} onChange={setComment} onSend={handleSend} />
    </>
  );
}
