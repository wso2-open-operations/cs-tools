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

import { Card, Skeleton, Stack, Typography, pxToRem, useTheme } from "@wso2/oxygen-ui";
import { ChevronRight, Clock4 } from "@wso2/oxygen-ui-icons-react";
import { Link } from "react-router-dom";
import type { CaseSummary } from "@src/types";
import { fromNow } from "@utils/dateTime";
import { SeverityChip, StatusChip } from "./Chips";
import { TYPE_CONFIG } from "./config";

export function CaseCard({ item }: { item: CaseSummary }) {
  const theme = useTheme();
  const { icon: Icon, color } = TYPE_CONFIG[item.type] ?? TYPE_CONFIG.case;

  return (
    <Card component={Link} to={`/cases/${item.id}`} sx={{ textDecoration: "none", p: 1.5, display: "block" }}>
      <Stack gap={0.75}>
        <Stack direction="row" justifyContent="space-between" alignItems="center">
          <Stack direction="row" alignItems="center" gap={1}>
            <Icon size={pxToRem(16)} color={color} />
            <Typography variant="subtitle2" color="text.secondary">
              {item.number}
            </Typography>
          </Stack>
          <ChevronRight size={pxToRem(18)} color={theme.palette.text.secondary} />
        </Stack>

        <Typography variant="body1" color="text.primary" noWrap>
          {item.subject}
        </Typography>

        <Stack direction="row" alignItems="center" gap={1} flexWrap="wrap">
          <StatusChip state={item.state} />
          {/* Only "case"-type items carry a severity; service requests/security reports/etc. don't. */}
          {item.severity && <SeverityChip severity={item.severity} />}
        </Stack>

        <Stack direction="row" justifyContent="space-between" alignItems="center" mt={0.5}>
          <Stack direction="row" alignItems="center" gap={0.5} flexShrink={0}>
            <Clock4 size={pxToRem(13)} color={theme.palette.text.secondary} />
            <Typography variant="subtitle2" color="text.secondary" sx={{ fontSize: pxToRem(12) }}>
              Updated {fromNow(item.updatedOn)}
            </Typography>
          </Stack>
          <Typography variant="subtitle2" color="text.secondary" noWrap sx={{ minWidth: 0 }}>
            {item.assignedEngineer?.name ?? "Unassigned"}
          </Typography>
        </Stack>
      </Stack>
    </Card>
  );
}

export function CaseCardSkeleton() {
  return (
    <Card sx={{ p: 1.5 }}>
      <Stack gap={0.75}>
        <Stack direction="row" justifyContent="space-between" alignItems="center">
          <Stack direction="row" alignItems="center" gap={1}>
            <Skeleton variant="circular" width={pxToRem(16)} height={pxToRem(16)} />
            <Skeleton variant="text" width={60} height={20} />
          </Stack>
          <Skeleton variant="circular" width={pxToRem(18)} height={pxToRem(18)} />
        </Stack>

        <Skeleton variant="text" width="80%" height={26} />

        <Stack direction="row" gap={1}>
          <Skeleton variant="rounded" width={70} height={24} sx={{ borderRadius: 1 }} />
          <Skeleton variant="rounded" width={70} height={24} sx={{ borderRadius: 1 }} />
        </Stack>

        <Stack direction="row" justifyContent="space-between" mt={0.5}>
          <Skeleton variant="text" width={100} height={18} />
          <Skeleton variant="text" width={80} height={18} />
        </Stack>
      </Stack>
    </Card>
  );
}
