import { Box } from "@mui/material";
import React from "react";
import {
  AlertCircleIcon,
  CheckCircleIcon,
  ClockIcon,
  FileTextIcon,
  MessageCircleIcon,
} from "@/assets/icons/common-icons";
import { StatCard } from "@/components/Support/Stats/StatCard";

interface AllCasesStatsProps {
  stats: {
    totalCasesCount: number;
    ongoingCasesCount: number; // using for Open
    inProgressCasesCount: number;
    resolvedCasesCount: number;
    awaitingCasesCount: number;
  };
}

export const AllCasesStats: React.FC<AllCasesStatsProps> = ({ stats }) => {
  return (
    <Box
      sx={{
        display: "grid",
        gridTemplateColumns: {
          xs: "1fr",
          sm: "repeat(2, 1fr)",
          md: "repeat(5, 1fr)",
        },
        gap: 2,
        mb: 3,
      }}
    >
      <StatCard
        icon={<AlertCircleIcon width={16} height={16} />}
        value={stats.ongoingCasesCount} // Assuming ongoing = open for stats mapping
        label="Open"
        iconColor="#2563eb" // blue-600
        compact
      />
      <StatCard
        icon={<ClockIcon width={16} height={16} />}
        value={stats.inProgressCasesCount}
        label="In Progress"
        iconColor="#ea580c" // orange-600
        compact
      />
      <StatCard
        icon={<MessageCircleIcon width={16} height={16} />}
        value={stats.awaitingCasesCount}
        label="Awaiting"
        iconColor="#ca8a04" // yellow-600
        compact
      />
      <StatCard
        icon={<CheckCircleIcon width={16} height={16} />}
        value={stats.resolvedCasesCount}
        label="Resolved"
        iconColor="#16a34a" // green-600
        compact
      />
      <StatCard
        icon={<FileTextIcon width={16} height={16} />}
        value={stats.totalCasesCount}
        label="Total"
        iconColor="#4b5563" // gray-600
        compact
      />
    </Box>
  );
};
