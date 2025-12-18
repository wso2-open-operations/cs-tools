import React from "react";
import { Box, Typography } from "@mui/material";
import {
  ErrorOutlineOutlined as AlertCircleIcon,
  ChatBubbleOutlineOutlined as MessageSquareIcon,
  CalendarTodayOutlined as CalendarIcon,
} from "@mui/icons-material";

export interface ProjectCardStatsProps {
  stats: {
    label: string;
    value: number | string;
    icon?: React.ReactNode;
    color?: string;
  }[];
  date: string;
}

const ProjectCardStats: React.FC<ProjectCardStatsProps> = ({ stats, date }) => {
  return (
    <Box
      sx={{
        borderBottom: (theme) => `1px solid ${theme.palette.grey[100]}`,
        paddingBottom: "16px",
        marginBottom: "16px",
        display: "flex",
        flexDirection: "column",
        gap: "12px",
      }}
    >
      {/* Open Cases */}
      <Box
        display="flex"
        justifyContent="space-between"
        alignItems="center"
        fontSize="0.875rem"
      >
        <Box display="flex" alignItems="center" gap={1} color="text.secondary">
          <AlertCircleIcon sx={{ width: 16, height: 16 }} />
          <Typography variant="body2" color="inherit">
            Open Cases
          </Typography>
        </Box>
        {/* Finding the stat value for Open Cases */}
        <Typography variant="body2" fontWeight={500} color="warning.main">
          {stats.find((s) => s.label === "Open Cases")?.value || 0}
        </Typography>
      </Box>

      {/* Active Chats */}
      <Box
        display="flex"
        justifyContent="space-between"
        alignItems="center"
        fontSize="0.875rem"
      >
        <Box display="flex" alignItems="center" gap={1} color="text.secondary">
          <MessageSquareIcon sx={{ width: 16, height: 16 }} />
          <Typography variant="body2" color="inherit">
            Active Chats
          </Typography>
        </Box>
        <Typography variant="body2" fontWeight={500} color="primary.main">
          {stats.find((s) => s.label === "Active Chats")?.value || 0}
        </Typography>
      </Box>

      {/* Date */}
      <Box
        display="flex"
        alignItems="center"
        gap={1}
        color="text.secondary"
        fontSize="0.875rem"
      >
        <CalendarIcon sx={{ width: 16, height: 16 }} />
        <Typography variant="body2" color="inherit">
          {date}
        </Typography>
      </Box>
    </Box>
  );
};

export default ProjectCardStats;
