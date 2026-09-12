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

import type { OxygenTheme } from "@wso2/oxygen-ui/styles/OxygenThemeBase";
import { Card, Chip, Skeleton, Stack, Typography, pxToRem, useTheme } from "@wso2/oxygen-ui";
import { ChevronRight, Clock4 } from "@wso2/oxygen-ui-icons-react";
import { Link } from "react-router-dom";
import type { IncidentSummary } from "@src/types";
import { fromNow } from "@utils/dateTime";
import { incidentPriorityColor, incidentPriorityLabel, incidentStateColor, incidentStateLabel } from "./incidentConfig";

// Mirrors ChangeRequestCard.tsx's layout exactly, swapping project name for caller and
// change-request impact for incident priority. `item.id` is nullable (an edge case the backend's
// type allows — see incident.dto.ts); when absent the card renders as plain, non-navigable content
// instead of linking to "/operations/incidents/null".
export function IncidentCard({ item }: { item: IncidentSummary }) {
  const theme = useTheme<OxygenTheme>();

  const content = (
    <Stack gap={0.75}>
      <Stack direction="row" justifyContent="space-between" alignItems="center">
        <Typography variant="subtitle2" color="text.secondary">
          {item.number}
        </Typography>
        {item.id && <ChevronRight size={pxToRem(18)} color={theme.vars.palette.text.secondary} />}
      </Stack>

      <Typography variant="body1" color="text.primary" noWrap>
        {item.subject}
      </Typography>

      {item.caller?.name && (
        <Typography variant="subtitle2" color="text.secondary" noWrap>
          {item.caller.name}
        </Typography>
      )}

      <Stack direction="row" alignItems="center" gap={1} flexWrap="wrap">
        {item.state && (
          <Chip size="small" label={incidentStateLabel(item.state)} color={incidentStateColor(item.state)} />
        )}
        {item.priority && (
          <Chip
            size="small"
            variant="outlined"
            label={incidentPriorityLabel(item.priority)}
            color={incidentPriorityColor(item.priority)}
          />
        )}
      </Stack>

      <Stack direction="row" alignItems="center" gap={0.5}>
        <Clock4 size={pxToRem(13)} color={theme.vars.palette.text.secondary} />
        <Typography variant="subtitle2" color="text.secondary" sx={{ fontSize: pxToRem(12) }}>
          Updated {item.updatedOn ? fromNow(item.updatedOn) : "—"}
        </Typography>
      </Stack>
    </Stack>
  );

  if (!item.id) {
    return <Card sx={{ p: 1.5 }}>{content}</Card>;
  }

  return (
    <Card
      component={Link}
      to={`/operations/incidents/${item.id}`}
      sx={{ textDecoration: "none", p: 1.5, display: "block" }}
    >
      {content}
    </Card>
  );
}

export function IncidentCardSkeleton() {
  return (
    <Card sx={{ p: 1.5 }}>
      <Stack gap={0.75}>
        <Stack direction="row" justifyContent="space-between">
          <Skeleton variant="text" width={60} height={20} />
          <Skeleton variant="circular" width={pxToRem(18)} height={pxToRem(18)} />
        </Stack>
        <Skeleton variant="text" width="80%" height={26} />
        <Skeleton variant="text" width="40%" height={18} />
        <Stack direction="row" gap={1}>
          <Skeleton variant="rounded" width={70} height={24} sx={{ borderRadius: 1 }} />
          <Skeleton variant="rounded" width={70} height={24} sx={{ borderRadius: 1 }} />
        </Stack>
      </Stack>
    </Card>
  );
}
