import type { CaseType } from "@shared/types";

export const ROUTES = {
  default_case: { all: "/cases/all", by: (id: string) => `/cases/${id}`, create: "/create" },
  service_request: { all: "/service-requests/all", by: (id: string) => `/service-requests/${id}` },
  change_request: { all: "/change-requests/all", by: (id: string) => `/change-requests/${id}` },
  chat: { all: "/conversations/all", by: (id: string) => `/conversations/${id}`, create: "/chat" },
  engagement: { all: "/engagements/all", by: (id: string) => `/engagements/${id}` },
  announcement: { all: "/announcements/all", by: (id: string) => `/announcements/${id}` },
  security_report_analysis: {
    all: "/security-report-analysis/all",
    by: (id: string) => `/security-report-analysis/${id}`,
  },
  users: { invite: "/users/invite", edit: "/users/edit" },
} satisfies Record<CaseType, unknown> & Record<string, unknown>;

export const PDF_JS_DIST_CDN = (version: string) => `https://unpkg.com/pdfjs-dist@${version}/build/pdf.worker.min.mjs`;
