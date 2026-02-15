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

import { FilterSlotBuilder, ItemCardExtended, type ItemCardProps } from "@components/features/support";
import { Stack } from "@wso2/oxygen-ui";
import { useSearchParams } from "react-router-dom";
import { useLayout } from "@context/layout";
import { useLayoutEffect } from "react";

import { useSuspenseQuery } from "@tanstack/react-query";
import { cases } from "@src/services/cases";
import { useProject } from "@context/project";

export default function AllItemsPage({ type }: { type: ItemCardProps["type"] }) {
  const [searchParams] = useSearchParams();
  const layout = useLayout();

  const filter = searchParams.get("filter") ?? "all";
  const search = (searchParams.get("search") ?? "").toLowerCase();

  const { projectId } = useProject();
  const { data } = useSuspenseQuery(
    cases.all(projectId!, filter !== "all" ? { filters: { statusId: Number(filter) } } : {}),
  );

  const items = data.filter((item) => {
    // const matchesFilter = !filter || filter === "all" ? true : item.statusId === filter;

    const matchesSearch =
      !search ||
      item.id.toLowerCase().includes(search) ||
      item.title.toLowerCase().includes(search) ||
      item.description?.toLowerCase().includes(search);

    return matchesSearch;
  });

  useLayoutEffect(() => {
    layout.setSubtitleSlotOverride(`${items.length} of ${data.length}`);

    return () => {
      layout.setSubtitleSlotOverride(null);
    };
  });

  return (
    <Stack gap={2}>
      {items.map((item, index) => (
        <ItemCardExtended key={index} type="case" to="/" {...item} />
      ))}
    </Stack>
  );
}

export function FilterAppBarSlot({ type }: { type: ItemCardProps["type"] | "notifications" }) {
  const { projectId } = useProject();
  const { data: filters } = useSuspenseQuery(cases.filters(projectId!));

  return (
    <FilterSlotBuilder
      searchPlaceholder="Search cases by ID, title, or description..."
      tabs={filters.statuses.map((filter) => ({ label: filter.label, value: filter.id }))}
    />
  );
}
