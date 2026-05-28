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
