// Copyright (c) 2025 WSO2 LLC. (https://www.wso2.com).
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

import { useLayoutEffect } from "react";
import { useSearchParams } from "react-router-dom";
import { Button, Stack, Typography } from "@wso2/oxygen-ui";
import { NotificationsListItem, type NotificationsListItemProps } from "@components/features/notifications";
import type { ItemType } from "@components/features/support";
import { useLayout } from "@context/layout";
import { FilterAppBarSlot } from "./AllItemsPage";

import { MOCK_NOTIFICATIONS } from "@src/mocks/data/notifications";

export type NotificationFilter = "unread" | Exclude<ItemType, "chat">;

export default function NotificationsPage() {
  const [searchParams] = useSearchParams();
  const layout = useLayout();

  const filter = searchParams.get("filter") ?? "all";
  const search = (searchParams.get("search") ?? "").toLowerCase();

  const items: NotificationsListItemProps[] = (
    filter === "all" ? Object.values(MOCK_NOTIFICATIONS).flat() : MOCK_NOTIFICATIONS[filter]
  ).filter(
    (item) =>
      !search ||
      item.id.toLowerCase().includes(search) ||
      item.title.toLowerCase().includes(search) ||
      item.description.toLowerCase().includes(search),
  );

  useLayoutEffect(() => {
    layout.setAppBarSlotsOverride(
      <>
        <Typography variant="subtitle2" fontWeight="regular" color="text.secondary" mt={1}>
          {MOCK_NOTIFICATIONS["unread"].length > 0 && `${MOCK_NOTIFICATIONS["unread"].length} unread · `}
          {items.length} of {Object.values(MOCK_NOTIFICATIONS).flat().length} total
        </Typography>
        <FilterAppBarSlot type="notifications" />
      </>,
    );

    layout.setEndSlotOverride(
      <Button variant="text" sx={{ fontWeight: "medium", textTransform: "initial" }} disableRipple>
        Mark All Read
      </Button>,
    );

    return () => {
      layout.setAppBarSlotsOverride(undefined);
      layout.setEndSlotOverride(undefined);
    };
  }, [searchParams]);

  return (
    <Stack gap={2}>
      {items.map((props, index) => (
        <NotificationsListItem key={index} {...props} />
      ))}
    </Stack>
  );
}
