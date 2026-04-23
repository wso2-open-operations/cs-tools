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

import {
  FilterSlotBuilder,
  ItemCardExtended,
  type FilterSlotBuilderProps,
  type ItemCardProps,
} from "@components/features/support";
import { Stack } from "@wso2/oxygen-ui";
import { useSearchParams } from "react-router-dom";
import { useLayout } from "@context/layout";
import { useLayoutEffect } from "react";

import { MOCK_EXTENDED_ITEMS } from "@src/mocks/data/support";

export default function AllItemsPage({ type }: { type: ItemCardProps["type"] }) {
  const [searchParams] = useSearchParams();
  const layout = useLayout();

  const filter = searchParams.get("filter") ?? "all";
  const search = (searchParams.get("search") ?? "").toLowerCase();

  const items = MOCK_EXTENDED_ITEMS[type].filter((item) => {
    const matchesFilter = !filter || filter === "all" ? true : item.status === filter;

    const matchesSearch =
      !search ||
      item.id.toLowerCase().includes(search) ||
      item.title.toLowerCase().includes(search) ||
      item.description.toLowerCase().includes(search);

    return matchesFilter && matchesSearch;
  });

  useLayoutEffect(() => {
    layout.setSubtitleSlotOverride(`${items.length} of ${MOCK_EXTENDED_ITEMS[type].length}`);

    return () => {
      layout.setSubtitleSlotOverride(null);
    };
  });

  return (
    <Stack gap={2}>
      {items.map((item, index) => (
        <ItemCardExtended key={index} {...item} />
      ))}
    </Stack>
  );
}

const config: Record<ItemCardProps["type"] | "notifications", FilterSlotBuilderProps> = {
  case: {
    searchPlaceholder: "Search cases by ID, title, or description...",
    tabs: [
      { label: "Open", value: "open" },
      { label: "In Progress", value: "in progress" },
      { label: "Waiting", value: "waiting" },
      { label: "Resolved", value: "resolved" },
      { label: "Closed", value: "closed" },
    ],
  },
  chat: {
    searchPlaceholder: "Search chats by ID, title, or message...",
    tabs: [
      { label: "Active", value: "active" },
      { label: "Resolved", value: "resolved" },
    ],
  },
  service: {
    searchPlaceholder: "Search requests by ID, title, or category...",
    tabs: [
      { label: "In Progress", value: "in progress" },
      { label: "Approved", value: "approved" },
      { label: "Open", value: "open" },
      { label: "Closed", value: "closed" },
    ],
  },
  change: {
    searchPlaceholder: "Search requests by ID, title, or category...",
    tabs: [
      { label: "In Progress", value: "in progress" },
      { label: "Scheduled", value: "scheduled" },
      { label: "Approved", value: "approved" },
      { label: "Draft", value: "draft" },
    ],
  },
  notifications: {
    searchPlaceholder: "Search Notifications",
    tabs: [
      { label: "Unread", value: "unread" },
      { label: "Cases", value: "case" },
      { label: "Service Requests", value: "service" },
      { label: "Change Requests", value: "change" },
    ],
  },
};

export function FilterAppBarSlot({ type }: { type: ItemCardProps["type"] | "notifications" }) {
  return <FilterSlotBuilder {...config[type]} />;
}
