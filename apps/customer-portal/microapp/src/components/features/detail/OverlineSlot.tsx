import { Stack, SvgIcon, Typography } from "@mui/material";
import type { ItemCardProps } from "../support";
import { TYPE_CONFIG } from "../support/config";

export function OverlineSlot({ type, id }: { type: ItemCardProps["type"]; id: string }) {
  const { icon, color } = TYPE_CONFIG[type];

  return (
    <Stack direction="row" alignItems="center" gap={1}>
      <SvgIcon component={icon} sx={(theme) => ({ color: color, fontSize: theme.typography.pxToRem(22) })} />
      <Typography variant="body2">{id}</Typography>
    </Stack>
  );
}
