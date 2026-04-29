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

export type LayoutOverrides = {
  title?: string | ReactNode;
  overline?: ReactNode | string | null;
  subtitle?: ReactNode | string | null;
  startSlot?: ReactNode;
  endSlot?: ReactNode;
  appBarSlots?: ReactNode;
};

export type LayoutContextType = {
  /* AppBar Properties */
  title: string | ReactNode;
  showAppBar: boolean;
  hasBackAction: boolean;
  appBarVariant: AppBarVariant;
  overlineSlot?: ReactNode | string | null;
  subtitleSlot?: ReactNode | string | null;
  startSlot?: ReactNode;
  endSlot?: ReactNode;
  appBarSlots?: ReactNode;
  setLayoutOverrides: (overrides: LayoutOverrides) => void;
  clearLayoutOverrides: () => void;

  /* TabBar Properties */
  activeTabIndex: number;
};

export const LayoutContext = createContext<LayoutContextType | null>(null);
