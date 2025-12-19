import { Box, Button, Card, Typography } from "@mui/material";
import React from "react";
import { ArrowRightIcon } from "../../../assets/icons/common-icons";

interface InfoCardProps {
  icon: React.ReactNode;
  title: string;
  subtitle: string;
  colorTheme: "blue" | "purple";
  descriptionTitle: string;
  descriptionText: string;
  listItems: string[];
  primaryAction?: {
    label: string;
    icon: React.ReactNode;
    onClick?: () => void;
  };
  secondaryAction: {
    label: string;
    onClick?: () => void;
  };
}

export const InfoCard: React.FC<InfoCardProps> = ({
  icon,
  title,
  subtitle,
  colorTheme,
  descriptionTitle,
  descriptionText,
  listItems,
  primaryAction,
  secondaryAction,
}) => {
  const colors = {
    blue: {
      iconBg: "#dbeafe", // blue-100
      iconColor: "#2563eb", // blue-600
      gradientFrom: "#eff6ff", // blue-50
      gradientTo: "rgba(219, 234, 254, 0.3)", // blue-100/30
      borderColor: "#bfdbfe", // blue-200
      bulletColor: "#2563eb",
    },
    purple: {
      iconBg: "#f3e8ff", // purple-100
      iconColor: "#9333ea", // purple-600
      gradientFrom: "#faf5ff", // purple-50
      gradientTo: "rgba(243, 232, 255, 0.3)", // purple-100/30
      borderColor: "#e9d5ff", // purple-200
      bulletColor: "#9333ea",
    },
  };

  const theme = colors[colorTheme];

  return (
    <Card
      sx={{
        p: 3,
        borderRadius: "12px",
        border: "1px solid",
        borderColor: "grey.200",
        height: "100%",
        boxShadow: "none",
      }}
    >
      <Box sx={{ display: "flex", alignItems: "center", gap: 1.5, mb: 3 }}>
        <Box
          sx={{
            width: 40,
            height: 40,
            backgroundColor: theme.iconBg,
            borderRadius: "8px",
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            color: theme.iconColor,
          }}
        >
          {icon}
        </Box>
        <Box>
          <Typography
            variant="subtitle1"
            sx={{ fontWeight: 400, color: "grey.900", lineHeight: 1.2 }}
          >
            {title}
          </Typography>
          <Typography variant="body2" sx={{ color: "grey.600" }}>
            {subtitle}
          </Typography>
        </Box>
      </Box>

      <Box
        sx={{
          background: `linear-gradient(to bottom right, ${theme.gradientFrom}, ${theme.gradientTo})`,
          border: "1px solid",
          borderColor: theme.borderColor,
          borderRadius: "8px",
          p: 3,
        }}
      >
        <Typography
          variant="subtitle1"
          sx={{ fontWeight: 400, color: "grey.900", mb: 1 }}
        >
          {descriptionTitle}
        </Typography>
        <Typography variant="body2" sx={{ color: "grey.600", mb: 2 }}>
          {descriptionText}
        </Typography>
        <Box sx={{ display: "grid", gap: 1 }}>
          {listItems.map((item, index) => (
            <Box
              key={index}
              sx={{ display: "flex", alignItems: "center", gap: 1 }}
            >
              <Box
                sx={{
                  width: 6,
                  height: 6,
                  borderRadius: "50%",
                  backgroundColor: theme.bulletColor,
                }}
              />
              <Typography variant="body2" sx={{ color: "grey.600" }}>
                {item}
              </Typography>
            </Box>
          ))}
        </Box>
      </Box>

      <Box sx={{ mt: 3, display: "flex", flexDirection: "column", gap: 1.5 }}>
        {primaryAction && (
          <Button
            variant="contained"
            startIcon={primaryAction.icon}
            fullWidth
            onClick={primaryAction.onClick}
            sx={{
              backgroundColor: "#ea580c", // orange-600
              textTransform: "none",
              height: 36, // h-9
              px: 2, // px-4
              fontSize: "0.875rem", // text-sm
              fontWeight: 500, // font-medium
              borderRadius: "6px", // rounded-md
              boxShadow: "none",
              "&:hover": {
                backgroundColor: "#c2410c", // hover:bg-orange-700
                boxShadow: "none",
              },
            }}
          >
            {primaryAction.label}
          </Button>
        )}
        <Button
          fullWidth
          endIcon={<ArrowRightIcon width={16} height={16} />}
          onClick={secondaryAction.onClick}
          sx={{
            justifyContent: "center",
            color: "text.primary", // text-secondary-foreground (usually dark)
            backgroundColor: "#f3f4f6", // bg-secondary (often slate-100/gray-100)
            "&:hover": { backgroundColor: "#e5e7eb" }, // hover:bg-secondary/80
            textTransform: "none",
            height: 36, // h-9
            px: 2, // px-4
            fontSize: "0.875rem", // text-sm
            fontWeight: 500, // font-medium
            borderRadius: "6px", // rounded-md
          }}
        >
          {secondaryAction.label}
        </Button>
      </Box>
    </Card>
  );
};
