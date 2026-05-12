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

import type React from "react";
import { useCallback, useRef } from "react";
import type { InfiniteData, UseInfiniteQueryResult } from "@tanstack/react-query";
import { Stack } from "@wso2/oxygen-ui";

interface InfiniteScrollProps<TPage, TError>
  extends Pick<
    UseInfiniteQueryResult<InfiniteData<TPage>, TError>,
    "data" | "hasNextPage" | "isFetchingNextPage" | "fetchNextPage"
  > {
  children: (data: InfiniteData<TPage>) => React.ReactNode;
  sentinel: React.ReactNode;
  tail?: React.ReactNode;
}

export function InfiniteScroll<TPage, TError>(props: InfiniteScrollProps<TPage, TError>) {
  const { children, sentinel, tail, data, hasNextPage, isFetchingNextPage, fetchNextPage } = props;
  const observer = useRef<IntersectionObserver | null>(null);

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
        {
          root: null,
          threshold: 0.1,
        },
      );

      if (node) observer.current.observe(node);
    },
    [isFetchingNextPage, hasNextPage, fetchNextPage],
  );

  return (
    <>
      {data && children(data)}

      {(!data || hasNextPage) && (
        <Stack ref={sentinelRef} gap={2}>
          {sentinel}
        </Stack>
      )}

      {data && !hasNextPage && tail}
    </>
  );
}
