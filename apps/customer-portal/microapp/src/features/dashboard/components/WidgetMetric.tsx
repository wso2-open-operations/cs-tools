import type { ReactNode } from "react";

import { Skeleton, Stack, Typography } from "@wso2/oxygen-ui";

import { WidgetRoot } from "@features/dashboard/components";

export interface WidgetMetricProps {
  label: string;
  value?: number | string;
  icon?: ReactNode;
  variant?: "default" | "minimal";
  onClick?: () => void;
}

export function WidgetMetric({ label, value, icon, variant = "default", onClick }: WidgetMetricProps) {
  return (
    <WidgetRoot onClick={onClick}>
      {variant === "default" && icon && (
        <Stack direction="row" alignItems="center">
          {icon}
        </Stack>
      )}
      <Typography variant="h4" fontWeight="medium" sx={{ mt: 1 }}>
        {value ?? <Skeleton width="60%" animation="wave" />}
      </Typography>
      <Typography variant="subtitle2" fontWeight="medium" color="text.secondary">
        {label}
      </Typography>
    </WidgetRoot>
  );
}
