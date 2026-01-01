import { Stack, SvgIcon, Typography } from "@mui/material";
import type { ElementType, ReactNode } from "react";

export function InfoField({ label, value, icon }: { label: string; value: string | ReactNode; icon?: ElementType }) {
  return (
    <Stack gap={0.5}>
      <Typography variant="subtitle2" fontWeight="regular" color="text.secondary">
        {label}
      </Typography>
      <Stack direction="row" alignItems="center" gap={1}>
        {icon && <SvgIcon component={icon} sx={(theme) => ({ fontSize: theme.typography.pxToRem(20) })} />}
        <Typography variant="body2" component="div">
          {value}
        </Typography>
      </Stack>
    </Stack>
  );
}
