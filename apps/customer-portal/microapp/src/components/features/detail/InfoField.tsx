import { Stack, Typography, pxToRem } from "@wso2/oxygen-ui";
import type { LucideIcon } from "@wso2/oxygen-ui-icons-react";
import type { ReactNode } from "react";

export function InfoField({ label, value, icon }: { label: string; value: string | ReactNode; icon?: LucideIcon }) {
  const Icon = icon;

  return (
    <Stack gap={0.5}>
      <Typography variant="subtitle2" color="text.secondary">
        {label}
      </Typography>
      <Stack direction="row" alignItems="center" gap={1}>
        {Icon && <Icon size={pxToRem(16)} />}
        <Typography variant="body2" component="div">
          {value}
        </Typography>
      </Stack>
    </Stack>
  );
}
