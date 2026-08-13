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
 * Types mirroring `apps/csm-portal/backend/openapi.yaml`. The CSM portal
 * backend proxies these calls to the entity service. Keep this file in sync
 * with the OpenAPI spec: every endpoint hook below imports its request /
 * response from here. Types are prefixed `Be` to distinguish them from
 * existing UI-shape types (`Case`, `Project`, etc.) without namespace imports.
 */

// ---------------------------------------------------------------------------
// Shared
// ---------------------------------------------------------------------------

export interface BePagination {
  /** Zero-based offset. */
  offset?: number;
  /** Page size; capped per endpoint (typically 100). */
  limit?: number;
}

export interface BeErrorPayload {
  message?: string;
}

export interface BeSearchResponseBase {
  total: number;
  limit: number;
  offset: number;
  hasMore: boolean;
}

// ---------------------------------------------------------------------------
// Cases
// ---------------------------------------------------------------------------

export type BeCaseSeverity =
  | "catastrophic"
  | "critical"
  | "high"
  | "medium"
  | "low";

export type BeCaseIssueType =
  | "error"
  | "partial_outage"
  | "performance_degradation"
  | "question"
  | "security_or_compliance"
  | "total_outage";

/** Case type (entity `type`). Only `case` is creatable from the portal. */
export type BeCaseType =
  | "case"
  | "service_request"
  | "security_report_analysis"
  | "announcement"
  | "engagement";

export type BeCaseState =
  | "open"
  | "work_in_progress"
  | "waiting_on_wso2"
  | "awaiting_info"
  | "reopened"
  | "solution_proposed"
  | "closed";

/**
 * Work sub-state of a `work_in_progress` case (entity `CaseWorkState`). `null`
 * when the case is not in progress. The backend rejects comment creation unless
 * the case is `work_in_progress` AND `ongoing`.
 */
export type BeCaseWorkState = "ongoing" | "paused";

/**
 * Resolution code for a closed or solution-proposed case (the Post
 * Resolution Activity — see UseCases.md ISSU-026). Only accepted by
 * `PATCH /cases/{id}` alongside `state: "closed"` or `"solution_proposed"`.
 */
export type BeCaseResolutionCode =
  | "SOLVED_FIXED_BY_SUPPORT_GUIDANCE_PROVIDED"
  | "SOLVED_FIXED_BY_CLOSING_RELATED_INCIDENT"
  | "SOLVED_FIXED_BY_CLOSING_RELATED_RD_TICKET"
  | "SOLVED_WORKAROUND_PROVIDED"
  | "SOLVED_BY_CUSTOMER"
  | "CONSIDERED_FOR_ROADMAP"
  | "INCONCLUSIVE_OUT_OF_SCOPE"
  | "INCONCLUSIVE_CANNOT_REPRODUCE"
  | "INCONCLUSIVE_NO_WORKAROUND"
  | "DUPLICATE_ISSUE"
  | "VOIDED_CANCELED"
  | "ON_HOLD"
  | "CONSIDERED_FOR_ROADMAP_ALT"
  | "SOLVED_FIXED_THE_ISSUE"
  | "SOLVED_WORKAROUND_PROVIDED_ALT"
  | "SOLVED_BY_CONTRIBUTOR"
  | "SOLVED_BY_NOVERA"
  | "ABRUPTLY_CLOSED_DUE_TO_NON_RESPONSIVENESS";

/** Root-cause category for a closed or solution-proposed case. Same gating as {@link BeCaseResolutionCode}. */
export type BeCaseCause =
  | "SOLUTION_ARCHITECTURE"
  | "DEPLOYMENT_ARCHITECTURE"
  | "USER_ERROR_CONFIGURATION"
  | "USER_ERROR_PRODUCT_CONCEPT"
  | "USER_ERROR_RUNTIME"
  | "USER_ERROR_RECOMMENDATION_BEST_PRACTICES"
  | "CUSTOMIZATION_LIMITATION"
  | "CUSTOMIZATION_BUG"
  | "DOCUMENTATION_GAP"
  | "DOCUMENTATION_ERROR"
  | "PRODUCT_LIMITATION"
  | "PRODUCT_BUG"
  | "PRODUCT_REGRESSION"
  | "PRODUCT_MIGRATION"
  | "INFRASTRUCTURE_DATABASE"
  | "INFRASTRUCTURE_OS"
  | "INFRASTRUCTURE_NETWORK"
  | "INFRASTRUCTURE_JDK"
  | "INFRASTRUCTURE_LDAP"
  | "INFRASTRUCTURE_LOAD_BALANCER"
  | "INFRASTRUCTURE_IAAS"
  | "INFRASTRUCTURE_EXTERNAL_PRODUCT"
  | "INFRASTRUCTURE_PROXY"
  | "INFRASTRUCTURE_OTHER"
  | "UNKNOWN";

export type BeCaseSortField = "createdOn" | "updatedOn" | "severity" | "state";

/**
 * Where a case sits in the backing data source's staged auto-closure sequence
 * (DEFAULT -> FIRST_COMMENT -> ON_HOLD -> SECOND_COMMENT). Read-only — the
 * only supported write is `autocloseHoldUntil` on `PATCH /cases/{id}`
 * (ServiceNow only).
 */
export type BeCaseAutoclosureStep =
  | "DEFAULT"
  | "FIRST_COMMENT"
  | "ON_HOLD"
  | "SECOND_COMMENT";

export interface BeCase {
  id: string;
  number?: string;
  wso2Id?: string;
  createdBy?: string;
  projectId?: string;
  deploymentId?: string;
  deployedProductId?: string;
  subject?: string;
  description?: string;
  severity?: BeCaseSeverity;
  issueType?: BeCaseIssueType;
  type?: BeCaseType;
  state?: BeCaseState;
  createdAt?: string;
  updatedAt?: string;
  closedOn?: string;
}

/**
 * Canonical reference to a person: id, email and display name, nothing else.
 * Emitted as a sibling of whatever actor field a response already carried
 * (`createdBy`, `assignedEngineer`, the watch-list entry, ...), so every
 * "who did this" value has one shape.
 *
 * `id` is nullable by design: it is populated only where the backing data
 * source already resolves the actor to a user record (comment/attachment
 * authors, the case assignee); it is deliberately left null elsewhere (case
 * creator, activity-feed actors, watchers) rather than adding a per-row user
 * lookup to hot list endpoints shared with the customer portal. `email` and
 * `name` are always populated, so a consumer that needs the id resolves it
 * from the email through its own cached user lookup — see
 * `useResolvedUserId`. The whole object is `null`/absent when there is no
 * actor at all, or when talking to a backend that predates this field.
 */
export interface BeUserReference {
  id: string | null;
  /** The actor as recorded. Usually an email, but can be a non-user
   * identifier such as an automation account (e.g. "system"). */
  email: string;
  name: string;
}

/** A referenced entity carrying its display name (project, deployment, ...). */
export interface BeEntityRef {
  id: string;
  name: string;
}

/** A referenced case carrying only its display number, e.g. the related case. */
export type BeParentCaseType = "case" | "incident" | "change_request" | "problem";

export interface BeCaseNumberRef {
  id: string;
  number?: string;
  /** Only populated on `parentCase`, since a case's parent can be any of these
   * task-derived record types; absent/undefined elsewhere (e.g. `relatedCase`,
   * always another case). */
  type?: BeParentCaseType | null;
}

/**
 * A service-request case whose parent points to this case or incident (the reverse
 * of `parentId`). Carries a display name in addition to {@link BeCaseNumberRef}'s
 * id/number.
 */
export interface BeLinkedServiceRequestRef {
  id: string;
  number: string;
  name: string;
}

/**
 * A change request raised from this service-request case (the reverse of the
 * change request's `caseId` link). One-to-many: promoting the same change
 * through multiple environments produces one change request per environment,
 * all pointing back at the same service request. Carries only id/number/name —
 * no state or target environment; fetch `GET /change-requests/{id}` per entry
 * for those.
 */
export interface BeLinkedChangeRequestRef {
  id: string;
  number: string;
  /** Subject, or `null` when the record has none — never `""`. */
  name: string | null;
}

/**
 * The assigned CS engineer embedded in case views. Carries `email` so the FE
 * can tell whether the case is assigned to the signed-in user (the only stable
 * identity the FE has from the JWT). `email` may be `null` depending on the data
 * source / endpoint — the GET populates it where available.
 */
export interface BeAssignedEngineerRef {
  id: string;
  name?: string;
  email?: string | null;
}

/**
 * A deployed product instance, together with the product catalogue entry it
 * was deployed from. The two are different records with different ids: `id`/
 * `displayName` describe the deployed instance (`displayName` is the product
 * name combined with its version), while `product` is the catalogue entry.
 * `id` and `displayName` are `null` when the case names a catalogue product
 * but no deployed instance of it — `product` is then the only populated
 * field.
 */
export interface BeDeployedProductRef {
  id: string | null;
  displayName: string | null;
  product?: BeEntityRef | null;
}

/**
 * Account summary embedded in the CaseView. `type` is the account/support tier
 * (e.g. "Enterprise"); it is a free-form string, not the PG `basic|enterprise`
 * enum, because ServiceNow-sourced cases pass their support tier through as-is.
 */
export interface BeCaseAccountRef {
  id: string;
  name?: string;
  type?: string;
  /** The account's assigned CRE (customer reliability engineering) team, when set (ServiceNow only). */
  creTeam?: BeEntityRef | null;
  /** The account's assigned SRE (site reliability engineering) team, when set (ServiceNow only). */
  sreTeam?: BeEntityRef | null;
}

/**
 * `GET /cases/{id}` response — the rich CaseView. Unlike {@link BeCase} (the
 * flat create/legacy shape), it embeds the related entities as objects, so the
 * UI gets account / project / deployment / deployed-product / reporter names
 * without any extra lookups.
 */
export interface BeCaseView {
  id: string;
  number?: string;
  /** Project-scoped WSO2 case reference (customer portal calls this the same). */
  internalId?: string;
  subject?: string;
  description?: string;
  /** Free-form display string from the backend, e.g. "Critical (P1)", "Low (P4)". */
  severity?: string;
  issueType?: BeCaseIssueType;
  type?: BeCaseType | null;
  /** Engagement type; only meaningful for `engagement` cases. Null otherwise. */
  engagementType?: string | null;
  state?: BeCaseState;
  /** Work sub-state; only meaningful while `state` is `work_in_progress`. */
  workState?: BeCaseWorkState | null;
  /**
   * States this case may transition into next. For a closed case, `reopened`
   * appearing here is not a real reopen (the data source has no such
   * transition) — it signals that a new case may still be created as related
   * to this one, within its 60-day window.
   */
  nextStates?: BeCaseState[];
  /** The case this one was created as related to, when any. */
  relatedCase?: BeCaseNumberRef | null;
  /** Canonical reference to the case creator. `id` is always null here — the
   * data source doesn't resolve the reporter to a user record on this view.
   * See {@link BeUserReference}. */
  createdBy?: BeUserReference | null;
  /** The CS engineer the case is assigned to, or null when unassigned. See
   * {@link BeUserReference}. */
  assignedEngineer?: BeUserReference | null;
  /**
   * The CS engineer who acknowledged the case — a first-write-wins claim that
   * someone has seen it and picked it up, which is **not** the same as being
   * assigned to it. `null` until someone acknowledges. The backing data source
   * clears it again when the case's type or severity changes or it is reopened,
   * so a materially changed case has to be acknowledged afresh.
   */
  acknowledgedBy?: BeAssignedEngineerRef | null;
  account?: BeCaseAccountRef;
  project?: BeEntityRef;
  /** Nullable: ServiceNow-sourced cases may have no deployment / product. */
  deployment?: BeEntityRef | null;
  /**
   * Deployed product instance named by the case, together with the product
   * catalogue entry it was deployed from. A case can name a catalogue product
   * without naming a deployed instance of it, in which case this object is
   * still returned with `id`/`displayName` null and `product` populated. See
   * {@link BeDeployedProductRef}.
   */
  deployedProduct?: BeDeployedProductRef | null;
  /** SR catalog refs (managed-cloud); null for non-catalog cases. */
  catalog?: BeEntityRef | null;
  catalogItem?: BeEntityRef | null;
  /** Assigned team and linked chat conversation; null when not set. */
  assignedTeam?: BeEntityRef | null;
  conversation?: BeEntityRef | null;
  createdOn?: string;
  updatedOn?: string;
  closedOn?: string | null;
  /** Timestamp when the case was resolved. Populated for resolved/closed cases; null otherwise. */
  resolvedOn?: string | null;
  /** Resolution code from a prior close/propose-solution. Populated for resolved/closed cases; null otherwise. */
  resolutionCode?: BeCaseResolutionCode | null;
  /** Root-cause category from a prior close/propose-solution. Populated for resolved/closed cases; null otherwise. */
  cause?: BeCaseCause | null;
  /** Free-text resolution/close notes from a prior close/propose-solution. Populated for resolved/closed cases; null otherwise. */
  resolutionNotes?: string | null;
  /**
   * Service-request cases whose parent points to this case. Populated on every
   * case detail response, not just high-severity cases.
   */
  linkedServiceRequests?: BeLinkedServiceRequestRef[] | null;
  /**
   * Change requests raised from this case. Only service-request cases carry
   * these; absent/null/empty otherwise. See {@link BeLinkedChangeRequestRef}.
   */
  linkedChangeRequests?: BeLinkedChangeRequestRef[] | null;
  /**
   * The case, incident, change request, or problem this case is linked to as
   * its parent (the hierarchical major-case/child-case relationship, set via
   * the PATCH `parentId` field). Null/absent when not linked.
   */
  parentCase?: BeCaseNumberRef | null;
  /**
   * Users on the case watch list (ServiceNow only). Null/absent when not set
   * or not supported by the current data source.
   */
  watchList?: BeWatchListUser[] | null;
  /**
   * Where the case sits in the backing data source's staged auto-closure
   * sequence. Read-only — see {@link BeCaseAutoclosureStep}.
   */
  autoclosureStep?: BeCaseAutoclosureStep | null;
  /**
   * When the auto-closure sequence next advances — e.g. the "eligible again
   * after" date for a held case (ServiceNow only). Read-only.
   */
  autoclosureStateTime?: string | null;
  /**
   * Internal-only best-case fix estimate, as a date-only "YYYY-MM-DD"
   * string. Settable via `PATCH /cases/{id}` (`bestCaseFixEta`). Never
   * surfaced to the customer.
   */
  bestCaseFixEta?: string | null;
  /**
   * Internal-only most-likely fix estimate, as a date-only "YYYY-MM-DD"
   * string. Settable via `PATCH /cases/{id}` (`mostLikelyFixEta`). Never
   * surfaced to the customer.
   */
  mostLikelyFixEta?: string | null;
  /**
   * Internal-only worst-case fix estimate, as a date-only "YYYY-MM-DD"
   * string. Settable via `PATCH /cases/{id}` (`worstCaseFixEta`). Never
   * surfaced to the customer.
   */
  worstCaseFixEta?: string | null;
  /** Free-text labels attached to the case. Null/absent when none are set. */
  tags?: BeTag[] | null;
}

/** A free-text tag attached to a case (`GET /cases/{id}`, `POST /cases/{id}/tags`). */
export interface BeTag {
  id: string;
  label: string;
  /** Display color for the tag, if one is set. Null when not set. */
  color?: string | null;
}

/** `POST /cases/{id}/tags` request body. */
export interface BeAddCaseTagPayload {
  label: string;
}

/** `POST /tags/search` request body. */
export interface BeSearchTagsPayload {
  filters?: {
    /** Partial, case-insensitive match against tag labels. Empty returns all known tags. */
    searchQuery?: string;
  };
  /** Maximum number of tags to return; defaults to 20 upstream, capped at 100. */
  limit?: number;
}

/** `POST /tags/search` response: existing free-text tag labels matching the query. */
export interface BeSearchTagsResponse {
  tags?: BeTag[];
  total?: number;
}

export interface BeCaseCreatePayload {
  /** Standard support case. */
  type: "case";
  projectId: string;
  deploymentId: string;
  deployedProductId: string;
  subject: string;
  description: string;
  severity: BeCaseSeverity;
  issueType: BeCaseIssueType;
  /**
   * UUID of the closed case this one is related to. The data source only
   * accepts this for a case closed within the last 60 days — otherwise it
   * rejects the create with a "related case too old" error.
   */
  relatedCaseId?: string;
  /** Optional supporting files (raw base64), like the customer portal. */
  attachments?: BeCaseAttachmentPayload[];
}

/** A single answered catalog-item variable in a service-request create. */
export interface BeCaseVariable {
  /** Variable (question) id, as returned by the catalog-item variables endpoint. */
  id: string;
  /** The engineer's answer for this variable. */
  value: string;
}

/**
 * Catalog-based service request (`type: "service_request"`). ServiceNow-only:
 * the catalog/catalog-item come from `POST /catalogs/search` (filtered by the
 * deployed product) and the variables from the catalog-item variables endpoint.
 */
export interface BeServiceRequestCreatePayload {
  type: "service_request";
  projectId: string;
  deploymentId: string;
  deployedProductId: string;
  catalogId: string;
  catalogItemId: string;
  variables: BeCaseVariable[];
  /**
   * UUID of the case this service request is linked to as its parent, when
   * created from a case's "Create service request" action. Mirrors
   * {@link BeCaseCreatePayload.relatedCaseId}, but for a service request the
   * link is to the (typically still-open) originating case rather than a
   * closed case within a reopen window.
   */
  relatedCaseId?: string;
}

/**
 * An inline attachment in a security-report create. Note `file` is **raw**
 * base64 (the file's bytes), NOT a `data:` URI — the SRA create path passes it
 * straight through. (The separate post-case attachment endpoint, by contrast,
 * requires a data URI.)
 */
export interface BeCaseAttachmentPayload {
  /** File name including extension. */
  name: string;
  /** Base64-encoded file content (raw base64, no `data:` prefix). */
  file: string;
}

/**
 * Security report analysis (`type: "security_report_analysis"`). ServiceNow-only;
 * requires a subject and description. Attachments are optional here — the case
 * is created first, then attachments upload separately via the post-case
 * attachment endpoint (see `CreateSecurityReportPage.tsx`), so a failed upload
 * never blocks or masks a successful report creation.
 */
export interface BeSecurityReportCreatePayload {
  type: "security_report_analysis";
  projectId: string;
  deploymentId: string;
  deployedProductId: string;
  subject: string;
  description: string;
  attachments?: BeCaseAttachmentPayload[];
}

/**
 * Engagement (`type: "engagement"`). ServiceNow-only; no severity/issueType —
 * engagements aren't triaged like support cases.
 */
export interface BeEngagementCreatePayload {
  type: "engagement";
  projectId: string;
  deploymentId: string;
  deployedProductId: string;
  subject: string;
  description: string;
  engagementType: BeEngagementType;
}

/**
 * Any body accepted by `POST /cases`: a standard support case, a catalog
 * service request, a security report analysis, or an engagement.
 */
export type BeCaseCreateBody =
  | BeCaseCreatePayload
  | BeServiceRequestCreatePayload
  | BeSecurityReportCreatePayload
  | BeEngagementCreatePayload;

/** The case summary embedded in the `POST /cases` success envelope. */
export interface BeCreatedCase {
  id: string;
  internalId?: string;
  number?: string;
  createdBy?: string;
  createdOn?: string;
  state?: string;
}

/** `POST /cases` response: a message plus the created case. */
export interface BeCaseCreateResponse {
  message?: string;
  case: BeCreatedCase;
}

// ---------------------------------------------------------------------------
// Service catalogs (ServiceNow only) — drive service-request creation
// ---------------------------------------------------------------------------

/** A catalog item (request form) within a catalog. */
export interface BeCatalogItemRef {
  id: string;
  name?: string;
}

/** A service catalog and the catalog items it offers. */
export interface BeCatalogRef {
  id: string;
  name?: string;
  catalogItems?: BeCatalogItemRef[];
}

/** Request body for `POST /catalogs/search`. */
export interface BeSearchCatalogsPayload {
  /** Deployed product to scope catalogs to (required). */
  deployedProductId: string;
  pagination?: { offset?: number; limit?: number };
}

/** `POST /catalogs/search` response. */
export interface BeSearchCatalogsResponse {
  catalogs?: BeCatalogRef[];
  total?: number;
  limit?: number;
  offset?: number;
}

/**
 * A catalog-item variable (form field). The contract carries the question
 * text, display order, and a free-form `type` hint, but no choice/option list
 * or mandatory flag — so the portal renders every variable as a text field.
 */
export interface BeCatalogItemVariable {
  id: string;
  questionText?: string;
  order?: number;
  type?: string;
}

/** `GET /catalogs/{catalogId}/items/{catalogItemId}/variables` response. */
export interface BeGetCatalogItemVariablesResponse {
  variables?: BeCatalogItemVariable[];
}

/**
 * Fields never allowed alongside another `PATCH /cases/{id}` variant — the
 * exactly-one-field contract. Every discriminated-union member below spreads
 * this (minus its own field) so a new variant only has to add its field name
 * here once, instead of updating every sibling variant by hand.
 */
interface BeCaseUpdateNever {
  state?: never;
  severity?: never;
  workState?: never;
  assigneeEmail?: never;
  watchList?: never;
  parentId?: never;
  subject?: never;
  description?: never;
  deploymentId?: never;
  deployedProductId?: never;
  relatedCaseId?: never;
  autocloseHoldUntil?: never;
  bestCaseFixEta?: never;
  mostLikelyFixEta?: never;
  worstCaseFixEta?: never;
  addPublicComment?: never;
  product?: never;
  publicTicket?: never;
  acknowledge?: never;
}

/**
 * Request body for `PATCH /cases/{id}` (mirrors the entity `UpdateCaseRequest`).
 * **Exactly one** of `state` / `severity` / `workState` / `assigneeEmail` /
 * `watchList` / `parentId` / `subject` / `description` / `deploymentId` /
 * `deployedProductId` / `relatedCaseId` / `autocloseHoldUntil` / the combined
 * fix-ETA variant (below) is sent per call — the backend rejects zero or more
 * than one. Encoded as a discriminated union (each variant carries every
 * other field as `never`, via {@link BeCaseUpdateNever}) so the
 * exactly-one-field contract is enforced at compile time, not just in docs.
 * `assigneeEmail`, `watchList`, `parentId`, and `autocloseHoldUntil` are
 * supported **only** for the ServiceNow data source. `workState` is only
 * accepted while the case is `work_in_progress`.
 */
export type BeCaseUpdatePayload =
  | (Omit<BeCaseUpdateNever, "state"> & {
      state: BeCaseState;
      /** Post Resolution Activity — only meaningful (and only accepted by the backend) alongside `state: "closed"` or `"solution_proposed"`. */
      resolutionCode?: BeCaseResolutionCode;
      cause?: BeCaseCause;
      closeNotes?: string;
    })
  | (Omit<BeCaseUpdateNever, "severity"> & { severity: BeCaseSeverity })
  /** Work sub-state toggle (`ongoing` / `paused`) for an in-progress case. */
  | (Omit<BeCaseUpdateNever, "workState"> & { workState: BeCaseWorkState })
  /** Email of the engineer to assign (ServiceNow only). */
  | (Omit<BeCaseUpdateNever, "assigneeEmail"> & { assigneeEmail: string })
  /** Full replacement watch list as emails (ServiceNow only). */
  | (Omit<BeCaseUpdateNever, "watchList"> & { watchList: string[] })
  /**
   * UUID of another case, incident, change request, or problem to link this
   * case to as its parent (ServiceNow only, the hierarchical
   * major-case/child-case relationship).
   */
  | (Omit<BeCaseUpdateNever, "parentId"> & { parentId: string })
  /** New subject/title for the case. */
  | (Omit<BeCaseUpdateNever, "subject"> & { subject: string })
  /** New description for the case. */
  | (Omit<BeCaseUpdateNever, "description"> & { description: string })
  /** UUID of the deployment to associate with this case, replacing the existing one. */
  | (Omit<BeCaseUpdateNever, "deploymentId"> & { deploymentId: string })
  /** UUID of the deployed product to associate with this case, replacing the existing one. */
  | (Omit<BeCaseUpdateNever, "deployedProductId"> & { deployedProductId: string })
  /** UUID of another case to cross-link to this one as a related case (looser than `parentId`; ServiceNow only). */
  | (Omit<BeCaseUpdateNever, "relatedCaseId"> & { relatedCaseId: string })
  /**
   * Acknowledge the case as the signed-in engineer. Typed as the literal `true`
   * because that is the only accepted value: acknowledgement is first-write-wins
   * and there is no way to remove one. Acknowledging an already-acknowledged
   * case succeeds and changes nothing, and the response then carries
   * `alreadyAcknowledged: true` with whoever claimed it first.
   */
  | (Omit<BeCaseUpdateNever, "acknowledge"> & { acknowledge: true })
  /**
   * Places the case on hold in the backing data source's staged auto-closure
   * sequence until this ISO date-time (ServiceNow only). The raw
   * `autoclosureStep` is not directly settable.
   */
  | (Omit<BeCaseUpdateNever, "autocloseHoldUntil"> & { autocloseHoldUntil: string })
  /**
   * Sets any combination of the three internal-only fix-ETA estimates
   * (date-only "YYYY-MM-DD") in one call — unlike the other variants, this is
   * a **combined** field, so at least one of the three must be present, but
   * all three are independently optional. When `addPublicComment` is true,
   * the backend also posts a customer-visible comment built from `product` /
   * `publicTicket` / the estimate(s) above to the case's comment thread; that
   * mode requires at least one estimate plus non-empty `product` and
   * `publicTicket`.
   */
  | (Omit<
      BeCaseUpdateNever,
      | "bestCaseFixEta"
      | "mostLikelyFixEta"
      | "worstCaseFixEta"
      | "addPublicComment"
      | "product"
      | "publicTicket"
    > & {
      bestCaseFixEta?: string;
      mostLikelyFixEta?: string;
      worstCaseFixEta?: string;
      addPublicComment?: boolean;
      product?: string;
      publicTicket?: string;
    });

/** A user in the case watch list, as echoed by `PATCH /cases/{id}`. */
export interface BeWatchListUser {
  id: string;
  userName: string;
  name?: string;
  email?: string;
  /** Canonical reference to this watcher. `id` is always null here. See
   * {@link BeUserReference}. */
  user?: BeUserReference | null;
}

/** The mutated case fields echoed by `PATCH /cases/{id}`. */
export interface BeUpdatedCase {
  id: string;
  updatedOn?: string;
  updatedBy?: string;
  state?: BeCaseState;
  severity?: BeCaseSeverity;
  workState?: BeCaseWorkState | null;
  watchList?: BeWatchListUser[];
  assignedTo?: BeEntityRef | null;
  /** Canonical reference to the newly assigned engineer, `id` populated. Key
   * is omitted entirely (not just null) when this update didn't set an
   * assignee — the FE currently ignores this echoed body and refetches the
   * case detail instead, but the field is typed here to match the contract.
   * See {@link BeUserReference}. */
  assignedToUser?: BeUserReference | null;
  /** Present when the update set `parentId` — the record this case is now linked to as its parent. */
  parentCase?: BeCaseNumberRef | null;
  /** Echoes the updated internal-only best-case fix estimate. Present when the update set `bestCaseFixEta`. */
  bestCaseFixEta?: string | null;
  /** Echoes the updated internal-only most-likely fix estimate. Present when the update set `mostLikelyFixEta`. */
  mostLikelyFixEta?: string | null;
  /** Echoes the updated internal-only worst-case fix estimate. Present when the update set `worstCaseFixEta`. */
  worstCaseFixEta?: string | null;
  /** Human-readable case number. Present when the update set `acknowledge`. */
  number?: string;
  /**
   * True when the case already had an acknowledger, so the request changed
   * nothing and `acknowledgedBy` names whoever claimed it first. Present when
   * the update set `acknowledge`.
   */
  alreadyAcknowledged?: boolean;
  /** Whoever now holds the acknowledgement. Present when the update set `acknowledge`. */
  acknowledgedBy?: BeAssignedEngineerRef | null;
}

/** `PATCH /cases/{id}` response: a message plus the mutated case fields. */
export interface BeUpdateCaseResponse {
  message?: string;
  case: BeUpdatedCase;
}

export type BeEngagementType =
  | "migration"
  | "consultancy"
  | "new_feature_improvement"
  | "follow_up"
  | "onboarding";

/**
 * `field` enum accepted by {@link BeCaseFieldFilter}. Mirrors the
 * entity-service's `caseFilterFieldSet` (see `case_filters.go`) exactly —
 * anything else is rejected by the backend.
 */
export type BeCaseFieldFilterField =
  | "type"
  | "state"
  | "severity"
  | "engagementType"
  | "issueType"
  | "workState"
  | "tag"
  | "projectId"
  | "deploymentId"
  | "assignedUserId"
  | "createdBy"
  | "createdOn"
  | "updatedOn"
  | "closedOn"
  | "product"
  | "projectOnboardingStatus"
  | "projectType"
  | "integrationCsTeam"
  | "resolutionNotes"
  | "parentId"
  | "taskSLABusinessElapsedPercent"
  | "escalationLevel"
  | "escalation"
  | "number"
  | "internalId";

/**
 * `op` enum accepted by {@link BeCaseFieldFilter}, independent of `field` —
 * which ops a given field actually supports is enforced only on the backend
 * (see `ParseCaseFieldFilters` in `case_filters.go`), not narrowed here.
 */
export type BeCaseFieldFilterOp =
  | "eq"
  | "in"
  | "notIn"
  | "isEmpty"
  | "isNotEmpty"
  | "gte"
  | "lte";

/**
 * One entry in `POST /cases/search`'s generic filter DSL — replaces the old
 * ~20 named filter fields (`states`, `severities`, `assignedUserIds`, ...)
 * with a single typed array. `values` is required by every op except
 * `isEmpty`/`isNotEmpty` (the backend rejects a missing/empty array
 * otherwise — see `requireCaseFilterValues`).
 *
 * Date-range bounds (`createdOn`/`updatedOn`/`closedOn`) are two separate
 * entries — one `gte`, one `lte` — instead of the old paired
 * `start`/`end` named fields; each `values` is a single RFC3339 timestamp or
 * `YYYY-MM-DD` date string.
 *
 * The old `createdByMe: true` boolean is now
 * `{ field: "createdBy", op: "eq", values: ["__current_user_email__"] }` —
 * see {@link BE_CURRENT_USER_FILTER_PLACEHOLDER}.
 */
export interface BeCaseFieldFilter {
  field: BeCaseFieldFilterField;
  op: BeCaseFieldFilterOp;
  values?: string[];
}

/**
 * The literal `values` entry a `createdBy`+`eq` filter must carry to mean
 * "the authenticated caller" — mirrors the old `createdByMe: true` request
 * field. See `currentUserFilterPlaceholder` in `case_filters.go`.
 */
export const BE_CURRENT_USER_FILTER_PLACEHOLDER = "__current_user_email__";

/**
 * Filter block for `POST /cases/search`; both fields are optional. Replaces
 * the old ~20 named filter fields with the generic `filters` array — see
 * {@link BeCaseFieldFilter}.
 */
export interface BeCaseSearchFilters {
  /** Searches across subject, number, and wso2Id (case-insensitive). */
  searchQuery?: string;
  /** The generic field/op/values filter array. Omit (or send `[]`) for an
   * unfiltered cross-project search. */
  filters?: BeCaseFieldFilter[];
}

export interface BeCaseSearchPayload {
  /** All filter fields are nested here; `sortBy`/`pagination` stay top-level. */
  filters?: BeCaseSearchFilters;
  pagination?: BePagination;
  sortBy?: {
    field?: BeCaseSortField;
    order?: "asc" | "desc";
  };
}

/**
 * Item shape returned by `POST /cases/search`. Like {@link BeCaseView} it
 * embeds the related entities, so the list gets project / deployment /
 * deployed-product names without extra lookups. (The account/customer is not
 * embedded.)
 */
export interface BeCaseSearchView {
  id: string;
  number?: string;
  /** Project-scoped WSO2 case reference (customer portal calls this the same). */
  internalId?: string;
  /**
   * Case subject. The search response now returns `subject`, matching the
   * `GET /cases/{id}` CaseView (the earlier `title` naming inconsistency is gone).
   */
  subject?: string;
  description?: string;
  /** Free-form display string from the backend, e.g. "Low (P4)", "Critical (P1)". */
  severity?: string;
  issueType?: BeCaseIssueType;
  /** Case type (e.g. `case`, `service_request`). */
  type?: BeCaseType;
  state?: BeCaseState;
  /** Work sub-state; only meaningful while `state` is `work_in_progress`. */
  workState?: BeCaseWorkState | null;
  /** The CS engineer the case is assigned to, or null when unassigned. See
   * {@link BeUserReference}. */
  assignedEngineer?: BeUserReference | null;
  createdOn?: string;
  /** Often absent on the search view (unlike the GET view); tolerate it missing. */
  updatedOn?: string;
  /** Canonical reference to the case creator. `id` is always null here. See
   * {@link BeUserReference}. */
  createdBy?: BeUserReference | null;
  project?: BeEntityRef;
  /** Nullable: ServiceNow-sourced cases may have no deployment / product. */
  deployment?: BeEntityRef | null;
  /** Embedded as `{ id, name }` (name already includes the version), not the
   * `displayName`-shaped ref the GET view uses. */
  deployedProduct?: BeEntityRef | null;
  /** The product itself (distinct from `deployedProduct`); used to populate the
   * list Product column when no deployed product is set (e.g. cloud cases). */
  product?: BeEntityRef | null;
}

export interface BeCaseSearchResponse extends BeSearchResponseBase {
  cases: BeCaseSearchView[];
}

// ---------------------------------------------------------------------------
// Case comments
// ---------------------------------------------------------------------------

export type BeCaseCommentType = "work_note" | "comment" | "activity";

/** Author block embedded in a comment; the BE hydrates it from the user store. */
export interface BeCaseCommentAuthor {
  id: string;
  firstName?: string;
  lastName?: string;
  fullName?: string;
}

export interface BeCaseComment {
  id: string;
  caseId: string;
  type: BeCaseCommentType;
  /** Rich-text HTML body (sanitised at render time). */
  content: string;
  createdBy: BeCaseCommentAuthor;
  createdOn: string;
}

/** Comment types a client may create. `activity` is system-generated only. */
export type BeCreatableCommentType = Exclude<BeCaseCommentType, "activity">;

export interface BeCaseCommentCreatePayload {
  type: BeCreatableCommentType;
  /** Rich-text HTML body. */
  content: string;
}

export interface BeCaseCommentSearchPayload {
  pagination?: BePagination;
}

export interface BeCaseCommentSearchResponse extends BeSearchResponseBase {
  comments: BeCaseComment[];
}

/**
 * Comment shape returned by the generic `POST /comments/search`, which — since
 * the entity-service refactor — backs BOTH `/cases/{id}/comments/search` and
 * `/conversations/{id}/messages`. It reuses `referenceId` (the case or
 * conversation id) rather than `caseId`; the backend normalizes `type` to the
 * `BeCaseCommentType` enum (unknown SN types default to `comment`).
 *
 * `createdBy` carries the canonical {@link BeUserReference} shape; `null`
 * when the comment has no resolvable author.
 */
export interface BeComment {
  id: string;
  /** Parent reference id — the case id or conversation id per the endpoint. */
  referenceId?: string;
  /** Rich-text HTML (case comment) or Markdown (Novera chat) body. */
  content: string;
  /** Normalized comment type; `string` (not the enum) to tolerate new values. */
  type: string;
  createdOn: string;
  createdBy: BeUserReference | null;
}

export interface BeCommentSearchResponse extends BeSearchResponseBase {
  /** Optional: the backend may omit the array on an empty result. */
  comments?: BeComment[];
}

// ---------------------------------------------------------------------------
// Conversations (Novera chat sessions)
// ---------------------------------------------------------------------------

export type BeConversationState =
  | "ACTIVE"
  | "RESOLVED"
  | "CONVERTED"
  | "ABANDONED"
  | "CLOSED";

/**
 * A chat session as returned by `POST /conversations/search` — the ServiceNow
 * "conversation" record a case may originate from. `id`/`number`/`state` are
 * nullable on the wire (a conversation that never resolved to a real SN
 * record can have gaps); `case` is null for a chat that never became a case.
 *
 * `createdBy` carries the canonical {@link BeUserReference} shape, same as
 * case `createdBy`. Known limitation specific to conversations: the upstream
 * data only carries a single identity string for the initiator, so only ONE
 * of `email`/`name` is usually populated (never both) — unlike cases, which
 * usually have both. `null` when the initiator has no resolvable identity.
 */
export interface BeConversationView {
  id: string | null;
  number: string | null;
  initialMessage: string | null;
  messageCount: number;
  project: BeEntityRef | null;
  case: BeEntityRef | null;
  state: BeConversationState | null;
  createdOn: string;
  createdBy: BeUserReference | null;
}

export interface BeSearchConversationsFilters {
  projectIds?: string[];
  states?: BeConversationState[];
  /** Free-text search across the conversation (matches the same fields the
   * data source indexes for it — number, initiator, initial message). */
  searchQuery?: string;
  /** A single conversation number (e.g. "CHAT0000012345"), matched exactly —
   * a first-class indexed filter rather than the free-text `searchQuery`
   * scan. See `classifyConversationQuery`. */
  number?: string;
  /** When `true`, scopes results to conversations initiated by the
   * signed-in user. */
  createdByMe?: boolean;
}

export interface BeSearchConversationsPayload {
  filters?: BeSearchConversationsFilters;
  sortBy?: { field: "createdOn" | "updatedOn"; order: "asc" | "desc" };
  pagination?: BePagination;
}

/** No `hasMore` on this response (unlike {@link BeSearchResponseBase}) —
 * `total` is the only continuation signal the entity service gives here. */
export interface BeSearchConversationsResponse {
  conversations: BeConversationView[];
  total: number;
  limit: number;
  offset: number;
}

// ---------------------------------------------------------------------------
// Case activities (unified comment / attachment / field-change stream)
// ---------------------------------------------------------------------------

/** One field changed within a single audited save-transaction. */
export interface BeFieldChange {
  /** Wire field name (e.g. `state`, `priority`, `assignedEngineer`). */
  field: string;
  /** Human-readable label for the field (e.g. "State", "Severity"). */
  fieldLabel: string;
  /** Absent/empty when the field was previously unset. */
  previousValue?: string;
  /** Absent/empty when the field was cleared. */
  newValue?: string;
}

export type BeCaseActivityType = "comment" | "attachment" | "field_change";

/**
 * One entry from `POST /cases/{id}/activities/search`. Shared fields are
 * present on every entry regardless of `type`; `changes` is populated only
 * for `type === "field_change"`. This endpoint intentionally excludes work
 * notes — the comments/work-notes feed continues to read from
 * `/cases/{id}/comments/search` (see {@link BeComment}).
 */
export interface BeCaseActivityEntry {
  id: string;
  type: BeCaseActivityType;
  content?: string;
  createdOn: string;
  createdByFirstName?: string;
  createdByLastName?: string;
  /** Canonical reference to the actor, `id` always null here. Its `name`
   * field is the actor's full display name; `createdByFirstName`/
   * `createdByLastName` remain separate because a full name cannot be split
   * back into them reliably. See {@link BeUserReference}. */
  createdBy?: BeUserReference | null;
  /** Only present on `type === "field_change"` entries. */
  changes?: BeFieldChange[];
}

export interface BeCaseActivitiesSearchPayload {
  pagination?: BePagination;
  /** Whether the response should include `field_change` entries. */
  includeFieldChanges?: boolean;
}

export interface BeCaseActivitiesSearchResponse extends BeSearchResponseBase {
  activity?: BeCaseActivityEntry[];
}

// ---------------------------------------------------------------------------
// Attachments
// ---------------------------------------------------------------------------

/**
 * Entity an attachment is linked to. Attachments are no longer case-scoped:
 * the same endpoints serve change requests, deployments, and conversations.
 */
export type BeReferenceType =
  | "case"
  | "conversation"
  | "change_request"
  | "deployment"
  | "incident";

/** A attachment as returned by `POST /attachments/search`. */
export interface BeAttachment {
  id: string;
  referenceId: string;
  referenceType: BeReferenceType;
  name: string;
  /** MIME type (e.g. image/png, application/pdf). */
  type: string;
  sizeBytes: number;
  description?: string | null;
  /** Uploader, in the canonical {@link BeUserReference} shape. Null when the
   * uploader isn't resolvable. */
  createdBy: BeUserReference | null;
  createdOn: string;
  downloadUrl?: string | null;
  previewUrl?: string | null;
}

/**
 * Search payload for `POST /attachments/search`. `referenceId` + `referenceType`
 * scope the search to one entity (both required by the BE).
 */
export interface BeAttachmentSearchPayload {
  referenceId: string;
  referenceType: BeReferenceType;
  pagination?: BePagination;
}

export interface BeAttachmentSearchResponse extends BeSearchResponseBase {
  attachments: BeAttachment[];
}

/**
 * Upload payload for `POST /attachments`. `referenceId` + `referenceType` link
 * the file to its owning entity; `file` is a base64 data URI (e.g.
 * `data:image/png;base64,...`); the BE caps the decoded size at 10 MB.
 */
export interface BeAttachmentCreatePayload {
  referenceId: string;
  referenceType: BeReferenceType;
  name: string;
  type: string;
  file: string;
  description?: string | null;
}

/** Thin ack returned by `POST /attachments`. */
export interface BeAttachmentDetail {
  id: string;
  sizeBytes: number;
  createdOn: string;
  createdBy: string;
  downloadUrl: string;
}

export interface BeAttachmentCreateResponse {
  message?: string;
  attachment?: BeAttachmentDetail;
}

/** Ack returned by `DELETE /attachments/{id}`. */
export interface BeDeleteAttachmentResponse {
  message?: string;
}

// ---------------------------------------------------------------------------
// Users
// ---------------------------------------------------------------------------

export type BeUserType = "internal" | "customer";

export interface BeUser {
  id?: string;
  userName?: string;
  firstName?: string;
  lastName?: string;
  email?: string;
  phone?: string;
  timezone?: string;
  userType?: BeUserType;
  createdAt?: string;
  updatedAt?: string;
}

export interface BeUserSearchFilters {
  /** Case-insensitive match against username and email. */
  searchQuery?: string;
  /** Filter by one or more role keys, as returned by `/roles/search`. Backing
   * data source only. */
  roleIds?: string[];
  /** Restrict to specific users. Intersects with the other filters and lifts
   * the active-only default. Backing data source only. */
  userIds?: string[];
  /** Restrict to members of these groups (group UUIDs). Backing data source
   * only. */
  groupIds?: string[];
  /** Restrict to members of these teams, by team registry key. Backing data
   * source only. */
  teamIds?: string[];
  /** Exact match. */
  userNames?: string[];
  /** Exact match. */
  emails?: string[];
  /** When set, restricts results to active or inactive users. Backing data
   * source only. */
  active?: boolean | null;
}

export interface BeUserSearchPayload {
  pagination?: BePagination;
  filters?: BeUserSearchFilters;
}

export interface BeUserSearchResponse extends BeSearchResponseBase {
  users: BeUser[];
}

// ---------------------------------------------------------------------------
// Accounts
// ---------------------------------------------------------------------------

export type BeAccountTier = "basic" | "enterprise";

export interface BeAccount {
  id: string;
  sfId?: string;
  name?: string;
  tier?: BeAccountTier;
  region?: string;
  activationDate?: string;
  deactivationDate?: string;
  ownerId?: string;
  technicalOwnerId?: string;
  agentEnabled?: boolean;
  kbReferencesEnabled?: boolean;
  createdAt?: string;
  updatedAt?: string;
}

export interface BeAccountSearchPayload {
  pagination?: BePagination;
  searchQuery?: string;
}

export interface BeAccountSearchResponse extends BeSearchResponseBase {
  accounts: BeAccount[];
}

// ---------------------------------------------------------------------------
// Projects
// ---------------------------------------------------------------------------

export type BeSubscriptionType =
  | "development_support"
  | "managed_cloud_subscription"
  | "evaluation_subscription"
  | "subscription"
  | "cloud_evaluation_support"
  | "internal"
  | "platformer_subscription"
  | "cloud_support"
  | "professional_services";

export interface BeProject {
  id: string;
  /** Nested on the wire (ServiceNow data source); absent when the project has no linked account. */
  account?: { id: string; name: string };
  sfId?: string;
  name?: string;
  projectKey?: string;
  subscriptionType?: BeSubscriptionType;
  startDate?: string;
  endDate?: string;
  createdAt?: string;
  updatedAt?: string;
}

export interface BeProjectSearchPayload {
  pagination?: BePagination;
  searchQuery?: string;
  /** Filter to projects belonging to this account (ServiceNow data source only). */
  accountId?: string;
}

export interface BeProjectSearchResponse extends BeSearchResponseBase {
  projects: BeProject[];
}

/**
 * A contact's attributes for one project, from `POST /projects/{id}/contacts/search`
 * (also the shape of `GET /projects/{id}/contacts/{contactId}`).
 */
export interface BeProjectContact {
  /**
   * The contact's user id, for linking the row to that user's profile.
   * Absent when the row has no contact record linked, or when the backing
   * instance predates the field — render the row unlinked rather than
   * treating it as an error.
   */
  id?: string;
  name?: string;
  email?: string;
  registrationState?: string;
  notificationsEnabled?: boolean;
  roles?: string[];
  /**
   * Whether a contact record is linked to this row at all. False is the same
   * fault an absent `id` signals, restated as an explicit boolean.
   */
  customerContactPresent?: boolean;
  /**
   * Whether this row would actually grant its person visibility into this
   * project's cases. Mirrors `customerContactPresent` directly — a row can
   * be listed here without granting access.
   */
  grantsCaseAccess?: boolean;
}

export interface BeProjectContactSearchPayload {
  filters?: { searchQuery?: string };
  pagination?: BePagination;
}

export interface BeProjectContactSearchResponse {
  contacts: BeProjectContact[];
  offset: number;
  limit: number;
  total: number;
}

// ---------------------------------------------------------------------------
// Deployments
// ---------------------------------------------------------------------------

export type BeDeploymentType =
  | "primary_production"
  | "staging"
  | "qa"
  | "stress"
  | "uat"
  | "development";

export interface BeDeployment {
  id: string;
  projectId?: string;
  name?: string;
  type?: BeDeploymentType;
  description?: string;
  createdBy?: string;
  createdOn?: string;
  updatedOn?: string;
}

/**
 * Detail-field update for `PATCH /deployments/{id}`. At least one field must be
 * present (BE rejects an empty body). `description: null` clears the value.
 * `active` is forbidden on this variant — use `BeDeploymentDeactivatePayload`.
 *
 * Per PR #957: the BE now accepts the string `type` enum directly, so
 * `typeKey` (the old integer) is gone.
 */
export type BeDeploymentDetailUpdatePayload = {
  name?: string;
  type?: BeDeploymentType;
  description?: string | null;
  active?: never;
} & ({ name: string } | { type: BeDeploymentType } | { description: string | null });

/** Deactivation for `PATCH /deployments/{id}`: `active` must be `false`, alone. */
export interface BeDeploymentDeactivatePayload {
  active: false;
  name?: never;
  type?: never;
  description?: never;
}

export type BeDeploymentUpdatePayload =
  | BeDeploymentDetailUpdatePayload
  | BeDeploymentDeactivatePayload;

export interface BeDeploymentUpdateResponse {
  message: string;
  deployment: {
    id: string;
    updatedOn: string;
    updatedBy: string;
  };
}

/**
 * Request body for `POST /deployments`. All four fields are required per the
 * BE contract (PR #957: `type` is the string enum, `typeKey` integer is gone).
 */
export interface BeDeploymentCreatePayload {
  projectId: string;
  name: string;
  type: BeDeploymentType;
  description: string;
}

export interface BeDeploymentCreateResponse {
  message: string;
  deployment: {
    id: string;
    projectId: string;
    name: string;
    type: BeDeploymentType;
    description: string;
    createdOn: string;
    createdBy: string;
  };
}

export interface BeDeploymentSearchPayload {
  pagination?: BePagination;
  searchQuery?: string;
  projectIds?: string[];
  deploymentTypes?: BeDeploymentType[];
}

export interface BeDeploymentSearchResponse extends BeSearchResponseBase {
  deployments: BeDeployment[];
}

// ---------------------------------------------------------------------------
// Products + product versions
// ---------------------------------------------------------------------------

export type BeProductClass = "software" | "service";
export type BeProductSupportStatus =
  | "available"
  | "extended"
  | "deprecated"
  | "discontinued";

export interface BeProduct {
  id: string;
  name?: string;
  class?: BeProductClass;
  createdAt?: string;
  updatedAt?: string;
}

export interface BeProductSearchPayload {
  pagination?: BePagination;
  searchQuery?: string;
}

export interface BeProductSearchResponse extends BeSearchResponseBase {
  products: BeProduct[];
}

export interface BeProductVersion {
  id: string;
  productId?: string;
  version?: string;
  currentSupportStatus?: BeProductSupportStatus;
  releaseDate?: string;
  supportEolDate?: string;
  earliestPossibleSupportEolDate?: string;
  createdAt?: string;
  updatedAt?: string;
}

export interface BeProductVersionSearchPayload {
  pagination?: BePagination;
  searchQuery?: string;
}

export interface BeProductVersionSearchResponse extends BeSearchResponseBase {
  productVersions: BeProductVersion[];
}

// ---------------------------------------------------------------------------
// Deployed products (deployment ↔ product link)
// ---------------------------------------------------------------------------

/** Product version as embedded in a deployed-product record. */
export interface BeDeployedProductVersion {
  id: string;
  name: string;
  releasedDate?: string | null;
  supportEoLDate?: string | null;
}

export interface BeDeployedProduct {
  id: string;
  deployment?: BeEntityRef;
  product?: BeEntityRef;
  version?: BeDeployedProductVersion | null;
  // SN-only sizing fields; the entity service returns them as strings (and
  // always null for the Postgres data source).
  cores?: string | null;
  tps?: string | null;
  category?: string | null;
  createdOn?: string;
  updatedOn?: string;
}

export interface BeDeployedProductSearchPayload {
  pagination?: BePagination;
}

export interface BeDeployedProductSearchResponse extends BeSearchResponseBase {
  deployedProducts: BeDeployedProduct[];
}

/**
 * Request body for `POST /deployments/{id}/products`. All three ID fields are
 * required per the OpenAPI spec. Sizing fields are optional.
 */
export interface BeDeployedProductCreatePayload {
  projectId: string;
  productId: string;
  versionId: string;
  cores?: number;
  tps?: number;
  description?: string;
}

export interface BeDeployedProductCreateResponse {
  message?: string;
  deployedProduct?: {
    id: string;
    createdOn: string;
    createdBy: string;
  };
}

/**
 * Detail-field update for `PATCH /deployments/{deploymentId}/products/{productId}`.
 * At least one field required. All fields nullable (clears the value).
 * `active` is forbidden on this variant -- use `BeDeployedProductDeactivatePayload`.
 */
export interface BeDeployedProductDetailUpdatePayload {
  cores?: number | null;
  tps?: number | null;
  description?: string | null;
  active?: never;
}

/** Deactivation variant: `active` must be `false`, alone. */
export interface BeDeployedProductDeactivatePayload {
  active: false;
  cores?: never;
  tps?: never;
  description?: never;
}

export type BeDeployedProductUpdatePayload =
  | BeDeployedProductDetailUpdatePayload
  | BeDeployedProductDeactivatePayload;

export interface BeDeployedProductUpdateResponse {
  message?: string;
  deployedProduct?: {
    id: string;
    updatedOn: string;
    updatedBy: string;
  };
}

// ---------------------------------------------------------------------------
// Call requests
// ---------------------------------------------------------------------------

/**
 * State of a call request. `id` may be an integer or string (per the OpenAPI
 * spec -- treat it as opaque and use `label` for display). `label` is the
 * human-readable state name.
 */
export interface BeCallRequestState {
  id: number | string;
  label?: string;
}

/**
 * The 8 state keys that `SearchCallRequestsPayload.filters.states` and
 * `UpdateCallRequestPayload.state` accept as plain string enums.
 */
export type BeCallRequestStateKey =
  | "pending_on_customer"
  | "pending_on_wso2"
  | "scheduled"
  | "customer_rejected"
  | "wso2_rejected"
  | "canceled"
  | "notes_pending"
  | "concluded";

/** A call request list/detail item returned by `POST /cases/{id}/call-requests/search`. */
export interface BeCallRequestView {
  id: string;
  number?: string;
  case?: {
    id: string;
    name?: string;
    number?: string;
  };
  reason?: string;
  preferredTimes?: string[];
  durationMin?: number;
  scheduleTime?: string;
  meetingLink?: string;
  createdOn?: string;
  updatedOn?: string;
  state?: BeCallRequestState;
  cancellationReason?: string;
  /** Agent (or team) assigned to run the call, once scheduled. */
  assignee?: string;
  /** Call notes recorded after the call concludes. */
  notes?: string;
  /** Follow-up plan recorded alongside the call notes. */
  plan?: string;
  /** Attendee list recorded alongside the call notes. */
  attendees?: string;
  /** Action items recorded alongside the call notes. */
  actionItems?: string;
  /** Actual call duration in minutes, recorded alongside the call notes. */
  actualDurationMin?: number;
}

/** `POST /cases/{id}/call-requests` request body. */
export interface BeCreateCallRequestPayload {
  reason: string;
  /** One or more preferred UTC datetimes (ISO strings). */
  utcTimes: string[];
  /** Duration of the call in minutes (min 1). */
  durationInMinutes: number;
}

/** `POST /cases/{id}/call-requests` response. */
export interface BeCreateCallRequestResponse {
  message?: string;
  callRequest?: {
    id: string;
    createdOn?: string;
    createdBy?: string;
    state?: BeCallRequestState;
  };
}

// ---------------------------------------------------------------------------
// GitHub issue creation from a case — `POST /cases/{id}/github-issues`
// (ServiceNow data source only; the BFF forwards the body opaquely to the
// entity service, which validates and routes it to the SN scoped app).
// ---------------------------------------------------------------------------

/**
 * Why the issue is being filed. Selects the label set applied on the GitHub
 * issue and collapses the three legacy "open git issue" flows into one:
 * `default` (Open Git Issue), `migration` (Open Migration Git Issue),
 * `rd_ticket` (Open R&D ticket).
 */
export type BeCaseGithubIssueReason = "default" | "migration" | "rd_ticket";

/** Explicit owner/repo, overriding the product-based routing lookup on the SN side. */
export interface BeCaseGithubIssueRepoOverride {
  owner: string;
  repo: string;
}

/** `POST /cases/{id}/github-issues` request body. */
export interface BeCreateCaseGithubIssuePayload {
  reason: BeCaseGithubIssueReason;
  title: string;
  description: string;
  /** Explicit target repo; when omitted, the SN side routes by the case's product unit. */
  repoOverride?: BeCaseGithubIssueRepoOverride;
  /** Product update level, appended to the issue body. */
  updateLevel?: string;
  /** Link to a related public-facing GitHub issue, appended to the issue body. */
  publicIssueUrl?: string;
  /** When true: adds a `regression` label and tags the case as a regression. */
  regression?: boolean;
  /** When true: appends "Hotfix Required : Yes" to the issue body. */
  hotFixRequired?: boolean;
  /** Issue-type label to apply on GitHub (e.g. "Type/Patch", "Type/Incident"). */
  issueTypeLabel?: string;
  /** Priority label, applied only when `issueTypeLabel` is "Type/Incident". */
  priorityLevel?: string;
}

/** `POST /cases/{id}/github-issues` response. */
export interface BeCreateCaseGithubIssueResponse {
  message?: string;
  issue?: {
    /** URL of the created GitHub issue. */
    url: string;
    /** GitHub issue number. */
    number: number;
    /** Repo the issue was created in. */
    repo: string;
  };
}

/** `POST /cases/{id}/call-requests/search` request body. */
export interface BeSearchCallRequestsPayload {
  filters?: {
    states?: BeCallRequestStateKey[];
  };
  pagination?: {
    offset?: number;
    /** Default 20, max 100. */
    limit?: number;
  };
}

/** `POST /cases/{id}/call-requests/search` response. */
export interface BeSearchCallRequestsResponse {
  callRequests: BeCallRequestView[];
  total: number;
  offset: number;
  limit: number;
}

/**
 * `PATCH /cases/{caseId}/call-requests/{callRequestId}` request body.
 * `state` selects the transition; the optional fields below are interpreted
 * according to the target `state`:
 * - `scheduled` (agent schedule/reschedule): `meetingDate` + `durationInMinutes`
 *   required, `assignee` optional.
 * - `wso2_rejected` (agent reject) / `canceled`: `cancellationReason` optional
 *   (used as the reject/cancel reason).
 * - `concluded` (agent send notes): `notes` required, `plan`/`attendees`/
 *   `actionItems`/`actualDurationMin` optional.
 * - `pending_on_wso2` (reschedule request back to the customer): `utcTimes` +
 *   `durationInMinutes`.
 */
export interface BeUpdateCallRequestPayload {
  state: BeCallRequestStateKey;
  /** Used as the reject/cancellation reason for `wso2_rejected`/`canceled`. */
  cancellationReason?: string;
  /** Updated preferred UTC datetimes (used when `state` is `pending_on_wso2`). */
  utcTimes?: string[];
  /** Updated duration in minutes (min 1); used for `pending_on_wso2`/`scheduled`. */
  durationInMinutes?: number;
  /** UTC datetime (ISO string) the call is scheduled for; required for `scheduled`. */
  meetingDate?: string;
  /** Agent (or team) assigned to run the call; used for `scheduled`. */
  assignee?: string;
  /** Call notes; required for `concluded`. */
  notes?: string;
  /** Follow-up plan recorded alongside the call notes; used for `concluded`. */
  plan?: string;
  /** Attendee list recorded alongside the call notes; used for `concluded`. */
  attendees?: string;
  /** Action items recorded alongside the call notes; used for `concluded`. */
  actionItems?: string;
  /** Actual call duration in minutes; used for `concluded`. */
  actualDurationMin?: number;
}

/** `PATCH /cases/{caseId}/call-requests/{callRequestId}` response. */
export interface BeUpdateCallRequestResponse {
  message?: string;
  callRequest?: {
    id: string;
    updatedOn?: string;
    updatedBy?: string;
  };
}

// ---------------------------------------------------------------------------
// Tasks — a lightweight, case-scoped to-do item (distinct from call requests
// and change requests: no multi-stage lifecycle). Only 2 real live `state`
// values are seen in this org's data (`OPEN`/`CLOSED`); `OTHER` is a genuine
// fallback for undocumented raw values and must be handled, not treated as
// unreachable. `dueDate` is sparsely populated (~13% of records) — render its
// absence as a normal, expected case rather than a broken layout.
// ---------------------------------------------------------------------------

export type BeTaskState = "OPEN" | "CLOSED" | "OTHER";

/** A task list item returned by `POST /cases/{caseId}/tasks/search`. */
export interface BeTaskSummary {
  id: string;
  subject: string;
  state: BeTaskState | null;
  dueDate: string | null;
  assignedTo: BeEntityRef | null;
  updatedOn: string;
}

/** `POST /cases/{caseId}/tasks/search` request payload. */
export interface BeCaseTasksSearchPayload {
  pagination?: BePagination;
}

/** `POST /cases/{caseId}/tasks/search` response. */
export interface BeListCaseTasksResponse {
  tasks: BeTaskSummary[];
  total: number;
  offset: number;
  limit: number;
}

/**
 * `GET /tasks/{id}` response. `visibleToCustomer` is confirmed `false` on
 * every sampled record for this org — it is shown as a plain fact, never used
 * to drive conditional UI.
 */
export interface BeTaskDetail {
  id: string;
  subject: string;
  state: BeTaskState | null;
  dueDate: string | null;
  visibleToCustomer: boolean;
  assignedTo: BeEntityRef | null;
  requestType: string | null;
  requestTypeLabel: string | null;
  environment: string | null;
  environmentLabel: string | null;
  product: BeEntityRef | null;
  parentCase: BeCaseNumberRef | null;
  createdOn: string;
  updatedOn: string;
}

/** `POST /cases/{caseId}/tasks` request body. Only `subject` is required. */
export interface BeCreateCaseTaskPayload {
  subject: string;
  /** ISO date-time; null/absent when the task carries no due date. */
  dueDate?: string | null;
  /** Email of the engineer to assign the new task to. */
  assignedToEmail?: string | null;
  /** Whether the task should be visible to the customer. */
  visibleToCustomer?: boolean | null;
}

/**
 * Fields never allowed alongside another `PATCH /tasks/{id}` variant — mirrors
 * {@link BeCaseUpdateNever}'s exactly-one-field pattern for the case PATCH.
 */
interface BeUpdateTaskNever {
  state?: never;
  assignedToEmail?: never;
  dueDate?: never;
}

/**
 * Request body for `PATCH /tasks/{id}`. **Exactly one** of `state` /
 * `assignedToEmail` / `dueDate` is sent per call — the backend rejects zero
 * or more than one.
 */
export type BeUpdateTaskPayload =
  | (Omit<BeUpdateTaskNever, "state"> & { state: string })
  | (Omit<BeUpdateTaskNever, "assignedToEmail"> & { assignedToEmail: string })
  | (Omit<BeUpdateTaskNever, "dueDate"> & { dueDate: string });

// ---------------------------------------------------------------------------
// Change requests (managed-cloud; ServiceNow data source only)
// ---------------------------------------------------------------------------

export type BeChangeRequestState =
  | "new"
  | "assess"
  | "authorize"
  | "customer_approval"
  | "scheduled"
  | "implement"
  | "review"
  | "customer_review"
  | "rollback"
  | "closed"
  | "canceled";

export type BeChangeRequestImpact = "high" | "medium" | "low";

export type BeChangeRequestType =
  | "standard"
  | "normal"
  | "emergency"
  | "model"
  | "site_reliability_ops"
  | "azure";

export type BeChangeRequestPriority = "critical" | "high" | "moderate" | "low";

export type BeChangeRequestRisk = "high" | "moderate" | "low";

export type BeChangeRequestCategory =
  | "hardware"
  | "software"
  | "service"
  | "system_software"
  | "applications_software"
  | "network"
  | "telecom"
  | "documentation"
  | "other"
  | "regular_release_cloud"
  | "hotfix_release_cloud"
  | "devops"
  | "cloud_computing";

/** List-item / shared shape for a change request (`POST /change-requests/search`). */
export interface BeChangeRequestSearchView {
  id: string;
  number?: string;
  subject?: string | null;
  description?: string | null;
  project?: BeEntityRef;
  case?: BeEntityRef | null;
  deployment?: BeEntityRef | null;
  deployedProduct?: BeEntityRef | null;
  product?: BeEntityRef | null;
  assignedEngineer?: BeEntityRef | null;
  assignedTeam?: BeEntityRef | null;
  plannedStartOn?: string | null;
  plannedEndOn?: string | null;
  duration?: string | null;
  impact?: string | null;
  state?: string | null;
  type?: string | null;
  createdOn?: string;
  updatedOn?: string;
}

/** `GET /change-requests/{id}` — the search view plus the heavy plan fields. */
export interface BeChangeRequestDetail extends BeChangeRequestSearchView {
  createdBy?: string;
  justification?: string | null;
  impactDescription?: string | null;
  serviceOutage?: string | null;
  communicationPlan?: string | null;
  rollbackPlan?: string | null;
  testPlan?: string | null;
  hasCustomerApproved?: boolean;
  hasCustomerReviewed?: boolean;
  approvedBy?: BeEntityRef | null;
  approvedOn?: string | null;
  /**
   * Backend-supplied legal transitions out of the CR's current state, mirroring
   * `nextStates` on a case (`CaseActionBar.tsx`) — render one action per entry
   * rather than hardcoding a `state === 'new'` check. Intentionally narrow today:
   * the only transition modeled so far is New -> Assess, so this is only ever
   * `["assess"]` or `[]`.
   */
  legalNextStates?: string[];
}

/** An approval stage seen on a change request, e.g. Assess, Authorize. */
export type BeChangeRequestApprovalStage = "Assess" | "Authorize" | "Customer Approval";

/** Who a change-request approval stage is assigned to. */
export type BeChangeRequestApproverType = "STATIC_GROUP" | "DYNAMIC_CONTACT";

/**
 * An individual approver's response within an approval stage
 * (`GET /change-requests/{id}/approvals`). `status` is an open, backend-passthrough
 * string (seen live: `APPROVED`, `NOT_REQUIRED`, `REQUESTED`; also possible:
 * `REJECTED`, `CANCELLED`, `NO_CONSENSUS`, or an unrecognized value) — the backend
 * does not validate/whitelist it, so render with a fallback rather than a closed union.
 */
export interface BeChangeRequestApprover {
  id: string;
  name?: string | null;
  status: string;
  respondedOn?: string | null;
}

/** One approval stage on a change request, with its individual approvers. */
export interface BeChangeRequestApproval {
  stage: string;
  approverType: BeChangeRequestApproverType | string;
  approverName?: string | null;
  status: string;
  approvers: BeChangeRequestApprover[];
}

/** `GET /change-requests/{id}/approvals` response. */
export interface BeChangeRequestApprovalsView {
  approvals: BeChangeRequestApproval[];
}

/** Caller's decision on their own pending change-request approval. */
export type BeChangeRequestApprovalDecision = "approved" | "rejected";

/** `POST /change-requests/{id}/approvals/decision` request body. */
export interface BeChangeRequestApprovalDecisionPayload {
  decision: BeChangeRequestApprovalDecision;
}

/** `POST /change-requests/{id}/approvals/decision` response. */
export interface BeChangeRequestApprovalDecisionResponse {
  id: string;
  state: string;
}

/**
 * `POST /change-requests` body (ServiceNow data source only). `subject` is
 * the only required field; every ID field (`serviceId`, `serviceOfferingId`,
 * `configurationItemId`, `groupId`, `assignedEngineerId`, `requestedById`)
 * is a portal UUID resolved server-side against the backing data source, via
 * the matching `/*\/search` endpoint (see `AsyncEntitySelect` usages in
 * `CreateChangeRequestPage.tsx`). `state` accepts any valid lifecycle state,
 * but the create form restricts the selectable options to the pre-workflow
 * states (new/assess/authorize), defaulting to "new", so a CR can't be created
 * already past its own approval flow.
 * `plannedStartDate`/`plannedEndDate` are `YYYY-MM-DD HH:MM:SS` strings.
 */
export interface BeCreateChangeRequestPayload {
  subject: string;
  category?: BeChangeRequestCategory;
  serviceId?: string;
  serviceOfferingId?: string;
  configurationItemId?: string;
  priority?: BeChangeRequestPriority;
  impact?: BeChangeRequestImpact;
  type?: BeChangeRequestType;
  state?: BeChangeRequestState;
  groupId?: string;
  assignedEngineerId?: string;
  risk?: BeChangeRequestRisk;
  requestedById?: string;
  description?: string;
  justification?: string;
  implementationPlan?: string;
  riskImpactAnalysis?: string;
  backoutPlan?: string;
  testPlan?: string;
  plannedStartDate?: string;
  plannedEndDate?: string;
  comment?: string;
  workNote?: string;
}

/** `POST /change-requests` response — the created identifiers. */
export interface BeCreateChangeRequestResponse {
  message: string;
  changeRequest: {
    id: string;
    number: string;
    createdOn: string;
    createdBy: string;
  };
}

// ---------------------------------------------------------------------------
// Change-request reference lookups (ServiceNow CMDB — groups, IT services,
// service offerings, configuration items). Each search response carries no
// `hasMore` (same as change requests), so these don't extend
// BeSearchResponseBase.
// ---------------------------------------------------------------------------

export interface BeGroup {
  id: string;
  name: string;
  active: boolean;
  parent?: BeEntityRef | null;
}

export interface BeGroupSearchPayload {
  filters?: { searchQuery?: string };
  pagination: BePagination;
}

export interface BeGroupSearchResponse {
  groups: BeGroup[];
  total: number;
  limit: number;
  offset: number;
}

// ---------------------------------------------------------------------------
// Roles & teams — the platform's own directory catalogues (`/roles/search`,
// `/teams/search`). Unlike `BeGroup` above, which is a live query against the
// backing data source's assignment groups, these two are curated vocabularies:
// `/roles/search` is the set of assignable role keys that
// `UserSearchFilters.roleIds` accepts, and `/teams/search` is the team
// registry. A team's `id` is its registry key (e.g. "alpha"), not a UUID —
// registry keys are stable across environments, whereas the backing group's id
// is not, which is why the key is what this API exposes.
// ---------------------------------------------------------------------------

export interface BeRole {
  id: string;
  name: string;
}

export interface BeRoleSearchPayload {
  filters?: { searchQuery?: string };
  pagination?: BePagination;
}

export interface BeRoleSearchResponse {
  roles: BeRole[];
  total: number;
  limit: number;
  offset: number;
}

export interface BeTeam {
  /** Registry team key, e.g. "alpha" — not the backing group's UUID. */
  id: string;
  name: string;
  family?: string;
  /** The backing data source's assignment group id, reformatted as this
   * platform's UUID — present only when the deployment's team registry has
   * one configured for this team. This is the id an `integrationCsTeam`
   * case filter entry actually needs (see
   * `BE_CURRENT_USER_FILTER_PLACEHOLDER`-style team filter substitution in
   * `teamFilterPlaceholder.ts`) — never `id` above, which is just the
   * registry key. */
  groupId?: string;
}

export interface BeTeamSearchPayload {
  filters?: { searchQuery?: string };
  pagination?: BePagination;
}

export interface BeTeamSearchResponse {
  teams: BeTeam[];
  total: number;
  limit: number;
  offset: number;
}

export interface BeItService {
  id: string;
  name?: string | null;
  class?: string | null;
  businessCriticality?: string | null;
  serviceClassification?: string | null;
}

export interface BeItServiceSearchPayload {
  filters?: { searchQuery?: string };
  pagination: BePagination;
}

export interface BeItServiceSearchResponse {
  services: BeItService[];
  total: number;
  limit: number;
  offset: number;
}

export interface BeServiceOffering {
  id: string;
  name: string;
  service?: BeEntityRef | null;
}

export interface BeServiceOfferingSearchPayload {
  /** Narrow to offerings under a specific service (its portal UUID). */
  filters?: { serviceIds?: string[]; searchQuery?: string };
  pagination: BePagination;
}

export interface BeServiceOfferingSearchResponse {
  serviceOfferings: BeServiceOffering[];
  total: number;
  limit: number;
  offset: number;
}

export interface BeConfigurationItem {
  id: string;
  name?: string | null;
  description?: string | null;
  class?: string | null;
}

export interface BeConfigurationItemSearchPayload {
  filters?: { searchQuery?: string };
  pagination: BePagination;
}

export interface BeConfigurationItemSearchResponse {
  configurationItems: BeConfigurationItem[];
  total: number;
  limit: number;
  offset: number;
}

/**
 * `PATCH /change-requests/{id}` body (ServiceNow data source only). At least
 * one field is required by the BE (`minProperties: 1`). `plannedStartOn` is a
 * `YYYY-MM-DD HH:MM:SS` string. `requestApproval` is mutually exclusive with
 * the other fields here — it drives the New -> Assess transition (see
 * `legalNextStates` on {@link BeChangeRequestDetail}) rather than editing a value.
 */
export interface BePatchChangeRequestPayload {
  plannedStartOn?: string;
  isCustomerApproved?: boolean;
  isCustomerReviewed?: boolean;
  assignedTeamId?: string;
  requestApproval?: true;
  /**
   * UUID of the service-request case this change request was raised from.
   * Only settable via PATCH — `POST /change-requests` does not accept it, so
   * the link is set by a follow-up PATCH once the change request exists.
   */
  caseId?: string;
}

/** `PATCH /change-requests/{id}` response — the touched identifiers. */
export interface BePatchChangeRequestResponse {
  id: string;
  updatedOn?: string;
  updatedBy?: string;
}

export interface BeChangeRequestSearchPayload {
  filters?: {
    projectIds?: string[];
    searchQuery?: string;
    states?: BeChangeRequestState[];
    impacts?: BeChangeRequestImpact[];
    closedStartDate?: string;
    closedEndDate?: string;
    /**
     * A single change request number (e.g. "CHG0038721"); matches exactly.
     * Routed as a first-class filter rather than through the free-text
     * searchQuery scan.
     */
    number?: string;
  };
  sortBy?: {
    field?: "createdOn" | "updatedOn";
    order?: "asc" | "desc";
  };
  pagination?: BePagination;
}

/** Note: the CR search response carries no `hasMore` (unlike the other searches). */
export interface BeChangeRequestSearchResponse {
  changeRequests: BeChangeRequestSearchView[];
  total: number;
  limit: number;
  offset: number;
}

// ---------------------------------------------------------------------------
// Incidents (ServiceNow data source only). Unlike change requests, every
// enum here is UPPER_SNAKE_CASE on the wire (matches
// apps/csm-portal/backend/internal/handler/incidents.go's validation maps
// exactly) — don't copy change requests' lowercase convention for these.
// ---------------------------------------------------------------------------

export type BeIncidentPriority = "CRITICAL" | "HIGH" | "MODERATE" | "LOW" | "PLANNING";

export type BeIncidentState =
  | "NEW"
  | "IN_PROGRESS"
  | "ON_HOLD"
  | "RESOLVED"
  | "CLOSED"
  | "CANCELLED";

export type BeIncidentCategory = "INQUIRY" | "SERVICE_INTERRUPTION" | "SECURITY";

export type BeIncidentSubcategory =
  | "DHCP"
  | "ORACLE"
  | "CPU"
  | "KEYBOARD"
  | "DOS_DDOS"
  | "PRIVILEGE_ESCALATIONS"
  | "THREAT_INTELLIGENCE"
  | "SCANS_AND_PROBES"
  | "APPLICATION_SECURITY"
  | "CONFIG_CHANGE_REQUEST"
  | "IP_ADDRESS"
  | "FULL_OUTAGE"
  | "SQL_SERVER"
  | "SLOWNESS"
  | "MEMORY"
  | "MOUSE"
  | "PRIVACY"
  | "DATA_BREACH"
  | "SYSTEM_COMPROMISES"
  | "DNS"
  | "OS"
  | "DISK"
  | "VPN"
  | "MALWARE"
  | "VULNERABILITY"
  | "UNAUTHORIZED_ACCESS"
  | "IDENTITY_PROTECTION"
  | "PHISHING"
  | "IMPROPER_CONFIGURATION"
  | "INFORMATION_REQUEST"
  | "DB2"
  | "PARTIAL_OUTAGE"
  | "EMAIL"
  | "MONITOR"
  | "WIRELESS";

export type BeIncidentContactType =
  | "SELF_SERVICE"
  | "EMAIL"
  | "WALK_IN"
  | "AZURE"
  | "EMAIL_INTERNAL"
  | "SITE_247"
  | "DIRECT"
  | "PHONE"
  | "SENTINEL"
  | "VIRTUAL_AGENT"
  | "CHAT"
  | "EMAIL_EXTERNAL";

export type BeIncidentImpact = "HIGH" | "MEDIUM" | "LOW";
export type BeIncidentUrgency = "HIGH" | "MEDIUM" | "LOW";

/** List-item shape for an incident (`POST /incidents/search`). */
export interface BeIncident {
  id: string | null;
  number: string | null;
  openedOn: string | null;
  subject: string | null;
  caller?: BeEntityRef | null;
  priority: BeIncidentPriority | null;
  state: BeIncidentState | null;
  category: BeIncidentCategory | null;
  parent?: BeEntityRef | null;
  assignmentGroup?: BeEntityRef | null;
  assignedTo?: BeEntityRef | null;
  createdOn?: string;
  createdBy?: string;
  updatedOn?: string;
  updatedBy?: string;
}

export interface BeIncidentWatchListItem {
  id: string;
  name: string;
  email: string;
}

/**
 * `GET /incidents/{id}` — the search view plus the fields only the detail
 * endpoint returns (subcategory, service/serviceOffering/configurationItem,
 * contactType, impact/urgency, the changeRequest/problem/causedBy links,
 * comments, and the watch list).
 */
export interface BeIncidentDetail extends BeIncident {
  subcategory?: BeIncidentSubcategory | null;
  service?: BeEntityRef | null;
  serviceOffering?: BeEntityRef | null;
  configurationItem?: BeEntityRef | null;
  contactType?: BeIncidentContactType | null;
  impact?: BeIncidentImpact | null;
  urgency?: BeIncidentUrgency | null;
  changeRequest?: BeEntityRef | null;
  problem?: BeEntityRef | null;
  causedBy?: BeEntityRef | null;
  additionalComments?: string | null;
  workNotes?: string | null;
  watchList?: BeIncidentWatchListItem[];
  /** Service-request cases whose parent points to this incident. */
  linkedServiceRequests?: BeLinkedServiceRequestRef[] | null;
}

/**
 * `POST /incidents` body. `callerId`, `category`, `serviceId`, `impact`,
 * `urgency`, and `subject` are required by the backend
 * (`validateCreateIncidentBody` in incidents.go); everything else is
 * optional. There is no `priority` field here — ServiceNow computes it
 * server-side from `impact` × `urgency`, so it only ever appears on read
 * (see {@link BeIncident.priority}).
 */
export interface BeCreateIncidentPayload {
  callerId: string;
  category: BeIncidentCategory;
  subcategory?: BeIncidentSubcategory;
  serviceId: string;
  serviceOfferingId?: string;
  configurationItemId?: string;
  contactType?: BeIncidentContactType;
  impact: BeIncidentImpact;
  urgency: BeIncidentUrgency;
  assignmentGroupId?: string;
  assignedEngineerId?: string;
  subject: string;
  watchList?: string[];
  additionalComments?: string;
  workNotes?: string;
  parentId?: string;
  changeRequestId?: string;
  problemId?: string;
  causedById?: string;
}

/** `POST /incidents` response — the created identifiers. */
export interface BeCreateIncidentResponse {
  message: string;
  incident: {
    id: string;
    number: string;
    createdOn: string;
    createdBy: string;
  };
}

/**
 * `PATCH /incidents/{id}` body (ServiceNow data source only,
 * `minProperties: 1`). Covers the in-scope subset the Edit dialog sends —
 * the full `UpdateIncidentPayload` schema also documents `incidentReport` /
 * `resolvedById`, deliberately left out here since there's no read-side
 * model for them yet either (see {@link BeIncidentDetail}). `resolutionCode`
 * / `resolutionNotes` ARE included (write-only, same as `incidentReport` —
 * `IncidentDetail` never echoes them back on read) since ServiceNow requires
 * them to move an incident to `RESOLVED`/`CLOSED` (confirmed live: those two
 * state values 500 without them).
 */
export interface BeUpdateIncidentPayload {
  subject?: string;
  category?: BeIncidentCategory;
  subcategory?: BeIncidentSubcategory;
  contactType?: BeIncidentContactType;
  impact?: BeIncidentImpact;
  urgency?: BeIncidentUrgency;
  state?: BeIncidentState;
  resolutionCode?: string;
  resolutionNotes?: string;
  // Reference/note fields are `nullable: true` on the documented schema — the
  // portal sends an explicit `null` to clear one (e.g. unassign an engineer),
  // as distinct from omitting the field entirely, which the BE treats as
  // "leave unchanged".
  serviceId?: string | null;
  serviceOfferingId?: string | null;
  configurationItemId?: string | null;
  assignmentGroupId?: string | null;
  assignedEngineerId?: string | null;
  watchList?: string[];
  workNotes?: string | null;
  additionalComments?: string | null;
  parentId?: string | null;
  changeRequestId?: string | null;
  problemId?: string | null;
  causedById?: string | null;
}

/** `PATCH /incidents/{id}` response — the full updated incident. */
export interface BePatchIncidentResponse {
  message: string;
  incident: BeIncidentDetail;
}

export interface BeIncidentSearchPayload {
  filters?: {
    searchQuery?: string;
    priorities?: BeIncidentPriority[];
    parentIds?: string[];
    /**
     * At least one breached SLA record (optional). `false` and omitted are
     * identical to the backend — it applies no SLA restriction either way,
     * `false` does NOT mean "SLA met" — so callers should omit this key
     * entirely rather than send `false`.
     */
    slaViolated?: boolean;
    /** Inclusive UTC bound on the creation timestamp: `YYYY-MM-DDTHH:MM:SSZ`. */
    startCreatedDate?: string;
    /** Inclusive UTC bound on the creation timestamp: `YYYY-MM-DDTHH:MM:SSZ`. */
    endCreatedDate?: string;
    /**
     * Union match on the name of the service the incident relates to
     * (optional). Incidents carry no product dimension of their own, so this
     * resolves against the related service's name, which is only ~43%
     * populated and mixes real products with customer names and service
     * categories — filtering by this misses roughly half of all incidents.
     */
    productNames?: string[];
    /**
     * A single incident number (e.g. "INC0090472"); matches exactly. Routed
     * as a first-class filter rather than through the free-text searchQuery
     * scan.
     */
    number?: string;
  };
  sortBy?: {
    field?: "createdOn" | "updatedOn" | "openedOn";
    order?: "asc" | "desc";
  };
  pagination?: BePagination;
}

export interface BeIncidentSearchResponse {
  incidents: BeIncident[];
  total: number;
  limit: number;
  offset: number;
}

// ---------------------------------------------------------------------------
// Problems (SRE-owned; ServiceNow data source only). Enum casing mirrors
// incidents (UPPER_SNAKE_CASE on the wire), not change requests.
// ---------------------------------------------------------------------------

export type BeProblemState =
  | "NEW"
  | "ASSESS"
  | "ROOT_CAUSE_ANALYSIS"
  | "FIX_IN_PROGRESS"
  | "RESOLVED"
  | "CLOSED";

/**
 * A reference to another record embedded within a problem. Deliberately
 * generic (id + number only, no record-type discriminant): the backend does
 * not tag these with a type, and `originCase` in particular can point at
 * either a Case or an Incident depending on the underlying data despite its
 * name. Render as a plain, non-navigable reference unless the caller
 * independently knows the target record type and has a route for it.
 */
export interface BeProblemRef {
  id: string;
  number?: string;
}

/**
 * List-item shape for `POST /problems/search`. Includes `state` plus the
 * assignment refs alongside id/number/subject; full record detail (priority,
 * category, resolution fields, etc.) is still only on `GET /problems/{id}`
 * (see {@link BeProblemDetail}).
 */
export interface BeProblemSearchView {
  id: string;
  number?: string;
  subject?: string;
  state?: BeProblemState;
  assignmentGroup?: BeEntityRef | null;
  assignedTo?: BeEntityRef | null;
}

export interface BeProblemSearchFilters {
  searchQuery?: string;
  /**
   * Filter by state. Not confirmed live against the backend at the time this
   * was written, so if the backend rejects or silently ignores this filter,
   * drop it here and from `ProblemsTab`'s payload, and remove the state
   * control from `ProblemsFilterBar`.
   */
  states?: BeProblemState[];
  /**
   * A single problem number (e.g. "PRB0040192"); matches exactly. Routed as
   * a first-class filter rather than through the free-text searchQuery scan.
   */
  number?: string;
}

export interface BeProblemSearchPayload {
  filters?: BeProblemSearchFilters;
  pagination?: BePagination;
}

/** Note: mirrors the change-request/incident search responses — no `hasMore`. */
export interface BeProblemSearchResponse {
  problems: BeProblemSearchView[];
  total: number;
  limit: number;
  offset: number;
}

/**
 * `GET /problems/{id}` response. `originCase` / `primaryIncident` /
 * `linkedChangeRequest` are singular refs (each may reference a different
 * record type — see {@link BeProblemRef}); `linkedIncidents` is a genuine
 * one-to-many and must be rendered as a list, not a single value.
 */
export interface BeProblemDetail {
  id: string;
  number?: string;
  subject?: string;
  state?: BeProblemState;
  priority?: string | null;
  /** May be null/empty on many records — render blank gracefully, not as an awkward empty field. */
  category?: string | null;
  subcategory?: string | null;
  /** Despite the name, may reference an Incident rather than a Case. */
  originCase?: BeProblemRef | null;
  primaryIncident?: BeProblemRef | null;
  linkedIncidents?: BeProblemRef[];
  linkedChangeRequest?: BeProblemRef | null;
  assignedTo?: BeEntityRef | null;
  resolutionCode?: string | null;
  causeNotes?: string | null;
  fixNotes?: string | null;
  workaround?: string | null;
  resolvedOn?: string | null;
  resolvedBy?: BeEntityRef | null;
  openedOn?: string | null;
  closedOn?: string | null;
}

/**
 * `POST /problems` body (ServiceNow data source only). `subject` is the only
 * required field. There is no `priority` field — priority is not settable on
 * create (SN computes/defaults it server-side, confirmed by live testing), so
 * it's deliberately omitted here and from the create form.
 */
export interface BeCreateProblemPayload {
  subject: string;
  category?: string;
  subcategory?: string;
  originCaseId?: string;
  primaryIncidentId?: string;
}

// ---------------------------------------------------------------------------
// Product vulnerabilities (managed-cloud; ServiceNow data source only)
// ---------------------------------------------------------------------------

/** Priority enum for product vulnerabilities. */
export type BeVulnerabilityPriority =
  | "info"
  | "low"
  | "medium"
  | "high"
  | "critical"
  | "unknown";

/**
 * Single product vulnerability as returned by both the search list and the
 * `GET /products/vulnerabilities/{id}` detail endpoint.
 */
export interface BeProductVulnerabilityView {
  id: string;
  cveId?: string;
  vulnerabilityId?: string;
  /** Priority label from the upstream (e.g. "High", "Critical"). */
  priority?: string;
  productName?: string | null;
  productVersion?: string | null;
  componentName?: string;
  version?: string;
  type?: string;
  componentType?: string | null;
  updateLevel?: string | null;
  useCase?: string | null;
  justification?: string | null;
  resolution?: string | null;
}

export interface BeSearchProductVulnerabilitiesFilters {
  searchQuery?: string;
  priority?: BeVulnerabilityPriority;
  productName?: string;
  productVersion?: string;
}

export interface BeSearchProductVulnerabilitiesPayload {
  filters?: BeSearchProductVulnerabilitiesFilters;
  pagination?: BePagination;
}

/** Note: the vulnerabilities search response carries no `hasMore`. */
export interface BeSearchProductVulnerabilitiesResponse {
  productVulnerabilities: BeProductVulnerabilityView[];
  total: number;
  limit: number;
  offset: number;
}

export type BeTimeCardState =
  | "pending"
  | "submitted"
  | "approved"
  | "rejected"
  | "processed"
  | "recalled";

export interface BeTimeCardRef {
  id: string;
  name: string;
}

export interface BeTimeCardCaseRef {
  id: string;
  name: string;
  number: string;
}

/**
 * A time card as returned by search and the mutation endpoints. Previously
 * documented as never echoing back category / issue complexity / hour
 * breakdown / lead comment, even though those are accepted on write — that
 * was wrong for issue complexity and the hour breakdown: confirmed live
 * against the real entity-service (`POST /time-cards/search`) that both come
 * back on every card, not just the one just created. `workLogComment` is also
 * echoed back — confirmed live separately. "Category" and any lead comment
 * other than {@link rejectionReason} are still write-only/unconfirmed.
 */
export interface BeTimeCardView {
  id: string;
  totalTime: number;
  /** ServiceNow rich-text HTML (same convention as the log-time form's
   * editor) — sanitize with `sanitizeRichTextHtml` before rendering. */
  workLogComment?: string;
  /**
   * The date the work was actually carried out (YYYY-MM-DD) — what the engineer
   * picked in the log form, so it can be in the past. This is the field to
   * display and to group/sort by ("the week this work happened").
   */
  workDate: string;
  /**
   * @deprecated Superseded by {@link workDate}; don't build new logic on it. It
   * currently reads the same underlying field, so the two hold the same value.
   */
  createdOn: string;
  hasBillable: boolean;
  state: BeTimeCardState;
  user?: BeTimeCardRef;
  /**
   * The approver who accepted the card — populated **only** when `state` is
   * `approved`. ServiceNow does not record who rejected a card, so this is null
   * for a `rejected` card (and for an undecided one); see {@link rejectionReason}.
   */
  approvedBy?: BeTimeCardRef | null;
  /**
   * The approver's comment when rejecting — populated **only** when `state` is
   * `rejected`, otherwise null. It's the only trace a rejection leaves: there is
   * no "rejected by" / "rejected on" field (a known backend gap).
   */
  rejectionReason?: string | null;
  project?: BeTimeCardRef;
  case?: BeTimeCardCaseRef;
  /**
   * The engineers eligible to decide this card (SN `approver_list`). Not in
   * the BFF's `openapi.yaml` (stale doc — `time-cards` responses are a raw
   * passthrough of the entity-service's `TimeCardView`, which does return
   * this), but confirmed present on the wire. Used to gate the "Review"
   * action in the UI: the backend 403s a decision from anyone not in this
   * list, regardless of team-lead status.
   */
  approvers?: BeTimeCardRef[];
  /** Per-activity whole minutes — confirmed live on `POST /time-cards/search`
   * responses, both for a just-created card and a pre-existing one. */
  timeAnalyzing?: number;
  timeSettingUp?: number;
  timeReproducingDebugging?: number;
  timeProvidingSolution?: number;
  timePatching?: number;
  /** Confirmed live on `POST /time-cards/search` responses. */
  issueComplexity?: string;
}

/**
 * `entity-service`'s `openapi.yaml` also documents a `caseId` filter here
 * (and it's genuinely implemented end-to-end, forwarded through to
 * ServiceNow) — deliberately omitted: confirmed live to be non-functional,
 * always returning `total: 0` even for a case with cards provably matching
 * that exact id. See the note on `useCaseTimeCards` in `useTimeCards.ts`
 * before re-adding it.
 */
export interface BeSearchTimeCardsFilters {
  projectIds?: string[];
  /** Only time cards logged against this case. */
  caseId?: string;
  /** Only time cards submitted by this user. */
  userId?: string;
  /** Only time cards submitted by any of these users (multi-engineer filter). */
  userIds?: string[];
  /** Only time cards this user is eligible to approve (SN `approver_list`);
   * the caller's own cards are excluded unconditionally when this is set. */
  approverId?: string;
  /** Only time cards actually approved by this user (SN `approved_by`). */
  approvedById?: string;
  /** ISO 8601 date (YYYY-MM-DD). */
  startDate?: string;
  /** ISO 8601 date (YYYY-MM-DD). */
  endDate?: string;
  states?: BeTimeCardState[];
}

export interface BeSearchTimeCardsPayload {
  filters?: BeSearchTimeCardsFilters;
  pagination?: BePagination;
}

export interface BeSearchTimeCardsResponse {
  timeCards: BeTimeCardView[];
  total: number;
  limit: number;
  offset: number;
}

export interface BeCreateTimeCardPayload {
  caseId: string;
  projectId: string;
  /** ISO 8601 date (YYYY-MM-DD). */
  date: string;
  /** Eligible approvers (approver_list). Must be non-empty. */
  approverIds: string[];
  isBillable?: boolean;
  issueComplexity?: string;
  workLogComment?: string;
  timeAnalyzing?: number;
  timeSettingUp?: number;
  timeReproducingDebugging?: number;
  timeProvidingSolution?: number;
  timePatching?: number;
}

/**
 * Either editable fields (no `state`), or a state transition (`state`:
 * "approved", or "rejected" with a `leadComment`) — mutually exclusive, per
 * the backend contract, and enforced server-side as submitter-only +
 * `submitted`-state-only (matches ServiceNow's own edit-in-place behavior).
 * Confirmed live: a content-fields PATCH with no `state` key persists and
 * round-trips correctly on the next search.
 */
export interface BeUpdateTimeCardPayload {
  state?: Extract<BeTimeCardState, "approved" | "rejected">;
  leadComment?: string;
  /** ISO 8601 date (YYYY-MM-DD). */
  date?: string;
  isBillable?: boolean;
  issueComplexity?: string;
  workLogComment?: string;
  timeAnalyzing?: number;
  timeSettingUp?: number;
  timeReproducingDebugging?: number;
  timeProvidingSolution?: number;
  timePatching?: number;
}

export interface BeTimeCardMutationResponse {
  message?: string;
  timeCard: BeTimeCardView;
}

// ---------------------------------------------------------------------------
// Users
// ---------------------------------------------------------------------------

/**
 * Minimal shape read out of `POST /users/search`'s response for the
 * email-to-id resolution lookup (see `useResolvedUserId`). The full response
 * is a `oneOf` (postgres `User` vs ServiceNow `SnUser` — see
 * `features/csm-users/types/csmUsers.ts`), but `id`/`email` are common to
 * both and are all this lookup reads.
 */
export interface BeUserSearchByEmailResponse {
  users?: Array<{ id: string; email: string }>;
}

// ---------------------------------------------------------------------------
// Dashboards
// ---------------------------------------------------------------------------

/**
 * Which resource a widget's filters search against — the widget resolves its
 * own data by issuing a `POST /{resourceType}s/search`-shaped request (see
 * `widgetResourceConfig.ts` for the real endpoint per type) with `filters`
 * forwarded verbatim.
 *
 * `service_request`, `security_report_analysis`, `announcement`, and
 * `engagement` are additional values of the case-search `type` field (see
 * `BeCaseType`/`ALL_CASE_TYPES` in `caseType.ts`) exposed as their own widget
 * resourceType alongside `case` itself — all five route to the same `POST
 * /cases/search`, the backend auto-injecting the implied `type` filter for
 * each at dashboard-load time when a widget doesn't already carry one
 * explicitly.
 */
export type BeWidgetResourceType =
  | "case"
  | "incident"
  | "change_request"
  | "account"
  | "project"
  | "user"
  | "time_card"
  | "problem"
  | "product_vulnerability"
  | "task"
  | "call_request"
  | "service_request"
  | "security_report_analysis"
  | "announcement"
  | "engagement";

/**
 * How a widget's resolved data should be rendered. `pie` and `bar` both
 * resolve the same way `count` does, just once per slice — see
 * {@link BeDashboardWidget.slices} — differing only in how the frontend
 * renders the resolved data (wedges vs. bars), not in how it's fetched.
 */
export type BeWidgetShape = "count" | "list" | "pie" | "bar";

/** Palette key a dashboard config can use to color a pie slice — the same
 * vocabulary `WidgetResourceConfig.iconColor` already uses elsewhere in this
 * system, so one dashboard has one consistent color language. */
export type BeWidgetPaletteColor =
  | "primary"
  | "secondary"
  | "success"
  | "error"
  | "info"
  | "warning";

/** One wedge of a `shape: "pie"` widget. Resolved by issuing this
 * resourceType's own `POST /{resourceType}s/search` with this slice's
 * `query` merged under the widget's own base `query` (this slice's keys win
 * on conflict) and `pagination: { limit: 1 }`, reading `total` — the exact
 * same mechanism `shape: "count"` uses, just once per slice. */
export interface BeDashboardPieSlice {
  label: string;
  /** Falls back to a fixed rotation over the same palette if omitted. */
  color?: BeWidgetPaletteColor;
  query: Record<string, unknown>;
}

/** Rendering hint for a {@link BeDashboardWidgetColumn}'s resolved value.
 * Omitted (or `"text"`) renders plain text; `"date"` formats a date/
 * date-time string the same way the app's existing hardcoded list renderers
 * already format one (see `formatDate` in `widgetListConfig.tsx`). */
export type BeDashboardWidgetColumnFormat = "text" | "date";

/** One column of a `shape: "list"` widget's generic column renderer (see
 * {@link BeDashboardWidget.columns}). Opaque config: the BE never resolves
 * `path` or interprets `format`, it only forwards this object. */
export interface BeDashboardWidgetColumn {
  /**
   * Dot-separated path into one item of that widget's own resourceType
   * search response, reaching into nested objects to arbitrary depth (e.g.
   * `"project.key"`, `"project.account.tier"`) — every resource's search
   * response embeds related entities as nested JSON objects, not flat
   * records. A path that resolves to nothing on a given row renders that
   * cell empty rather than erroring the whole widget.
   */
  path: string;
  /** Column header text. */
  label: string;
  format?: BeDashboardWidgetColumnFormat;
}

/**
 * A single widget template, embedded in {@link BeDashboard}: display metadata
 * plus its already-resolved search criteria. The caller resolves the
 * widget's own data by issuing its own `POST /{resourceType}s/search`,
 * posting `query` as that request's `filters` and reading `total` (or the
 * item list) off the response.
 */
export interface BeDashboardWidget {
  widgetId: string;
  displayName: string;
  /** Explanatory subtitle shown under `displayName` — config-owned text,
   * not hardcoded per resourceType/shape on the frontend. */
  description?: string;
  resourceType: BeWidgetResourceType;
  shape: BeWidgetShape;
  /** CSS grid columns out of 12 this widget should occupy. */
  gridWidth: number;
  /**
   * Opaque search criteria, with any current-user placeholder already
   * substituted. Pass this directly as the `filters` of that
   * `resourceType`'s own `POST /{resourceType}s/search` request — the
   * request-body key stays `filters`; only this widget-config key is
   * `query`. For shapes "pie"/"bar" this is a shared base merged under
   * every slice's own `query` (see {@link BeDashboardPieSlice}), rather
   * than queried on its own.
   */
  query: Record<string, unknown>;
  /** Present on the wire; unused today — `slices` is what actually drives
   * pie/bar grouping. */
  groupBy?: string;
  /** Only meaningful for shapes "pie"/"bar": one search per slice, each
   * read via its own `total`. */
  slices?: BeDashboardPieSlice[];
  /** Only meaningful for shape list; how many records to show. */
  listLimit?: number;
  /** Groups widgets sharing the same (non-empty) value under a titled
   * sub-section within the dashboard, in the order that value first
   * appears among the dashboard's widgets. Widgets with no `section` (the
   * common case) render in one untitled group, same as before this field
   * existed. */
  section?: string;
  /** Only meaningful for shape "list": an ordered set of columns to render
   * instead of that resourceType's own hardcoded list renderer (see
   * `WIDGET_LIST_RENDERERS` in `widgetListConfig.tsx`). Absent/empty is a
   * no-op — the existing hardcoded per-resourceType renderer applies
   * exactly as before this field existed. */
  columns?: BeDashboardWidgetColumn[];
  /** Only meaningful for shape "list": opaque sort criteria, forwarded
   * verbatim as the `sortBy` of that resourceType's own
   * `POST /{resourceType}s/search` request — same passthrough philosophy
   * as `query`/`filters`. The caller is responsible for using a field name
   * valid for that resourceType's own search contract; an invalid one is
   * rejected by that search endpoint, not caught here. */
  sortBy?: Record<string, unknown>;
}

/**
 * One entry from `GET /dashboards`: every dashboard registered in the
 * config-driven pilot, without its widgets. A small static registry, not
 * user-configurable — drives the dashboard switcher and the initial
 * dashboard selection (the `isDefault` entry).
 */
export interface BeDashboardListItem {
  id: string;
  displayName: string;
  /** Who the dashboard is built for. Drives which team `family` the team
   * picker requests (see `abtFamilyForDashboardType` in `useTeams.ts`) —
   * `cre` teams see only `cre-abt` teams, `sre` only `sre-abt`. Omitted only
   * for a definition that predates the field (the deprecated single-variable
   * configuration path). */
  type?: "cre" | "sre" | "cs";
  isDefault: boolean;
  /** Whether this dashboard should show a team selector (from
   * `POST /teams/search`) alongside the dashboard switcher when selected —
   * and default that selector to the signed-in user's own team, once
   * resolved. The selected team scopes widget data client-side via the
   * `__current_team__` filter placeholder (see `teamFilterPlaceholder.ts`
   * in the webapp). */
  isTeamBased: boolean;
}

/**
 * Response of `GET /dashboards/{dashboardId}`: a dashboard's display
 * metadata plus every widget template registered for it. `widgets` is
 * always an array; every dashboard in the registry has at least one.
 */
export interface BeDashboard {
  id: string;
  displayName: string;
  isDefault: boolean;
  /** Descriptive metadata for which team this dashboard targets; not
   * enforced — every dashboard is returned to every caller. */
  targetTeam?: string;
  /** See {@link BeDashboardListItem.isTeamBased}. */
  isTeamBased: boolean;
  widgets: BeDashboardWidget[];
}

/** One team from `POST /teams/search`. `id` is the registry team key,
 * stable across environments (unlike a group id). */
export interface BeTeam {
  id: string;
  name: string;
  family?: string;
}
