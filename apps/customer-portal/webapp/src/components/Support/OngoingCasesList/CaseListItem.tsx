import { Box, Typography } from "@mui/material";
import React from "react";
import { useNavigate, useParams } from "react-router-dom";
import { formatRelativeTime } from "../../../utils/dateUtils";
import type { Case } from "../../../types/support.types";
// import { getStatusColor, getStatusIcon } from "../../../utils/color";

interface CaseListItemProps {
  item: Case;
}

import {
  AlertCircleIcon,
  CheckCircleIcon,
  ClockIcon,
  MessageCircleIcon,
} from "../../../assets/icons/common-icons";

const severityConfig: Record<string, { color: string; label: string }> = {
  "3": { color: "#3b82f6", label: "Low" }, // Blue
  "2": { color: "#eab308", label: "Medium" }, // Yellow
  "1": { color: "#f97316", label: "High" }, // Orange
  "0": { color: "#ef4444", label: "Critical" }, // Red
  // Fallbacks
  "S0 - Critical": { color: "#ef4444", label: "Critical" },
  "S1 - High": { color: "#f97316", label: "High" },
  "S2 - Medium": { color: "#eab308", label: "Medium" },
  "S3 - Low": { color: "#3b82f6", label: "Low" },
  "S4 - Minimal": { color: "#6b7280", label: "Minimal" },
};

const statusConfig: Record<
  string,
  { bg: string; text: string; border: string; icon: React.FC<any> }
> = {
  open: {
    bg: "#dbeafe", // blue-100
    text: "#1d4ed8", // blue-700
    border: "#bfdbfe", // blue-200
    icon: AlertCircleIcon,
  },
  "in progress": {
    bg: "#ffedd5", // orange-100
    text: "#c2410c", // orange-700
    border: "#fed7aa", // orange-200
    icon: ClockIcon,
  },
  "awaiting response": {
    bg: "#fef9c3", // yellow-100
    text: "#a16207", // yellow-700
    border: "#fde047", // yellow-200
    icon: MessageCircleIcon,
  },
  resolved: {
    bg: "#dcfce7",
    text: "#15803d",
    border: "#bbf7d0",
    icon: CheckCircleIcon,
  },
  closed: {
    bg: "#f3f4f6", // gray-100
    text: "#374151", // gray-700
    border: "#e5e7eb", // gray-200
    icon: CheckCircleIcon,
  },
};

export const CaseListItem: React.FC<CaseListItemProps> = ({ item }) => {
  const navigate = useNavigate();
  const { sysId } = useParams<{ sysId: string }>();

  const severity =
    severityConfig[item.severity] || severityConfig["S4 - Minimal"];
  const statusKey = item.status.toLowerCase();
  const status = statusConfig[statusKey] || statusConfig["closed"];
  const StatusIcon = status.icon;

  return (
    <Box
      onClick={() => navigate(`/${sysId}/support/cases/${item.sysId}`)}
      sx={{
        p: 2, // p-4
        border: "1px solid",
        borderColor: "grey.200",
        borderRadius: "8px", // rounded-lg
        cursor: "pointer",
        transition: "all 0.2s",
        "&:hover": {
          borderColor: "#fdba74", // hover:border-orange-300
          boxShadow: "0 1px 2px 0 rgba(0, 0, 0, 0.05)", // hover:shadow-sm
        },
      }}
    >
      <Box
        sx={{
          display: "flex",
          alignItems: "flex-start",
          justifyContent: "space-between",
          mb: 1, // mb-2
        }}
      >
        <Box sx={{ flex: 1 }}>
          <Box sx={{ display: "flex", alignItems: "center", gap: 1, mb: 1 }}>
            <Typography
              variant="body2"
              sx={{ color: "grey.900", fontSize: "0.875rem" }}
            >
              {item.number}
            </Typography>
            <Box
              sx={{
                width: 8, // w-2
                height: 8, // h-2
                borderRadius: "50%",
                backgroundColor: severity.color,
              }}
            />
            <Typography
              variant="caption"
              sx={{ color: "grey.600", fontSize: "0.75rem" }}
            >
              {severity.label}
            </Typography>
          </Box>
          <Typography
            variant="body2"
            sx={{
              color: "grey.900",
              mb: 1,
              display: "-webkit-box",
              WebkitLineClamp: 1,
              WebkitBoxOrient: "vertical",
              overflow: "hidden",
            }}
          >
            {item.title}
          </Typography>
        </Box>
      </Box>

      <Box
        sx={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
        }}
      >
        <Box
          sx={{
            display: "inline-flex",
            alignItems: "center",
            justifyContent: "center",
            borderRadius: "6px", // rounded-md
            px: 1, // px-2
            py: 0.25, // py-0.5
            fontSize: "0.75rem", // text-xs
            fontWeight: 500, // font-medium
            whiteSpace: "nowrap",
            border: "1px solid",
            backgroundColor: status.bg,
            color: status.text,
            borderColor: status.border,
            gap: 0.5,
          }}
        >
          <StatusIcon width={12} height={12} />
          {item.status}
        </Box>

        <Box
          sx={{
            display: "flex",
            alignItems: "center",
            gap: 1.5,
            color: "grey.500",
            fontSize: "0.75rem",
          }}
        >
          {item.assignee && <span>Assigned to {item.assignee}</span>}
          <span>{formatRelativeTime(item.lastUpdated)}</span>
        </Box>
      </Box>
    </Box>
  );
};
