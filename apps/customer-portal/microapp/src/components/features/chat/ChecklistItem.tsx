import { Box, pxToRem, Stack, Typography, type SvgIconProps } from "@wso2/oxygen-ui";
import { Check, CircleCheck } from "@wso2/oxygen-ui-icons-react";
import type { ElementType } from "react";

export function ChecklistItem({
  children,
  variant = "check",
  color = "success",
  icon,
}: {
  children: string;
  variant?: "check" | "checkbox";
  icon?: ElementType<SvgIconProps>;
  color?: SvgIconProps["color"];
}) {
  const Icon = icon ?? (variant === "checkbox" ? CircleCheck : Check);

  return (
    <Stack direction="row" gap={2}>
      <Box sx={(theme) => ({ color: theme.palette[color]?.main ?? theme.palette.primary.main })}>
        <Icon size={pxToRem(18)} />
      </Box>
      <Typography variant="body2">{children}</Typography>
    </Stack>
  );
}
