// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

import { Card, Chip, pxToRem, Stack, Typography } from "@wso2/oxygen-ui";
import { Circle } from "@mui/icons-material";
import type { ItemType } from "../support";
import { TYPE_CONFIG } from "../support/config";

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
  const { icon: Icon, color } = TYPE_CONFIG[type];

  return (
    <Card
      component={Stack}
      direction="row"
      position="relative"
      minHeight={120}
      gap={2}
      p={1}
      sx={{
        border: 1,
        borderColor: "divider",
        borderLeft: unread ? 5 : 1,
        borderLeftColor: unread ? "primary.main" : "divider",
        bgcolor: "background.paper",
      }}
    >
      <Icon size={pxToRem(24)} color={color} />
      <Stack sx={{ width: "100%" }} justifyContent="space-between" gap={1}>
        <Stack>
          <Typography variant="body2" fontWeight="medium">
            {title}
          </Typography>
          <Typography
            variant="caption"
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
          <Typography variant="caption" fontWeight="regular" color="text.secondary">
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
