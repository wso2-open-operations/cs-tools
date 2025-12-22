import type { ReactNode } from "react";
import { TrendingDown, TrendingUp } from "@mui/icons-material";
import { Stack, Typography } from "@mui/material";
import { WidgetBox } from "@components/ui";

export interface MetricWidgetProps {
  label: string;
  value: number | string;
  icon: ReactNode;
  trend?: {
    direction: "up" | "down";
    value: number | string;
  };
}

export function MetricWidget({ label, value, icon, trend }: MetricWidgetProps) {
  const TrendIcon = trend?.direction === "up" ? TrendingUp : TrendingDown;

  return (
    <WidgetBox>
      <Stack direction="row" justifyContent="space-between" alignItems="center">
        {icon}

        {trend && (
          <Stack direction="row" gap={0.5} alignItems="center">
            <TrendIcon sx={{ color: "semantic.portal.accent.green" }} />
            <Typography variant="body2" fontWeight="medium" sx={{ color: "semantic.portal.accent.green" }}>
              {trend.value}
            </Typography>
          </Stack>
        )}
      </Stack>

      <Typography variant="h3" fontWeight="bold">
        {value}
      </Typography>

      <Typography variant="h6" fontWeight="medium" color="text.secondary">
        {label}
      </Typography>
    </WidgetBox>
  );
}
