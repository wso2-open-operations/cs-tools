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
// What the STATE-CHANGING local specs need beyond a customer's browser session:
// a way to put the seeded change requests back, a way to read what the stack
// really recorded, and the two other parties of a change request's life — the
// customer's own API calls and WSO2 staff deciding in the CSM portal.
//
// LOCAL STACK ONLY (docker-compose + the mock identity provider). Nothing here
// talks to a deployed environment.
//
// Where the pieces come from
//   - The customer backend and the identity provider are READ FROM THE APP UNDER
//     TEST: the webapp serves them in /config.js, so a spec can never aim its API
//     calls at another stack than the browser it drives.
//   - The database and the CSM BFF cannot be derived, so they are named in the
//     environment, and a spec that needs one SKIPS (never fails, never guesses a
//     default) when it is unset. That is deliberate: re-seeding rewrites rows, so
//     a default container name could silently reset somebody else's stack.
//       E2E_POSTGRES_CONTAINER   Docker container of the stack's Postgres
//                                (isolated stack: csmenv-postgres-1)
//       E2E_CSM_BFF_URL          the CSM portal backend as the browser reaches it
//                                (isolated stack: http://localhost:18082)
//       E2E_POSTGRES_USER / E2E_POSTGRES_DB   default postgres / csm_platform
//   - After every reset the helpers read the fixtures back THROUGH THE APP's
//     backend and fail loudly if they are not in their starting state, which is
//     what catches an E2E_POSTGRES_CONTAINER that belongs to a different stack.
//

import { spawn } from "node:child_process";
import { createHash } from "node:crypto";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import type { Page, test } from "@playwright/test";
import { EXAMPLE_CORP_PROJECT_ID, LOCAL_PERSONAS, type LocalPersona } from "../auth/localSessions";

/** The seeded change requests the specs drive (scripts/csm-compose/seed-entity-service.sql). */
export const FIXTURES = {
  projectId: EXAMPLE_CORP_PROJECT_ID,
  /** Normal change in Customer Approval; dave and erin are asked. */
  approval: { id: "00000000-0000-0000-0000-000000001303", number: "CHG-FIXED-007" },
  /** Change in Customer Review; dave and erin are asked. */
  review: { id: "00000000-0000-0000-0000-000000001304", number: "CHG-FIXED-008" },
  /** Normal change in Review with Customer Review ticked (not asked of anyone yet). */
  inReview: { id: "00000000-0000-0000-0000-000000001202", number: "CHG-FIXED-006" },
  /** Standard change in New with Customer Approval ticked. */
  standardNew: { id: "00000000-0000-0000-0000-000000001201", number: "CHG-FIXED-005" },
} as const;

export type FixtureChange = { id: string; number: string };

/** The staff persona that decides internal stages (a CAB / peer approver). */
export const STAFF_APPROVERS = {
  alice: "alice.perera@example.com",
  bob: "bob.fernando@example.com",
  carol: "carol.silva@example.com",
} as const;

const SEED_FILE = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "../../../../../../scripts/csm-compose/seed-entity-service.sql",
);

/** The origin the run targets (same default as playwright.config.ts). */
function appOrigin(): string {
  return new URL(process.env.E2E_BASE_URL ?? "http://localhost:3000").origin;
}

// --- The app under test: where its backend and identity provider are -------------

export type StackEndpoints = {
  /** Customer backend (the BFF the webapp calls). */
  customerApi: string;
  /** The mock OIDC provider. */
  oidc: string;
  /** The webapp's own client id. */
  customerClientId: string;
};

let endpoints: Promise<StackEndpoints> | undefined;

/**
 * Reads the backend and identity-provider origins from the webapp's own
 * `/config.js`, so the API calls of a spec go to the very stack its browser does.
 */
export function stackEndpoints(): Promise<StackEndpoints> {
  endpoints ??= (async () => {
    const response = await fetch(`${appOrigin()}/config.js`, {
      signal: AbortSignal.timeout(10_000),
    });
    const text = await response.text();
    const read = (key: string): string => {
      const match = new RegExp(`${key}\\s*:\\s*"([^"]+)"`).exec(text);
      if (!match) throw new Error(`${appOrigin()}/config.js has no ${key}`);
      return match[1].replace(/\/+$/, "");
    };
    return {
      customerApi: read("CUSTOMER_PORTAL_BACKEND_BASE_URL"),
      oidc: read("CUSTOMER_PORTAL_AUTH_BASE_URL"),
      customerClientId: read("CUSTOMER_PORTAL_AUTH_CLIENT_ID"),
    };
  })();
  return endpoints;
}

// --- Tokens (the mock identity provider signs in any email, no credential) ---------

const FORM = { "content-type": "application/x-www-form-urlencoded" };
const tokenCache = new Map<string, { token: string; expiresAt: number }>();

/**
 * An access token from the mock provider for `email` as `clientId`, by the same
 * authorization-code exchange the apps perform (no PKCE).
 */
async function mintAccessToken(
  email: string,
  clientId: string,
  groups: string,
): Promise<string> {
  const key = `${clientId}|${email}|${groups}`;
  const cached = tokenCache.get(key);
  if (cached && cached.expiresAt > Date.now()) return cached.token;

  const { oidc } = await stackEndpoints();
  const redirectUri = appOrigin();
  const authorize = await fetch(`${oidc}/oauth2/authorize`, {
    method: "POST",
    redirect: "manual",
    headers: FORM,
    body: new URLSearchParams({
      client_id: clientId,
      redirect_uri: redirectUri,
      state: "e2e",
      scope: "openid",
      email,
      groups,
    }),
  });
  const location = authorize.headers.get("location");
  const code = location ? new URL(location).searchParams.get("code") : null;
  if (!code) {
    throw new Error(`the identity provider at ${oidc} gave no code for ${email} (HTTP ${authorize.status})`);
  }
  const token = await fetch(`${oidc}/oauth2/token`, {
    method: "POST",
    headers: FORM,
    body: new URLSearchParams({
      grant_type: "authorization_code",
      code,
      client_id: clientId,
      redirect_uri: redirectUri,
    }),
  });
  const body = (await token.json()) as { access_token?: string };
  if (!token.ok || !body.access_token) {
    throw new Error(`the identity provider at ${oidc} gave no token for ${email} (HTTP ${token.status})`);
  }
  // Tokens live an hour; reuse one for at most ten minutes.
  tokenCache.set(key, { token: body.access_token, expiresAt: Date.now() + 10 * 60_000 });
  return body.access_token;
}

export type ApiResult<T = unknown> = { status: number; body: T };

async function call<T>(
  method: string,
  url: string,
  token: string,
  body?: unknown,
): Promise<ApiResult<T>> {
  const response = await fetch(url, {
    method,
    headers: {
      authorization: `Bearer ${token}`,
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

// --- A customer's own API calls ----------------------------------------------------

/** What the customer API says about a change request (the part the specs read). */
export type CustomerChangeRequest = {
  id: string;
  number: string;
  state?: { id?: string; label?: string } | null;
  customerCanAnswer?: boolean;
  /** Whether WSO2 holds the change (the reason is never sent to a customer). */
  isOnHold?: boolean;
  hasCustomerApproved?: boolean;
  startDate?: string | null;
  endDate?: string | null;
  title?: string;
  /** The time a customer proposed and where WSO2's answer stands (no names or e-mails ever reach a customer). */
  customerProposal?: {
    startDate: string;
    endDate?: string | null;
    answer: "pending" | "agreed" | "disagreed" | "unanswered";
    /** Whether the proposer is the person reading (only while pending, and only when it is knowable). */
    proposedByViewer?: boolean;
  } | null;
};

/**
 * Calls the customer backend as a seeded customer, exactly as the webapp does
 * (their bearer token; no groups, so a customer and nothing else).
 */
export function customerApi(persona: LocalPersona) {
  return customerApiFor(LOCAL_PERSONAS[persona].email);
}

/** What the customer API's change request search / stats answer (the parts the specs read). */
export type CustomerListItem = { id: string; number: string; state?: { id?: string; label?: string } | null };
export type CustomerStats = {
  totalCount: number;
  activeCount: number;
  outstandingCount: number;
  actionRequiredCount: number;
  stateCount: { id: string; label: string; count: number }[];
  resolvedCount?: { total: number; currentMonth: number; pastThirtyDays: number };
};

/**
 * The same calls for ANY email the identity provider signs in (a seeded persona, or a contact a spec registered
 * itself): the mock provider needs no credential, so the email alone is the caller.
 */
export function customerApiFor(email: string) {
  const token = async () => {
    const { customerClientId } = await stackEndpoints();
    return mintAccessToken(email, customerClientId, "");
  };
  return {
    /** `GET /change-requests/{id}`. */
    async get(changeRequestId: string): Promise<ApiResult<CustomerChangeRequest>> {
      const { customerApi: base } = await stackEndpoints();
      return call("GET", `${base}/change-requests/${changeRequestId}`, await token());
    },
    /** `PATCH /change-requests/{id}`: whatever body the caller wants to try. */
    async patch(changeRequestId: string, body: unknown): Promise<ApiResult> {
      const { customerApi: base } = await stackEndpoints();
      return call("PATCH", `${base}/change-requests/${changeRequestId}`, await token(), body);
    },
    /** `GET /change-requests/{id}/approvals`: the raw body (a spec reads it for names it must NOT hold). */
    async approvals(changeRequestId: string): Promise<ApiResult<{ approvals?: { stage?: string; status?: string }[] }>> {
      const { customerApi: base } = await stackEndpoints();
      return call("GET", `${base}/change-requests/${changeRequestId}/approvals`, await token());
    },
    /** `POST /change-requests/{id}/approvals/decision`: whatever body the caller wants to try. */
    async decision(changeRequestId: string, body: unknown): Promise<ApiResult> {
      const { customerApi: base } = await stackEndpoints();
      return call("POST", `${base}/change-requests/${changeRequestId}/approvals/decision`, await token(), body);
    },
    /**
     * `POST /cases/{id}/comments` with a change request's id: the customer backend forwards that route for ANY id, so it is
     * a way a customer could comment on (or probe) a change request by its id. (A body that would be a real comment.)
     */
    async commentViaCaseRoute(workItemId: string, content = "E2E probe comment"): Promise<ApiResult> {
      const { customerApi: base } = await stackEndpoints();
      return call("POST", `${base}/cases/${workItemId}/comments`, await token(), { content });
    },
    /**
     * What the customer backend's other id-taking routes (the case's attachments, escalations, call requests and activities)
     * answer for a work item id, one line per route with the id blanked out: a hidden change request must read exactly as an id
     * that exists nowhere, or the answer is an oracle for whether it exists.
     */
    async caseRouteAnswers(workItemId: string): Promise<string[]> {
      const { customerApi: base } = await stackEndpoints();
      const t = await token();
      const page = { pagination: { offset: 0, limit: 10 } };
      const out: string[] = [];
      for (const [method, route, body] of [
        ["GET", `/cases/${workItemId}/attachments`, undefined],
        ["POST", `/cases/${workItemId}/escalations/search`, page],
        ["POST", `/cases/${workItemId}/call-requests/search`, page],
        ["POST", `/cases/${workItemId}/activities/search`, page],
      ] as const) {
        const r = await call(method, `${base}${route}`, t, body);
        out.push(`${method} ${route.replace(workItemId, "{id}")} -> ${r.status} ${JSON.stringify(r.body)}`);
      }
      return out;
    },
    /** The customer portal's global search (`POST /search`): whether its answer mentions `text` anywhere. */
    async globalSearchMentions(text: string): Promise<boolean> {
      const { customerApi: base } = await stackEndpoints();
      const r = await call("POST", `${base}/search`, await token(), { query: text, pagination: { offset: 0, limit: 10 } });
      return r.status === 200 && JSON.stringify(r.body).includes(text);
    },
    /** `POST /projects/{id}/change-requests/search`: the numbers the project lists. */
    async listedNumbers(projectId: string, filters: Record<string, unknown> = {}): Promise<string[]> {
      return (await this.listed(projectId, filters)).map((c) => c.number);
    },
    /** The same search, each item with its state (`{id, label}`). */
    async listed(projectId: string, filters: Record<string, unknown> = {}): Promise<CustomerListItem[]> {
      const { customerApi: base } = await stackEndpoints();
      const result = await call<{ changeRequests?: CustomerListItem[] }>(
        "POST",
        `${base}/projects/${projectId}/change-requests/search`,
        await token(),
        // 50 is the most the API takes per page (a larger limit answers 400, "limit cannot exceed 50").
        { filters, pagination: { offset: 0, limit: 50 } },
      );
      // A project the caller may not read answers 403/404 and lists nothing; any other refusal (a 400 for the
      // page size, say) must not read as "an empty list", or the assertions on what is NOT listed pass vacuously.
      if (result.status === 403 || result.status === 404) return [];
      if (result.status !== 200) {
        throw new Error(`listing ${projectId}'s change requests answered ${result.status}: ${JSON.stringify(result.body)}`);
      }
      return (result.body.changeRequests ?? []).filter((c) => c.number);
    },
    /** `GET /projects/{id}/stats/change-requests`: the stat cards' counts (undefined when the project is refused). */
    async stats(projectId: string): Promise<CustomerStats | undefined> {
      const { customerApi: base } = await stackEndpoints();
      const result = await call<CustomerStats>("GET", `${base}/projects/${projectId}/stats/change-requests`, await token());
      if (result.status === 403 || result.status === 404) return undefined;
      if (result.status !== 200) throw new Error(`stats of ${projectId} answered ${result.status}: ${JSON.stringify(result.body)}`);
      return result.body;
    },
    /** `GET /projects/{id}/stats`: the dashboard's "Outstanding" change request count. */
    async outstandingChangeRequests(projectId: string): Promise<number | undefined> {
      const { customerApi: base } = await stackEndpoints();
      const result = await call<{ projectStats?: { outstandingChangeRequestCount?: number } }>(
        "GET", `${base}/projects/${projectId}/stats`, await token());
      if (result.status === 403 || result.status === 404) return undefined;
      if (result.status !== 200) throw new Error(`project stats of ${projectId} answered ${result.status}`);
      return result.body.projectStats?.outstandingChangeRequestCount;
    },
    /** The id of the project this customer is a contact of, found by name (generated projects have random ids). */
    async projectIdByName(name: string): Promise<string | undefined> {
      const { customerApi: base } = await stackEndpoints();
      const result = await call<{ projects?: { id: string; name: string }[] }>(
        "POST",
        `${base}/projects/search`,
        await token(),
        {},
      );
      return result.body.projects?.find((p) => p.name === name)?.id;
    },
  };
}

// --- WSO2 staff, deciding internal approvals through the CSM portal's backend -------

/** The CSM portal backend, as named by E2E_CSM_BFF_URL; undefined when unset. */
export function csmBffUrl(): string | undefined {
  return process.env.E2E_CSM_BFF_URL?.trim().replace(/\/+$/, "") || undefined;
}

/**
 * A staff member decides their pending approval on a change request — the same
 * call the CSM portal's Approvals tab makes (`POST /change-requests/{id}/approvals/decision`).
 *
 * @param email - A seeded approver (see {@link STAFF_APPROVERS}).
 * @param changeRequestId - The change request.
 * @param decision - approved / rejected.
 */
export async function decideAsStaff(
  email: string,
  changeRequestId: string,
  decision: "approved" | "rejected",
): Promise<ApiResult> {
  const bff = csmBffUrl();
  if (!bff) throw new Error("E2E_CSM_BFF_URL is not set");
  // The seeded engineers' group: the CSM portal's own sign-in pre-fills it.
  const token = await mintAccessToken(email, "csm-portal-webapp", "cs_engineer");
  return call("POST", `${bff}/change-requests/${changeRequestId}/approvals/decision`, token, { decision });
}

/**
 * A staff member changes a change request through the CSM portal's backend
 * (`PATCH /change-requests/{id}`), e.g. `{ state: "assess" }`, the one human action
 * out of New that the CSM portal calls "Request Approval".
 *
 * @param email - A seeded staff persona (see {@link STAFF_APPROVERS}).
 * @param changeRequestId - The change request.
 * @param body - The PATCH body.
 */
export async function patchAsStaff(
  email: string,
  changeRequestId: string,
  body: unknown,
): Promise<ApiResult> {
  const bff = csmBffUrl();
  if (!bff) throw new Error("E2E_CSM_BFF_URL is not set");
  const token = await mintAccessToken(email, "csm-portal-webapp", "cs_engineer");
  return call("PATCH", `${bff}/change-requests/${changeRequestId}`, token, body);
}

// --- WSO2 staff driving a WHOLE change request, through the CSM portal's backend --------------

/** The staff who raise and approve in the specs: jane is the requester (in no approval group), the rest approve. */
export const STAFF = { jane: "jane.doe@example.com", ...STAFF_APPROVERS } as const;
export type StaffPersona = keyof typeof STAFF;

/** "Example Corp ABT" (group 901): the assigned team of every seeded fixture, whose members (alice, bob, carol) are the Peer approvers. */
export const EXAMPLE_CORP_ABT_GROUP_ID = "00000000-0000-0000-0000-000000000901";

/**
 * Every change request a spec RAISES (as opposed to the seeded fixtures it moves) carries this prefix in its
 * subject, so {@link deleteRaisedChanges} can take them away again and the lists the specs count stay the same
 * from one run to the next.
 */
export const RAISED_PREFIX = "E2E raised: ";

/** A change request a spec raised: its id and number. */
export type RaisedChange = { id: string; number: string; title: string };

/**
 * A staff member's own calls to the CSM portal's backend (the one the CSM webapp talks to): the `cs_engineer`
 * group, which is what the portal's sign-in pre-fills.
 */
export function staffApi(who: StaffPersona) {
  const email = STAFF[who];
  const base = () => {
    const bff = csmBffUrl();
    if (!bff) throw new Error("E2E_CSM_BFF_URL is not set");
    return bff;
  };
  const token = () => mintAccessToken(email, "csm-portal-webapp", "cs_engineer");
  return {
    email,
    /** `GET /change-requests/{id}`: the staff view (state is lower case: `customer_approval`). */
    async get(id: string): Promise<ApiResult<StaffChangeRequest>> {
      return call("GET", `${base()}/change-requests/${id}`, await token());
    },
    /** `PATCH /change-requests/{id}`. */
    async patch(id: string, body: unknown): Promise<ApiResult<{ message?: string }>> {
      return call("PATCH", `${base()}/change-requests/${id}`, await token(), body);
    },
    /** `POST /change-requests/{id}/approvals/decision`. */
    async decide(id: string, decision: "approved" | "rejected"): Promise<ApiResult> {
      return call("POST", `${base()}/change-requests/${id}/approvals/decision`, await token(), { decision });
    },
    /** `GET /change-requests/{id}/approvals`: every stage with every approver (what staff see). */
    async approvals(id: string): Promise<ApiResult<StaffApprovals>> {
      return call("GET", `${base()}/change-requests/${id}/approvals`, await token());
    },
    /** `POST /change-requests`. */
    async create(body: unknown): Promise<ApiResult<{ changeRequest?: { id: string; number: string } }>> {
      return call("POST", `${base()}/change-requests`, await token(), body);
    },
  };
}

/**
 * What WSO2 staff are told about a time a customer proposed (entity-service's `customerProposal`): the proposed start
 * (RFC 3339), the end it implies (while pending), where WSO2's answer stands, and who proposed it when that is knowable.
 */
export type StaffProposal = {
  startOn: string;
  endOn?: string;
  answer: "pending" | "agreed" | "disagreed" | "unanswered";
  proposerRecorded?: boolean;
  proposedByName?: string;
  proposedByEmail?: string;
  proposedOn?: string;
  canAccept?: boolean;
  acceptBlockedReason?: string;
};

/** The part of the staff view of a change request the specs read. */
export type StaffChangeRequest = {
  id: string;
  number: string;
  state: string;
  customerApprovalRequired?: boolean;
  customerReviewRequired?: boolean;
  project?: { id: string; name: string } | null;
  customerContacts?: { name?: string; email?: string }[];
  /** The planned window, as the staff view prints it (what a staff answer names as the window it saw). */
  plannedStartOn?: string | null;
  plannedEndOn?: string | null;
  /** The time a customer proposed and where WSO2's answer stands (absent when nothing was ever proposed). */
  customerProposal?: StaffProposal | null;
  legalNextStates?: string[];
  message?: string;
};

/** The staff view of the approvals: every stage with its approvers (name, status, canDecide). */
export type StaffApprovals = {
  approvals?: { stage: string; status?: string; approvers?: { name?: string; status?: string; canDecide?: boolean }[] }[];
};

/**
 * Raises a change request through the CSM portal's backend as `jane` (the requester persona, in no approval
 * group, so alice, bob and carol may all approve it), assigned to Example Corp ABT, and returns it.
 *
 * @param options.projectId - The Customer Project (`null`: none).
 * @param options.approval / options.review - The two creation-form boxes.
 * @param options.type - normal (default), standard or emergency.
 */
export async function raiseChange(options: {
  title: string;
  projectId: string | null;
  approval: boolean;
  review: boolean;
  type?: "normal" | "standard" | "emergency";
}): Promise<RaisedChange> {
  const title = `${RAISED_PREFIX}${options.title}`;
  const created = await staffApi("jane").create({
    subject: title,
    type: options.type ?? "normal",
    groupId: EXAMPLE_CORP_ABT_GROUP_ID,
    ...(options.projectId ? { projectId: options.projectId } : {}),
    customerApprovalRequired: options.approval,
    customerReviewRequired: options.review,
  });
  const change = created.body.changeRequest;
  if (created.status !== 201 || !change) {
    throw new Error(`raising "${title}" answered ${created.status}: ${JSON.stringify(created.body)}`);
  }
  return { id: change.id, number: change.number, title };
}

/** Request Approval (New -> Assess), as the requester. */
export async function requestApproval(id: string): Promise<ApiResult<StaffChangeRequest>> {
  return staffApi("jane").patch(id, { state: "assess" }) as Promise<ApiResult<StaffChangeRequest>>;
}

/** A staff decision that must succeed. */
export async function staffDecides(who: StaffPersona, id: string, decision: "approved" | "rejected" = "approved"): Promise<void> {
  const result = await staffApi(who).decide(id, decision);
  if (result.status !== 200) throw new Error(`${who}'s ${decision} on ${id} answered ${result.status}: ${JSON.stringify(result.body)}`);
}

/**
 * A staff state change that must succeed (`implement`, `review`, `customer_review`, `closed`, ...). It must be a LEGAL next
 * state of the change's current one: the backend refuses a jump over a state or an approval gate (`implement` out of New
 * with the customer's approval ticked, say), a move out of a final state (Closed, Canceled, Rollback), and the customer's
 * own answer (`scheduled` out of Customer Approval, `closed` out of Customer Review), so a spec walks a change one edge at a
 * time -- Request Approval, the approvals, then these moves -- and lets the customer give their own answer.
 */
export async function staffMoves(who: StaffPersona, id: string, state: string): Promise<void> {
  const result = await staffApi(who).patch(id, { state });
  if (result.status !== 200) throw new Error(`${who} moving ${id} to ${state} answered ${result.status}: ${JSON.stringify(result.body)}`);
}

// --- WSO2's side of a customer's proposed time, through the CSM portal's backend -----------------------

/**
 * A time window as the staff API takes it: RFC 3339 instants in UTC (what {@link futureWindow} returns as
 * `startUtc` / `endUtc`).
 */
export type StaffWindow = { startUtc: string; endUtc: string };

/** What a staff answer to a proposal names as seen: the proposal's version and the planned window the page showed. */
async function whatStaffSee(who: StaffPersona, id: string): Promise<{
  proposal: StaffProposal | null;
  expected: { expectedPlannedStartOn?: string; expectedPlannedEndOn?: string };
  read: ApiResult<StaffChangeRequest>;
}> {
  const read = await staffApi(who).get(id);
  if (read.status !== 200) throw new Error(`${who} reading ${id} answered ${read.status}: ${JSON.stringify(read.body)}`);
  const body = read.body;
  return {
    read,
    proposal: body.customerProposal ?? null,
    expected: {
      ...(body.plannedStartOn ? { expectedPlannedStartOn: body.plannedStartOn } : {}),
      ...(body.plannedEndOn ? { expectedPlannedEndOn: body.plannedEndOn } : {}),
    },
  };
}

/**
 * WSO2 ACCEPTS the proposed time (the CSM portal's "Accept proposed time"): `confirmCustomerUpdatedDate: "agree"` with the
 * proposal's version and the planned window the page showed, which the service applies in one write -- the proposed start
 * on the planned length, state Scheduled, no CAB, no second ask of the customer. Returns the raw result so a spec can also
 * assert a refusal.
 */
export async function staffAcceptsProposal(who: StaffPersona, id: string): Promise<ApiResult<{ message?: string }>> {
  const { proposal, expected } = await whatStaffSee(who, id);
  return staffApi(who).patch(id, {
    confirmCustomerUpdatedDate: "agree",
    ...(proposal ? { expectedCustomerUpdatedOn: proposal.startOn } : {}),
    ...expected,
  });
}

/**
 * WSO2 PROPOSES A DIFFERENT TIME (the CSM portal's counter): `{state: "authorize", window}` with the proposal's version and
 * the planned window the page showed. The window is WSO2's; the customers are asked again, and no CAB is involved. (The wire
 * name `authorize` is the Time Change loop's name: the state stays Customer Approval.)
 */
export async function staffCountersProposal(
  who: StaffPersona,
  id: string,
  window: StaffWindow,
): Promise<ApiResult<{ message?: string }>> {
  const { proposal, expected } = await whatStaffSee(who, id);
  return staffApi(who).patch(id, {
    state: "authorize",
    plannedStartOn: window.startUtc,
    plannedEndOn: window.endUtc,
    ...(proposal ? { expectedCustomerUpdatedOn: proposal.startOn } : {}),
    ...expected,
  });
}

/**
 * WSO2 DECLINES the proposed time: "Propose a different time" with the window left as it is (the previous system's Disagree). The
 * customers' live requests are untouched, and the planned window stays.
 */
export async function staffDeclinesProposal(who: StaffPersona, id: string): Promise<ApiResult<{ message?: string }>> {
  const { proposal, expected, read } = await whatStaffSee(who, id);
  return staffApi(who).patch(id, {
    state: "authorize",
    ...(read.body.plannedStartOn ? { plannedStartOn: read.body.plannedStartOn } : {}),
    ...(read.body.plannedEndOn ? { plannedEndOn: read.body.plannedEndOn } : {}),
    ...(proposal ? { expectedCustomerUpdatedOn: proposal.startOn } : {}),
    ...expected,
  });
}

/**
 * A plain Re-schedule (nobody proposed anything): `{state: "authorize", window}` out of Customer Approval. The customers are
 * asked again of the new window, no CAB is involved, and the state does not move.
 */
export async function staffReschedules(
  who: StaffPersona,
  id: string,
  window: StaffWindow,
): Promise<ApiResult<{ message?: string }>> {
  const { expected } = await whatStaffSee(who, id);
  return staffApi(who).patch(id, { state: "authorize", plannedStartOn: window.startUtc, plannedEndOn: window.endUtc, ...expected });
}

/** A proposal as the database holds it: the proposed start (UTC instant, "" when none) and WSO2's answer ("" none, AGREE, DISAGREE). */
export async function proposalRow(id: string): Promise<{ proposedUtc: string; answer: string }> {
  const out = await psql(
    `select coalesce(to_char(customer_updated_on at time zone 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"'),''), ` +
      `coalesce(customer_updated_date_confirmation::text,'') from change_request where id = '${id}'`,
  );
  const [proposedUtc, answer] = out.split("|");
  return { proposedUtc, answer };
}

/** Plans a change request's window straight in the database (the seeded fixtures carry none), as UTC instants. */
export async function planWindow(id: string, window: StaffWindow): Promise<void> {
  await psql(`update change_request set start_on = '${window.startUtc}', end_on = '${window.endUtc}' where id = '${id}'`);
}

/**
 * The window the Customer Approval fixture is given by {@link resetFixtures}: two hours, a month ahead, so a customer has a
 * planned start to move (a proposal moves the START and keeps the length, and is refused on a change that has no window).
 */
export function plannedFixtureWindow(): StaffWindow {
  const { startUtc, endUtc } = futureWindow("UTC", { daysAhead: 30, startHour: 10, hours: 2 });
  return { startUtc, endUtc };
}

/** The stored state of a change request, as the stack's database holds it (UPPER_SNAKE). */
export async function storedState(id: string): Promise<string> {
  return (await psql(`select state from change_request where id = '${id}'`)).trim();
}

/**
 * Takes away every change request a spec raised ({@link RAISED_PREFIX}); the cascade removes their stages and
 * approver rows. Run it before and after a spec that raises, so a failed run leaves nothing behind either.
 */
export async function deleteRaisedChanges(): Promise<void> {
  await psql(`delete from work_item where type = 'CHANGE_REQUEST' and subject like '${RAISED_PREFIX}%'`);
}

/** The id of "Lumen Works Platform" (mira and noel's project; random per database), found as mira finds it. */
let lumenId: Promise<string> | undefined;
export function lumenProjectId(): Promise<string> {
  lumenId ??= (async () => {
    const id = await customerApi("mira").projectIdByName(LOCAL_PERSONAS.mira.project);
    if (!id) throw new Error(`${LOCAL_PERSONAS.mira.project} is not one of mira's projects: is the stack seeded?`);
    return id;
  })();
  return lumenId;
}

/**
 * Saves a screenshot of `page` as `<E2E_SHOT_DIR>/<name>.png` when that directory is named (and does nothing
 * otherwise): the way a run leaves pictures of its key screens without a spec ever failing for want of a folder.
 */
export async function shot(page: Page, name: string): Promise<void> {
  const dir = process.env.E2E_SHOT_DIR?.trim();
  if (!dir) return;
  fs.mkdirSync(dir, { recursive: true });
  await page.screenshot({ path: path.join(dir, `${name}.png`) });
}

// --- What the stat cards and the dashboard say to a customer --------------------------------

/** What one state adds to the project's counts for a customer who sees a change request in it (entity-service's active / outstanding / action-required states; Authorize is outstanding for a customer only). */
export const STATE_ADDS: Record<string, { active: number; outstanding: number; actionRequired: number; resolved: number }> = {
  Authorize: { active: 1, outstanding: 1, actionRequired: 0, resolved: 0 },
  "Customer Approval": { active: 1, outstanding: 1, actionRequired: 1, resolved: 0 },
  Scheduled: { active: 1, outstanding: 1, actionRequired: 0, resolved: 0 },
  Implement: { active: 1, outstanding: 1, actionRequired: 0, resolved: 0 },
  Review: { active: 1, outstanding: 1, actionRequired: 0, resolved: 0 },
  "Customer Review": { active: 1, outstanding: 1, actionRequired: 1, resolved: 0 },
  Rollback: { active: 1, outstanding: 1, actionRequired: 0, resolved: 0 },
  // Closed is "resolved" (the resolved card counts it), no longer active or outstanding
  Closed: { active: 0, outstanding: 0, actionRequired: 0, resolved: 1 },
  Canceled: { active: 0, outstanding: 0, actionRequired: 0, resolved: 0 },
};

/** The project's stat cards and the dashboard's Outstanding count, as one customer is told them. */
export type Counts = {
  total: number;
  active: number;
  outstanding: number;
  actionRequired: number;
  /** The "resolved" card's total (Closed change requests). */
  resolved: number;
  /** The dashboard's "Outstanding" change request count (GET /projects/{id}/stats). */
  dashboard: number;
  byState: Record<string, number>;
};

/** What the project's stat cards and the dashboard say right now to `email` (undefined: the project is refused them). */
export async function customerCounts(email: string, projectId: string): Promise<Counts | undefined> {
  const api = customerApiFor(email);
  const stats = await api.stats(projectId);
  if (!stats) return undefined;
  const dashboard = (await api.outstandingChangeRequests(projectId)) ?? -1;
  return {
    total: stats.totalCount,
    active: stats.activeCount,
    outstanding: stats.outstandingCount,
    actionRequired: stats.actionRequiredCount,
    resolved: stats.resolvedCount?.total ?? 0,
    dashboard,
    byState: Object.fromEntries(stats.stateCount.map((s) => [s.label, s.count])),
  };
}

/**
 * The counts a customer must be told once one more change request is visible to them in each of `labels` (an empty
 * list, or null: none is, and the counts are `base`).
 */
export function countsWith(base: Counts, labels: string | string[] | null): Counts {
  const list = labels === null ? [] : Array.isArray(labels) ? labels : [labels];
  const out: Counts = { ...base, byState: { ...base.byState } };
  for (const label of list) {
    const add = STATE_ADDS[label];
    out.total += 1;
    out.active += add.active;
    out.outstanding += add.outstanding;
    out.actionRequired += add.actionRequired;
    out.resolved += add.resolved;
    out.dashboard += add.outstanding;
    out.byState[label] = (out.byState[label] ?? 0) + 1;
  }
  return out;
}

// --- Contacts registered AFTER a stage was provisioned ----------------------------------------

/** The late contact: registered on Lumen Works Platform only once a change request had been put to mira and noel. */
export const LATE_CONTACT = { email: "ozzy.late@lumenworks.example", name: "Ozzy Late" } as const;

/** SQL that registers LATE_CONTACT on Lumen Works Platform exactly as the seed registers mira and noel (a customer user, a REGISTERED PORTAL_USER contact). */
const REGISTER_LATE_CONTACT_SQL = `
BEGIN;
INSERT INTO "user" (id, created_on, updated_on, created_by, updated_by, user_name, name, first_name, last_name, email, is_active, is_system_user)
VALUES ('00000000-0000-0000-0000-00000e2e0001', now(), now(), 'e2e', 'e2e', '${LATE_CONTACT.email}', '${LATE_CONTACT.name}', 'Ozzy', 'Late', '${LATE_CONTACT.email}', true, false)
ON CONFLICT (id) DO NOTHING;
INSERT INTO user_role (id, created_on, updated_on, user_id, role_id)
VALUES ('00000000-0000-0000-0000-00000e2e0002', now(), now(), '00000000-0000-0000-0000-00000e2e0001', '00000000-0000-0000-0000-000000000102')
ON CONFLICT (id) DO NOTHING;
WITH lumen AS (SELECT id, account_id FROM project WHERE name = 'Lumen Works Platform' AND account_id IS NOT NULL ORDER BY created_on, id LIMIT 1)
INSERT INTO account_contact (id, created_on, updated_on, created_by, updated_by, is_active, user_name, account_id)
SELECT '00000000-0000-0000-0000-00000e2e0003', now(), now(), 'e2e', 'e2e', true, '${LATE_CONTACT.email}', lumen.account_id FROM lumen
ON CONFLICT (id) DO NOTHING;
WITH lumen AS (SELECT id FROM project WHERE name = 'Lumen Works Platform' AND account_id IS NOT NULL ORDER BY created_on, id LIMIT 1)
INSERT INTO project_contact (id, created_on, updated_on, created_by, updated_by, email, state, account_contact_id, project_id)
SELECT '00000000-0000-0000-0000-00000e2e0004', now(), now(), 'e2e', 'e2e', '${LATE_CONTACT.email}', 'REGISTERED', '00000000-0000-0000-0000-00000e2e0003', lumen.id FROM lumen
ON CONFLICT (id) DO UPDATE SET state = 'REGISTERED';
INSERT INTO project_contact_group (id, created_on, updated_on, created_by, updated_by, project_contact_id, project_group_id)
SELECT '00000000-0000-0000-0000-00000e2e0005', now(), now(), 'e2e', 'e2e', '00000000-0000-0000-0000-00000e2e0004', pg.id
FROM project_group pg WHERE pg."group" = 'General Access'
ON CONFLICT (id) DO NOTHING;
COMMIT;`;

/** Registers {@link LATE_CONTACT} on Lumen Works Platform (idempotent). */
export async function registerLateContact(): Promise<void> {
  await psql(REGISTER_LATE_CONTACT_SQL);
}

/** Removes {@link LATE_CONTACT} again (the cascade takes their approver rows too, if any). */
export async function removeLateContact(): Promise<void> {
  await psql(`
    DELETE FROM approval_stage_approver WHERE approver_user_id = '00000000-0000-0000-0000-00000e2e0001';
    DELETE FROM project_contact_group WHERE id = '00000000-0000-0000-0000-00000e2e0005';
    DELETE FROM project_contact WHERE id = '00000000-0000-0000-0000-00000e2e0004';
    DELETE FROM account_contact WHERE id = '00000000-0000-0000-0000-00000e2e0003';
    DELETE FROM user_role WHERE id = '00000000-0000-0000-0000-00000e2e0002';
    DELETE FROM "user" WHERE id = '00000000-0000-0000-0000-00000e2e0001';`);
}

// --- Legacy (migrated from ServiceNow) change requests -----------------------------------------

const LEGACY_SEED_FILE = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "../fixtures/legacy-change-requests.sql",
);

/** The numbers of the legacy rows (fixtures/legacy-change-requests.sql), by what they are. */
export const LEGACY = {
  /** project 401, one per state (flags false, nothing asked of anybody). */
  new: "CHG0039101",
  assess: "CHG0039102",
  authorize: "CHG0039103",
  customerApproval: "CHG0039104",
  scheduled: "CHG0039105",
  implement: "CHG0039106",
  review: "CHG0039107",
  customerReview: "CHG0039108",
  rollback: "CHG0039109",
  closed: "CHG0039110",
  canceled: "CHG0039111",
  /** Customer Approval with a window already planned (to propose another on). */
  customerApprovalToPropose: "CHG0039112",
  /** Customer Approval for the second contact (erin) to answer. */
  customerApprovalForErin: "CHG0039113",
  /** Customer Approval with a window planned and the previous system's own unlabeled customer stage (dave and erin REQUESTED). */
  customerApprovalAskedByPreviousSystem: "CHG0039114",
  /** Lumen Works Platform: the user's "Demo Test 1" shape, and a Scheduled one. */
  lumenDemoTest: "CHG0039201",
  lumenScheduled: "CHG0039202",
  /** Synced stages with no label. */
  emergencyInAuthorize: "CHG0039301",
  staleStageScheduled: "CHG0039302",
  /** Either side of the cutover instant. */
  oneSecondBefore: "CHG0039401",
  atTheInstant: "CHG0039402",
} as const;

/** The states in which a customer has always been shown a change request (everything past Authorize). */
export const LEGACY_VISIBLE = [
  LEGACY.customerApproval, LEGACY.scheduled, LEGACY.implement, LEGACY.review, LEGACY.customerReview, LEGACY.rollback,
  LEGACY.closed, LEGACY.canceled, LEGACY.customerApprovalToPropose, LEGACY.customerApprovalForErin, LEGACY.customerApprovalAskedByPreviousSystem,
  LEGACY.staleStageScheduled, LEGACY.oneSecondBefore,
] as const;

/** A legacy row's id: ServiceNow-style (md5 of the number rendered as a UUID), as the seed file writes it. */
export function legacyId(number: string): string {
  const hex = createHash("md5").update(`legacy-${number}`).digest("hex");
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`;
}

/**
 * The cutover instant the stack's entity-service runs with (`CR_STRICT_VISIBILITY_FROM`); the local compose default is
 * 2000-01-01T00:00:00Z. Name it in `E2E_CR_STRICT_VISIBILITY_FROM` when the stack under test runs with another (it must be
 * AFTER 1999-12-20, the age of the legacy rows).
 */
export function strictVisibilityFrom(): string {
  return process.env.E2E_CR_STRICT_VISIBILITY_FROM?.trim() || "2000-01-01T00:00:00Z";
}

/** (Re)writes the legacy rows to their starting state; the two boundary rows are put either side of the cutover instant. */
export async function seedLegacyChangeRequests(): Promise<void> {
  await psql(fs.readFileSync(LEGACY_SEED_FILE, "utf8"));
  const instant = strictVisibilityFrom().replace(/'/g, "");
  await psql(
    `update work_item set created_on = '${instant}'::timestamptz - interval '1 second', updated_on = '${instant}'::timestamptz - interval '1 second' where number = '${LEGACY.oneSecondBefore}';
     update work_item set created_on = '${instant}'::timestamptz, updated_on = '${instant}'::timestamptz where number = '${LEGACY.atTheInstant}';`,
  );
}

/** Takes the legacy rows away again, so the specs that count a project's list do not see them. */
export async function deleteLegacyChangeRequests(): Promise<void> {
  await psql("delete from work_item where created_by = 'sn-sync' and number ~ '^CHG0039[1234]'");
}

/** One approver row of a change request: stage label (or "" for an unlabeled one), who, and its state. */
export async function stageRows(id: string): Promise<string[]> {
  const out = await psql(
    "select coalesce(s.checkpoint_label, '(no label)') || '|' || coalesce(u.email, '(no user)') || '|' || a.state " +
      "from approval_stage s join approval_stage_approver a on a.stage_id = s.id " +
      'left join "user" u on u.id = a.approver_user_id ' +
      `where s.work_item_id = '${id}' order by s.created_on, s.id, u.email nulls last`,
  );
  return out ? out.split("\n") : [];
}

// --- Calls straight at entity-service, as the customer portal's backend makes them ------------

/** entity-service's published port (isolated stack: http://localhost:18081); undefined when unset. */
export function entityServiceUrl(): string | undefined {
  return process.env.E2E_ENTITY_SERVICE_URL?.trim().replace(/\/+$/, "") || undefined;
}

/**
 * A customer's call straight to entity-service, bypassing the customer portal's backend (and so its early
 * validation): what that backend forwards is the backend's own machine token plus the customer's own ID token
 * (`x-user-id-token`). It proves a rule is the SERVICE's, not just the backend's.
 */
export async function entityAsCustomer(
  email: string,
  method: string,
  pathAndQuery: string,
  body?: unknown,
): Promise<ApiResult> {
  const base = entityServiceUrl();
  if (!base) throw new Error("E2E_ENTITY_SERVICE_URL is not set");
  const { oidc, customerClientId } = await stackEndpoints();
  // The customer's ID token, by the same authorization-code exchange the apps perform.
  const authorize = await fetch(`${oidc}/oauth2/authorize`, {
    method: "POST", redirect: "manual", headers: FORM,
    body: new URLSearchParams({ client_id: customerClientId, redirect_uri: appOrigin(), state: "e2e", scope: "openid", email, groups: "" }),
  });
  const code = new URL(authorize.headers.get("location") ?? "http://x").searchParams.get("code");
  if (!code) throw new Error(`no code for ${email}`);
  const tokens = (await (await fetch(`${oidc}/oauth2/token`, {
    method: "POST", headers: FORM,
    body: new URLSearchParams({ grant_type: "authorization_code", code, client_id: customerClientId, redirect_uri: appOrigin() }),
  })).json()) as { id_token?: string };
  // The backend's own machine token (client credentials).
  const machine = (await (await fetch(`${oidc}/oauth2/token`, {
    method: "POST",
    headers: { ...FORM, authorization: `Basic ${Buffer.from("customer-portal-backend-dev-client:dev-secret").toString("base64")}` },
    body: new URLSearchParams({ grant_type: "client_credentials" }),
  })).json()) as { access_token?: string };
  if (!tokens.id_token || !machine.access_token) throw new Error("the identity provider gave no token");
  const response = await fetch(`${base}${pathAndQuery}`, {
    method,
    headers: {
      authorization: `Bearer ${machine.access_token}`,
      "x-user-id-token": tokens.id_token,
      ...(body === undefined ? {} : { "content-type": "application/json" }),
    },
    body: body === undefined ? undefined : JSON.stringify(body),
    signal: AbortSignal.timeout(30_000),
  });
  const text = await response.text();
  let parsed: unknown = text;
  try { parsed = text ? JSON.parse(text) : null; } catch { /* keep the text */ }
  return { status: response.status, body: parsed };
}

// --- The stack's Postgres (the isolated stack's, named by the environment) ----------

/** The Postgres container named by E2E_POSTGRES_CONTAINER; undefined when unset. */
export function postgresContainer(): string | undefined {
  return process.env.E2E_POSTGRES_CONTAINER?.trim() || undefined;
}

/**
 * Runs SQL in the stack's Postgres (psql inside its container) and resolves with
 * what it printed (unaligned, tuples only). The SQL goes in on stdin so the whole
 * seed file fits.
 */
export async function psql(sql: string): Promise<string> {
  const container = postgresContainer();
  if (!container) throw new Error("E2E_POSTGRES_CONTAINER is not set");
  const user = process.env.E2E_POSTGRES_USER ?? "postgres";
  const db = process.env.E2E_POSTGRES_DB ?? "csm_platform";
  return await new Promise<string>((resolve, reject) => {
    const child = spawn("docker", [
      "exec", "-i", container, "psql", "-U", user, "-d", db,
      "-v", "ON_ERROR_STOP=1", "-q", "-t", "-A", "-f", "-",
    ]);
    let stdout = "";
    let stderr = "";
    child.stdout.on("data", (d: Buffer) => (stdout += d.toString()));
    child.stderr.on("data", (d: Buffer) => (stderr += d.toString()));
    child.on("error", reject);
    child.on("close", (code) =>
      code === 0 ? resolve(stdout.trim()) : reject(new Error(`psql exited ${code}: ${stderr}`)),
    );
    child.stdin.end(sql);
  });
}

/** Whether the named Postgres container is running (false when docker is absent). */
async function containerRunning(): Promise<boolean> {
  const container = postgresContainer();
  if (!container) return false;
  return await new Promise<boolean>((resolve) => {
    const child = spawn("docker", ["inspect", "-f", "{{.State.Running}}", container]);
    let out = "";
    child.stdout.on("data", (d: Buffer) => (out += d.toString()));
    child.on("error", () => resolve(false));
    child.on("close", (code) => resolve(code === 0 && out.trim() === "true"));
  });
}

/**
 * Makes a spec SKIP, with the reason, unless it can reset the seeded fixtures:
 * E2E_POSTGRES_CONTAINER names a running container. Call it at the top of every
 * spec file that moves a fixture on.
 *
 * @param t - The `test` object of the spec file.
 * @param alsoNeedsCsmBff - True for specs in which WSO2 staff decide something.
 */
export function withFixtureStack(t: typeof test, alsoNeedsCsmBff = false): void {
  t.beforeEach(async () => {
    t.skip(
      !postgresContainer(),
      "This spec moves seeded change requests on and puts them back by re-running the seed, so it " +
        "needs the Postgres of the stack under test named explicitly: E2E_POSTGRES_CONTAINER=<container> " +
        "(the isolated stack's is csmenv-postgres-1). It never guesses one.",
    );
    t.skip(
      !(await containerRunning()),
      `E2E_POSTGRES_CONTAINER=${postgresContainer()} is not a running container (is docker up?).`,
    );
    if (alsoNeedsCsmBff) {
      t.skip(
        !csmBffUrl(),
        "This spec has WSO2 staff decide the internal approval through the CSM portal's backend: " +
          "set E2E_CSM_BFF_URL (the isolated stack's is http://localhost:18082).",
      );
    }
  });

  // Leave the stack as it was found: the fixtures back in their starting state, so
  // the read-only smoke spec (and the next person) finds CHG-FIXED-007 waiting.
  // Guarded the same way, since afterAll also runs when every test skipped.
  t.afterAll(async () => {
    if (!postgresContainer() || !(await containerRunning())) return;
    await psql(fs.readFileSync(SEED_FILE, "utf8"));
  });
}

/**
 * Puts the CHG-FIXED-* fixtures back to their starting state by re-running the
 * (self-healing) seed in the stack's Postgres, then proves the stack under test
 * sees it: through the CUSTOMER backend, dave must be asked in 007 and 008. (The
 * Customer Approval fixture is then given a planned window, {@link plannedFixtureWindow}.) A
 * container that belongs to another stack fails here, with that said, rather
 * than as a puzzling assertion later.
 */
export async function resetFixtures(): Promise<void> {
  await psql(fs.readFileSync(SEED_FILE, "utf8"));
  // The seed plans no window; a customer's proposal moves the planned START, so the fixture gets one.
  await planWindow(FIXTURES.approval.id, plannedFixtureWindow());
  const dave = customerApi("dave");
  const deadline = Date.now() + 15_000;
  let last = "";
  for (;;) {
    const [approval, review] = await Promise.all([
      dave.get(FIXTURES.approval.id),
      dave.get(FIXTURES.review.id),
    ]);
    last = `${FIXTURES.approval.number}: HTTP ${approval.status} ${approval.body?.state?.label} answer=${approval.body?.customerCanAnswer}; ` +
      `${FIXTURES.review.number}: HTTP ${review.status} ${review.body?.state?.label} answer=${review.body?.customerCanAnswer}`;
    if (
      approval.status === 200 && approval.body.state?.label === "Customer Approval" && approval.body.customerCanAnswer === true &&
      review.status === 200 && review.body.state?.label === "Customer Review" && review.body.customerCanAnswer === true
    ) {
      return;
    }
    if (Date.now() > deadline) {
      throw new Error(
        `After re-seeding ${postgresContainer()}, dave does not see the fixtures waiting for him through ` +
          `${(await stackEndpoints()).customerApi} (${last}). Either E2E_POSTGRES_CONTAINER is not the database ` +
          "of the stack at E2E_BASE_URL, or that stack is older than the customerCanAnswer field.",
      );
    }
    await new Promise((r) => setTimeout(r, 500));
  }
}

/** What the database holds about a change request's state and window. */
export type ChangeRequestRow = {
  state: string;
  /** `YYYY-MM-DDTHH:MM:SSZ` or "" when unset. */
  startUtc: string;
  endUtc: string;
  title: string;
};

/** Reads a change request's raw row (state enum, planned window in UTC, subject). */
export async function changeRequestRow(id: string): Promise<ChangeRequestRow> {
  const fmt = (col: string) => `coalesce(to_char(cr.${col} at time zone 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"'),'')`;
  const out = await psql(
    `select cr.state, ${fmt("start_on")}, ${fmt("end_on")}, wi.subject ` +
      `from change_request cr join work_item wi on wi.id = cr.id where cr.id = '${id}'`,
  );
  const [state, startUtc, endUtc, ...title] = out.split("|");
  return { state, startUtc, endUtc, title: title.join("|") };
}

/** One approver row: the stage it belongs to, who, and its approval_stage_approver.state (UPPER_SNAKE_CASE: REQUESTED, APPROVED, REJECTED, CANCELLED, ...). */
export type ApproverRow = { stage: string; email: string; state: string };

/** Every approver row of a change request, oldest stage first (the whole history). */
export async function approverRows(id: string): Promise<ApproverRow[]> {
  const out = await psql(
    "select s.checkpoint_label, u.email, a.state from approval_stage s " +
      "join approval_stage_approver a on a.stage_id = s.id " +
      'join "user" u on u.id = a.approver_user_id ' +
      `where s.work_item_id = '${id}' order by s.created_on, s.id, u.email`,
  );
  return out
    ? out.split("\n").map((line) => {
        const [stage, email, state] = line.split("|");
        return { stage, email, state };
      })
    : [];
}

// --- Wall-clock helpers for the Propose New Time dialog (independent of the app's) ---

/** `YYYY-MM-DDTHH:mm` on `zone`'s wall clock at the instant `at`. */
export function wallTime(at: Date, zone: string): string {
  const parts = new Intl.DateTimeFormat("en-CA", {
    timeZone: zone,
    year: "numeric", month: "2-digit", day: "2-digit",
    hour: "2-digit", minute: "2-digit", hourCycle: "h23",
  }).formatToParts(at);
  const p = (type: string) => parts.find((x) => x.type === type)?.value ?? "00";
  return `${p("year")}-${p("month")}-${p("day")}T${p("hour")}:${p("minute")}`;
}

/** The UTC instant at which `zone`'s wall clock reads `local` (`YYYY-MM-DDTHH:mm`). */
export function wallTimeToUtc(local: string, zone: string): Date {
  const [datePart, timePart] = local.split("T");
  const [y, mo, d] = datePart.split("-").map(Number);
  const [h, mi] = timePart.split(":").map(Number);
  const asUtc = Date.UTC(y, mo - 1, d, h, mi);
  let instant = asUtc;
  // Two passes settle the zone's offset (it can differ around the guess).
  for (let i = 0; i < 2; i++) {
    const shown = wallTime(new Date(instant), zone);
    const [sd, st] = shown.split("T");
    const [sy, smo, sdd] = sd.split("-").map(Number);
    const [sh, smi] = st.split(":").map(Number);
    instant -= Date.UTC(sy, smo - 1, sdd, sh, smi) - asUtc;
  }
  return new Date(instant);
}

/** `utc` (`YYYY-MM-DDTHH:MM:SSZ`) moved by `hours` of real time, in the same form: the end a proposed start implies. */
export function addHours(utc: string, hours: number): string {
  return new Date(Date.parse(utc) + hours * 3_600_000).toISOString().replace(/\.\d{3}Z$/, "Z");
}

/**
 * A proposed window `daysAhead` days from today on `zone`'s calendar, starting at
 * `startHour`:00 and lasting `hours`, as the two `datetime-local` strings the
 * dialog takes and the UTC instants the backend must end up holding.
 */
export function futureWindow(
  zone: string,
  options: { daysAhead: number; startHour?: number; hours?: number },
): { start: string; end: string; startUtc: string; endUtc: string } {
  const { daysAhead, startHour = 16, hours = 4 } = options;
  const today = wallTime(new Date(), zone).split("T")[0];
  const [y, mo, d] = today.split("-").map(Number);
  const day = new Date(Date.UTC(y, mo - 1, d + daysAhead));
  const date = day.toISOString().slice(0, 10);
  const pad = (n: number) => String(n).padStart(2, "0");
  const start = `${date}T${pad(startHour)}:00`;
  const end = `${date}T${pad(startHour + hours)}:00`;
  const iso = (local: string) => wallTimeToUtc(local, zone).toISOString().replace(/\.\d{3}Z$/, "Z");
  return { start, end, startUtc: iso(start), endUtc: iso(end) };
}
