import { Skeleton, Stack, Typography, pxToRem } from "@wso2/oxygen-ui";
import type { LucideIcon } from "@wso2/oxygen-ui-icons-react";
import type { ReactNode } from "react";

interface InfoFieldProps {
  label: string;
  value?: string | ReactNode;
  icon?: LucideIcon;
}

export function InfoField({ label, value, icon }: InfoFieldProps) {
  const Icon = icon;

  return (
    <Stack gap={0.5}>
      <Typography variant="caption" color="text.secondary">
        {label}
      </Typography>
      <Stack direction="row" alignItems="center" gap={1}>
        {Icon && <Icon size={pxToRem(16)} />}
        <Typography variant="body2" component="div" sx={{ flex: 1, wordBreak: "break-all" }}>
          {value ?? <Skeleton variant="text" width="100%" height={30} />}
        </Typography>
      </Stack>
    </Stack>
  );
}
