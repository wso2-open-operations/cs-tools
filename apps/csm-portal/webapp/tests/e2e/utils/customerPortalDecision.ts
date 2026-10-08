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
// Stands in for the CUSTOMER portal in the CSM portal's real-stack specs.
//
// Customers do not sign in to the CSM portal: they answer a change request's
// Customer Approval / Customer Review in the customer portal, whose backend
// forwards the customer's own ID token to entity-service. A CSM spec that needs
// the customer's answer (to assert what the CSM Approvals tab then SHOWS) must
// therefore not drive a customer through the CSM UI; it applies the answer the
// same way the customer portal does, straight to entity-service with the
// customer's own token, and then looks at the CSM page as an internal user.
//
// The token comes from the local mock-oidc provider, which signs an ID token for
// any email without a credential check (the same sign-in the customer webapp
// performs, minus the browser): an authorization-code exchange for the customer
// portal's client id, no PKCE. Local stack only.
//

const OIDC_URL = process.env.E2E_OIDC_URL ?? "http://localhost:9100";
/** entity-service's own published port (the CSM BFF and the customer backend both sit in front of it). */
const ENTITY_SERVICE_URL = process.env.E2E_ENTITY_SERVICE_URL ?? "http://localhost:8081";
const CUSTOMER_PORTAL_ORIGIN = process.env.E2E_CUSTOMER_PORTAL_URL ?? "http://localhost:3000";
const CUSTOMER_PORTAL_CLIENT_ID = "customer-portal-webapp";

const FORM = { "content-type": "application/x-www-form-urlencoded" };

/** A customer's ID token from the local mock-oidc (no groups: a customer is not CSM staff). */
export async function customerIdToken(email: string): Promise<string> {
  const authorize = await fetch(`${OIDC_URL}/oauth2/authorize`, {
    method: "POST",
    redirect: "manual",
    headers: FORM,
    body: new URLSearchParams({
      client_id: CUSTOMER_PORTAL_CLIENT_ID,
      redirect_uri: CUSTOMER_PORTAL_ORIGIN,
      state: "e2e",
      scope: "openid",
      email,
      groups: "",
    }),
  });
  const location = authorize.headers.get("location");
  const code = location ? new URL(location).searchParams.get("code") : null;
  if (!code) throw new Error(`mock-oidc gave no authorization code for ${email} (HTTP ${authorize.status})`);

  const token = await fetch(`${OIDC_URL}/oauth2/token`, {
    method: "POST",
    headers: FORM,
    body: new URLSearchParams({
      grant_type: "authorization_code",
      code,
      client_id: CUSTOMER_PORTAL_CLIENT_ID,
      redirect_uri: CUSTOMER_PORTAL_ORIGIN,
    }),
  });
  const body = (await token.json()) as { id_token?: string };
  if (!token.ok || !body.id_token) throw new Error(`mock-oidc gave no ID token for ${email} (HTTP ${token.status})`);
  return body.id_token;
}

/**
 * The customer `email` approves or rejects the Customer Approval / Customer Review
 * of change request `crId`, as the customer portal would send it. Resolves with
 * the status and body entity-service answered, so a spec can assert the success
 * (200) or a refusal; the caller then reloads the CSM page to see the outcome.
 */
export async function decideAsCustomer(
  crId: string,
  email: string,
  decision: "approved" | "rejected",
): Promise<{ status: number; body: string }> {
  const idToken = await customerIdToken(email);
  const response = await fetch(`${ENTITY_SERVICE_URL}/change-requests/${crId}/approvals/decision`, {
    method: "POST",
    headers: { "content-type": "application/json", "x-user-id-token": idToken },
    body: JSON.stringify({ decision }),
  });
  return { status: response.status, body: await response.text() };
}

/**
 * The customer `email` PROPOSES a new implementation time for change request `crId`, as the customer portal would
 * send it: `PATCH {plannedStartOn, plannedEndOn?}` to entity-service with the customer's own ID token. A proposal is a START
 * (the previous system's own `customer_updated_on`): the change STAYS in Customer Approval and the planned window is untouched until
 * WSO2 answers; `plannedEndOn`, when sent, must be the start plus the planned length. Resolves with the status and body
 * entity-service answered; the caller then reloads the CSM page to see the banner.
 */
export async function proposeAsCustomer(
  crId: string,
  email: string,
  proposal: { plannedStartOn: string; plannedEndOn?: string },
): Promise<{ status: number; body: string }> {
  const idToken = await customerIdToken(email);
  const response = await fetch(`${ENTITY_SERVICE_URL}/change-requests/${crId}`, {
    method: "PATCH",
    headers: { "content-type": "application/json", "x-user-id-token": idToken },
    body: JSON.stringify(proposal),
  });
  return { status: response.status, body: await response.text() };
}
