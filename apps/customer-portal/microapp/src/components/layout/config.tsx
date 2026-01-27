import type { ReactNode } from "react";
import type { AppBarVariant } from "@src/context/layout";
import { FilterAppBarSlot } from "@pages/AllItemsPage";
import { Assistant } from "@mui/icons-material";

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
  { path: "/", title: "Dashboard", appBarVariant: "extended", tabIndex: 0 },
  { path: "/select", title: "Select Project", tabIndex: -1 },
  { path: "/support", title: "Support Center", tabIndex: 1 },
  { path: "/users", title: "Project Users", tabIndex: 2 },
  { path: "/users/invite", title: "Invite User", appBarVariant: "notifications", hasBackAction: true, tabIndex: -1 },
  { path: "/users/edit", title: "Edit User", appBarVariant: "notifications", hasBackAction: true, tabIndex: -1 },
  { path: "/profile", title: "My Profile", appBarVariant: "minimal", tabIndex: 3 },
  {
    path: "/notifications",
    title: "Notifications",
    appBarVariant: "notifications",
    hasBackAction: true,
    tabIndex: -1,
  },
  {
    path: "/chat",
    startSlot: <Assistant color="primary" sx={(theme) => ({ fontSize: theme.typography.pxToRem(36) })} />,
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
