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

import type { CsmNotification } from "@features/csm-notifications/types/csmNotifications";

const minutesAgo = (n: number): string =>
  new Date(Date.now() - n * 60_000).toISOString();

const SEED: CsmNotification[] = [
  {
    id: "ntf-1",
    kind: "sla_breach",
    title: "First-response SLA breached",
    summary: "CS-1001 · Identity Server token issuance latency · Acme Financial",
    href: "/cases/case-1001",
    createdAt: minutesAgo(22),
    isRead: false,
  },
  {
    id: "ntf-2",
    kind: "new_comment",
    title: "Customer replied",
    summary: "CS-1002 · Janet Park · Provided thread dump and gateway logs",
    href: "/cases/case-1002",
    createdAt: minutesAgo(4),
    isRead: false,
  },
  {
    id: "ntf-3",
    kind: "sla_at_risk",
    title: "Resolution SLA at risk",
    summary: "CS-1002 · 12m until breach · API Manager gateway 502",
    href: "/cases/case-1002",
    createdAt: minutesAgo(15),
    isRead: false,
  },
  {
    id: "ntf-4",
    kind: "case_assigned",
    title: "Case assigned to you",
    summary: "CS-1018 · Tenant admin lost MFA device · Acme Financial",
    href: "/cases/case-1018",
    createdAt: minutesAgo(10),
    isRead: false,
  },
  {
    id: "ntf-5",
    kind: "escalation_opened",
    title: "Escalation opened",
    summary: "CS-1007 · Streaming Integrator backpressure · Initech Systems",
    href: "/cases/case-1007",
    createdAt: minutesAgo(60 * 4),
    isRead: true,
  },
  {
    id: "ntf-6",
    kind: "new_comment",
    title: "Customer replied",
    summary: "CS-1004 · OIDC userinfo claims missing groups",
    href: "/cases/case-1004",
    createdAt: minutesAgo(80),
    isRead: true,
  },
  {
    id: "ntf-7",
    kind: "sla_breach",
    title: "Resolution SLA breached",
    summary: "CS-1007 · breached 3h ago · Initech Systems",
    href: "/cases/case-1007",
    createdAt: minutesAgo(180),
    isRead: true,
  },
];

const runtime: CsmNotification[] = SEED.map((n) => ({ ...n }));

export function getMockCsmNotifications(): CsmNotification[] {
  // Most recent first.
  return runtime
    .slice()
    .sort((a, b) => b.createdAt.localeCompare(a.createdAt));
}

export function markMockNotificationRead(id: string): void {
  const target = runtime.find((n) => n.id === id);
  if (target) target.isRead = true;
}

export function markAllMockNotificationsRead(): void {
  for (const n of runtime) n.isRead = true;
}
