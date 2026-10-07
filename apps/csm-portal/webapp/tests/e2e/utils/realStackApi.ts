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

//
// What a spec needs to RAISE and WALK a change request on the real local stack, without a browser:
// WSO2 staff's own calls to the CSM portal's backend, with a token minted at the local mock
// identity provider for the persona (no credential is checked there). It is the real
// backend, entity-service and database all the way: nothing here is faked.
//
// Needs `E2E_CSM_BFF_URL` (the CSM portal's backend as the host reaches it; the stock stack's is
// http://localhost:8082, the isolated stack's http://localhost:18082; NO default, because these
// calls write) and `E2E_OIDC_URL` (the mock provider; stock http://localhost:9100, isolated
// http://localhost:19100), like `customerPortalDecision.ts`, which this sits beside (that one is
// the CUSTOMER's answer).
//
// The staff personas are the local seed's (entity-service/CLAUDE.md, "Local seed personas"):
// jane raises (a requester in no approval group, so she is nobody's approver), alice, bob and
// carol sit in the Peer, CAB and ECAB groups.
//

const OIDC_URL = process.env.E2E_OIDC_URL ?? "http://localhost:9100";
const CSM_ORIGIN = process.env.E2E_BASE_URL ?? "http://localhost:3001";

/**
 * The CSM portal's backend the specs raise and walk change requests through. There is NO default: these calls WRITE, and a
 * guessed address could be somebody else's running stack (the stock :8082 is the very one a developer is usually using),
 * so a spec that needs it names it -- E2E_CSM_BFF_URL, the isolated stack's is http://localhost:18082 -- and SKIPS
 * (see {@link realStackNamed}) when it is not named.
 */
export function bffUrl(): string | undefined {
  return process.env.E2E_CSM_BFF_URL?.trim().replace(/\/+$/, "") || undefined;
}

/**
 * True when the specs that raise change requests through the CSM backend have been told WHICH stack: the backend, the
 * identity provider that signs the staff in and entity-service (where a customer's answer is applied) are all named. Naming
 * only one would leave the others on the stock defaults of `customerPortalDecision.ts` (:9100, :8081), i.e. possibly another
 * stack than the one written to.
 */
export function realStackNamed(): boolean {
  return !!bffUrl() && !!process.env.E2E_OIDC_URL?.trim() && !!process.env.E2E_ENTITY_SERVICE_URL?.trim();
}

const FORM = { "content-type": "application/x-www-form-urlencoded" };

export const STAFF = {
  jane: "jane.doe@example.com",
  alice: "alice.perera@example.com",
  bob: "bob.fernando@example.com",
  carol: "carol.silva@example.com",
} as const;
export type StaffPersona = keyof typeof STAFF;

/** "Example Corp ABT" (group 901): the assigned team of every seeded fixture; its members are the Peer approvers. */
export const EXAMPLE_CORP_ABT_GROUP_ID = "00000000-0000-0000-0000-000000000901";
/** "Example Corp Production": its registered contacts are dave.mendis and erin.jayawardena. */
export const EXAMPLE_CORP = { id: "00000000-0000-0000-0000-000000000401", name: "Example Corp Production" } as const;
/** "Other Corp Production": its one registered contact is sam.other. */
export const OTHER_CORP = { id: "00000000-0000-0000-0000-000000000402", name: "Other Corp Production" } as const;

export interface ApiResult<T = unknown> {
  status: number;
  body: T;
}

/** The part of the staff view of a change request the specs read. */
export interface StaffChangeRequest {
  id: string;
  number: string;
  state: string;
  title?: string;
  subject?: string;
  customerApprovalRequired?: boolean;
  customerReviewRequired?: boolean;
  project?: { id: string; name: string } | null;
  /** The project's registered contacts the customer stages ask (the Customer Group): empty when nobody can be asked. */
  customerContacts?: Array<{ id: string; name: string; email?: string }>;
  message?: string;
}

const tokens = new Map<string, { token: string; expiresAt: number }>();

/** A staff access token from the mock provider (the CSM portal's client, the engineers' group), reused for ten minutes. */
async function staffToken(email: string): Promise<string> {
  const cached = tokens.get(email);
  if (cached && cached.expiresAt > Date.now()) return cached.token;
  const authorize = await fetch(`${OIDC_URL}/oauth2/authorize`, {
    method: "POST",
    redirect: "manual",
    headers: FORM,
    body: new URLSearchParams({
      client_id: "csm-portal-webapp",
      redirect_uri: CSM_ORIGIN,
      state: "e2e",
      scope: "openid",
      email,
      groups: "cs_engineer",
    }),
  });
  const location = authorize.headers.get("location");
  const code = location ? new URL(location).searchParams.get("code") : null;
  if (!code) throw new Error(`mock-oidc gave no authorization code for ${email} (HTTP ${authorize.status})`);
  const response = await fetch(`${OIDC_URL}/oauth2/token`, {
    method: "POST",
    headers: FORM,
    body: new URLSearchParams({ grant_type: "authorization_code", code, client_id: "csm-portal-webapp", redirect_uri: CSM_ORIGIN }),
  });
  const body = (await response.json()) as { access_token?: string };
  if (!response.ok || !body.access_token) throw new Error(`mock-oidc gave no token for ${email} (HTTP ${response.status})`);
  tokens.set(email, { token: body.access_token, expiresAt: Date.now() + 10 * 60_000 });
  return body.access_token;
}

async function call<T>(method: string, path: string, email: string, body?: unknown): Promise<ApiResult<T>> {
  const base = bffUrl();
  if (!base) throw new Error("E2E_CSM_BFF_URL is not set: name the CSM portal backend of the stack under test");
  const response = await fetch(`${base}${path}`, {
    method,
    headers: {
      authorization: `Bearer ${await staffToken(email)}`,
      ...(body === undefined ? {} : { "content-type": "application/json" }),
    },
    body: body === undefined ? undefined : JSON.stringify(body),
    signal: AbortSignal.timeout(30_000),
  });
  const text = await response.text();
  let parsed: unknown = text;
  try {
    parsed = text ? JSON.parse(text) : null;
  } catch {
    /* not JSON: keep the text */
  }
  return { status: response.status, body: parsed as T };
}

/** A staff member's own calls to the CSM portal's backend. */
export function staff(who: StaffPersona) {
  const email = STAFF[who];
  return {
    get: (id: string) => call<StaffChangeRequest>("GET", `/change-requests/${id}`, email),
    patch: (id: string, body: unknown) => call<StaffChangeRequest>("PATCH", `/change-requests/${id}`, email, body),
    decide: (id: string, decision: "approved" | "rejected") =>
      call("POST", `/change-requests/${id}/approvals/decision`, email, { decision }),
    create: (body: unknown) => call<{ changeRequest?: { id: string; number: string } }>("POST", "/change-requests", email, body),
  };
}

/** A change request a spec raised. */
export interface Raised {
  id: string;
  number: string;
}

/**
 * Raises a change request as `jane`, assigned to Example Corp ABT (so Request Approval is not blocked for want of a team).
 *
 * @param options.subject - The whole subject (the spec picks a prefix its clean-up deletes by).
 * @param options.projectId - The Customer Project, or null for none.
 */
export async function raise(options: {
  subject: string;
  projectId: string | null;
  approval: boolean;
  review: boolean;
  type?: "normal" | "standard" | "emergency";
}): Promise<Raised> {
  const created = await staff("jane").create({
    subject: options.subject,
    type: options.type ?? "normal",
    groupId: EXAMPLE_CORP_ABT_GROUP_ID,
    ...(options.projectId ? { projectId: options.projectId } : {}),
    customerApprovalRequired: options.approval,
    customerReviewRequired: options.review,
  });
  const change = created.body.changeRequest;
  if (created.status !== 201 || !change) {
    throw new Error(`raising "${options.subject}" answered ${created.status}: ${JSON.stringify(created.body)}`);
  }
  return { id: change.id, number: change.number };
}

/** A call that must succeed (200). */
export async function ok(label: string, result: ApiResult): Promise<void> {
  if (result.status !== 200) throw new Error(`${label} answered ${result.status}: ${JSON.stringify(result.body)}`);
}

/** The states a change request can be walked to, by the staff the seed names. */
export type WalkTarget = "assess" | "authorize" | "scheduled" | "implement" | "review" | "closed" | "canceled";

/**
 * Walks a NORMAL change request to `target` through the real flow, one LEGAL edge of the state machine at a time (the
 * backend refuses a jump over a state or an approval gate, and a move out of a final state): Request Approval (jane), the
 * Peer approval and the CAB approval (alice), then the engineer's own moves (`implement`, `review`, `closed`).
 *
 * A ticked customer box puts the customer in the way, so the walk would not land where it says: with the approval box ticked
 * the CAB's approval goes to Customer Approval, not Scheduled, and with the review box ticked Review has no Close. A spec
 * that wants a customer gate walks it by hand (and the customer answers in the Customer Portal), so this refuses up front
 * rather than failing three calls later on a refusal that says "cannot be set manually". The boxes may still be ticked for
 * the targets that stop short of the gate they control (`assess`, `authorize`, `canceled`; `review` with the review box).
 */
export async function walkTo(change: Raised, target: WalkTarget): Promise<void> {
  const stored = (await staff("jane").get(change.id)).body;
  const approvalInTheWay = !!stored.customerApprovalRequired && !["assess", "authorize", "canceled"].includes(target);
  const reviewInTheWay = !!stored.customerReviewRequired && target === "closed";
  if (approvalInTheWay || reviewInTheWay) {
    throw new Error(
      `walkTo(${change.number}, "${target}"): the ${approvalInTheWay ? "customer approval" : "customer review"} box is ticked, so the ` +
        `walk would stop at ${approvalInTheWay ? "Customer Approval" : "Customer Review"} (the customer's own answer is the only way on): walk it by hand`,
    );
  }
  const steps: Array<() => Promise<void>> = [
    async () => ok("Request Approval", await staff("jane").patch(change.id, { state: "assess" })), // -> assess
    async () => ok("Peer approval", await staff("alice").decide(change.id, "approved")), // -> authorize
    async () => ok("CAB approval", await staff("alice").decide(change.id, "approved")), // -> scheduled
    async () => ok("implement", await staff("alice").patch(change.id, { state: "implement" })),
    async () => ok("review", await staff("alice").patch(change.id, { state: "review" })),
    async () => ok("close", await staff("alice").patch(change.id, { state: "closed" })),
  ];
  const reach: Record<Exclude<WalkTarget, "canceled">, number> = { assess: 1, authorize: 2, scheduled: 3, implement: 4, review: 5, closed: 6 };
  if (target === "canceled") {
    await steps[0]!();
    await ok("cancel", await staff("alice").patch(change.id, { state: "canceled" }));
    return;
  }
  for (let i = 0; i < reach[target]; i++) await steps[i]!();
}

/** The stored state of a change request as the staff API words it (`customer_approval`). */
export async function stateOf(id: string): Promise<string> {
  return (await staff("jane").get(id)).body.state;
}
