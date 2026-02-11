import type { ReactNode } from "react";
import { pxToRem, Stack, Typography, useTheme } from "@wso2/oxygen-ui";
import { ChevronRight, type LucideIcon } from "@wso2/oxygen-ui-icons-react";

export function SettingListItem({
  name,
  value,
  icon,
  iconColor,
  iconBackgroundColor,
  description,
  suffix,
}: {
  name: string;
  icon: LucideIcon;
  iconColor?: string;
  iconBackgroundColor?: string;
  value?: string;
  description?: string;
  suffix?: "chevron" | ReactNode;
}) {
  const theme = useTheme();
  const Icon = icon;

  return (
    <Stack
      direction="row"
      justifyContent="space-between"
      alignItems="center"
      bgcolor="background.paper"
      sx={{ cursor: "pointer" }}
      p={1.5}
    >
      <Stack direction="row" alignItems="center" gap={1.5}>
        <Stack
          width={40}
          height={40}
          alignItems="center"
          justifyContent="center"
          borderRadius={1}
          bgcolor={iconBackgroundColor}
        >
          <Icon size={pxToRem(18)} color={iconColor} />
        </Stack>
        <Stack>
          {value && (
            <Typography variant="caption" color="text.secondary">
              {name}
            </Typography>
          )}

          <Typography variant="body1">{value ?? name}</Typography>

          {description && (
            <Typography variant="caption" fontWeight="regular" color="text.secondary">
              {description}
            </Typography>
          )}
        </Stack>
      </Stack>
      {suffix && suffix === "chevron" ? (
        <ChevronRight size={pxToRem(16)} color={theme.palette.text.secondary} />
      ) : (
        suffix
      )}
    </Stack>
  );
}
