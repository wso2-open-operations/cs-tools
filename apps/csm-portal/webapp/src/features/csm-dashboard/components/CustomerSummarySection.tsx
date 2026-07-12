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

import { Box, Chip, Skeleton, Typography } from "@wso2/oxygen-ui";
import type { JSX } from "react";
import SectionCard from "@features/csm-dashboard/components/SectionCard";
import RelativeTime from "@components/RelativeTime";
import type { CsmCustomerSummary } from "@features/csm-dashboard/types/abtDashboard";

interface CustomerSummarySectionProps {
  customers?: CsmCustomerSummary[];
  isLoading: boolean;
  scopeLabel: string;
  isError?: boolean;
}

function TierChip({ tier }: { tier: string }): JSX.Element {
  const color =
    tier === "Platinum"
      ? "primary"
      : tier === "Gold"
        ? "warning"
        : tier === "Silver"
          ? "default"
          : "default";
  return <Chip size="small" variant="outlined" label={tier} color={color} />;
}

export default function CustomerSummarySection({
  customers,
  isLoading,
  scopeLabel,
  isError,
}: CustomerSummarySectionProps): JSX.Element {
  if (isError) {
    return (
      <SectionCard title="Customers" subtitle={scopeLabel}>
        <Typography variant="body2" color="text.secondary">
          Couldn’t load customer summaries right now.
        </Typography>
      </SectionCard>
    );
  }

  return (
    <SectionCard
      title="Customers"
      subtitle={scopeLabel}
      action={
        !isLoading && customers ? (
          <Chip
            size="small"
            label={`${customers.length} accounts`}
            variant="outlined"
          />
        ) : undefined
      }
    >
      <Box
        sx={{
          display: "grid",
          gridTemplateColumns: "minmax(0, 1fr) auto auto auto auto auto",
          columnGap: 1.5,
          rowGap: 1,
          alignItems: "center",
        }}
      >
        <Typography variant="caption" color="text.secondary">
          Account
        </Typography>
        <Typography variant="caption" color="text.secondary">
          Tier
        </Typography>
        <Typography variant="caption" color="text.secondary" sx={{ textAlign: "right" }}>
          Open
        </Typography>
        <Typography variant="caption" color="text.secondary" sx={{ textAlign: "right" }}>
          S0/S1
        </Typography>
        <Typography variant="caption" color="text.secondary" sx={{ textAlign: "right" }}>
          Breached
        </Typography>
        <Typography variant="caption" color="text.secondary" sx={{ textAlign: "right" }}>
          Last activity
        </Typography>

        {isLoading &&
          [0, 1, 2, 3, 4].map((i) => (
            <Box key={i} sx={{ gridColumn: "1 / -1" }}>
              <Skeleton variant="rounded" height={32} />
            </Box>
          ))}

        {!isLoading &&
          customers?.map((c) => (
            <Box
              key={c.accountId}
              sx={{ display: "contents" }}
            >
              <Typography variant="body2" noWrap>
                {c.accountName}
              </Typography>
              <Box><TierChip tier={c.tier} /></Box>
              <Typography variant="body2" sx={{ textAlign: "right" }}>
                {c.openCaseCount}
              </Typography>
              <Typography
                variant="body2"
                color={c.s0s1Count > 0 ? "error" : "text.primary"}
                sx={{ textAlign: "right" }}
              >
                {c.s0s1Count}
              </Typography>
              <Typography
                variant="body2"
                color={c.breachedCount > 0 ? "error" : "text.primary"}
                sx={{ textAlign: "right" }}
              >
                {c.breachedCount}
              </Typography>
              <Typography
                variant="caption"
                color="text.secondary"
                sx={{ textAlign: "right" }}
              >
                <RelativeTime iso={c.lastActivityAt} />
              </Typography>
            </Box>
          ))}
      </Box>
    </SectionCard>
  );
}
