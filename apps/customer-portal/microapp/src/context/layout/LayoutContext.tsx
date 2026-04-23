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

import { createContext, type ReactNode } from "react";

export type AppBarVariant = "minimal" | "default" | "extended" | "notifications";

export type LayoutContextType = {
  /* AppBar Properties */
  title: string;
  showAppBar: boolean;
  hasBackAction: boolean;
  appBarVariant: AppBarVariant;
  overlineSlot?: ReactNode | string;
  subtitleSlot?: ReactNode | string;
  startSlot?: ReactNode;
  endSlot?: ReactNode;
  appBarSlots?: ReactNode;
  setTitleOverride: (title: string | undefined) => void;
  setOverlineSlotOverride: (slot: ReactNode | string | undefined) => void;
  setSubtitleSlotOverride: (slot: ReactNode | string | undefined) => void;
  setStartSlotOverride: (slot: ReactNode | undefined) => void;
  setEndSlotOverride: (slot: ReactNode | undefined) => void;
  setAppBarSlotsOverride: (slot: ReactNode | undefined) => void;

  /* TabBar Properties */
  activeTabIndex: number;
};

export const LayoutContext = createContext<LayoutContextType | null>(null);
