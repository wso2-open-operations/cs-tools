import { alpha, Chip, pxToRem, Stack } from "@wso2/oxygen-ui";
import { Sparkle } from "@wso2/oxygen-ui-icons-react";

export function ClassificationBadge({ label }: { label: string }) {
  return (
    <Chip
      size="small"
      label={
        <Stack direction="row" alignItems="center" gap={1}>
          <Sparkle size={pxToRem(12)} />
          {label}
        </Stack>
      }
      sx={(theme) => ({
        bgcolor: alpha(theme.palette.success.light, 0.1),
        color: theme.palette.success.light,
        alignSelf: "end",
      })}
    />
  );
}
