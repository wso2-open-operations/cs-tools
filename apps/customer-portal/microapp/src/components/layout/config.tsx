import type { ReactNode } from "react";
import type { AppBarVariant } from "@src/context/layout";
import { DetailedPageAppBarSlot } from "@root/src/pages/CaseDetailPage";
import { FilterAppBarSlot } from "@root/src/pages/AllItemsPage";

type AppBarConfig = {
  showNotifications: boolean;
  showProjectSelector: boolean;
  showChips: boolean;
};

interface MainLayoutConfigType {
  path: string;
  title: string;
  tabIndex: number;
  showAppBar?: boolean;
  hasBackAction?: boolean;
  appBarVariant?: AppBarVariant;
  appBarSlots?: ReactNode;
}

export const APP_BAR_CONFIG: Record<AppBarVariant, AppBarConfig> = {
  default: {
    showNotifications: true,
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
  { path: "/", title: "Dashboard", appBarVariant: "extended", tabIndex: 0 },
  { path: "/select", title: "Select Project", tabIndex: -1 },
  { path: "/support", title: "Support Center", tabIndex: 1 },
  { path: "/users", title: "Project Users", tabIndex: 2 },
  { path: "/profile", title: "My Profile", appBarVariant: "minimal", tabIndex: 3 },
  {
    path: "/notifications",
    title: "Notifications",
    appBarVariant: "minimal",
    hasBackAction: true,
    tabIndex: -1,
  },
  { path: "/chat", title: "Chat with Novera", appBarVariant: "minimal", hasBackAction: true, tabIndex: -1 },
  { path: "/create", title: "Create Support Case", appBarVariant: "minimal", hasBackAction: true, tabIndex: -1 },
  {
    path: "/cases/all",
    title: "All Cases",
    appBarVariant: "minimal",
    hasBackAction: true,
    tabIndex: -1,
    appBarSlots: <FilterAppBarSlot type="case" />,
  },
  {
    path: "/cases/:id",
    title: "Authentication Service Issue",
    appBarVariant: "minimal",
    hasBackAction: true,
    tabIndex: -1,
    appBarSlots: <DetailedPageAppBarSlot />,
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
    title: "How do I configure custom claims in JWT tokens?",
    appBarVariant: "minimal",
    hasBackAction: true,
    tabIndex: -1,
    appBarSlots: <DetailedPageAppBarSlot />,
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
    title: "Enable additional API Manager environment",
    appBarVariant: "minimal",
    hasBackAction: true,
    tabIndex: -1,
    appBarSlots: <DetailedPageAppBarSlot />,
  },
  {
    path: "/changes/all",
    title: "All Change Requests",
    appBarVariant: "minimal",
    hasBackAction: true,
    tabIndex: -1,
    appBarSlots: <FilterAppBarSlot type="change" />,
  },
];
