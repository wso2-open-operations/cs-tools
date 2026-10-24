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

export type SubscriptionType =
  | "development_support"
  | "managed_cloud_subscription"
  | "evaluation_subscription"
  | "subscription"
  | "cloud_evaluation_support"
  | "internal"
  | "platformer_subscription"
  | "cloud_support"
  | "professional_services";

export type ProjectOnboardingStatus =
  | "Not-Started"
  | "In-Progress"
  | "OnHold"
  | "On-Hold"
  | "Completed"
  | "Expired"
  | "Cancelled"
  | "Not-Applicable"
  | null;

/** The onboarding consultant/owner assigned to a project. `null` for most
 *  projects — only set for onboarding-enabled ones. `email` can genuinely be
 *  `null` (unlike `UserReference.email`, which the backend always populates
 *  with something, even a non-email placeholder) — this is a raw
 *  ServiceNow-sourced contact and may have no email on file. */
export interface ProjectOnboardingOwnerRef {
  id: string;
  name: string;
  email: string | null;
}

export interface Project {
  id: string;
  /** Null/absent when the project has no linked account. */
  account?: { id: string; name: string } | null;
  name: string;
  // The search endpoint returns this as `key` (not `projectKey`).
  key: string;
  subscriptionType: SubscriptionType;
  /** Start of the current renewed period. Only recently added to the search
   *  response, so treat as genuinely nullable. */
  startDate: string | null;
  /** End of the current renewed period. */
  endDate: string | null;
  /** When the project record was created. Never moves; distinct from
   *  `startDate`, which advances on each renewal. */
  createdOn: string;
  /** Free-form closure-state string (e.g. `open`, `notify`, `read_only`,
   *  `restricted`, `suspended`, `closed`). Unknown values must render as a
   *  neutral chip, not crash — see `closureStatePresentation`. */
  closureState: string | null;
  endDateClosureState: string | null;
  invoiceDueDateClosureState: string | null;
  complianceViolationClosureState: string | null;
  complianceViolationDate: string | null;
  /** Opaque JSON describing in-flight suspension processing; not rendered
   *  today. */
  suspensionProcessState: unknown;
  /** Onboarding engagement status. `null`/absent when the project has no
   *  onboarding engagement at all. Treat the project as onboarding-enabled —
   *  and only then show {@link onboardingOwner} — when this is present and
   *  not `"Not-Applicable"`. Also on the search row (not just the single-
   *  project detail response), per entity-service's `ProjectView` schema. */
  onboardingStatus?: ProjectOnboardingStatus;
  /** See {@link ProjectOnboardingOwnerRef}. Only meaningful when
   *  {@link onboardingStatus} indicates onboarding is enabled. */
  onboardingOwner?: ProjectOnboardingOwnerRef | null;
}

/**
 * Parent-account reference embedded in the project detail response
 * (`GET /projects/{id}`). The backend JOINs the account, so the project view
 * gets the account name (and a few account facts) with no extra call. `tier` is
 * free-form here (e.g. "Enterprise"), not the lowercase {@link AccountTier} enum.
 */
export interface ProjectAccountRef {
  id: string;
  name: string;
  activationDate: string | null;
  tier: string;
  region?: string | null;
  agentEnabled: boolean;
  kbReferencesEnabled: boolean;
}

/**
 * Enriched single-project shape returned by `GET /projects/{id}`. Distinct from
 * {@link Project} (the search-result row): it embeds the full parent
 * `account`, and additionally carries `sfId` and `updatedOn`, neither of
 * which the search row returns.
 */
export interface ProjectDetails {
  id: string;
  account: ProjectAccountRef;
  sfId: string;
  name: string;
  key: string;
  subscriptionType: SubscriptionType;
  startDate: string | null;
  endDate: string | null;
  createdOn: string;
  updatedOn: string;
  /** Free-form closure-state string; see {@link Project.closureState}. */
  closureState: string | null;
  /** Whether this project is eligible to raise service requests, as
   *  precomputed by the backing data source. */
  hasSr?: boolean;
  /** Onboarding engagement status (ServiceNow data source only; `null`/absent
   *  elsewhere and when the project has no onboarding engagement at all).
   *  Treat the project as onboarding-enabled — and only then show
   *  {@link onboardingOwner} — when this is present and not `"Not-Applicable"`. */
  onboardingStatus?: ProjectOnboardingStatus;
  /** See {@link ProjectOnboardingOwnerRef}. Only meaningful when
   *  {@link onboardingStatus} indicates onboarding is enabled. */
  onboardingOwner?: ProjectOnboardingOwnerRef | null;
  /** Onboarding calendar dates. Each is a date with no time of day, returned
   *  as an RFC 3339 timestamp at midnight UTC (or `null`/absent when not set).
   *  Render and edit the UTC calendar day, never the viewer's local day -- see
   *  `utcDateOnlyValue`/`formatUtcDateForDisplay` in `@utils/dateTime`. */
  onboardingStartDate?: string | null;
  /** Planned go-live date; see {@link onboardingStartDate} for the format. */
  goLivePlanDate?: string | null;
  /** Actual go-live date; see {@link onboardingStartDate} for the format. */
  goLiveDate?: string | null;
  /** Onboarding expiry date; see {@link onboardingStartDate} for the format. */
  onboardingExpiryDate?: string | null;
  /** Onboarding hours entitlement, in hours. `null`/absent when not tracked. */
  totalOnboardingHours?: number | null;
  /** Onboarding hours consumed, in hours. `null`/absent when not tracked. */
  consumedOnboardingHours?: number | null;
  /** Onboarding hours remaining, in hours. `null`/absent when not tracked;
   *  `0` means none remaining. */
  remainingOnboardingHours?: number | null;
}

export interface SearchProjectsRequest {
  pagination?: {
    limit?: number;
    offset?: number;
  };
  searchQuery?: string;
  /** Filter to projects belonging to this account (ServiceNow data source only). */
  accountId?: string;
  /** Filter to projects whose onboarding engagement is in one of these
   *  states — see {@link Project.onboardingStatus} for the value set. Also
   *  the field name a dashboard widget's `resourceType: "project"` tile uses
   *  on the wire (its `query` is passed verbatim as this request body). */
  onboardingStatus?: string[];
}

export interface SearchProjectsResponse {
  projects: Project[];
  total: number;
  limit: number;
  offset: number;
  hasMore: boolean;
}

/**
 * A file attached to a deployment (`referenceType: "deployment"` on the
 * shared, reference-generic `/attachments*` endpoints — see
 * `BeAttachment` in `@api/backend/types`).
 */
export interface DeploymentAttachment {
  id: string;
  name: string;
  /** MIME type (e.g. image/png, application/pdf). */
  contentType: string;
  sizeBytes: number;
  description?: string | null;
  uploadedBy: string;
  uploadedOn: string;
  downloadUrl?: string | null;
}
