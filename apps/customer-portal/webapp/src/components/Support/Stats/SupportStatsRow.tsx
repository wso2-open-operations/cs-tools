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

import { Box, Card, Typography } from "@mui/material";
import React from "react";
import {
  BotIcon,
  CheckCircleIcon,
  ClockIcon,
  FileTextIcon,
  ChatIcon,
  TrendingUpIcon,
} from "../../../assets/icons/common-icons";
import { StatCard } from "./StatCard";

export interface SupportStatsRowData {
  totalCasesCount: number;
  ongoingCasesCount: number;
  inProgressCasesCount: number;
  resolvedCasesCount: number;
  awaitingCasesCount: number;
}

interface SupportStatsRowProps {
  stats: SupportStatsRowData;
}

export const SupportStatsRow: React.FC<SupportStatsRowProps> = ({ stats }) => {
  return (
    <Box
      sx={{
        display: "grid",
        gridTemplateColumns: {
          xs: "1fr",
          md: "repeat(2, 1fr)",
          lg: "repeat(4, 1fr)",
        },
        gap: 3,
        mb: 4,
      }}
    >
      <StatCard
        icon={<FileTextIcon width={32} height={32} />}
        secondaryIcon={<TrendingUpIcon width={16} height={16} />}
        value={stats.ongoingCasesCount}
        label="Ongoing Cases"
        iconColor="#ea580c" // orange-600
      />
      <StatCard
        icon={<ChatIcon width={32} height={32} />}
        secondaryIcon={<BotIcon width={16} height={16} />}
        value={stats.totalCasesCount}
        label="Total Cases"
        iconColor="#2563eb" // blue-600
      />
      <StatCard
        icon={<CheckCircleIcon width={32} height={32} />}
        value={stats.resolvedCasesCount}
        label="Resolved Cases"
        iconColor="#16a34a" // green-600
      />
      <StatCard
        icon={<ClockIcon width={32} height={32} />}
        value={stats.inProgressCasesCount}
        label="In Progress Cases"
        iconColor="#ca8a04" // yellow-600
      />
    </Box>
  );
};
