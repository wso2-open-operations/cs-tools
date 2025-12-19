import React from "react";
import { Box, Typography, Chip, Avatar } from "@mui/material";
import {
  InfoIcon,
  CalendarIcon,
  ClockIcon,
  ServerIcon,
} from "@/assets/icons/common-icons";

import type { CaseDetails } from "@/types/case.types";
import {
  BuildingIcon,
  PackageIcon,
  TagIcon,
} from "@/assets/icons/support-icons";

interface CaseInfoProps {
  caseData: CaseDetails;
}

const statusConfig: Record<
  string,
  { bg: string; text: string; border: string }
> = {
  "in progress": {
    bg: "#FFEDD5",
    text: "#C2410C",
    border: "#FED7AA",
  },
  open: {
    bg: "#DBEAFE",
    text: "#1D4ED8",
    border: "#BFDBFE",
  },
  resolved: {
    bg: "#DCFCE7",
    text: "#15803D",
    border: "#BBF7D0",
  },
  closed: {
    bg: "#F3F4F6",
    text: "#374151",
    border: "#E5E7EB",
  },
};

const priorityConfig: Record<
  string,
  { bg: string; text: string; border: string; dotColor: string }
> = {
  "S0 - Critical": {
    bg: "#FEE2E2",
    text: "#991B1B",
    border: "#FECACA",
    dotColor: "#EF4444",
  },
  "S1 - High": {
    bg: "#FFEDD5",
    text: "#C2410C",
    border: "#FED7AA",
    dotColor: "#F97316",
  },
  "S2 - Medium": {
    bg: "#FEF9C3",
    text: "#A16207",
    border: "#FDE047",
    dotColor: "#EAB308",
  },
  "S3 - Low": {
    bg: "#DBEAFE",
    text: "#1D4ED8",
    border: "#BFDBFE",
    dotColor: "#3B82F6",
  },
  "S4 - Minimal": {
    bg: "#F3F4F6",
    text: "#374151",
    border: "#E5E7EB",
    dotColor: "#6B7280",
  },
};

export const CaseInfo: React.FC<CaseInfoProps> = ({ caseData }) => {
  const statusKey = caseData.status.toLowerCase();
  const status = statusConfig[statusKey] || statusConfig["open"];
  const priority =
    priorityConfig[caseData.severity] || priorityConfig["S3 - Low"];

  // Extract initials from assigned engineer name (only alphabetic characters)
  const getInitials = (name: string) => {
    const parts = name.split(" ");
    return parts
      .map((p) => p.charAt(0))
      .filter((char) => /[a-zA-Z]/.test(char))
      .join("");
  };

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      {/* Case Overview Card */}
      <Box
        sx={{
          bgcolor: "white",
          borderRadius: "12px",
          border: "1px solid #E5E7EB",
          p: 3,
        }}
      >
        <Typography
          variant="h6"
          sx={{
            color: "#111827",
            fontSize: "1rem",
            mb: 2,
            display: "flex",
            alignItems: "center",
            gap: 1,
          }}
        >
          <InfoIcon width={20} height={20} />
          Case Overview
        </Typography>

        <Box
          sx={{
            display: "grid",
            gridTemplateColumns: "repeat(2, 1fr)",
            gap: "32px 32px",
          }}
        >
          {/* Case ID */}
          <Box>
            <Typography
              sx={{
                fontSize: "0.875rem",
                color: "#6B7280",
                mb: 0.5,
              }}
            >
              Case ID
            </Typography>
            <Typography
              sx={{
                fontSize: "0.875rem",
                color: "#111827",
              }}
            >
              {caseData.number}
            </Typography>
          </Box>

          {/* Status */}
          <Box>
            <Typography
              sx={{
                fontSize: "0.875rem",
                color: "#6B7280",
                mb: 0.5,
              }}
            >
              Status
            </Typography>
            <Chip
              label={caseData.status}
              size="small"
              sx={{
                bgcolor: status.bg,
                color: status.text,
                border: `1px solid ${status.border}`,
                fontSize: "0.75rem",
                fontWeight: 500,
                height: "24px",
              }}
            />
          </Box>

          {/* Priority */}
          <Box>
            <Typography
              sx={{
                fontSize: "0.875rem",
                color: "#6B7280",
                mb: 0.5,
              }}
            >
              Priority
            </Typography>
            <Chip
              label={caseData.severity.split(" - ")[0]}
              icon={
                <Box
                  sx={{
                    width: 8,
                    height: 8,
                    borderRadius: "50%",
                    bgcolor: priority.dotColor,
                  }}
                />
              }
              size="small"
              sx={{
                bgcolor: priority.bg,
                color: priority.text,
                border: `1px solid ${priority.border}`,
                fontSize: "0.75rem",
                fontWeight: 500,
                height: "24px",
                "& .MuiChip-icon": {
                  ml: 0.5,
                  mr: 0.5,
                },
              }}
            />
          </Box>

          {/* Category */}
          <Box>
            <Typography
              sx={{
                fontSize: "0.875rem",
                color: "#6B7280",
                mb: 0.5,
              }}
            >
              Category
            </Typography>
            <Box sx={{ display: "flex", alignItems: "center", gap: 0.75 }}>
              <TagIcon width={16} height={16} color="#9CA3AF" />
              <Typography
                sx={{
                  fontSize: "0.875rem",
                  color: "#111827",
                }}
              >
                {caseData.category}
              </Typography>
            </Box>
          </Box>

          {/* Created Date */}
          <Box>
            <Typography
              sx={{
                fontSize: "0.875rem",
                color: "#6B7280",
                mb: 0.5,
              }}
            >
              Created Date
            </Typography>
            <Box sx={{ display: "flex", alignItems: "center", gap: 0.75 }}>
              <CalendarIcon width={16} height={16} color="#9CA3AF" />
              <Typography
                sx={{
                  fontSize: "0.875rem",
                  color: "#111827",
                }}
              >
                {new Date(caseData.createdOn).toLocaleString("en-US", {
                  month: "long",
                  day: "numeric",
                  year: "numeric",
                  hour: "numeric",
                  minute: "2-digit",
                  hour12: true,
                })}
              </Typography>
            </Box>
          </Box>

          {/* Last Updated */}
          <Box>
            <Typography
              sx={{
                fontSize: "0.875rem",
                color: "#6B7280",
                mb: 0.5,
              }}
            >
              Last Updated
            </Typography>
            <Box sx={{ display: "flex", alignItems: "center", gap: 0.75 }}>
              <ClockIcon width={16} height={16} color="#9CA3AF" />
              <Typography
                sx={{
                  fontSize: "0.875rem",
                  color: "#111827",
                }}
              >
                {new Date(caseData.updatedOn).toLocaleString("en-US", {
                  month: "long",
                  day: "numeric",
                  year: "numeric",
                  hour: "numeric",
                  minute: "2-digit",
                  hour12: true,
                })}
              </Typography>
            </Box>
          </Box>

          {/* SLA Response Time */}
          <Box>
            <Typography
              sx={{
                fontSize: "0.875rem",
                color: "#6B7280",
                mb: 0.5,
              }}
            >
              SLA Response Time
            </Typography>
            <Typography
              sx={{
                fontSize: "0.875rem",
                color: "#111827",
              }}
            >
              {caseData.sla}
            </Typography>
          </Box>

          {/* Assigned Engineer */}
          <Box>
            <Typography
              sx={{
                fontSize: "0.875rem",
                color: "#6B7280",
                mb: 0.5,
              }}
            >
              Assigned Engineer
            </Typography>
            <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
              <Avatar
                sx={{
                  width: 24,
                  height: 24,
                  bgcolor: "#DBEAFE",
                  color: "#1D4ED8",
                  fontSize: "0.75rem",
                }}
              >
                {getInitials(caseData.assignedEngineer)}
              </Avatar>
              <Typography
                sx={{
                  fontSize: "0.875rem",
                  color: "#111827",
                }}
              >
                {caseData.assignedEngineer}
              </Typography>
            </Box>
          </Box>
        </Box>
      </Box>

      {/* Project & Environment Card */}
      <Box
        sx={{
          bgcolor: "white",
          borderRadius: "12px",
          border: "1px solid #E5E7EB",
          p: 3,
        }}
      >
        <Typography
          variant="h6"
          sx={{
            color: "#111827",
            fontSize: "1rem",
            mb: 2,
            display: "flex",
            alignItems: "center",
            gap: 1,
          }}
        >
          <PackageIcon width={20} height={20} />
          Project & Environment
        </Typography>

        <Box
          sx={{
            display: "grid",
            gridTemplateColumns: "repeat(2, 1fr)",
            gap: "32px 32px",
          }}
        >
          {/* Product */}
          <Box>
            <Typography
              sx={{
                fontSize: "0.875rem",
                color: "#6B7280",
                mb: 0.5,
              }}
            >
              Product
            </Typography>
            <Box sx={{ display: "flex", alignItems: "center", gap: 0.75 }}>
              <PackageIcon width={16} height={16} color="#9CA3AF" />
              <Typography
                sx={{
                  fontSize: "0.875rem",
                  color: "#111827",
                }}
              >
                {caseData.product}
              </Typography>
            </Box>
          </Box>

          {/* Environment */}
          <Box>
            <Typography
              sx={{
                fontSize: "0.875rem",
                color: "#6B7280",
                mb: 0.5,
              }}
            >
              Environment
            </Typography>
            <Box sx={{ display: "flex", alignItems: "center", gap: 0.75 }}>
              <ServerIcon width={16} height={16} color="#9CA3AF" />
              <Typography
                sx={{
                  fontSize: "0.875rem",
                  color: "#111827",
                }}
              >
                {caseData.environment}
              </Typography>
            </Box>
          </Box>

          {/* Account Type */}
          <Box>
            <Typography
              sx={{
                fontSize: "0.875rem",
                color: "#6B7280",
                mb: 0.5,
              }}
            >
              Account Type
            </Typography>
            <Typography
              sx={{
                fontSize: "0.875rem",
                color: "#111827",
              }}
            >
              {caseData.accountType}
            </Typography>
          </Box>

          {/* Organization */}
          <Box>
            <Typography
              sx={{
                fontSize: "0.875rem",
                color: "#6B7280",
                mb: 0.5,
              }}
            >
              Organization
            </Typography>
            <Typography
              sx={{
                fontSize: "0.875rem",
                color: "#111827",
              }}
            >
              {caseData.organization || "N/A"}
            </Typography>
          </Box>
        </Box>
      </Box>

      {/* Customer Information Card */}
      <Box
        sx={{
          bgcolor: "white",
          borderRadius: "12px",
          border: "1px solid #E5E7EB",
          p: 3,
        }}
      >
        <Typography
          variant="h6"
          sx={{
            color: "#111827",
            fontSize: "1rem",
            mb: 2,
            display: "flex",
            alignItems: "center",
            gap: 1,
          }}
        >
          <BuildingIcon width={20} height={20} />
          Customer Information
        </Typography>

        <Box
          sx={{
            display: "grid",
            gridTemplateColumns: "repeat(2, 1fr)",
            gap: "16px 16px",
          }}
        >
          {/* Organization */}
          <Box>
            <Typography
              sx={{
                fontSize: "0.875rem",
                color: "#6B7280",
                mb: 0.5,
              }}
            >
              Organization
            </Typography>
            <Typography
              sx={{
                fontSize: "0.875rem",
                color: "#111827",
              }}
            >
              {caseData.organization || "N/A"}
            </Typography>
          </Box>

          {/* Account Type */}
          <Box>
            <Typography
              sx={{
                fontSize: "0.875rem",
                color: "#6B7280",
                mb: 0.5,
              }}
            >
              Account Type
            </Typography>
            <Chip
              label={caseData.accountType}
              size="small"
              sx={{
                bgcolor: "#F3F4F6",
                color: "#374151",
                border: "1px solid transparent",
                fontSize: "0.75rem",
                fontWeight: 500,
                height: "24px",
              }}
            />
          </Box>

          {/* Assigned Engineer */}
          <Box>
            <Typography
              sx={{
                fontSize: "0.875rem",
                color: "#6B7280",
                mb: 0.5,
              }}
            >
              Assigned Engineer
            </Typography>
            <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
              <Avatar
                sx={{
                  width: 24,
                  height: 24,
                  bgcolor: "#DBEAFE",
                  color: "#1D4ED8",
                  fontSize: "0.75rem",
                }}
              >
                {getInitials(caseData.assignedEngineer)}
              </Avatar>
              <Typography
                sx={{
                  fontSize: "0.875rem",
                  color: "#111827",
                }}
              >
                {caseData.assignedEngineer}
              </Typography>
            </Box>
          </Box>

          {/* Engineer Email */}
          {/* <Box>
            <Typography
              sx={{
                fontSize: "0.875rem",
                color: "#6B7280",
                mb: 0.5,
              }}
            >
              Engineer Email
            </Typography>
            <Typography
              sx={{
                fontSize: "0.875rem",
                color: "#111827",
              }}
            >
              {caseData.engineerEmail}
            </Typography>
          </Box> */}
        </Box>
      </Box>
    </Box>
  );
};
