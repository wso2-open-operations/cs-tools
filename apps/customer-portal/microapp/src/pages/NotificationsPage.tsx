import { useLayoutEffect } from "react";
import { useSearchParams } from "react-router-dom";
import { Button, Stack, Typography } from "@wso2/oxygen-ui";
import { NotificationsListItem, type NotificationsListItemProps } from "@components/features/notifications";
import { useLayout } from "@context/layout";
import { FilterAppBarSlot } from "./AllItemsPage";

import { MOCK_NOTIFICATIONS } from "@src/mocks/data/notifications";

const VALID_FILTERS = ["unread", "case", "service", "change"] as const;
export type NotificationFilter = (typeof VALID_FILTERS)[number];

export default function NotificationsPage() {
  const [searchParams] = useSearchParams();
  const layout = useLayout();

  const filter = searchParams.get("filter") ?? "all";
  const search = (searchParams.get("search") ?? "").toLowerCase();

  const items: NotificationsListItemProps[] = (
    filter === "all" || !VALID_FILTERS.includes(filter as NotificationFilter)
      ? Object.values(MOCK_NOTIFICATIONS).flat()
      : MOCK_NOTIFICATIONS[filter]
  ).filter(
    (item: NotificationsListItemProps) =>
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
