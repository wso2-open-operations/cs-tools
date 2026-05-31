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
    {
      id: "cmt-1005-3",
      caseId: "case-1005",
      authorName: "Sajith Ekanayaka",
      authorRole: "wso2_engineer",
      internal: true,
      bodyHtml:
        "<p><strong>Work note:</strong> Suspect the new ingress controller default timeouts. Will pair with @Priya on tomorrow's call before sharing anything externally.</p>",
      createdAt: minutesAgo(95),
    },
  ],

  // ------------------------------------------------------------------------
  // case-1015: Long-running technical thread used to evaluate engineer UX on
  // dense comment trails — Java stack traces, code blocks, config diffs,
  // long-form work notes. Spans ~36 hours of back-and-forth.
  // ------------------------------------------------------------------------
  "case-1015": [
    {
      id: "cmt-1015-1",
      caseId: "case-1015",
      authorName: "Daniel Owens",
      authorRole: "customer",
      bodyHtml:
        "<p>Choreo runtime is going OOM under sustained load (~120 RPS sustained). The crash takes the whole pod down and the autoscaler can't keep up. Started this morning around <code>05:40 UTC</code> after the rollout of release <code>v4.2.1</code>. Reverting to <code>v4.2.0</code> resolves it.</p><p>Attaching the heap dump (<code>heap-prod-01.hprof</code>, ~480MB) and the last 2000 lines of the runtime log.</p>",
      createdAt: minutesAgo(60 * 35),
    },
    {
      id: "cmt-1015-2",
      caseId: "case-1015",
      authorName: "Sajith Ekanayaka",
      authorRole: "wso2_engineer",
      bodyHtml:
        "<p>Hi Daniel — picked this up. Can you confirm:</p><ol><li>Are <strong>all</strong> pods OOMing, or just specific ones?</li><li>What's the <code>resources.limits.memory</code> on the deployment?</li><li>Is the workload steady-state ~120 RPS, or are there bursts above that?</li></ol><p>Pulling the heap dump now — will analyze with Eclipse MAT and report back.</p>",
      createdAt: minutesAgo(60 * 34 + 40),
    },
    {
      id: "cmt-1015-3",
      caseId: "case-1015",
      authorName: "Daniel Owens",
      authorRole: "customer",
      bodyHtml:
        "<p>1) All pods, every ~40min under steady load.</p><p>2) Limits are:</p><pre><code>resources:\n  limits:\n    memory: 2Gi\n    cpu: 1500m\n  requests:\n    memory: 1Gi\n    cpu: 500m</code></pre><p>3) Steady-state, no bursts. The dashboard confirms 117–122 RPS for the last 6 hours.</p>",
      createdAt: minutesAgo(60 * 34 + 20),
    },
    {
      id: "cmt-1015-wn-1",
      caseId: "case-1015",
      authorName: "Sajith Ekanayaka",
      authorRole: "wso2_engineer",
      internal: true,
      bodyHtml:
        "<p><strong>Work note:</strong> Heap dump downloaded. MAT leak suspect points at <code>io.netty.buffer.PooledByteBufAllocator</code> retaining ~1.4GB. Could be the same Netty pool leak we saw in INC-4877 (Choreo runtime 4.1.x). Will diff <code>v4.2.0</code> → <code>v4.2.1</code> for Netty config changes.</p>",
      createdAt: minutesAgo(60 * 33 + 50),
    },
    {
      id: "cmt-1015-4",
      caseId: "case-1015",
      authorName: "Sajith Ekanayaka",
      authorRole: "wso2_engineer",
      bodyHtml:
        '<p>The heap dump shows a Netty buffer pool leak. The full leak suspect from Eclipse MAT:</p><pre><code>The thread io.netty.channel.nio.NioEventLoop @ 0x7f8a2c3d4f00 NioEventLoop-3-1\nkeeps local variables with total size 1,432,891,032 (71.46%) bytes.\n\nThe memory is accumulated in one instance of\n"io.netty.buffer.PooledByteBufAllocator$PoolThreadLocalCache" loaded by\n"jdk.internal.loader.ClassLoaders$AppClassLoader @ 0x7f8a2c000028".</code></pre><p>And the suspicious thread stack from the dump:</p><pre><code>at io.netty.buffer.PoolArena.allocateNormal(PoolArena.java:241)\nat io.netty.buffer.PoolArena.allocate(PoolArena.java:215)\nat io.netty.buffer.PoolArena.allocate(PoolArena.java:147)\nat io.netty.buffer.PooledByteBufAllocator.newDirectBuffer(PooledByteBufAllocator.java:393)\nat io.netty.buffer.AbstractByteBufAllocator.directBuffer(AbstractByteBufAllocator.java:188)\nat io.netty.buffer.AbstractByteBufAllocator.directBuffer(AbstractByteBufAllocator.java:179)\nat io.netty.buffer.AbstractByteBufAllocator.ioBuffer(AbstractByteBufAllocator.java:140)\nat io.netty.channel.DefaultMaxMessagesRecvByteBufAllocator$MaxMessageHandle.allocate(DefaultMaxMessagesRecvByteBufAllocator.java:114)\nat io.netty.channel.nio.AbstractNioByteChannel$NioByteUnsafe.read(AbstractNioByteChannel.java:147)\nat io.netty.channel.nio.NioEventLoop.processSelectedKey(NioEventLoop.java:788)\nat io.netty.channel.nio.NioEventLoop.processSelectedKeysOptimized(NioEventLoop.java:724)\nat io.netty.channel.nio.NioEventLoop.processSelectedKeys(NioEventLoop.java:650)\nat io.netty.channel.nio.NioEventLoop.run(NioEventLoop.java:562)\nat io.netty.util.concurrent.SingleThreadEventExecutor$4.run(SingleThreadEventExecutor.java:997)\nat java.base/java.lang.Thread.run(Thread.java:840)</code></pre><p>This pattern matches a known issue we fixed in <code>v4.2.2</code>: the new Netty connection-keepalive default was tuned aggressively and caused pool buffers to be cached per-thread without an eviction policy.</p>',
      createdAt: minutesAgo(60 * 32 + 10),
    },
    {
      id: "cmt-1015-5",
      caseId: "case-1015",
      authorName: "Daniel Owens",
      authorRole: "customer",
      bodyHtml:
        "<p>That looks like exactly what we're seeing. Is <code>v4.2.2</code> available? Should we wait for the patch or is there a workaround?</p>",
      createdAt: minutesAgo(60 * 31 + 40),
    },
    {
      id: "cmt-1015-6",
      caseId: "case-1015",
      authorName: "Sajith Ekanayaka",
      authorRole: "wso2_engineer",
      bodyHtml:
        '<p>Two options:</p><p><strong>(A) Workaround (apply now, no downtime):</strong> override the Netty allocator settings via JVM args on the deployment:</p><pre><code>env:\n  - name: JAVA_OPTS\n    value: "-Dio.netty.allocator.maxOrder=9 -Dio.netty.allocator.numDirectArenas=4 -Dio.netty.allocator.cacheTrimInterval=4000"</code></pre><p>This caps the per-thread direct-buffer cache and trims it every 4s. We\'ve validated this on three customer environments — it brings the steady-state direct memory back to ~280MB.</p><p><strong>(B) Patch:</strong> <code>v4.2.2</code> is rolling out next Tuesday (full release). We can backport the fix for you to <code>v4.2.1-hotfix-3</code> today if you prefer staying on .1 line. Let me know.</p>',
      createdAt: minutesAgo(60 * 31 + 15),
    },
    {
      id: "cmt-1015-wn-2",
      caseId: "case-1015",
      authorName: "Lakshmi I.",
      authorRole: "wso2_engineer",
      internal: true,
      bodyHtml:
        "<p>Coordinating with R&amp;D on the hotfix release. Budhi confirmed the cherry-pick from <code>v4.2.2</code> → <code>v4.2.1-hotfix-3</code> is straightforward (single commit on the Netty allocator config).</p><p>FYI Daniel is on the Acme Choreo account, managed-cloud tier — we should prioritize hotfix path B.</p>",
      createdAt: minutesAgo(60 * 30 + 50),
    },
    {
      id: "cmt-1015-7",
      caseId: "case-1015",
      authorName: "Daniel Owens",
      authorRole: "customer",
      bodyHtml:
        "<p>Applied (A) on the staging deployment first. Direct memory steady at 240–290MB across 4 pods for the last 30 min, no OOMs. Rolling to prod in 15 min unless you say otherwise.</p>",
      createdAt: minutesAgo(60 * 30),
    },
    {
      id: "cmt-1015-8",
      caseId: "case-1015",
      authorName: "Sajith Ekanayaka",
      authorRole: "wso2_engineer",
      bodyHtml:
        "<p>Looks good. Go ahead with prod rollout. Keep us posted on the first 2 hours of steady-state metrics.</p><p>Re: option B — we'll prep <code>v4.2.1-hotfix-3</code> regardless; you can take it whenever your change window allows.</p>",
      createdAt: minutesAgo(60 * 29 + 50),
    },
    {
      id: "cmt-1015-9",
      caseId: "case-1015",
      authorName: "Daniel Owens",
      authorRole: "customer",
      bodyHtml:
        "<p>Prod rollout complete. Memory profile looks flat for the last 90 min. The 40-min OOM cycle is gone.</p><p>One follow-up: we noticed a slight latency increase (p99 from 18ms → 27ms) since applying the JVM args. Is that expected? If so, we can live with it; just want to confirm.</p>",
      createdAt: minutesAgo(60 * 28),
    },
    {
      id: "cmt-1015-10",
      caseId: "case-1015",
      authorName: "Sajith Ekanayaka",
      authorRole: "wso2_engineer",
      bodyHtml:
        "<p>Yes — expected. <code>cacheTrimInterval=4000</code> trims the per-thread buffer cache every 4s; that adds a small reallocation cost on the next request after each trim. Trade-off is ~5–10ms p99 bump vs ~1.2GB direct memory headroom. Most customers prefer the headroom.</p><p>You can tune via <code>cacheTrimInterval</code> if you want a different balance — e.g. <code>8000</code> halves the latency penalty but doubles peak retained memory.</p>",
      createdAt: minutesAgo(60 * 27 + 50),
    },
    {
      id: "cmt-1015-11",
      caseId: "case-1015",
      authorName: "Daniel Owens",
      authorRole: "customer",
      bodyHtml:
        "<p>Understood. We'll stick with 4000 for now. Will plan to take <code>v4.2.1-hotfix-3</code> during next Wednesday's change window.</p>",
      createdAt: minutesAgo(60 * 27 + 40),
    },
    {
      id: "cmt-1015-wn-3",
      caseId: "case-1015",
      authorName: "Sajith Ekanayaka",
      authorRole: "wso2_engineer",
      internal: true,
      bodyHtml:
        "<p><strong>Work note:</strong> Linking this case to INC-5012 for the platform-wide tracker. Daniel's account is the third managed-cloud customer hitting this since the v4.2.1 rollout — pattern confirmed. R&amp;D is fast-tracking the proper fix into v4.2.2 GA next week.</p><p>Adding @Asanka as watcher since he owns the Choreo runtime SRE on-call this week.</p>",
      createdAt: minutesAgo(60 * 26),
    },
    {
      id: "cmt-1015-12",
      caseId: "case-1015",
      authorName: "Asanka R.",
      authorRole: "wso2_engineer",
      internal: true,
      bodyHtml:
        "<p>Thanks for the loop in. Picking up the platform-side rollout coordination — I'll make sure the v4.2.2 release notes call out this fix prominently so customers know to take it.</p>",
      createdAt: minutesAgo(60 * 25 + 40),
    },
    {
      id: "cmt-1015-13",
      caseId: "case-1015",
      authorName: "Daniel Owens",
      authorRole: "customer",
      bodyHtml:
        "<p>Update: 18 hours of steady-state production traffic with the workaround. Zero OOMs, memory profile flat, latency stable at p99=27ms. Calling this resolved on our end.</p><p>One question — for the hotfix rollout next Wednesday, do we need to revert the JVM args, or do they coexist safely with the fixed allocator?</p>",
      createdAt: minutesAgo(60 * 9),
    },
    {
      id: "cmt-1015-14",
      caseId: "case-1015",
      authorName: "Sajith Ekanayaka",
      authorRole: "wso2_engineer",
      bodyHtml:
        "<p>Great to hear. Yes, please revert the JVM args when you apply the hotfix — the new allocator defaults in <code>v4.2.1-hotfix-3</code> are tuned to be safe out-of-the-box, and leaving the manual override on top will give you back the latency penalty without any benefit.</p><p>Suggested rollout:</p><ol><li>Apply <code>v4.2.1-hotfix-3</code> to staging, remove the <code>JAVA_OPTS</code> overrides.</li><li>Run for ~2 hours, confirm flat memory profile + p99 back to ~18ms.</li><li>Roll to prod with the same change.</li></ol><p>Marking this case as solution proposed once you confirm the hotfix path is acceptable.</p>",
      createdAt: minutesAgo(60 * 8 + 40),
    },
    {
      id: "cmt-1015-15",
      caseId: "case-1015",
      authorName: "Daniel Owens",
      authorRole: "customer",
      bodyHtml:
        "<p>Sounds good. Hotfix path acceptable. We'll execute Wednesday change window. Thanks for the thorough investigation and fast turnaround.</p>",
      createdAt: minutesAgo(60 * 7),
    },
  ],
};

// Seed a few work notes on cases that already have comment threads to make
// the unified Activities feed look realistic out of the box.
const WORK_NOTES: CsmCaseComment[] = [
  {
    id: "wn-1001-1",
    caseId: "case-1001",
    authorName: "Sajith Ekanayaka",
    authorRole: "wso2_engineer",
    internal: true,
    bodyHtml:
      "<p><strong>Work note:</strong> Pulled GC logs from IS node 2. Heap usage normal; suspect connection pool exhaustion on the JDBC userstore.</p>",
    createdAt: minutesAgo(60 * 5),
  },
  {
    id: "wn-1001-2",
    caseId: "case-1001",
    authorName: "Lakshmi I.",
    authorRole: "wso2_engineer",
    internal: true,
    bodyHtml:
      "<p>Looped in @Priya for a second look. Will reply to customer once we have a confirmed root cause.</p>",
    createdAt: minutesAgo(60 * 4),
  },
  {
    id: "wn-1002-1",
    caseId: "case-1002",
    authorName: "Sajith Ekanayaka",
    authorRole: "wso2_engineer",
    internal: true,
    bodyHtml:
      "<p><strong>Work note:</strong> Pod-2 thread dump shows 92 threads stuck in <code>BlockingChannel.read</code> — looks like the same pattern from INC-4877.</p>",
    createdAt: minutesAgo(60 * 1 + 30),
  },
];
for (const wn of WORK_NOTES) {
  const bucket = SEED_COMMENTS[wn.caseId] ?? [];
  bucket.push(wn);
  SEED_COMMENTS[wn.caseId] = bucket;
}

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
  internal?: boolean;
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
    internal: input.internal ?? false,
  };
  ensureBucket(input.caseId).push(comment);
  return comment;
}
