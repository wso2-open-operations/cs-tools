import React from "react";
import { Box, Typography, Chip, Button, Avatar } from "@mui/material";
import type { CaseDetails } from "@/types/case.types";
import {
  ArrowLeftIcon,
  CheckCircleIcon,
  CirclePauseIcon,
  CirclePlayIcon,
  CircleCloseIcon,
} from "@/assets/icons/common-icons";

interface CaseHeaderProps {
  caseData: CaseDetails;
  onBack: () => void;
}

const statusColors: Record<
  string,
  "default" | "primary" | "secondary" | "error" | "info" | "success" | "warning"
> = {
  open: "info",
  "in-progress": "warning",
  "pending-customer": "warning",
  resolved: "success",
  closed: "default",
};

const priorityColors: Record<
  string,
  { bg: string; text: string; dot: string }
> = {
  S0: { bg: "#FEE2E2", text: "#B91C1C", dot: "#EF4444" }, // Red
  S1: { bg: "#FFEDD5", text: "#C2410C", dot: "#F97316" }, // Orange
  S2: { bg: "#FEF9C3", text: "#A16207", dot: "#EAB308" }, // Yellow
  S3: { bg: "#DBEAFE", text: "#1D4ED8", dot: "#3B82F6" }, // Blue
  S4: { bg: "#F3F4F6", text: "#374151", dot: "#6B7280" }, // Gray
};

export const CaseHeader: React.FC<CaseHeaderProps> = ({ caseData, onBack }) => {
  const priorityStyle = priorityColors[caseData.severity] || priorityColors.S3;

  return (
    <Box sx={{ borderBottom: "1px solid #E2E8F0", bgcolor: "white" }}>
      <Box sx={{ px: 4, py: 3 }}>
        <Button
          onClick={onBack}
          startIcon={<ArrowLeftIcon width={16} height={16} />}
          sx={{
            mb: 2,
            color: "#64748B",
            textTransform: "none",
            minWidth: 0,
            p: 0,
            "&:hover": { bgcolor: "transparent", color: "#0F172A" },
          }}
        >
          Back to Support Center
        </Button>

        <Box
          sx={{
            display: "flex",
            justifyContent: "space-between",
            alignItems: "flex-start",
            mb: 2,
          }}
        >
          <Box sx={{ flex: 1 }}>
            <Box
              sx={{ display: "flex", alignItems: "center", gap: 1.5, mb: 1 }}
            >
              <Typography
                variant="h6"
                sx={{ color: "#0F172A", fontWeight: 600 }}
              >
                {caseData.number}
              </Typography>
              <Chip
                label={caseData.status}
                size="small"
                color={statusColors[caseData.status] || "default"}
                sx={{ textTransform: "capitalize", height: 24 }}
              />
              <Box
                sx={{
                  bgcolor: priorityStyle.bg,
                  color: priorityStyle.text,
                  border: `1px solid ${priorityStyle.bg}`,
                  px: 1,
                  py: 0.25,
                  borderRadius: "9999px",
                  display: "flex",
                  alignItems: "center",
                  gap: 0.5,
                  fontSize: "0.75rem",
                  fontWeight: 500,
                }}
              >
                <Box
                  sx={{
                    width: 6,
                    height: 6,
                    borderRadius: "50%",
                    bgcolor: priorityStyle.dot,
                  }}
                />
                {caseData.severity}
              </Box>
            </Box>
            <Typography variant="body1" sx={{ color: "#334155" }}>
              {caseData.title}
            </Typography>
          </Box>
        </Box>

        <Box
          sx={{
            display: "flex",
            justifyContent: "space-between",
            alignItems: "center",
            py: 1,
            px: 1.5,
            background: "linear-gradient(to right, #F8FAFC, #FFFFFF)",
            border: "1px solid #E2E8F0",
            borderRadius: 2,
          }}
        >
          <Box sx={{ display: "flex", alignItems: "center", gap: 1.5 }}>
            <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
              <Avatar
                sx={{
                  width: 28,
                  height: 28,
                  bgcolor: "#DBEAFE",
                  color: "#1D4ED8",
                  fontSize: "0.75rem",
                }}
              >
                {caseData.assignedEngineer
                  ?.split(" ")
                  .map((n: string) => n[0])
                  .join("")
                  .slice(0, 2) || "SE"}
              </Avatar>
              <Box>
                <Typography
                  variant="subtitle2"
                  sx={{
                    color: "#0F172A",
                    fontSize: "0.75rem",
                    lineHeight: 1.2,
                  }}
                >
                  {caseData.assignedEngineer || "Unassigned"}
                </Typography>
                <Typography
                  variant="caption"
                  sx={{ color: "#64748B", fontSize: "0.75rem" }}
                >
                  Support Engineer
                </Typography>
              </Box>
            </Box>

            {/* Divider */}
            <Box sx={{ height: 24, width: "1px", bgcolor: "#D1D5DB" }} />

            {/* Manage case status indicator */}
            <Box
              sx={{
                display: "flex",
                alignItems: "center",
                gap: 1,
                color: "#EA580C",
              }}
            >
              <CirclePlayIcon width={16} height={16} />
              <Typography
                variant="caption"
                sx={{ color: "#4B5563", fontSize: "0.75rem" }}
              >
                Manage case status
              </Typography>
            </Box>
          </Box>

          <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
            <Button
              variant="outlined"
              size="small"
              startIcon={<CirclePauseIcon width={16} height={16} />}
              sx={{
                color: "#0F172A",
                borderColor: "#FED7AA",
                bgcolor: "transparent",
                textTransform: "none",
                fontSize: "0.875rem",
                height: 32,
                px: 1.5,
                "&:hover": { bgcolor: "#FFF7ED", borderColor: "#FDBA74" },
              }}
            >
              Waiting on WSO2
            </Button>
            <Button
              variant="outlined"
              size="small"
              startIcon={<CheckCircleIcon width={16} height={16} />}
              sx={{
                color: "#15803D",
                borderColor: "#86EFAC",
                bgcolor: "transparent",
                textTransform: "none",
                fontSize: "0.875rem",
                height: 32,
                px: 1.5,
                "&:hover": { bgcolor: "#F0FDF4", borderColor: "#4ADE80" },
              }}
            >
              Mark as Resolved
            </Button>
            <Button
              variant="outlined"
              size="small"
              startIcon={<CircleCloseIcon width={16} height={16} />}
              sx={{
                color: "#374151",
                borderColor: "#D1D5DB",
                bgcolor: "white",
                textTransform: "none",
                fontSize: "0.875rem",
                height: 32,
                px: 1.5,
                boxShadow: "0 1px 2px 0 rgba(0, 0, 0, 0.05)",
                "&:hover": { bgcolor: "#F9FAFB", borderColor: "#9CA3AF" },
              }}
            >
              Close Case
            </Button>
          </Box>
        </Box>
      </Box>
    </Box>
  );
};
