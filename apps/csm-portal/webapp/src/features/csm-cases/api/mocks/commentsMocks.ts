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

import type { CsmCaseComment } from "@features/csm-cases/types/csmCases";

const minutesAgo = (n: number): string =>
  new Date(Date.now() - n * 60_000).toISOString();

const SEED_COMMENTS: Record<string, CsmCaseComment[]> = {
  "case-1001": [
    {
      id: "cmt-1001-1",
      caseId: "case-1001",
      authorName: "Rohan Mehta",
      authorRole: "customer",
      bodyHtml:
        "<p>P99 token issuance latency on prod jumped from <strong>~80ms</strong> to <strong>~1.2s</strong> starting around 09:15 UTC. No deploy on our side. Attaching the IS access log slice.</p>",
      createdAt: minutesAgo(60 * 7),
    },
    {
      id: "cmt-1001-2",
      caseId: "case-1001",
      authorName: "Sajith Ekanayaka",
      authorRole: "wso2_engineer",
      bodyHtml:
        "<p>Picked up. Could you confirm whether you see the spike on the LDAP-backed userstore or only the JDBC primary? Also, are sessions persisting across cluster nodes?</p>",
      createdAt: minutesAgo(60 * 6),
    },
    {
      id: "cmt-1001-3",
      caseId: "case-1001",
      authorName: "System",
      authorRole: "system",
      bodyHtml:
        "<p>First-response SLA breached by 22 minutes.</p>",
      createdAt: minutesAgo(22),
    },
  ],
  "case-1002": [
    {
      id: "cmt-1002-1",
      caseId: "case-1002",
      authorName: "Janet Park",
      authorRole: "customer",
      bodyHtml:
        "<p>API Manager gateway returning 502 for ~3% of requests during the 12:00–13:00 peak window. Backend services are healthy.</p>",
      createdAt: minutesAgo(60 * 3),
    },
    {
      id: "cmt-1002-2",
      caseId: "case-1002",
      authorName: "Sajith Ekanayaka",
      authorRole: "wso2_engineer",
      bodyHtml:
        "<p>Looking at this now. Can you share gateway pod resource usage during the spike, plus a thread dump from one of the affected pods?</p>",
      createdAt: minutesAgo(60 * 2 + 20),
    },
    {
      id: "cmt-1002-3",
      caseId: "case-1002",
      authorName: "Janet Park",
      authorRole: "customer",
      bodyHtml:
        "<p>Provided thread dump and gateway logs from the peak window. CPU was at ~85% on two of the three pods during the spike.</p>",
      createdAt: minutesAgo(4),
    },
  ],
  "case-1004": [
    {
      id: "cmt-1004-1",
      caseId: "case-1004",
      authorName: "Rohan Mehta",
      authorRole: "customer",
      bodyHtml:
        "<p>Userinfo endpoint isn't returning the <code>groups</code> claim for users authenticated via the corporate IdP. Worked last week.</p>",
      createdAt: minutesAgo(75),
    },
  ],
  "case-1005": [
    {
      id: "cmt-1005-1",
      caseId: "case-1005",
      authorName: "Sajith Ekanayaka",
      authorRole: "wso2_engineer",
      bodyHtml:
        "<p>Customer report: Choreo deployment stuck at 'Provisioning' for ~45 minutes. Component is a Go service.</p>",
      createdAt: minutesAgo(60 * 4),
    },
    {
      id: "cmt-1005-2",
      caseId: "case-1005",
      authorName: "Sajith Ekanayaka",
      authorRole: "wso2_engineer",
      bodyHtml:
        "<p>Asked customer for <code>kubectl describe</code> output on the stuck pod.</p>",
      createdAt: minutesAgo(115),
    },
  ],
};

const runtime: Map<string, CsmCaseComment[]> = new Map(
  Object.entries(SEED_COMMENTS).map(([k, v]) => [k, [...v]]),
);

function ensureBucket(caseId: string): CsmCaseComment[] {
  let bucket = runtime.get(caseId);
  if (!bucket) {
    bucket = [];
    runtime.set(caseId, bucket);
  }
  return bucket;
}

export function getMockCsmCaseComments(caseId: string): CsmCaseComment[] {
  // Return a fresh array so callers can't mutate our store.
  return [...ensureBucket(caseId)].sort((a, b) =>
    a.createdAt.localeCompare(b.createdAt),
  );
}

export interface MockPostCsmCaseCommentInput {
  caseId: string;
  bodyHtml: string;
  authorName: string;
  authorRole: "wso2_engineer";
}

export function postMockCsmCaseComment(
  input: MockPostCsmCaseCommentInput,
): CsmCaseComment {
  const comment: CsmCaseComment = {
    id: `cmt-${input.caseId}-${Date.now()}`,
    caseId: input.caseId,
    authorName: input.authorName,
    authorRole: input.authorRole,
    bodyHtml: input.bodyHtml,
    createdAt: new Date().toISOString(),
  };
  ensureBucket(input.caseId).push(comment);
  return comment;
}
