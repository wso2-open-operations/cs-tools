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

import { Box, Card, Chip, Typography } from "@wso2/oxygen-ui";
import DOMPurify from "dompurify";
import type { JSX } from "react";
import RelativeTime from "@components/RelativeTime";
import {
  ENGAGEMENT_HEALTH_COLOR,
  ENGAGEMENT_HEALTH_LABEL,
} from "@features/csm-engagements/utils/engagements";
import type { CsmEngagementStatusUpdate } from "@features/csm-engagements/types/csmEngagements";

interface EngagementStatusUpdatesPanelProps {
  updates: CsmEngagementStatusUpdate[];
}

const PURIFY = {
  ALLOWED_TAGS: ["p", "br", "strong", "b", "em", "i", "code", "pre", "ul", "ol", "li", "a"],
  ALLOWED_ATTR: ["href", "target", "rel"],
};

export default function EngagementStatusUpdatesPanel({
  updates,
}: EngagementStatusUpdatesPanelProps): JSX.Element {
  const ordered = [...updates].sort(
    (a, b) => new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime(),
  );
  return (
    <Card variant="outlined" sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2 }}>
      <Typography variant="subtitle2">Status updates</Typography>
      {ordered.length === 0 ? (
        <Typography variant="body2" color="text.secondary">
          No status updates yet.
        </Typography>
      ) : (
        ordered.map((u) => (
          <Box
            key={u.id}
            sx={{
              p: 1.5,
              border: 1,
              borderColor: "divider",
              borderRadius: 1,
              display: "flex",
              flexDirection: "column",
              gap: 0.75,
            }}
          >
            <Box sx={{ display: "flex", alignItems: "center", gap: 1.5, flexWrap: "wrap" }}>
              <Chip
                size="small"
                color={ENGAGEMENT_HEALTH_COLOR[u.health]}
                label={ENGAGEMENT_HEALTH_LABEL[u.health]}
              />
              <Typography variant="body2" sx={{ fontWeight: 500, flex: 1 }}>
                {u.headline}
              </Typography>
              <Typography variant="caption" color="text.secondary">
                <RelativeTime iso={u.createdAt} />
              </Typography>
            </Box>
            <Typography variant="caption" color="text.secondary">
              {u.authorName}
            </Typography>
            <Box
              sx={{ "& p": { m: 0 }, "& p + p": { mt: 0.75 } }}
              dangerouslySetInnerHTML={{
                __html: DOMPurify.sanitize(u.bodyHtml, PURIFY),
              }}
            />
          </Box>
        ))
      )}
    </Card>
  );
}
