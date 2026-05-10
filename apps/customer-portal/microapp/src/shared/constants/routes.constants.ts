export const ROUTES = {
  multiple: { all: "/multiple/all" },
  cases: { all: "/cases/all", by: (id: string) => `/cases/${id}`, create: "/create" },
  service_requests: { all: "/services/all", by: (id: string) => `/services/${id}` },
  change_requests: { all: "/changes/all", by: (id: string) => `/changes/${id}` },
  chat: "/chat",
} as const;
