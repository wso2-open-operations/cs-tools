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

import { useMemo, useState, type ReactNode } from "react";
import { matchPath, useLocation } from "react-router-dom";
import { LayoutContext, type LayoutContextType } from "@src/context/layout";
import { MAIN_LAYOUT_CONFIG, type MainLayoutConfigType } from "@src/components/layout/config";
import { Logger } from "@root/src/utils/logger";

export default function LayoutProvider({ children }: { children: React.ReactNode }) {
  const location = useLocation();
  const [titleOverride, setTitleOverride] = useState<string | undefined>(undefined);
  const [overlineSlotOverride, setOverlineSlotOverride] = useState<ReactNode | string | undefined>(undefined);
  const [subtitleSlotOverride, setSubtitleSlotOverride] = useState<ReactNode | string | undefined>(undefined);
  const [startSlotOverride, setStartSlotOverride] = useState<ReactNode | undefined>(undefined);
  const [endSlotOverride, setEndSlotOverride] = useState<ReactNode | undefined>(undefined);
  const [appBarSlotsOverride, setAppBarSlotsOverride] = useState<ReactNode | undefined>(undefined);

  const meta: MainLayoutConfigType = useMemo(() => {
    const config = MAIN_LAYOUT_CONFIG.find((route) => matchPath({ path: route.path, end: true }, location.pathname));

    if (!config) {
      Logger.error("Route Configuration Not Found");
      return { path: location.pathname, title: "", tabIndex: -1 };
    }

    return config;
  }, [location.pathname]);

  const value: LayoutContextType = useMemo(
    () => ({
      title: titleOverride ?? meta.title,
      showAppBar: meta.showAppBar ?? true,
      hasBackAction: meta.hasBackAction ?? false,
      appBarVariant: meta.appBarVariant ?? "default",
      overlineSlot: overlineSlotOverride ?? meta.overlineSlot ?? null,
      subtitleSlot: subtitleSlotOverride ?? meta.subtitleSlot ?? null,
      startSlot: startSlotOverride ?? meta.startSlot ?? null,
      endSlot: endSlotOverride ?? meta.endSlot ?? null,
      appBarSlots: appBarSlotsOverride ?? meta.appBarSlots ?? null,
      setTitleOverride,
      setOverlineSlotOverride,
      setSubtitleSlotOverride,
      setStartSlotOverride,
      setEndSlotOverride,
      setAppBarSlotsOverride,

      activeTabIndex: meta.tabIndex,
    }),

    [
      meta,
      titleOverride,
      overlineSlotOverride,
      subtitleSlotOverride,
      startSlotOverride,
      endSlotOverride,
      appBarSlotsOverride,
    ],
  );
  return <LayoutContext.Provider value={value}>{children}</LayoutContext.Provider>;
}
