import { Stack, Typography, pxToRem } from "@wso2/oxygen-ui";
import type { ItemCardProps } from "../support";
import { TYPE_CONFIG } from "../support/config";

export function OverlineSlot({ type, id }: { type: ItemCardProps["type"]; id: string }) {
  const { icon: Icon, color } = TYPE_CONFIG[type];

  return (
    <Stack direction="row" alignItems="center" gap={1}>
      <Icon color={color} size={pxToRem(18)} />
      <Typography variant="body2">{id}</Typography>
    </Stack>
  );
}
