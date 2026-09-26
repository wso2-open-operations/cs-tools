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

import { useCallback, useEffect } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useBackendApi } from "@api/backend/client";
import type {
  BeReorderSavedFilterViewPayload,
  BeSavedFilterViewList,
  BeSaveSavedFilterViewPayload,
} from "@api/backend/types";
import { ApiQueryKeys } from "@constants/apiConstants";
import {
  clearLegacySavedFilterViews,
  readLegacySavedFilterViews,
  writeLegacySavedFilterViews,
  type SavedFilterListKey,
  type SavedFilterView,
} from "@features/saved-filter-views/legacyStorage";

export type { SavedFilterListKey, SavedFilterView };

const migrating = new Set<SavedFilterListKey>();

function queryKey(listKey: SavedFilterListKey): [string, SavedFilterListKey] {
  return [ApiQueryKeys.SAVED_FILTER_VIEWS, listKey];
}

function listPath(listKey: SavedFilterListKey): string {
  return `/users/me/saved-filter-views?listKey=${encodeURIComponent(listKey)}`;
}

/**
 * Named list-filter bookmarks for one CSM list, persisted in Postgres via
 * the BFF. `qs` stays the opaque query string the list already serializes.
 * Leftover localStorage views are uploaded last-to-first so display order
 * is preserved. The legacy key is cleared only after every remaining PATCH
 * succeeds; a failed PATCH keeps the unuploaded entries for a later retry.
 */
export function useSavedFilterViews(listKey: SavedFilterListKey): {
  views: SavedFilterView[];
  isLoading: boolean;
  isSaving: boolean;
  saveError: Error | null;
  isDeleting: boolean;
  deleteError: Error | null;
  isMoving: boolean;
  moveError: Error | null;
  saveFilterView: (name: string, qs: string) => Promise<void>;
  deleteFilterView: (name: string) => Promise<void>;
  moveFilterView: (name: string, direction: "up" | "down") => Promise<void>;
  reorderFilterView: (name: string, position: number) => Promise<void>;
  resetSaveError: () => void;
} {
  const api = useBackendApi();
  const queryClient = useQueryClient();

  const query = useQuery({
    queryKey: queryKey(listKey),
    queryFn: async (): Promise<{ views: SavedFilterView[]; fromServer: boolean }> => {
      const res = await api.get<BeSavedFilterViewList>(listPath(listKey));
      if (res == null) {
        return { views: [], fromServer: false };
      }
      return { views: res.views ?? [], fromServer: true };
    },
  });

  useEffect(() => {
    if (!query.isSuccess || query.data === undefined) return;
    if (!query.data.fromServer) return;
    if (migrating.has(listKey)) return;
    const pending = readLegacySavedFilterViews(listKey).slice(0, 50);
    if (pending.length === 0) return;

    migrating.add(listKey);
    void (async () => {
      const remaining = [...pending];
      try {
        for (let i = remaining.length - 1; i >= 0; i -= 1) {
          await api.patch<BeSaveSavedFilterViewPayload, BeSavedFilterViewList>(
            "/users/me/saved-filter-views",
            { listKey, name: remaining[i].name, qs: remaining[i].qs },
          );
          remaining.splice(i, 1);
          writeLegacySavedFilterViews(listKey, remaining);
          await queryClient.invalidateQueries({ queryKey: queryKey(listKey) });
        }
        clearLegacySavedFilterViews(listKey);
        await queryClient.invalidateQueries({ queryKey: queryKey(listKey) });
      } catch {
        writeLegacySavedFilterViews(listKey, remaining);
        await queryClient.invalidateQueries({ queryKey: queryKey(listKey) });
      } finally {
        migrating.delete(listKey);
      }
    })();
  }, [api, listKey, query.data, query.isSuccess, queryClient]);

  const saveMutation = useMutation({
    mutationFn: (input: { name: string; qs: string }) =>
      api.patch<BeSaveSavedFilterViewPayload, BeSavedFilterViewList>(
        "/users/me/saved-filter-views",
        { listKey, name: input.name, qs: input.qs },
      ),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: queryKey(listKey) });
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (name: string) =>
      api.del<BeSavedFilterViewList>(
        `${listPath(listKey)}&name=${encodeURIComponent(name)}`,
      ),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: queryKey(listKey) });
    },
  });

  const reorderMutation = useMutation({
    mutationFn: (input: { name: string; direction?: "up" | "down"; position?: number }) =>
      api.post<BeReorderSavedFilterViewPayload, BeSavedFilterViewList>(
        "/users/me/saved-filter-views/reorder",
        {
          listKey,
          name: input.name,
          ...(input.position !== undefined
            ? { position: input.position }
            : { direction: input.direction }),
        },
      ),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: queryKey(listKey) });
    },
  });

  const saveFilterView = useCallback(
    async (name: string, qs: string): Promise<void> => {
      const trimmed = name.trim();
      if (!trimmed) return;
      await saveMutation.mutateAsync({ name: trimmed, qs });
    },
    [saveMutation.mutateAsync],
  );

  const deleteFilterView = useCallback(
    async (name: string): Promise<void> => {
      const trimmed = name.trim();
      if (!trimmed) return;
      await deleteMutation.mutateAsync(trimmed);
    },
    [deleteMutation.mutateAsync],
  );

  const moveFilterView = useCallback(
    async (name: string, direction: "up" | "down"): Promise<void> => {
      const trimmed = name.trim();
      if (!trimmed) return;
      await reorderMutation.mutateAsync({ name: trimmed, direction });
    },
    [reorderMutation.mutateAsync],
  );

  const reorderFilterView = useCallback(
    async (name: string, position: number): Promise<void> => {
      const trimmed = name.trim();
      if (!trimmed || position < 0) return;
      await reorderMutation.mutateAsync({ name: trimmed, position });
    },
    [reorderMutation.mutateAsync],
  );

  const resetSaveError = useCallback(() => {
    saveMutation.reset();
  }, [saveMutation.reset]);

  return {
    views: query.data?.views ?? [],
    isLoading: query.isLoading,
    isSaving: saveMutation.isPending,
    saveError: saveMutation.error,
    isDeleting: deleteMutation.isPending,
    deleteError: deleteMutation.error,
    isMoving: reorderMutation.isPending,
    moveError: reorderMutation.error,
    saveFilterView,
    deleteFilterView,
    moveFilterView,
    reorderFilterView,
    resetSaveError,
  };
}
