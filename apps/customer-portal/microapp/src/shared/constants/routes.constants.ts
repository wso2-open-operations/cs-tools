import type { CaseType } from "../types";

export const ROUTES = {
  multiple: { all: "/multiple/all" },
  default_case: { all: "/cases/all", by: (id: string) => `/cases/${id}`, create: "/create" },
  service_request: { all: "/services/all", by: (id: string) => `/services/${id}` },
  change_request: { all: "/changes/all", by: (id: string) => `/changes/${id}` },
  chat: { all: "/chats/all", by: (id: string) => `/chats/${id}`, create: "/chat" },
  security_report_analysis: { all: "/sras/all", by: (id: string) => `/sras/${id}` },
  engagement: { all: "/engagements/all", by: (id: string) => `/engagements/${id}` },
  announcement: { all: "/announcements/all", by: (id: string) => `/announcements/${id}` },
} satisfies Record<CaseType, unknown> & Record<string, unknown>;
