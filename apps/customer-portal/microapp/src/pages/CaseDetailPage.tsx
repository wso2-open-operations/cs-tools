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
import { User, Users } from "@wso2/oxygen-ui-icons-react";
import { Grid, Stack } from "@wso2/oxygen-ui";
import { InfoField, OverlineSlot, StickyCommentBar, TimelineEntry } from "@components/features/detail";
import { PriorityChip, StatusChip } from "@components/features/support";
import { SectionCard } from "@components/shared";
import { Timeline } from "@components/ui";
import { useLayout } from "@context/layout";

import { MOCK_ACTIVITY_TIMELINE } from "@src/mocks/data/case";

export default function CaseDetailPage() {
  const layout = useLayout();
  const [comment, setComment] = useState("");
  const [activities, setActivities] = useState(MOCK_ACTIVITY_TIMELINE);

  const handleSend = () => {
    if (!comment.trim()) return;

    setActivities((prev) => [...prev, { author: "You", timestamp: "Just Now", comment: comment }]);
  };

  const AppBarSlot = () => (
    <Stack direction="row" gap={1.5} mt={1}>
      <StatusChip status="in progress" size="small" />
      <PriorityChip priority="high" size="small" />
    </Stack>
  );

  useLayoutEffect(() => {
    layout.setTitleOverride("Authentication Service Issue 2");
    layout.setOverlineSlotOverride(<OverlineSlot type="case" id="CASE-1234" />);
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
        <SectionCard title="Case Information">
          <Grid spacing={1.5} container>
            <Grid size={12}>
              <InfoField
                label="Description"
                value="We are experiencing issues with the authentication service. Users are unable to log in, and we are seeing
            error messages related to JWT token expiration."
              />
            </Grid>
            <Grid size={6}>
              <InfoField label="Assignee" value="Support Team" icon={Users} />
            </Grid>
            <Grid size={6}>
              <InfoField label="Reporter" value="John Smith" icon={User} />
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
        </SectionCard>
        <SectionCard title="Activity Timeline">
          <Timeline>
            {activities.map((props, index) => (
              <TimelineEntry key={index} variant="activity" {...props} last={index === activities.length - 1} />
            ))}
          </Timeline>
        </SectionCard>
      </Stack>
      <StickyCommentBar placeholder="Add Comment" value={comment} onChange={setComment} onSend={handleSend} />
    </>
  );
}
