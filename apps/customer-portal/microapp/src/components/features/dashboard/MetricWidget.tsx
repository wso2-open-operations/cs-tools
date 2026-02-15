import type { ReactNode } from "react";
import { colors, pxToRem, Stack, Typography } from "@wso2/oxygen-ui";
import { TrendingDown, TrendingUp } from "@wso2/oxygen-ui-icons-react";
import { WidgetBox } from "@components/ui";

export interface MetricWidgetProps {
  label: string;
  value: number | string;
  icon?: ReactNode;
  size?: "small" | "large";
  base?: boolean;
  trend?: {
    direction: "up" | "down";
    value: number | string;
  };
}

export function MetricWidget({ label, value, icon, size, base, trend }: MetricWidgetProps) {
  const small = size === "small";
  const TrendIcon = trend?.direction === "up" ? TrendingUp : TrendingDown;

  return (
    <WidgetBox>
      <Stack direction="row" justifyContent="space-between" alignItems="center">
        {!base && icon}

        {trend && !base && (
          <Stack direction="row" gap={0.5} alignItems="center">
            <TrendIcon size={pxToRem(20)} color={colors.green[500]} />
            <Typography variant="body2" fontWeight="medium" sx={{ color: "components.portal.accent.green" }}>
              {trend.value}
            </Typography>
          </Stack>
        )}
      </Stack>

      <Typography variant={small ? "h4" : "h4"} fontWeight="medium" sx={{ mt: 1 }}>
        {value}
      </Typography>

      <Typography variant={small ? "subtitle2" : "subtitle2"} fontWeight="medium" color="text.secondary">
        {label}
      </Typography>
    </WidgetBox>
  );
}
