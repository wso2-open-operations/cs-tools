import { Card, Chip, Stack, SvgIcon, Typography } from "@mui/material";
import type { ItemType } from "../support";
import { TYPE_CONFIG } from "../support/config";
import { Circle } from "@mui/icons-material";

export interface NotificationsListItemProps {
  type: ItemType;
  id: string;
  title: string;
  description: string;
  timestamp: string;
  unread?: boolean;
}

export function NotificationsListItem({
  type,
  id,
  title,
  description,
  timestamp,
  unread = false,
}: NotificationsListItemProps) {
  const { icon, color } = TYPE_CONFIG[type];

  return (
    <Card
      component={Stack}
      direction="row"
      position="relative"
      minHeight={120}
      gap={2}
      p={1}
      elevation={0}
      sx={{ borderLeft: "5px solid", borderColor: unread ? "primary.main" : "transparent" }}
    >
      <SvgIcon component={icon} sx={(theme) => ({ color: color, fontSize: theme.typography.pxToRem(24) })} />
      <Stack sx={{ width: "100%" }} justifyContent="space-between" gap={1}>
        <Stack>
          <Typography variant="body2" fontWeight="medium">
            {title}
          </Typography>
          <Typography
            variant="subtitle2"
            fontWeight="regular"
            color="text.secondary"
            sx={{
              display: "-webkit-box",
              WebkitBoxOrient: "vertical",
              WebkitLineClamp: 2,
              overflow: "hidden",
            }}
          >
            {description}
          </Typography>
        </Stack>
        <Stack direction="row" justifyContent="space-between">
          <Typography variant="subtitle2" fontWeight="regular" color="text.secondary">
            {timestamp}
          </Typography>
          <Chip size="small" label={id} />
        </Stack>
      </Stack>
      {unread && (
        <Circle
          color="primary"
          sx={(theme) => ({ fontSize: theme.typography.pxToRem(10), position: "absolute", right: 5, top: 5 })}
        />
      )}
    </Card>
  );
}
