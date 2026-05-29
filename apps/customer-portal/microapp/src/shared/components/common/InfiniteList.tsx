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
import { useCallback, useMemo, useRef } from "react";

import type { InfiniteData, UseInfiniteQueryResult } from "@tanstack/react-query";
import { useVirtualizer } from "@tanstack/react-virtual";

interface InfiniteListProps<TItem, TError>
  extends Pick<
    UseInfiniteQueryResult<InfiniteData<TItem[]>, TError>,
    "data" | "hasNextPage" | "isFetchingNextPage" | "fetchNextPage"
  > {
  children: (item: TItem, index: number) => React.ReactNode;
  sentinel: React.ReactNode;
  tail?: React.ReactNode;
  virtualize?: boolean;
}

export function InfiniteList<TItem, TError>({ virtualize = true, ...props }: InfiniteListProps<TItem, TError>) {
  const { children, sentinel, tail, data, hasNextPage, isFetchingNextPage, fetchNextPage } = props;
  const scrollRef = useRef<HTMLDivElement>(null);
  const observer = useRef<IntersectionObserver | null>(null);

  const allItems = useMemo(() => data?.pages.flat() ?? [], [data]);

  const virtualizer = useVirtualizer({
    count: allItems.length,
    getScrollElement: () => scrollRef.current,
    estimateSize: () => 200,
    measureElement: (element) => element.getBoundingClientRect().height,
    overscan: 10,
  });

  const sentinelRef = useCallback(
    (node: HTMLElement | null) => {
      if (isFetchingNextPage) return;
      if (observer.current) observer.current.disconnect();

      observer.current = new IntersectionObserver(
        (entries) => {
          const [entry] = entries;
          if (entry.isIntersecting && hasNextPage) {
            fetchNextPage();
          }
        },
        { threshold: 0.1 },
      );

      if (node) observer.current.observe(node);
    },
    [isFetchingNextPage, hasNextPage, fetchNextPage],
  );

  return (
    <div ref={scrollRef} style={{ height: "100%", overflow: "auto" }}>
      {virtualize ? (
        <div
          style={{
            height: virtualizer.getTotalSize(),
            width: "100%",
            position: "relative",
          }}
        >
          {virtualizer.getVirtualItems().map((virtualItem) => (
            <div
              key={virtualItem.key}
              data-index={virtualItem.index}
              ref={virtualizer.measureElement}
              style={{
                position: "absolute",
                top: 0,
                left: 0,
                width: "100%",
                transform: `translateY(${virtualItem.start}px)`,
              }}
            >
              {children(allItems[virtualItem.index], virtualItem.index)}
            </div>
          ))}
        </div>
      ) : (
        <div>
          {allItems.map((item, index) => (
            <div key={index}>{children(item, index)}</div>
          ))}
        </div>
      )}

      {(!data || hasNextPage) && <div ref={sentinelRef}>{sentinel}</div>}

      {data && !hasNextPage && tail}
    </div>
  );
}
