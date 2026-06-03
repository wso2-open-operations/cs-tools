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

import { useQueryClient } from "@tanstack/react-query";
import {
  createContext,
  useCallback,
  useContext,
  useMemo,
  useState,
  type JSX,
  type ReactNode,
} from "react";

const STORAGE_KEY = "csm.useMocks";

interface MockModeContextValue {
  /** When true, hooks short-circuit and return seeded mock data. */
  useMocks: boolean;
  /** Set the flag directly. */
  setUseMocks: (next: boolean) => void;
  /** Convenience: flip the flag. */
  toggle: () => void;
}

const MockModeContext = createContext<MockModeContextValue | null>(null);

/**
 * Read the saved mock preference at module load and apply it to
 * `window.config.CSM_PORTAL_USE_MOCKS` BEFORE any hook reads the flag.
 *
 * This must run before React mounts, so callers should invoke it from
 * `main.tsx` next to the other early-boot side effects (e.g. Prism, CSS).
 */
export function hydrateMockModeFromStorage(): void {
  if (typeof window === "undefined") return;
  try {
    const saved = window.localStorage.getItem(STORAGE_KEY);
    if (saved !== null) {
      // Mutate `window.config` in place so existing hooks (which read
      // `window.config?.CSM_PORTAL_USE_MOCKS` directly) pick up the
      // override without any code change.
      const cfg = (window as unknown as { config?: { CSM_PORTAL_USE_MOCKS?: boolean } }).config;
      if (cfg) {
        cfg.CSM_PORTAL_USE_MOCKS = saved === "1";
      }
    }
  } catch {
    // localStorage can throw in sandboxed contexts — ignore and use the
    // build-time default already on window.config.
  }
}

function readInitial(): boolean {
  if (typeof window === "undefined") return false;
  try {
    const saved = window.localStorage.getItem(STORAGE_KEY);
    if (saved !== null) return saved === "1";
  } catch {
    /* fall through to config default */
  }
  const cfg = (window as unknown as { config?: { CSM_PORTAL_USE_MOCKS?: boolean } }).config;
  return !!cfg?.CSM_PORTAL_USE_MOCKS;
}

/**
 * Provides the runtime mock-vs-backend toggle. Lives inside the
 * QueryClientProvider so a toggle can invalidate every cached query and
 * force the next render to refetch through whichever code path the new
 * flag selects.
 */
export function MockModeProvider({ children }: { children: ReactNode }): JSX.Element {
  const qc = useQueryClient();
  const [useMocks, setUseMocksState] = useState<boolean>(() => readInitial());

  const setUseMocks = useCallback(
    (next: boolean): void => {
      setUseMocksState(next);
      try {
        window.localStorage.setItem(STORAGE_KEY, next ? "1" : "0");
      } catch {
        /* ignore */
      }
      const cfg = (window as unknown as { config?: { CSM_PORTAL_USE_MOCKS?: boolean } }).config;
      if (cfg) cfg.CSM_PORTAL_USE_MOCKS = next;
      // Drop all cached results so the next render takes the new code path.
      qc.invalidateQueries();
    },
    [qc],
  );

  const value = useMemo<MockModeContextValue>(
    () => ({
      useMocks,
      setUseMocks,
      toggle: () => setUseMocks(!useMocks),
    }),
    [useMocks, setUseMocks],
  );

  return <MockModeContext.Provider value={value}>{children}</MockModeContext.Provider>;
}

export function useMockMode(): MockModeContextValue {
  const ctx = useContext(MockModeContext);
  if (!ctx) {
    throw new Error("useMockMode must be used within a MockModeProvider");
  }
  return ctx;
}
