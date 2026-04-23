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

import type { ReactNode } from "react";
import type { AppBarVariant } from "@src/context/layout";
import { FilterAppBarSlot } from "@pages/AllItemsPage";
import { MessageSquareQuote } from "@wso2/oxygen-ui-icons-react";
import { Box, pxToRem } from "@wso2/oxygen-ui";

type AppBarConfig = {
  showNotifications: boolean;
  showProjectSelector: boolean;
  showChips: boolean;
};

export interface MainLayoutConfigType {
  path: string;
  title: string;
  tabIndex: number;
  showAppBar?: boolean;
  hasBackAction?: boolean;
  appBarVariant?: AppBarVariant;
  overlineSlot?: ReactNode | string;
  subtitleSlot?: ReactNode | string;
  startSlot?: ReactNode;
  endSlot?: ReactNode;
  appBarSlots?: ReactNode;
}

export const APP_BAR_CONFIG: Record<AppBarVariant, AppBarConfig> = {
  default: {
    showNotifications: true,
    showProjectSelector: true,
    showChips: false,
  },
  notifications: {
    showNotifications: false,
    showProjectSelector: true,
    showChips: false,
  },
  minimal: {
    showNotifications: false,
    showProjectSelector: false,
    showChips: false,
  },
  extended: {
    showNotifications: true,
    showProjectSelector: true,
    showChips: true,
  },
};

export const MAIN_LAYOUT_CONFIG: MainLayoutConfigType[] = [
  { path: "/", appBarVariant: "extended", tabIndex: 0 },
  { path: "/select", title: "Select Project", tabIndex: -1 },
  { path: "/support", tabIndex: 1 },
  { path: "/users", tabIndex: 2 },
  { path: "/users/invite", title: "Invite User", appBarVariant: "notifications", hasBackAction: true, tabIndex: -1 },
  { path: "/users/edit", title: "Edit User", appBarVariant: "notifications", hasBackAction: true, tabIndex: -1 },
  { path: "/profile", appBarVariant: "minimal", tabIndex: 3 },
  {
    path: "/notifications",
    title: "Notifications",
    appBarVariant: "notifications",
    hasBackAction: true,
    tabIndex: -1,
  },
  {
    path: "/chat",
    startSlot: (
      <Box color="primary.main">
        <MessageSquareQuote size={pxToRem(36)} />
      </Box>
    ),
    title: "Chat with Novera",
    subtitleSlot: "AI-powered support assistant",
    appBarVariant: "minimal",
    hasBackAction: true,
    tabIndex: -1,
  },
  {
    path: "/create",
    title: "Create Support Case",
    subtitleSlot: "Auto-populated from chat",
    appBarVariant: "minimal",
    hasBackAction: true,
    tabIndex: -1,
  },
  {
    path: "/cases/all",
    title: "All Cases",
    appBarVariant: "minimal",
    hasBackAction: true,
    appBarSlots: <FilterAppBarSlot type="case" />,
    tabIndex: -1,
  },
  {
    path: "/cases/:id",
    title: "", // DYNAMIC
    appBarVariant: "minimal",
    hasBackAction: true,
    tabIndex: -1,
  },
  {
    path: "/chats/all",
    title: "All Chats",
    appBarVariant: "minimal",
    hasBackAction: true,
    tabIndex: -1,
    appBarSlots: <FilterAppBarSlot type="chat" />,
  },
  {
    path: "/chats/:id",
    title: "", // DYNAMIC
    appBarVariant: "minimal",
    hasBackAction: true,
    tabIndex: -1,
  },
  {
    path: "/services/all",
    title: "All Service Requests",
    appBarVariant: "minimal",
    hasBackAction: true,
    tabIndex: -1,
    appBarSlots: <FilterAppBarSlot type="service" />,
  },
  {
    path: "/services/:id",
    title: "", // DYNAMIC
    appBarVariant: "minimal",
    hasBackAction: true,
    tabIndex: -1,
  },
  {
    path: "/changes/all",
    title: "All Change Requests",
    appBarVariant: "minimal",
    hasBackAction: true,
    tabIndex: -1,
    appBarSlots: <FilterAppBarSlot type="change" />,
  },
  {
    path: "/changes/:id",
    title: "", // DYNAMIC
    appBarVariant: "minimal",
    hasBackAction: true,
    tabIndex: -1,
  },
];
