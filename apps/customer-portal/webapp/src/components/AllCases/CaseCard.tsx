import { Box, Typography } from "@mui/material";
import React from "react";
import { useNavigate, useParams } from "react-router-dom";
import {
  CalendarIcon,
  AlertCircleIcon,
  CheckCircleIcon,
  ClockIcon,
  FileTextIcon,
  MessageCircleIcon,
  UserIcon,
} from "@/assets/icons/common-icons";
import { formatRelativeTime } from "@/utils/dateUtils";
import type { Case } from "@/types/support.types";

interface CaseCardProps {
  item: Case;
}

const severityConfig: Record<string, { color: string; label: string }> = {
  "S0 - Critical": { color: "#ef4444", label: "Critical" }, // red-500
  "S1 - High": { color: "#f97316", label: "High" }, // orange-500
  "S2 - Medium": { color: "#eab308", label: "Medium" }, // yellow-500
  "S3 - Low": { color: "#3b82f6", label: "Low" }, // blue-500
  "S4 - Minimal": { color: "#6b7280", label: "Minimal" }, // gray-500
};

const statusConfig: Record<
  string,
  { bg: string; text: string; border: string; icon: React.FC<any> }
> = {
  open: {
    bg: "#dbeafe",
    text: "#1d4ed8",
    border: "#bfdbfe",
    icon: AlertCircleIcon,
  }, // blue-100, blue-700, blue-200
  "in progress": {
    bg: "#ffedd5",
    text: "#c2410c",
    border: "#fed7aa",
    icon: ClockIcon,
  }, // orange-100, orange-700, orange-200
  "awaiting response": {
    bg: "#fef9c3",
    text: "#a16207",
    border: "#fde047",
    icon: MessageCircleIcon,
  }, // yellow-100, yellow-700, yellow-200
  resolved: {
    bg: "#dcfce7",
    text: "#15803d",
    border: "#bbf7d0",
    icon: CheckCircleIcon,
  }, // green-100, green-700, green-200
  closed: {
    bg: "#f3f4f6",
    text: "#374151",
    border: "#e5e7eb",
    icon: CheckCircleIcon,
  }, // gray-100, gray-700, gray-200
};

export const CaseCard: React.FC<CaseCardProps> = ({ item }) => {
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
        p: 2.5, // p-5 (20px)
        border: "1px solid",
        borderColor: "grey.200",
        borderRadius: "12px", // rounded-xl
        cursor: "pointer",
        transition: "all 0.2s",
        borderLeft: `4px solid ${severity.color}`, // border-l-4
        backgroundColor: "background.paper",
        "&:hover": {
          boxShadow:
            "0 4px 6px -1px rgba(0, 0, 0, 0.1), 0 2px 4px -1px rgba(0, 0, 0, 0.06)", // hover:shadow-md
        },
        display: "flex",
        flexDirection: "column",
        gap: 2,
      }}
    >
      <Box
        sx={{
          display: "flex",
          justifyContent: "space-between",
          alignItems: "flex-start",
          gap: 2,
        }}
      >
        <Box sx={{ flex: 1, minWidth: 0 }}>
          {/* Header Row */}
          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              gap: 1.5, // gap-3
              mb: 1, // mb-2
              flexWrap: "wrap",
            }}
          >
            {/* Case Number */}
            <Typography
              variant="body2" // text-sm
              sx={{ color: "grey.900" }}
            >
              {item.number}
            </Typography>

            {/* Severity */}
            <Box sx={{ display: "flex", alignItems: "center", gap: 0.75 }}>
              <Box
                sx={{
                  width: 8, // w-2
                  height: 8, // h-2
                  borderRadius: "50%",
                  backgroundColor: severity.color,
                }}
              />
              <Typography
                variant="caption" // text-xs
                sx={{ color: "grey.600" }}
              >
                {severity.label}
              </Typography>
            </Box>

            {/* Status Badge */}
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
                border: "1px solid",
                backgroundColor: status.bg,
                color: status.text,
                borderColor: status.border,
                gap: 0.5,
                whiteSpace: "nowrap",
              }}
            >
              <StatusIcon width={12} height={12} />
              {item.status}
            </Box>

            {/* Category Badge */}
            <Box
              sx={{
                display: "inline-flex",
                alignItems: "center",
                justifyContent: "center",
                borderRadius: "6px",
                border: "1px solid",
                borderColor: "grey.200",
                px: 1,
                py: 0.25,
                fontSize: "0.75rem",
                fontWeight: 500,
                color: "text.primary",
                whiteSpace: "nowrap",
              }}
            >
              {item.category}
            </Box>
          </Box>

          {/* Title */}
          <Box
            onClick={() => navigate(`/${sysId}/support/cases/${item.sysId}`)}
            sx={{
              cursor: "pointer",
              flex: 1,
              minWidth: 0,
            }}
          >
            <Typography
              variant="subtitle1"
              sx={{
                fontWeight: 600,
                mb: 0.5,
                whiteSpace: "nowrap",
                overflow: "hidden",
                textOverflow: "ellipsis",
                "&:hover": { color: "primary.main" },
              }}
            >
              {item.title}
            </Typography>
          </Box>

          {/* Description */}
          <Typography
            variant="body2"
            sx={{
              color: "grey.600",
              mb: 1.5,
              display: "-webkit-box",
              WebkitLineClamp: 2,
              WebkitBoxOrient: "vertical",
              overflow: "hidden",
              "& p": {
                m: 0,
              },
            }}
            dangerouslySetInnerHTML={{ __html: item.description || "" }}
          />

          {/* Metadata Row */}
          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              gap: 2,
              flexWrap: "wrap",
              color: "grey.500",
              fontSize: "0.75rem",
            }}
          >
            <Box sx={{ display: "flex", alignItems: "center", gap: 0.5 }}>
              <CalendarIcon width={12} height={12} />
              <Typography variant="caption" color="inherit">
                Created {formatRelativeTime(item.createdDate)}
              </Typography>
            </Box>
            {item.assignee && (
              <Box sx={{ display: "flex", alignItems: "center", gap: 0.5 }}>
                <UserIcon width={12} height={12} />
                <Typography variant="caption" color="inherit">
                  Assigned to {item.assignee}
                </Typography>
              </Box>
            )}
            <Box sx={{ display: "flex", alignItems: "center", gap: 0.5 }}>
              <FileTextIcon width={12} height={12} />
              <Typography variant="caption" color="inherit">
                {item.product}
              </Typography>
            </Box>
          </Box>
        </Box>
      </Box>
    </Box>
  );
};
