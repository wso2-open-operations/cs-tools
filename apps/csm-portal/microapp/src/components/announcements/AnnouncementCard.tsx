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

import { Card, Skeleton, Stack, Typography } from "@wso2/oxygen-ui";
import type { CaseSummary } from "@src/types";
import { fromNow } from "@utils/dateTime";
import { AnnouncementStateChip } from "./AnnouncementStateChip";

// Read-only announcement card (not tappable — mirrors the webapp's read-only
// list). Announcements are cases of type "announcement", so `item` is a
// `CaseSummary`; only the fields the list shows are rendered.
export function AnnouncementCard({ item }: { item: CaseSummary }) {
  return (
    <Card variant="outlined" sx={{ p: 2 }}>
      <Stack gap={1}>
        <Stack direction="row" alignItems="flex-start" justifyContent="space-between" gap={1}>
          <Typography variant="subtitle2" noWrap>
            {item.number || "—"}
          </Typography>
          <AnnouncementStateChip state={item.state} />
        </Stack>
        <Typography variant="body2">{item.subject}</Typography>
        <Typography variant="caption" color="text.secondary" noWrap>
          {item.project?.name ?? "—"}
        </Typography>
        <Typography variant="caption" color="text.secondary">
          {item.createdBy || "—"} · Updated {fromNow(item.updatedOn)}
        </Typography>
      </Stack>
    </Card>
  );
}

export function AnnouncementCardSkeleton() {
  return (
    <Card variant="outlined" sx={{ p: 2 }}>
      <Stack gap={1}>
        <Skeleton variant="rounded" width="40%" height={18} />
        <Skeleton variant="rounded" width="90%" height={18} />
        <Skeleton variant="rounded" width="60%" height={14} />
      </Stack>
    </Card>
  );
}
