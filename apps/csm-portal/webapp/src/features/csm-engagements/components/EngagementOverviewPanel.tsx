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

import { Box, Card, Chip, LinearProgress, Typography } from "@wso2/oxygen-ui";
import type { JSX } from "react";
import RelativeTime from "@components/RelativeTime";
import {
  ENGAGEMENT_DELIVERY_LABEL,
  ENGAGEMENT_HEALTH_COLOR,
  ENGAGEMENT_HEALTH_LABEL,
  ENGAGEMENT_PAYMENT_TYPE_LABEL,
  ENGAGEMENT_STAGE_LABEL,
  ENGAGEMENT_STATE_COLOR,
  ENGAGEMENT_STATE_LABEL,
  ENGAGEMENT_TYPE_LABEL,
  formatDateOnly,
} from "@features/csm-engagements/utils/engagements";
import type { CsmEngagementDetail } from "@features/csm-engagements/types/csmEngagements";

function Meta({ label, children }: { label: string; children: React.ReactNode }): JSX.Element {
  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 0.25 }}>
      <Typography variant="caption" color="text.secondary">
        {label}
      </Typography>
      <Box>{children}</Box>
    </Box>
  );
}

interface EngagementOverviewPanelProps {
  engagement: CsmEngagementDetail;
}

export default function EngagementOverviewPanel({
  engagement: e,
}: EngagementOverviewPanelProps): JSX.Element {
  return (
    <Box
      sx={{
        display: "grid",
        gap: 2,
        gridTemplateColumns: { xs: "1fr", lg: "minmax(0, 7fr) minmax(0, 5fr)" },
        alignItems: "start",
      }}
    >
      <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2 }}>
        <Typography variant="subtitle2">Summary</Typography>
        <Box
          sx={{
            display: "grid",
            gridTemplateColumns: { xs: "1fr 1fr", md: "repeat(4, minmax(0, 1fr))" },
            gap: 2,
          }}
        >
          <Meta label="Type">
            <Typography variant="body2">{ENGAGEMENT_TYPE_LABEL[e.type]}</Typography>
          </Meta>
          <Meta label="State">
            <Chip
              size="small"
              variant="outlined"
              color={ENGAGEMENT_STATE_COLOR[e.state]}
              label={ENGAGEMENT_STATE_LABEL[e.state]}
            />
          </Meta>
          <Meta label="Stage">
            <Typography variant="body2">{ENGAGEMENT_STAGE_LABEL[e.stage]}</Typography>
          </Meta>
          <Meta label="Health">
            {e.health ? (
              <Chip
                size="small"
                color={ENGAGEMENT_HEALTH_COLOR[e.health]}
                label={ENGAGEMENT_HEALTH_LABEL[e.health]}
              />
            ) : (
              <Typography variant="body2" color="text.disabled">
                —
              </Typography>
            )}
          </Meta>
          <Meta label="Customer">
            <Typography variant="body2">{e.customer}</Typography>
          </Meta>
          <Meta label="Project">
            <Typography variant="body2">{e.projectName ?? "—"}</Typography>
          </Meta>
          <Meta label="Owner">
            <Typography variant="body2">
              {e.ownerIsMe ? <strong>{e.ownerName}</strong> : e.ownerName}
            </Typography>
          </Meta>
          <Meta label="Delivery">
            <Typography variant="body2">
              {ENGAGEMENT_DELIVERY_LABEL[e.deliveryMode]}
            </Typography>
          </Meta>
          <Meta label="Planned start">
            <Typography variant="body2">{formatDateOnly(e.plannedStartDate)}</Typography>
          </Meta>
          <Meta label="Planned end">
            <Typography variant="body2">{formatDateOnly(e.plannedEndDate)}</Typography>
          </Meta>
          <Meta label="Created">
            <Typography variant="body2">
              <RelativeTime iso={e.createdAt} />
            </Typography>
          </Meta>
          <Meta label="Last update">
            <Typography variant="body2">
              <RelativeTime iso={e.updatedAt} />
            </Typography>
          </Meta>
        </Box>
        <Box sx={{ display: "flex", alignItems: "center", gap: 1.5 }}>
          <Box sx={{ flex: 1 }}>
            <Typography variant="caption" color="text.secondary">
              Progress
            </Typography>
            <LinearProgress
              variant="determinate"
              value={e.progressPct}
              sx={{ height: 8, borderRadius: 4 }}
            />
          </Box>
          <Typography variant="h6" sx={{ minWidth: 56, textAlign: "right" }}>
            {e.progressPct}%
          </Typography>
        </Box>
        <Box sx={{ pt: 1, borderTop: 1, borderColor: "divider" }}>
          <Typography variant="caption" color="text.secondary">
            Scope
          </Typography>
          <Typography variant="body2" sx={{ mt: 0.5, whiteSpace: "pre-wrap" }}>
            {e.scope}
          </Typography>
        </Box>
        <Box sx={{ pt: 1, borderTop: 1, borderColor: "divider" }}>
          <Typography variant="caption" color="text.secondary">
            Description
          </Typography>
          <Typography variant="body2" sx={{ mt: 0.5, whiteSpace: "pre-wrap" }}>
            {e.description}
          </Typography>
        </Box>
      </Card>

      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 1 }}>
          <Typography variant="subtitle2">Commercial</Typography>
          <Meta label="Payment type">
            <Typography variant="body2">
              {ENGAGEMENT_PAYMENT_TYPE_LABEL[e.billing.paymentType]}
            </Typography>
          </Meta>
          <Meta label="Salesforce opportunity">
            <Typography variant="body2">
              {e.billing.opportunityRef ? (
                <code style={{ fontSize: "0.8rem" }}>{e.billing.opportunityRef}</code>
              ) : (
                <Typography component="span" variant="body2" color="text.disabled">
                  Not from SF
                </Typography>
              )}
            </Typography>
          </Meta>
        </Card>

        <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 1 }}>
          <Typography variant="subtitle2">Resources allocated</Typography>
          {e.allocations.length === 0 ? (
            <Typography variant="body2" color="text.secondary">
              No allocations recorded.
            </Typography>
          ) : (
            e.allocations.map((a) => (
              <Box
                key={a.id}
                sx={{
                  display: "flex",
                  alignItems: "center",
                  gap: 1,
                  py: 0.5,
                  borderBottom: 1,
                  borderColor: "divider",
                  "&:last-of-type": { borderBottom: 0 },
                }}
              >
                <Typography variant="body2" sx={{ flex: 1 }}>
                  {a.userName}
                </Typography>
                <Typography variant="caption" color="text.secondary">
                  {formatDateOnly(a.startDate)} → {formatDateOnly(a.endDate)}
                </Typography>
                <Chip size="small" variant="outlined" label={`${a.allocationPct}%`} />
              </Box>
            ))
          )}
        </Card>

        <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 1 }}>
          <Typography variant="subtitle2">Watchers</Typography>
          {e.watchers.map((w) => (
            <Box
              key={w.id}
              sx={{
                display: "flex",
                alignItems: "center",
                gap: 1,
                py: 0.5,
              }}
            >
              <Typography variant="body2" sx={{ flex: 1 }}>
                {w.isMe ? <strong>{w.name}</strong> : w.name}
              </Typography>
              <Typography variant="caption" color="text.secondary">
                {w.role.replace(/_/g, " ")}
              </Typography>
            </Box>
          ))}
        </Card>
      </Box>
    </Box>
  );
}
