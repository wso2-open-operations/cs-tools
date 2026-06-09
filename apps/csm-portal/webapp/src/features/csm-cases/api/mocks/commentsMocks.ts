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
        "<p>Hi WSO2 support,</p>" +
        "<p>P99 token issuance latency on prod jumped from <strong>~80ms</strong> to <strong>~1.2s</strong> starting around <code>09:15 UTC</code>. No deploy on our side in the last 72 hours.</p>" +
        "<p>Quick summary of what we've ruled out:</p>" +
        "<ul>" +
        "<li>GC pauses on all IS nodes are under 50ms</li>" +
        "<li>Mutual TLS handshake times look normal (~40ms)</li>" +
        "<li>JDBC userstore pool sits at <code>18/100</code> — not exhausted</li>" +
        "<li>LDAP-backed userstore latency unchanged</li>" +
        "</ul>" +
        "<p>For reference, our last green run is in this Grafana snapshot: <a href=\"https://grafana.acmefinancial.example/d/is-token-latency?from=now-7d&to=now-1d\" target=\"_blank\" rel=\"noopener\">is-token-latency (last 7d)</a>.</p>" +
        "<p>I've attached the IS access log slice from 09:00–10:30 UTC and a thread dump captured during the spike. Treating as high priority — our CISO is asking for an ETA.</p>" +
        "<p>Thanks,<br>Rohan Mehta<br>Platform Engineering, Acme Financial</p>",
      createdAt: minutesAgo(60 * 7),
    },
    {
      id: "cmt-1001-2",
      caseId: "case-1001",
      authorName: "Sajith Ekanayaka",
      authorRole: "wso2_engineer",
      bodyHtml:
        "<p>Hi Rohan,</p>" +
        "<p>Picked this up. A few clarifying questions before I dig into the thread dump:</p>" +
        "<ol>" +
        "<li>Does the spike show up on the LDAP-backed userstore or only the JDBC primary?</li>" +
        "<li>Are sessions persisting across cluster nodes (sticky sessions vs. distributed cache)?</li>" +
        "<li>Have any TLS truststore or KMS configs been updated on the load balancer in front of IS?</li>" +
        "</ol>" +
        "<p>While you're checking — we documented a similar token-issuance-latency investigation in <a href=\"https://docs.wso2.com/identity-server/troubleshooting/token-latency\" target=\"_blank\" rel=\"noopener\">our IS troubleshooting playbook</a>. The connection pool exhaustion section is what I'm comparing your numbers against.</p>" +
        "<p>Will follow up with the thread-dump analysis shortly.</p>" +
        "<p>Best regards,<br>Sajith</p>",
      createdAt: minutesAgo(60 * 6),
    },
    {
      id: "cmt-1001-3",
      caseId: "case-1001",
      authorName: "Rohan Mehta",
      authorRole: "customer",
      bodyHtml:
        "<p>Hi Sajith,</p>" +
        "<p>Answers:</p>" +
        "<ol>" +
        "<li>Only the JDBC primary. LDAP-backed userstore latency is unchanged at ~30ms.</li>" +
        "<li>Sticky sessions on the LB; distributed cache (Hazelcast) is enabled across the cluster.</li>" +
        "<li>No truststore or KMS changes on the LB or on IS in the last 30 days. Last network change was 2 weeks ago — added a WAF rule, nothing in the IS path.</li>" +
        "</ol>" +
        "<p>Two more data points we noticed:</p>" +
        "<blockquote>Hazelcast cluster log shows ~12 partition migrations between 09:00 and 09:30 UTC. We don't usually see that during steady-state operation.</blockquote>" +
        "<p>Could that be connected?</p>" +
        "<p>Thanks,<br>Rohan</p>",
      createdAt: minutesAgo(60 * 5 + 30),
    },
    {
      id: "cmt-1001-4",
      caseId: "case-1001",
      authorName: "Sajith Ekanayaka",
      authorRole: "wso2_engineer",
      bodyHtml:
        "<p>Hi Rohan,</p>" +
        "<p>That Hazelcast detail is exactly what I needed. The thread dump confirms it — the IS auth threads are blocked on <code>HazelcastInstance.getMap(\"session-cache\")</code> waiting for partition stabilisation. Snippet from <code>is-prod-2</code>:</p>" +
        "<pre><code>\"http-nio-9443-exec-127\" #847 daemon\n  at com.hazelcast.spi.impl.operationservice.impl.Invocation_Future.get(Invocation_Future.java:71)\n  at com.hazelcast.map.impl.proxy.MapProxyImpl.getInternal(MapProxyImpl.java:158)\n  at org.wso2.carbon.identity.session.SessionDataStore.getSessionContextData(SessionDataStore.java:412)\n  at org.wso2.carbon.identity.oauth2.token.TokenIssuanceHandler.handle(TokenIssuanceHandler.java:289)</code></pre>" +
        "<p>The 12 partition migrations match a known Hazelcast issue when the cluster sees a transient network hiccup and re-balances under load. See <a href=\"https://github.com/wso2/product-is/issues/18234\" target=\"_blank\" rel=\"noopener\">product-is#18234</a> for the upstream context.</p>" +
        "<p>Two paths forward:</p>" +
        "<ul>" +
        "<li><strong>Immediate workaround:</strong> raise <code>hazelcast.client.invocation.timeout.seconds</code> from default 120s to <code>240s</code> on the IS nodes, and increase <code>partition.migration.commit.timeout.seconds</code> to <code>60s</code>. This won't prevent migrations but it will stop them from cascading into request timeouts.</li>" +
        "<li><strong>Permanent fix:</strong> upgrade to <code>wso2is-7.1.0.43</code> (released yesterday) which includes the Hazelcast 5.3.6 client with the fix for #18234.</li>" +
        "</ul>" +
        "<p>Let me know which path you want to take. Happy to hop on a call to walk through the workaround config if it helps.</p>" +
        "<p>Best regards,<br>Sajith</p>",
      createdAt: minutesAgo(60 * 4),
    },
    {
      id: "cmt-1001-5",
      caseId: "case-1001",
      authorName: "Rohan Mehta",
      authorRole: "customer",
      bodyHtml:
        "<p>Hi Sajith,</p>" +
        "<p>Going with the workaround for now. Change window opens at 15:30 UTC — we'll apply the Hazelcast config bump first, monitor for an hour, then plan the .43 upgrade for next week's maintenance.</p>" +
        "<p>Will report back. Appreciate the fast turnaround.</p>" +
        "<p>Thanks,<br>Rohan</p>",
      createdAt: minutesAgo(60 * 3 + 15),
    },
    {
      id: "cmt-1001-6",
      caseId: "case-1001",
      authorName: "System",
      authorRole: "system",
      bodyHtml: "<p>First-response SLA breached by 22 minutes.</p>",
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
        "<p>Hi WSO2 team,</p>" +
        "<p>API Manager gateway is returning intermittent 502s to roughly <strong>3% of traffic</strong> during the 12:00–13:00 UTC peak window. Backend services behind the gateway are healthy:</p>" +
        "<ul>" +
        "<li>Upstream 5xx rate: 0.01% (unchanged)</li>" +
        "<li>Upstream p99 latency: 180ms (normal)</li>" +
        "<li>No backend timeouts in upstream logs</li>" +
        "</ul>" +
        "<p>Failure pattern on the gateway side:</p>" +
        "<blockquote>Two of three gateway pods show CPU at ~85% during the spike. The third stays at 40%. Pod with low CPU is the newest — restarted 3 hours ago.</blockquote>" +
        "<p>Suspect upstream connection pool saturation but we can't see pool metrics from outside. Change window is open if you need us to apply anything. Reference dashboard: <a href=\"https://grafana.initech.example/d/apim-gateway?from=now-24h\" target=\"_blank\" rel=\"noopener\">apim-gateway-24h</a>.</p>" +
        "<p>Best regards,<br>Janet Park<br>Initech SRE</p>",
      createdAt: minutesAgo(60 * 3),
    },
    {
      id: "cmt-1002-2",
      caseId: "case-1002",
      authorName: "Sajith Ekanayaka",
      authorRole: "wso2_engineer",
      bodyHtml:
        "<p>Hi Janet,</p>" +
        "<p>On it. Two asks while I get our team looking at the dashboard:</p>" +
        "<ol>" +
        "<li>Thread dump from one of the high-CPU gateway pods captured <strong>during</strong> the spike (not after). <code>jstack &lt;pid&gt; &gt; gw-pod-dump.txt</code> works.</li>" +
        "<li>Current values for these from the gateway deployment.toml:" +
        "<pre><code>[transport.http.properties]\nthread.pool.size = 200\nmax.connections.per.host = 100\nconnection.keep.alive = true</code></pre>" +
        "</li>" +
        "</ol>" +
        "<p>The low-CPU pod being the freshly-restarted one is a strong hint that the pool is leaking. We've seen this pattern in 4.3.x; see <a href=\"https://github.com/wso2/api-manager/issues/14087\" target=\"_blank\" rel=\"noopener\">api-manager#14087</a>.</p>" +
        "<p>Thanks,<br>Sajith</p>",
      createdAt: minutesAgo(60 * 2 + 20),
    },
    {
      id: "cmt-1002-3",
      caseId: "case-1002",
      authorName: "Janet Park",
      authorRole: "customer",
      bodyHtml:
        "<p>Hi Sajith,</p>" +
        "<p>Thread dump and configs uploaded. The relevant transport block:</p>" +
        "<pre><code>[transport.http.properties]\nthread.pool.size = 200\nmax.connections.per.host = 100\nconnection.keep.alive = true\nsocket.timeout = 60000\nconnection.timeout = 5000</code></pre>" +
        "<p>From the thread dump on <code>apim-gw-2</code> (high CPU):</p>" +
        "<blockquote>92 threads stuck in <code>PoolingHttpClientConnectionManager.lease()</code>. About 60 of those are blocked on <code>BlockingChannel.read()</code> waiting for a connection.</blockquote>" +
        "<p>This matches the pool exhaustion hypothesis. Should we just bump <code>max.connections.per.host</code> or is there a smarter fix?</p>" +
        "<p>Thanks,<br>Janet</p>",
      createdAt: minutesAgo(60 * 1 + 50),
    },
    {
      id: "cmt-1002-4",
      caseId: "case-1002",
      authorName: "Sajith Ekanayaka",
      authorRole: "wso2_engineer",
      bodyHtml:
        "<p>Hi Janet,</p>" +
        "<p>That's the exact pattern from #14087. Two recommended changes — both safe to apply hot:</p>" +
        "<ol>" +
        "<li>Raise <code>max.connections.per.host</code> from 100 to <strong>500</strong>. Your traffic profile justifies it; default of 100 was calibrated for low-RPS deployments.</li>" +
        "<li>Set <code>connection.eviction.idle.timeout = 30000</code> to actively reap stale connections. The default leak path keeps half-open connections around for 60+s.</li>" +
        "</ol>" +
        "<p>After applying, restart pods one at a time and watch:</p>" +
        "<pre><code>kubectl rollout restart deployment/apim-gateway -n apim\nkubectl logs -f -l app=apim-gateway -n apim | grep -E 'lease|eviction'</code></pre>" +
        "<p>Marking solution_proposed once you confirm. If steady-state is still bumpy after 24h we can dig into the leak source (looks like a custom mediator holding refs, but let's see if the config fix is enough first).</p>" +
        "<p>Best regards,<br>Sajith</p>",
      createdAt: minutesAgo(60 + 30),
    },
    {
      id: "cmt-1002-5",
      caseId: "case-1002",
      authorName: "Janet Park",
      authorRole: "customer",
      bodyHtml:
        "<p>Hi Sajith,</p>" +
        "<p>Applied both changes 30 minutes ago. Rolling restart complete. Gateway CPU dropped from 85% to 45% across all pods, 5xx rate at 0.02% (effectively gone). Will keep monitoring for the next peak window and report back tomorrow.</p>" +
        "<p>Thanks,<br>Janet</p>",
      createdAt: minutesAgo(4),
    },
  ],
  "case-1003": [
    {
      id: "cmt-1003-1",
      caseId: "case-1003",
      authorName: "Bill Lumbergh",
      authorRole: "customer",
      bodyHtml:
        "<p>Hi WSO2 support,</p>" +
        "<p>Our MI cluster fails to start after applying update level <code>wso2mi-4.4.0.7</code> last night. Two pods stay in CrashLoopBackOff.</p>" +
        "<p>The Carbon log shows:</p>" +
        "<pre><code>org.osgi.framework.BundleException: Could not resolve module: com.initech.security.userstore [318]\n  Caused by: java.lang.ClassNotFoundException: \n    org.wso2.carbon.security.user.api.UserStoreException</code></pre>" +
        "<p>This is from a custom security extension we built a year ago. It worked fine on <code>wso2mi-4.4.0.4</code> through <code>4.4.0.6</code>.</p>" +
        "<p>What changed in 4.4.0.7 around the user-API classloader? If you can confirm a class move/rename we'll just rebuild the extension.</p>" +
        "<p>Best regards,<br>Bill Lumbergh<br>Initech Platform Team</p>",
      createdAt: minutesAgo(60 * 6),
    },
    {
      id: "cmt-1003-wn-1",
      caseId: "case-1003",
      authorName: "Sajith Ekanayaka",
      authorRole: "wso2_engineer",
      internal: true,
      bodyHtml:
        "<p><strong>Work note:</strong> 4.4.0.7 included a JDK 17 cleanup that moved <code>org.wso2.carbon.security.user.api</code> from the old <code>org.wso2.carbon.core</code> bundle into <code>org.wso2.carbon.identity.framework</code>. Customer's extension is targeting the old import-package. Need to give them the new manifest entries.</p>",
      createdAt: minutesAgo(60 * 5 + 30),
    },
    {
      id: "cmt-1003-2",
      caseId: "case-1003",
      authorName: "Sajith Ekanayaka",
      authorRole: "wso2_engineer",
      bodyHtml:
        "<p>Hi Bill,</p>" +
        "<p>Confirmed — 4.4.0.7 moved the user-API classes during a JDK 17 cleanup. Your extension is importing the old bundle.</p>" +
        "<p>Old (broken) import in your MANIFEST.MF:</p>" +
        "<pre><code>Import-Package: org.wso2.carbon.user.api;version=\"[1.0,2)\"</code></pre>" +
        "<p>New entry to add:</p>" +
        "<pre><code>Import-Package: \n  org.wso2.carbon.identity.user.api;version=\"[1.0,2)\",\n  org.wso2.carbon.user.api;version=\"[1.0,2)\";resolution:=optional</code></pre>" +
        "<p>The <code>resolution:=optional</code> on the old package keeps your extension backward-compatible with 4.4.0.6 and earlier — same JAR will load on both.</p>" +
        "<p>Full migration notes: <a href=\"https://docs.wso2.com/mi/4.4/migration/jdk17-package-changes\" target=\"_blank\" rel=\"noopener\">MI 4.4 JDK 17 package changes</a>.</p>" +
        "<p>Let me know when you've rebuilt and we can verify in staging.</p>" +
        "<p>Best regards,<br>Sajith</p>",
      createdAt: minutesAgo(60 * 5),
    },
    {
      id: "cmt-1003-3",
      caseId: "case-1003",
      authorName: "Bill Lumbergh",
      authorRole: "customer",
      bodyHtml:
        "<p>Hi Sajith,</p>" +
        "<p>That worked. Rebuilt the extension with the dual-import, deployed to staging on 4.4.0.7 — cluster came up clean, all health checks green. Also tested on 4.4.0.6 to confirm backward compatibility, no regression.</p>" +
        "<p>Planning to roll to prod tonight during the change window. Thanks for the fast turnaround.</p>" +
        "<p>Thanks,<br>Bill</p>",
      createdAt: minutesAgo(60 * 1 + 30),
    },
  ],
  "case-1004": [
    {
      id: "cmt-1004-1",
      caseId: "case-1004",
      authorName: "Rohan Mehta",
      authorRole: "customer",
      bodyHtml:
        "<p>Hi support,</p>" +
        "<p>The userinfo endpoint stopped returning the <code>groups</code> claim for users authenticated via our corporate IdP. Worked last week, no client config change on our side.</p>" +
        "<p>Repro:</p>" +
        "<ol>" +
        "<li>Authenticate as <code>bob@acmefinancial.com</code> via the corporate IdP federation</li>" +
        "<li>Exchange code for tokens at <code>/oauth2/token</code></li>" +
        "<li>Call <code>/oauth2/userinfo</code> with the access token</li>" +
        "<li>Response includes <code>sub</code>, <code>email</code>, <code>name</code> — but no <code>groups</code></li>" +
        "</ol>" +
        "<p>Expected: <code>groups</code> should be a JSON array of LDAP DN strings.</p>" +
        "<p>Reference for our claim mapping: <a href=\"https://docs.wso2.com/identity-server/oidc-claims-mapping\" target=\"_blank\" rel=\"noopener\">OIDC claims mapping docs</a>.</p>" +
        "<p>Could a federated IdP attribute mapping have been changed, or is this a known issue in 7.1.0? Test account available on request.</p>" +
        "<p>Best regards,<br>Rohan Mehta<br>Acme Financial</p>",
      createdAt: minutesAgo(75),
    },
    {
      id: "cmt-1004-2",
      caseId: "case-1004",
      authorName: "Sajith Ekanayaka",
      authorRole: "wso2_engineer",
      bodyHtml:
        "<p>Hi Rohan,</p>" +
        "<p>Picked up. While I dig in, can you check whether the <code>groups</code> claim appears in the ID token vs. just the userinfo response?</p>" +
        "<pre><code># Decode the ID token from /oauth2/token response\necho \"&lt;id_token&gt;\" | cut -d. -f2 | base64 -d | jq .</code></pre>" +
        "<p>If <code>groups</code> shows up in the ID token but not userinfo, that's a claim-scope-mapping issue on the userinfo endpoint. If it's missing from both, it's an attribute-mapping issue on the federated authenticator.</p>" +
        "<p>Thanks,<br>Sajith</p>",
      createdAt: minutesAgo(60),
    },
  ],
  "case-1005": [
    {
      id: "cmt-1005-1",
      caseId: "case-1005",
      authorName: "Daniel Owens",
      authorRole: "customer",
      bodyHtml:
        "<p>Hi WSO2 team,</p>" +
        "<p>Our Choreo deployment for the <code>checkout-v2</code> component has been stuck at \"Provisioning\" for about 45 minutes. Component is a Go service, build succeeded, but the pod never reaches Ready.</p>" +
        "<p>Tried already:</p>" +
        "<ul>" +
        "<li>Deleting and redeploying — same outcome</li>" +
        "<li>Smaller resource ask (256Mi → 128Mi) — same outcome</li>" +
        "<li>Switching the build profile — no effect</li>" +
        "</ul>" +
        "<p>Nothing useful in the Choreo UI beyond \"Provisioning\". Can you check the underlying pod state from the platform side?</p>" +
        "<p>Thanks,<br>Daniel Owens<br>Engineering, Acme Choreo</p>",
      createdAt: minutesAgo(60 * 4),
    },
    {
      id: "cmt-1005-wn-1",
      caseId: "case-1005",
      authorName: "Sajith Ekanayaka",
      authorRole: "wso2_engineer",
      internal: true,
      bodyHtml:
        "<p><strong>Work note:</strong> Suspect the new ingress controller default timeouts. Will pair with @Priya on tomorrow's call before sharing anything externally.</p>",
      createdAt: minutesAgo(60 * 3 + 30),
    },
    {
      id: "cmt-1005-2",
      caseId: "case-1005",
      authorName: "Sajith Ekanayaka",
      authorRole: "wso2_engineer",
      bodyHtml:
        "<p>Hi Daniel,</p>" +
        "<p>Pulled the pod state. The container is alive but failing the readiness probe — TCP check on port 8080 times out:</p>" +
        "<pre><code>Events:\n  Warning  Unhealthy  43m (x12)  kubelet  Readiness probe failed: \n    dial tcp 10.4.7.23:8080: i/o timeout</code></pre>" +
        "<p>What port is the Go service binding to? In the Choreo component descriptor (the <code>.choreo/component.yaml</code>), the default expected port is 8080. If your service binds to anything else (3000, 9000, etc.) the probe will never pass.</p>" +
        "<p>Reference: <a href=\"https://wso2.com/choreo/docs/develop-components/configure-endpoints/\" target=\"_blank\" rel=\"noopener\">Choreo component endpoint config</a>.</p>" +
        "<p>Thanks,<br>Sajith</p>",
      createdAt: minutesAgo(60 * 2 + 50),
    },
    {
      id: "cmt-1005-3",
      caseId: "case-1005",
      authorName: "Daniel Owens",
      authorRole: "customer",
      bodyHtml:
        "<p>Hi Sajith,</p>" +
        "<p>Found it. The service binds to <code>:9001</code> in the Go code, but our <code>component.yaml</code> still has the scaffold default:</p>" +
        "<pre><code>endpoints:\n  - name: checkout\n    port: 8080  # &lt;-- wrong\n    type: REST</code></pre>" +
        "<p>Updating to 9001, rebuilding. Will close this out if the next deploy succeeds.</p>" +
        "<p>Thanks for the quick turnaround.</p>" +
        "<p>Thanks,<br>Daniel</p>",
      createdAt: minutesAgo(60 * 2),
    },
    {
      id: "cmt-1005-4",
      caseId: "case-1005",
      authorName: "Daniel Owens",
      authorRole: "customer",
      bodyHtml:
        "<p>Deploy succeeded with the corrected port. Component is healthy. Closing.</p>" +
        "<p>Thanks,<br>Daniel</p>",
      createdAt: minutesAgo(60),
    },
  ],

  "case-1006": [
    {
      id: "cmt-1006-1",
      caseId: "case-1006",
      authorName: "Helena Voss",
      authorRole: "customer",
      bodyHtml:
        "<p>Hi WSO2 team,</p>" +
        "<p>SAML response signature validation is failing on our Asgardeo tenant for the staging Salesforce SP. Error from the IS logs:</p>" +
        "<pre><code>org.wso2.carbon.identity.sso.saml.exception.IdentityException:\n  Signature validation failed for SAML response\n  at org.wso2.carbon.identity.sso.saml.processors.SPInitSSOAuthnRequestValidator\n    .validateSignature(SPInitSSOAuthnRequestValidator.java:331)</code></pre>" +
        "<p>Started after we rotated the Salesforce signing cert yesterday. New cert is RSA-2048, SHA-256, re-uploaded to the SP config in Asgardeo. Production tenant is on the old cert and working fine; staging only.</p>" +
        "<p>Reference: <a href=\"https://help.salesforce.com/s/articleView?id=sf.identity_saml_certificate_rotation.htm\" target=\"_blank\" rel=\"noopener\">SF cert rotation docs</a>.</p>" +
        "<p>Best regards,<br>Helena Voss<br>Globex IAM Team</p>",
      createdAt: minutesAgo(60 * 8),
    },
    {
      id: "cmt-1006-2",
      caseId: "case-1006",
      authorName: "Sajith Ekanayaka",
      authorRole: "wso2_engineer",
      bodyHtml:
        "<p>Hi Helena,</p>" +
        "<p>Two things to verify:</p>" +
        "<ol>" +
        "<li>The cert thumbprint stored on the SP record actually matches what you uploaded. From the Asgardeo console: <em>SP &gt; Inbound auth &gt; SAML &gt; Certificate</em> — copy the displayed thumbprint.</li>" +
        "<li>The signature algorithm on the request matches what's configured. Staging SF may be sending SHA-1 even after the rotation; the SP record needs to accept it.</li>" +
        "</ol>" +
        "<p>Quick check from your side — paste the relevant block from the SP record (you can redact the cert body, just keep the algorithm fields):</p>" +
        "<pre><code>&lt;ds:SignatureMethod Algorithm=\"...\"/&gt;\n&lt;ds:DigestMethod Algorithm=\"...\"/&gt;</code></pre>" +
        "<p>Thanks,<br>Sajith</p>",
      createdAt: minutesAgo(60 * 7),
    },
    {
      id: "cmt-1006-3",
      caseId: "case-1006",
      authorName: "Helena Voss",
      authorRole: "customer",
      bodyHtml:
        "<p>Hi Sajith,</p>" +
        "<p>Thumbprint on the SP record matches the new cert exactly. From the SAML config:</p>" +
        "<pre><code>&lt;ds:SignatureMethod Algorithm=\"http://www.w3.org/2001/04/xmldsig-more#rsa-sha256\"/&gt;\n&lt;ds:DigestMethod Algorithm=\"http://www.w3.org/2001/04/xmlenc#sha256\"/&gt;</code></pre>" +
        "<p>Both SHA-256. The request from SF staging also uses SHA-256 (I checked the raw POST).</p>" +
        "<p>One thing I noticed in the IS logs:</p>" +
        "<blockquote>The cert thumbprint in the failed validation log shows the OLD cert thumbprint, not the new one we uploaded.</blockquote>" +
        "<p>Cache?</p>" +
        "<p>Thanks,<br>Helena</p>",
      createdAt: minutesAgo(60 * 6),
    },
    {
      id: "cmt-1006-4",
      caseId: "case-1006",
      authorName: "Sajith Ekanayaka",
      authorRole: "wso2_engineer",
      bodyHtml:
        "<p>Hi Helena,</p>" +
        "<p>Yep — Asgardeo caches the SP cert for 1 hour by default. Your upload landed but validation is still hitting the old cached cert. Two options:</p>" +
        "<ul>" +
        "<li><strong>Wait it out</strong> — cache TTL expires within the hour, then re-test.</li>" +
        "<li><strong>Force refresh</strong> — toggle the SP's <em>Enable signature validation</em> off + save, then back on + save. This invalidates the cache entry.</li>" +
        "</ul>" +
        "<p>Production wasn't affected because cert was rotated during a window where the cache happened to be cold.</p>" +
        "<p>Background: <a href=\"https://wso2.com/asgardeo/docs/references/sp-cert-cache\" target=\"_blank\" rel=\"noopener\">Asgardeo SP cert cache reference</a>.</p>" +
        "<p>Best regards,<br>Sajith</p>",
      createdAt: minutesAgo(60 * 5),
    },
    {
      id: "cmt-1006-5",
      caseId: "case-1006",
      authorName: "Helena Voss",
      authorRole: "customer",
      bodyHtml:
        "<p>Hi Sajith,</p>" +
        "<p>Force-refreshed via the toggle. Validation works immediately. Closing this out.</p>" +
        "<p>Thanks,<br>Helena</p>",
      createdAt: minutesAgo(60 * 4 + 30),
    },
  ],
  "case-1011": [
    {
      id: "cmt-1011-1",
      caseId: "case-1011",
      authorName: "Marcus Liang",
      authorRole: "customer",
      bodyHtml:
        "<p>Hi WSO2 team,</p>" +
        "<p>Need help tuning the LDAP userstore on our IS deployment. Search queries against the userstore are taking <strong>1.2–1.8s</strong> for queries that should be instant. We have ~85,000 user entries.</p>" +
        "<p>Current config (from <code>user-mgt.xml</code>):</p>" +
        "<pre><code>&lt;Property name=\"ConnectionPoolingEnabled\"&gt;true&lt;/Property&gt;\n&lt;Property name=\"MaxActiveConnections\"&gt;50&lt;/Property&gt;\n&lt;Property name=\"ConnectionPoolMinIdle\"&gt;10&lt;/Property&gt;\n&lt;Property name=\"SearchScope\"&gt;SUB&lt;/Property&gt;\n&lt;Property name=\"UserSearchBase\"&gt;ou=people,dc=globex,dc=com&lt;/Property&gt;</code></pre>" +
        "<p>LDAP-side indexes look fine — pres/sub/eq on <code>uid, mail, cn, member</code>. Network RTT to LDAP is stable at ~2ms.</p>" +
        "<p>Suspect we're paginating poorly or doing redundant searches. Reference doc we've been working from: <a href=\"https://docs.wso2.com/identity-server/userstore-tuning\" target=\"_blank\" rel=\"noopener\">userstore tuning guide</a>.</p>" +
        "<p>Thanks,<br>Marcus Liang<br>Globex Security Engineering</p>",
      createdAt: minutesAgo(60 * 10),
    },
    {
      id: "cmt-1011-2",
      caseId: "case-1011",
      authorName: "Sajith Ekanayaka",
      authorRole: "wso2_engineer",
      bodyHtml:
        "<p>Hi Marcus,</p>" +
        "<p>The pool config looks fine for 85k entries. Two likely culprits:</p>" +
        "<ol>" +
        "<li>Group lookup amplification — IS hits the LDAP for the user, then for each group the user is in, then for each user in those groups (when role-based authorisation is enabled). Easy to verify; enable LDAP-side query logging for 5 minutes during a normal load period and count queries per login.</li>" +
        "<li><code>SearchScope=SUB</code> from a deep base DN walks the whole subtree. If your tree has many OUs under <code>ou=people</code>, this is slow even with indexes.</li>" +
        "</ol>" +
        "<p>Can you share:</p>" +
        "<ul>" +
        "<li>A 1-min LDAP query log slice during peak (sanitised)</li>" +
        "<li>The OU layout under <code>ou=people,dc=globex,dc=com</code></li>" +
        "</ul>" +
        "<p>Best regards,<br>Sajith</p>",
      createdAt: minutesAgo(60 * 9 + 30),
    },
    {
      id: "cmt-1011-3",
      caseId: "case-1011",
      authorName: "Marcus Liang",
      authorRole: "customer",
      bodyHtml:
        "<p>Hi Sajith,</p>" +
        "<p>You called it. Query log shows 47 LDAP queries <em>per login</em> on average. The amplification pattern is clear:</p>" +
        "<blockquote>1 bind + 1 user search + 3 group memberOf lookups + 12 nested group enumerations + 30 attribute fetches per nested member.</blockquote>" +
        "<p>OU layout is flat — everyone under <code>ou=people</code> directly, no sub-OUs. So SUB scope shouldn't be the issue, but the nested group walk definitely is.</p>" +
        "<p>How do we cut the amplification?</p>" +
        "<p>Thanks,<br>Marcus</p>",
      createdAt: minutesAgo(60 * 8 + 30),
    },
    {
      id: "cmt-1011-4",
      caseId: "case-1011",
      authorName: "Sajith Ekanayaka",
      authorRole: "wso2_engineer",
      bodyHtml:
        "<p>Hi Marcus,</p>" +
        "<p>Three tweaks that compound to cut your per-login query count from ~47 to ~5:</p>" +
        "<ol>" +
        "<li><strong>Enable group caching</strong> on the userstore:" +
        "<pre><code>&lt;Property name=\"GroupListCacheEnabled\"&gt;true&lt;/Property&gt;\n&lt;Property name=\"GroupListCacheTimeout\"&gt;300&lt;/Property&gt;</code></pre>" +
        "5-minute TTL. Drops repeated group enumeration calls.</li>" +
        "<li><strong>Cap nested group depth</strong>. Default is unlimited; for most enterprise directories depth 3 is the natural ceiling:" +
        "<pre><code>&lt;Property name=\"MaxRoleNamesLimit\"&gt;100&lt;/Property&gt;\n&lt;Property name=\"NestedGroupSearchDepth\"&gt;3&lt;/Property&gt;</code></pre></li>" +
        "<li><strong>Switch to memberOf-based group resolution</strong>. If your LDAP supports the <code>memberOf</code> reverse-index (RFC 4511 attribute), single-query group lookup replaces the recursive walk:" +
        "<pre><code>&lt;Property name=\"ReadGroups\"&gt;true&lt;/Property&gt;\n&lt;Property name=\"GroupMembershipAttribute\"&gt;memberOf&lt;/Property&gt;\n&lt;Property name=\"BackLinksEnabled\"&gt;true&lt;/Property&gt;</code></pre></li>" +
        "</ol>" +
        "<p>Apply 1 + 2 first (safe, no LDAP-side requirements). 3 needs your AD/LDAP admin to confirm memberOf is populated — most Active Directory deployments have it on by default. OpenLDAP needs the <a href=\"https://www.openldap.org/doc/admin26/overlays.html#Reverse Group Membership Maintenance\" target=\"_blank\" rel=\"noopener\">memberof overlay</a>.</p>" +
        "<p>Restart IS after applying. Watch <code>org.wso2.carbon.user.core.ldap</code> at DEBUG for 10 min to confirm the query count drops.</p>" +
        "<p>Best regards,<br>Sajith</p>",
      createdAt: minutesAgo(60 * 7),
    },
    {
      id: "cmt-1011-5",
      caseId: "case-1011",
      authorName: "Marcus Liang",
      authorRole: "customer",
      bodyHtml:
        "<p>Hi Sajith,</p>" +
        "<p>Applied 1 + 2 in staging. Per-login query count dropped from 47 to 12. Login latency for the test users came down from 1.6s to ~280ms.</p>" +
        "<p>Will check with our LDAP admin on memberOf overlay for #3 (we run OpenLDAP), but 280ms is already inside our SLO. Rolling 1 + 2 to prod tomorrow.</p>" +
        "<p>Thanks for the deep dive.</p>" +
        "<p>Thanks,<br>Marcus</p>",
      createdAt: minutesAgo(60 * 5),
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
