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

// UsageMetricsPage's data layer is a heavily imperative, debounced,
// infinite-scroll, multi-parallel-fetch state machine (project search
// dropdown, deployment tabs, per-product metrics fetched only once
// expanded) — the source app's own usePostApi/useParallelPostApi
// (features/spl/api/useSplApi.ts) hooks were purpose-built for exactly this
// "fetch imperatively, on this event, with this URL" shape, which React
// Query's declarative queryFn model does not fit naturally (there is no
// single stable query key driving most of these calls — they're triggered
// by scroll position, debounce timers, and expand/collapse clicks).
//
// Rather than force this specific page through React Query (high risk of
// subtly breaking its debounce/pagination/cache logic during the port) or
// keep the source's own useSplApi.ts (a second, incompatible fetch
// abstraction this app's CLAUDE.md says not to have), these two hooks
// reproduce the ORIGINAL hooks' exact call signatures 1:1, so
// UsageMetricsPage needed zero changes to its calling code — only its
// import line — while the actual HTTP work underneath is this app's real
// authenticated useBackendApi(), not a bespoke fetch. Every usage-metrics
// backend endpoint is raw JSON passthrough (see internal/servicenow/
// usage_metrics.go), so an untyped `unknown` payload/response is a faithful
// match for what this domain actually is, not a shortcut.
import { useCallback, useRef, useState } from "react";
import { useBackendApi, BackendApiError } from "@api/backend/client";

export interface UsageMetricsApiError {
  message: string;
  status: number;
}

interface PostApiResponse<T> {
  data?: T;
  loading: boolean;
  error?: UsageMetricsApiError;
  postApiData: (payload: unknown, url: string) => Promise<void>;
}

function toApiError(err: unknown): UsageMetricsApiError {
  if (err instanceof BackendApiError) {
    return { message: err.message, status: err.status };
  }
  return { message: "unexpected error occurred while invoking the api", status: 0 };
}

/**
 * Imperative POST, matching the source app's usePostApi call shape exactly:
 * `postApiData(payload, path)` — path is relative to CSM_PORTAL_BACKEND_BASE_URL,
 * forwarded straight to useBackendApi().post.
 */
export function usePostApi<T>(): PostApiResponse<T> {
  const api = useBackendApi();
  const [data, setData] = useState<T>();
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<UsageMetricsApiError>();
  // Guards against an older in-flight call's response landing after a newer
  // one and clobbering it — e.g. UsageMetricsPage's project-search effect
  // re-invokes postApiData per keystroke, and a slow response for an earlier
  // query arriving last must not overwrite the current query's results.
  const requestIdRef = useRef(0);

  const postApiData = useCallback(
    async (payload: unknown, url: string) => {
      const requestId = ++requestIdRef.current;
      setLoading(true);
      setError(undefined);
      try {
        const result = await api.post<unknown, T>(url, payload);
        if (requestId !== requestIdRef.current) return;
        setData(result);
      } catch (err) {
        if (requestId !== requestIdRef.current) return;
        setError(toApiError(err));
      } finally {
        if (requestId === requestIdRef.current) setLoading(false);
      }
    },
    [api],
  );

  return { data, loading, error, postApiData };
}

interface ParallelPostApiResponse<T> {
  dataMap: Map<string, T>;
  loading: boolean;
  /** When merge=true, only fetches IDs not already in the map and merges
   * results in. `url` may be a per-item builder for path-param endpoints. */
  postAll: (items: { id: string; payload: unknown }[], url: string | ((id: string) => string), merge?: boolean) => Promise<void>;
  clearAll: () => void;
}

/** Matches the source app's useParallelPostApi call shape exactly. */
export function useParallelPostApi<T>(): ParallelPostApiResponse<T> {
  const api = useBackendApi();
  const [dataMap, setDataMap] = useState<Map<string, T>>(new Map());
  const [loading, setLoading] = useState(false);
  // Bumped only by a replace call (merge=false) or clearAll — these
  // invalidate any earlier in-flight call outright. A merge call must NOT
  // bump it: UsageMetricsPage fires one merge call per expanded product,
  // and an unrelated product's call landing while another is still in
  // flight must not cause that other call's own results to be discarded.
  const generationRef = useRef(0);
  // Counts merge calls currently in flight, so `loading` reflects "at least
  // one call running" instead of only the most recent one.
  const inFlightRef = useRef(0);
  const dataMapRef = useRef(dataMap);
  dataMapRef.current = dataMap;

  const clearAll = () => {
    generationRef.current++;
    setDataMap(new Map());
  };

  const postAll = useCallback(
    async (
      items: { id: string; payload: unknown }[],
      url: string | ((id: string) => string),
      merge = false,
    ) => {
      const toFetch = merge ? items.filter(({ id }) => !dataMapRef.current.has(id)) : items;
      if (toFetch.length === 0) return;

      const generation = merge ? generationRef.current : ++generationRef.current;
      inFlightRef.current++;
      setLoading(true);
      try {
        const results = await Promise.all(
          toFetch.map(async ({ id, payload }) => {
            try {
              const itemUrl = typeof url === "function" ? url(id) : url;
              const data = await api.post<unknown, T>(itemUrl, payload);
              return { id, data };
            } catch {
              return { id, data: null as unknown as T };
            }
          }),
        );
        if (generation !== generationRef.current) return;
        if (merge) {
          setDataMap((prev) => {
            const merged = new Map(prev);
            results.forEach(({ id, data }) => {
              if (data !== null) merged.set(id, data);
            });
            return merged;
          });
        } else {
          const newMap = new Map<string, T>();
          results.forEach(({ id, data }) => {
            if (data !== null) newMap.set(id, data);
          });
          setDataMap(newMap);
        }
      } finally {
        inFlightRef.current--;
        if (inFlightRef.current === 0) setLoading(false);
      }
    },
    [api],
  );

  return { dataMap, loading, postAll, clearAll };
}
