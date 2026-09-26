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

/**
 * The Phase 2 draft/approval workflow's state machine — see entity-service's
 * CLAUDE.md ("Announcement requests") for the authoritative definition. Every
 * transition is enforced server-side; the UI only reflects it.
 */
export type AnnouncementRequestState = "draft" | "pending_approval" | "approved" | "published";

export type AnnouncementRequestKind = "customer" | "eol";

/**
 * The customer flow's audience shape, mirroring the scope/exclusion fields
 * `CreateCustomerAnnouncementForm`/`AudienceScopeControls` already collect —
 * see that form's `audienceFilters`/`targetProjectIds`. Stored verbatim as
 * `audienceDefinition` (opaque JSONB to the backend) so a draft can be
 * re-opened without re-deriving anything.
 */
export interface CustomerAudienceDefinition {
  scope: "specific" | "all";
  /** Only meaningful when `scope` is `"specific"`. */
  projectIds?: string[];
  /** Only meaningful when `scope` is `"all"` — see `useResolveAnnouncementAudience`'s `AudienceFilters`. */
  excludeClosureStates?: string[];
  excludeSubscriptionTypes?: string[];
}

/** The EOL/product-version flow's audience shape. */
export interface EolAudienceDefinition {
  productId: string;
  productVersionId: string;
}

export type AnnouncementRequestAudienceDefinition =
  | CustomerAudienceDefinition
  | EolAudienceDefinition;

/**
 * Passthrough shape from csm-portal-backend's own `AnnouncementRequest`
 * (itself a passthrough from entity-service) — see that OpenAPI schema's doc
 * comment. `audienceDefinition` is untyped JSON on the wire; callers narrow it
 * by `kind` (`"customer"` → {@link CustomerAudienceDefinition}, `"eol"` →
 * {@link EolAudienceDefinition}).
 */
export interface AnnouncementRequest {
  id: string;
  kind: AnnouncementRequestKind;
  state: AnnouncementRequestState;
  subject: string;
  description: string;
  isSecurityAnnouncement: boolean;
  audienceDefinition: Record<string, unknown>;
  resolvedProjectIds?: string[] | null;
  resolvedProjectCount?: number | null;
  dryRunCaseId?: string | null;
  dryRunAt?: string | null;
  dryRunBy?: string | null;
  createdBy: string;
  /**
   * Display-only companion to createdBy (an opaque IdP account id, not
   * human-readable) — the actor's resolved email at the moment of the
   * action, for showing something readable instead of that id. Never used
   * for any creator/ownership check — those must always compare against
   * the *By id field. Null for a row written before this field existed.
   */
  createdByEmail?: string | null;
  createdAt: string;
  updatedAt: string;
  submittedBy?: string | null;
  submittedByEmail?: string | null;
  submittedAt?: string | null;
  approvedBy?: string | null;
  approvedByEmail?: string | null;
  approvedAt?: string | null;
  publishedBy?: string | null;
  publishedByEmail?: string | null;
  publishedAt?: string | null;
  /** The real case id created for each project in resolvedProjectIds. Null until published. */
  publishedCaseIds?: string[] | null;
  /** Set once, automatically, on submit (now + one month) — display only. Null until submitted. */
  dueOn?: string | null;
  /**
   * Set/cleared only via useScheduleAnnouncementRequest, never by the
   * generic update. When set on an approved request, it's automatically
   * published once this time arrives (see operations/csm-scheduled-tasks'
   * publish_scheduled_announcements sub-cron).
   */
  scheduledFor?: string | null;
}

export interface CreateAnnouncementRequestPayload {
  kind: AnnouncementRequestKind;
  subject: string;
  description: string;
  isSecurityAnnouncement: boolean;
  audienceDefinition: AnnouncementRequestAudienceDefinition;
}

/** `additionalProperties: false` server-side — only send fields that actually changed. */
export interface UpdateAnnouncementRequestPayload {
  subject?: string;
  description?: string;
  isSecurityAnnouncement?: boolean;
  audienceDefinition?: AnnouncementRequestAudienceDefinition;
}

export interface RecordAnnouncementRequestDryRunPayload {
  caseId: string;
}

/** null explicitly clears the schedule. Non-null must be strictly in the future. */
export interface ScheduleAnnouncementRequestPayload {
  scheduledFor: string | null;
}

export interface SearchAnnouncementRequestsPayload {
  state?: AnnouncementRequestState;
  createdBy?: string;
  pagination: { offset: number; limit: number };
}

export interface SearchAnnouncementRequestsResponse {
  requests: AnnouncementRequest[];
  total: number;
  limit: number;
  offset: number;
  hasMore: boolean;
}

/** One dated follow-up comment applied, after the fact, to every case a published announcement request created. */
export interface AnnouncementRequestUpdate {
  id: string;
  announcementRequestId: string;
  content: string;
  createdBy: string;
  /** Display-only companion to createdBy — see AnnouncementRequest.createdByEmail. */
  createdByEmail?: string | null;
  createdOn: string;
}

export interface CreateAnnouncementRequestUpdatePayload {
  content: string;
}

export interface SearchAnnouncementRequestUpdatesResponse {
  updates: AnnouncementRequestUpdate[];
}

/**
 * The outcome of one project's attempt within an announcement request's own
 * Publish fan-out. "tag_failed" (not just succeeded/failed) exists because a
 * security announcement's case-create and its mandatory security-tag attach
 * are two separate calls that can fail independently — a case that exists
 * but is missing its tag must not be treated as "not sent" (a retry would
 * create a duplicate case) or as "fully sent" (the tag is mandatory).
 */
export type AnnouncementRequestDeliveryStatus = "succeeded" | "tag_failed" | "failed";

/**
 * The durable record of one project's outcome within an announcement
 * request's own Publish fan-out — see usePublishAnnouncementRequest's own
 * doc comment for why this exists (replacing purely in-memory retry
 * tracking that was lost if the dialog closed mid-retry).
 */
export interface AnnouncementRequestDelivery {
  id: string;
  announcementRequestId: string;
  projectId: string;
  /** Set for succeeded/tag_failed (the case is real either way). Null for failed. */
  caseId?: string | null;
  status: AnnouncementRequestDeliveryStatus;
  errorMessage?: string | null;
  createdOn: string;
  updatedOn: string;
}

/** One project's outcome within a RecordAnnouncementRequestDeliveriesPayload batch. */
export interface RecordAnnouncementRequestDeliveryEntry {
  projectId: string;
  /** Required for status succeeded/tag_failed. */
  caseId?: string;
  status: AnnouncementRequestDeliveryStatus;
  errorMessage?: string;
}

/**
 * One Publish fan-out pass's worth of per-project outcomes — one entry per
 * project attempted in that pass, not the full resolved audience (a pass
 * that only retried failures need not resend every already-succeeded
 * project's own unchanged row).
 */
export interface RecordAnnouncementRequestDeliveriesPayload {
  deliveries: RecordAnnouncementRequestDeliveryEntry[];
}

export interface SearchAnnouncementRequestDeliveriesResponse {
  deliveries: AnnouncementRequestDelivery[];
}
