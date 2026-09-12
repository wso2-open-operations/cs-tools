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
// MASTER TEST DATA for the E2E suite — VALUES ONLY.
//
// Every spec sources its environment-specific data from here, not just
// case creation. This file holds no locators, no helpers, and no page logic:
// selectors live in `../utils/selectors.ts`, actions in `../pages/`.
//
// Pointing the suite at a different tenant means editing this file and nothing
// else. Values below were captured against **staging**
// (https://support-stg.wso2.com), which is what `.env.e2e` targets.
//

// ─────────────────────────────────────────────────────────────────────────────
// Project fixtures
// ─────────────────────────────────────────────────────────────────────────────

/** Project types this suite exercises.
 *
 * The portal derives feature visibility from `GET /projects/{id}/features`
 * rather than from these labels, but the type still drives a few rules —
 * deployment filtering, case/SR product category, and which severities are on
 * offer — so each type needs its own dedicated project. */
export const ProjectType = {
  SUBSCRIPTION: "Subscription",
  MANAGED_CLOUD_SUBSCRIPTION: "Managed Cloud Subscription",
  CLOUD_SUPPORT: "Cloud Support",
} as const;

export type ProjectType = (typeof ProjectType)[keyof typeof ProjectType];

/** A project the suite runs against, plus the option labels its forms offer.
 *
 * `deployment` and `productVersion` are per-project rather than shared: those
 * dropdowns are populated from the project's own deployments, so the labels
 * differ between projects even for the same project type. */
export interface ProjectFixture {
  /** Project id as it appears in the URL (`/projects/<id>/...`).
   *
   * An empty string means "not yet captured for this environment" — specs must
   * skip rather than run against the wrong project. */
  id: string;
  /** Project name exactly as displayed in the portal. */
  name: string;
  /** Short project key, shown as a chip beside the name on the Overview tab.
   * Empty when not yet captured for this environment. */
  projectKey: string;
  /** The project's type label, for specs that assert type-driven behaviour. */
  type: ProjectType;
  /** Deployment option label, exactly as rendered in the Deployment dropdown.
   * Empty when `autoSelectsDeployment` is true — there is nothing to pick. */
  deployment: string;
  /** Sysid of the deployment above. The case form selects by label, so this is
   * only needed by specs that assert against the API payload or filter by
   * deployment in a URL. */
  deploymentId: string;
  /** True when the case form hides the Deployment field and locks it to the
   * project's primary production deployment. Cloud Support and Cloud Evaluation
   * Support behave this way (`shouldRestrictToPrimaryProductionDeployments` →
   * `hideDeploymentField` in CreateCasePage.tsx), so specs must not try to pick
   * a deployment for them. */
  autoSelectsDeployment: boolean;
  /** Product option label, exactly as rendered. The field itself is labelled
   * "Product Version" on most types but "Product" on Cloud Support. */
  productVersion: string;
  /** The same product's name *without* its version — i.e. `product.label` as
   * the API returns it, not the dropdown text.
   *
   * Needed because the security-report form builds its title from this rather
   * than from the option label (see the auto-fill effect in
   * CreateCasePage.tsx). Cannot be derived from `productVersion` by trimming, so
   * it is recorded explicitly. */
  productName: string;
  /** Whether the case Details tab renders a "Production Version" field.
   *
   * It is omitted when the project's product carries no version — Cloud
   * Support's WSO2 Developer Platform, verified live — so the details assertions
   * must not demand it there. */
  hasProductVersionField: boolean;
  /** Whether the project offers the "Security Report" item in the Get Help
   * dropdown. Gated on the project's SRA write access
   * (`isSecurityReportVisible` in GetHelpDropdown.tsx) — Cloud Support does not
   * have it, so specs assert its absence rather than trying to raise one. */
  hasSecurityReport: boolean;
}

/**
 * The dedicated automation projects, one per project type.
 *
 * `deployment` and `productVersion` still need capturing for every entry — take
 * them from each project's own case form, since the options come from that
 * project's deployments. Specs that need them must skip while they are empty.
 */
export const PROJECTS: Record<ProjectType, ProjectFixture> = {
  [ProjectType.SUBSCRIPTION]: {
    id: "641058e63b5a87103e1e088aa4e45a13",
    name: "Automation Test Customer Project - Subscription",
    projectKey: "",
    type: ProjectType.SUBSCRIPTION,
    deployment: "Production",
    deploymentId: "8f8a33693bee8b503e1e088aa4e45ab4",
    autoSelectsDeployment: false,
    productVersion: "WSO2 API Manager 4.5.0",
    productName: "WSO2 API Manager",
    hasProductVersionField: true,
    hasSecurityReport: true,
  },
  [ProjectType.MANAGED_CLOUD_SUBSCRIPTION]: {
    id: "a0873629eba28f90fcf5f5dabad0cda0",
    name: "Automation Test MS Customer Project - Managed Cloud Subscription",
    projectKey: "AUTOMATIONTESTCUSMSSUB",
    type: ProjectType.MANAGED_CLOUD_SUBSCRIPTION,
    deployment: "Production",
    deploymentId: "f40cf7e53b2a4b9091404c6aa5e45a00",
    autoSelectsDeployment: false,
    productVersion: "WSO2 Identity Server 7.1.0",
    productName: "WSO2 Identity Server",
    hasProductVersionField: true,
    hasSecurityReport: true,
  },
  [ProjectType.CLOUD_SUPPORT]: {
    id: "cd9776ed3ba28b503e1e088aa4e45a81",
    name: "Automation Test Cloud Customer Project - Cloud Support",
    projectKey: "",
    type: ProjectType.CLOUD_SUPPORT,
    // Deployment is hidden and auto-locked to primary production for this type.
    deployment: "",
    deploymentId: "",
    autoSelectsDeployment: true,
    productVersion: "WSO2 Developer Platform",
    productName: "WSO2 Developer Platform",
    hasProductVersionField: false,
    hasSecurityReport: false,
  },
};

// ─────────────────────────────────────────────────────────────────────────────
// Form option vocabularies
// ─────────────────────────────────────────────────────────────────────────────

/** Case severity levels as rendered in the Severity Level dropdown. S4 carries
 * no space before the parenthesis — it is exactly "S4(Query)" (see
 * `CaseSeverityLevel` in src/features/support/constants/supportConstants.ts).
 *
 * Which of these appear depends on the project's `acceptedSeverityValues`:
 * S0 shows only when the project accepts P0, and severity locks to S4 when P4
 * is the only accepted value — so the available set differs by project. */
export const Severity = {
  S0: "S0",
  S1: "S1",
  S2: "S2",
  S3: "S3",
  S4: "S4(Query)",
} as const;

export type Severity = (typeof Severity)[keyof typeof Severity];

/** Issue Type options offered on the case form. */
export const IssueType = {
  TOTAL_OUTAGE: "Total Outage",
  QUESTION: "Question",
  PERFORMANCE_DEGRADATION: "Performance Degradation",
  PARTIAL_OUTAGE: "Partial Outage",
  ERROR: "Error",
  SECURITY_OR_COMPLIANCE: "Security or Compliance",
} as const;

export type IssueType = (typeof IssueType)[keyof typeof IssueType];

// ─────────────────────────────────────────────────────────────────────────────
// Per-flow input data
// ─────────────────────────────────────────────────────────────────────────────

/** Case content submitted by the create-case flow. */
export interface CaseInput {
  /** Goes in the field labelled "Title" — "Case Details" is the section
   * heading above it, not the input. */
  title: string;
  description: string;
  issueType: IssueType;
  severity: Severity;
}

/**
 * Case content per project type. Kept per-type so each project's cases are
 * distinguishable, and because the available severities differ by project.
 *
 * ⚠️ `POST /cases` has no delete counterpart, so every run leaves a permanent
 * record. Keep `description` self-describing so those records stay identifiable
 * in the target environment.
 */
export const CASE_INPUT: Record<ProjectType, CaseInput> = {
  [ProjectType.SUBSCRIPTION]: {
    title: "subscription case",
    description: "This is a test case for subscription project",
    issueType: IssueType.QUESTION,
    severity: Severity.S4,
  },
  [ProjectType.MANAGED_CLOUD_SUBSCRIPTION]: {
    title: "MS subscription case",
    description: "This is a test case for MS subscription project",
    issueType: IssueType.QUESTION,
    severity: Severity.S4,
  },
  [ProjectType.CLOUD_SUPPORT]: {
    title: "Cloud support case",
    description: "This is a test case for cloud support project",
    issueType: IssueType.QUESTION,
    severity: Severity.S4,
  },
};

/** Content submitted by the create-service-request flow.
 *
 * The Request Details fields are **dynamic**: they are rendered from the
 * selected catalog item's ServiceNow variables (`variable.questionText` in
 * VariableFormFields.tsx), so their labels come from the catalog rather than
 * from the frontend. `requestDetailsLabel` and `descriptionLabel` therefore
 * record what those fields are actually called on the target environment. */
export interface ServiceRequestInput {
  /** Catalog accordion to expand, e.g. "Generic Requests". */
  catalog: string;
  /** Radio item to select inside that catalog. Note this is NOT always the same
   * wording as the catalog it lives under. */
  catalogItem: string;
  requestDetails: string;
  description: string;
}

/**
 * Service request content for the Managed Cloud Subscription project.
 *
 * The catalog and item names come from ServiceNow, not the frontend, so they
 * are environment data. Verified live: the "Generic Requests" catalog holds a
 * single item named "General Requests" — the wording genuinely differs.
 *
 * ⚠️ Creates a permanent record on every run, like case creation.
 */
export const SERVICE_REQUEST_INPUT: ServiceRequestInput = {
  catalog: "Generic Requests",
  catalogItem: "General Requests",
  requestDetails: "This is test Generic Request SR for MS sub",
  description: "This is a test Generic Request SR for MS subscription project",
};

/** Short code for each severity, for building case subjects. The display label
 * cannot be used directly: S4's is "S4(Query)", which would produce subjects like
 * "subscription case S4(Query)". */
export const SEVERITY_CODES: Record<Severity, string> = {
  [Severity.S0]: "S0",
  [Severity.S1]: "S1",
  [Severity.S2]: "S2",
  [Severity.S3]: "S3",
  [Severity.S4]: "S4",
};

/** Severities the case matrix covers, per project. */
export const CASE_MATRIX_SEVERITIES: Severity[] = [
  Severity.S1,
  Severity.S2,
  Severity.S3,
  Severity.S4,
];

/**
 * Naming for the case matrix: one case per project type per severity.
 *
 * Subjects are deterministic — `<titlePrefix> <severity code>` — because that is
 * what makes the spec idempotent: it searches for the subject and only creates a
 * case when none is found. Changing a prefix orphans the existing cases and the
 * next run recreates the whole row, so treat these as fixed.
 */
export const CASE_MATRIX: Record<
  ProjectType,
  { titlePrefix: string; descriptionPrefix: string }
> = {
  [ProjectType.SUBSCRIPTION]: {
    titlePrefix: "subscription case",
    descriptionPrefix:
      "This is a test case for subscription project with severity",
  },
  [ProjectType.MANAGED_CLOUD_SUBSCRIPTION]: {
    titlePrefix: "MS subscription case",
    descriptionPrefix:
      "This is a test case for MS subscription project with severity",
  },
  [ProjectType.CLOUD_SUPPORT]: {
    titlePrefix: "Cloud support case",
    descriptionPrefix:
      "This is a test case for cloud support project with severity",
  },
};

/** A case to open and inspect on its detail page. */
export interface CaseView {
  /** Severity the case was raised at, as the header renders it (S1, S2, …). */
  severity: Severity;
  /** Case sysid, as it appears in `/support/cases/<id>`. */
  caseId: string;
  /** Subject shown in the header. */
  subject: string;
  /** Comment on the Activity tab — the case's original description. */
  comment: string;
}

/** One project's set of cases for the view-case spec. */
export interface CaseViewSet {
  projectType: ProjectType;
  /** WSO2 Case ID format for this project. The prefix is project-specific and
   * is NOT the same as `ProjectFixture.projectKey` — Cloud Support's cases read
   * `AUTOMATIONTESTCUSCLSUB-<n>`, verified live. */
  wso2CaseIdPattern: RegExp;
  cases: CaseView[];
}

/**
 * Existing cases to open and assert, one per severity per project. Read-only —
 * these are opened and checked, never modified.
 *
 * Sysids, subjects and comments are environment data, all read off the live
 * pages, so this has to be recaptured for any other tenant.
 *
 * Subjects and comments are pinned per case rather than derived from a pattern,
 * because the naming is not uniform: the Subscription and Cloud Support S4 cases
 * carry no severity suffix and no "…with severity" in their comment (they came
 * from `CASE_INPUT`), while every other case — including MCS's S4 — follows the
 * per-severity naming.
 */
/**
 * The case a call is requested against, and what to request.
 *
 * The reason is fixed rather than stamped per run, which is what lets the spec
 * recognise its own earlier request and not file another: a call request cannot
 * be removed — the delete action cancels it and leaves the record — so an
 * unguarded create would add one on every run.
 *
 * `durationLabel` is the default duration as the card renders it; the spec
 * leaves the duration select untouched.
 */
export const CALL_REQUEST_INPUT = {
  projectType: ProjectType.SUBSCRIPTION,
  caseId: "3f18677f3ba6c35091404c6aa5e45ae0",
  reason: "This is a test call request from Automation Test",
  durationLabel: "30 minutes",
  /** Time of day to request, on tomorrow's date. Late enough in the morning to
   * clear the earliest-allowed time the modal computes from the case severity,
   * which is a floor measured from now. */
  preferredTimeOfDay: "10:00",
  /** What a reschedule moves the request to: the day after tomorrow, at the
   * same time of day, for an hour.
   *
   * The two duration strings differ because the modal and the card disagree on
   * how to say it — the Meeting Duration option reads "1 hour", while the card
   * always renders the raw minutes. */
  rescheduledDurationOption: "1 hour",
  rescheduledDurationLabel: "60 minutes",
  /**
   * Cases whose status sits outside CALL_SCHEDULABLE_CASE_STATUSES, where the
   * Calls tab is withheld altogether rather than merely empty.
   *
   * Read-only fixtures — the suite never changes their status, since doing so
   * would be what makes them stop testing the rule.
   */
  callsUnavailableCaseIds: [
    "b686e2933baa4f103e1e088aa4e45a9b",
    "12350ecf3bee4b103e1e088aa4e45a9b",
  ],
  /**
   * The cancellation test's own request, kept separate from the one above.
   *
   * Cancelling is terminal — it disables both Reschedule and Cancel — so a test
   * that cancelled the shared request would leave the reschedule test with a
   * dead button. Two records, two purposes, same split as the attachment suite's
   * kept and transient cases.
   *
   * The reason deliberately shares no wording with the request above: cards are
   * matched by reason as a substring, so one reason containing the other would
   * make each test find the wrong card.
   *
   * `reason` is a *prefix*: the spec stamps it per run. A cancelled request
   * drops off the tab — it is filtered out of its own list — but one left
   * pending by a run that failed mid-flow does not, and two cards sharing a
   * reason would have the spec acting on whichever it happened to resolve to.
   */
  cancel: {
    reason: "Automation Test call request awaiting cancellation",
    cancellationReason: "Cancelled by the automated test run",
  },
} as const;

/**
 * The product added to a freshly created deployment.
 *
 * ⚠️ Written to a real backend — `POST /deployments/{id}/products`. It attaches
 * to the deployment the same run just created, so it does not accumulate against
 * any pre-existing record.
 *
 * Only Product Name and Version are required by the modal; the rest are filled
 * so the optional fields are exercised too.
 */
export const DEPLOYMENT_PRODUCT_INPUT = {
  /** Option label in the modal's Product Name select. */
  productName: "API Manager",
  /** The same product as the deployment's product list renders it.
   *
   * The two differ: the select offers the short name while the list shows the
   * full product label, so the row's control reads "Edit WSO2 API Manager" for
   * a product picked as "API Manager". Verified live. */
  listedProductName: "WSO2 API Manager",
  version: "4.4.0",
  cores: "4",
  tps: "100",
  description: "Automation test add product.",
  /** What the Manage Product modal changes the description to.
   *
   * Saved on its own: "Save Changes" closes the modal, and the update save that
   * follows sends only the update list, so a description left unsaved would be
   * discarded rather than carried along. */
  updatedDescription: "product details updated by automation test.",
  /** The entry added on the Update History tab.
   *
   * The level select offers only levels above the product's current one, so 12
   * is available because every run adds its product to a deployment it just
   * created — a product with no update history. Re-running against an existing
   * product at level 12 or higher would find no such option. */
  update: {
    level: "12",
    description: "This update level added by automation test.",
    /** What the entry's description is edited to. */
    editedDescription: "Update description edited by automation test.",
    /** A second entry, added after the first. The level select offers only
     * levels above the product's current one, so this must be higher than
     * `level`. */
    nextLevel: "13",
  },
  /** New Core Count and TPS for the manage-product edit, and an invalid value
   * for the validation case.
   *
   * `validateFiniteNonNegative` turns a negative into `undefined`, which
   * `JSON.stringify` then drops — so an invalid number is silently omitted from
   * the request rather than rejected in the form. */
  editedCores: "8",
  editedTps: "200",
  invalidCores: "-5",
} as const;

/**
 * The update-level search the updates suite runs.
 *
 * Product and version are the API's own identifiers as the selects render them,
 * not display names — the options come from `GET /updates/product-update-levels`.
 *
 * Updates access is a per-project feature flag; Cloud Support does not have it
 * (see SIDE_NAV_VISIBILITY), so this runs against Subscription.
 */
export const UPDATES_SEARCH_INPUT = {
  projectType: ProjectType.SUBSCRIPTION,
  productName: "wso2am",
  productVersion: "4.4.0",
  startLevel: "0",
  endLevel: "10",
  /** The level whose details are opened from the results table. Must fall inside
   * the range above. */
  viewLevel: "1",
} as const;

/**
 * The user whose roles the settings suite edits.
 *
 * ⚠️ This is the signed-in account itself — the only user the suite can safely
 * change, since it can restore what it altered. The Lead role is removed and put
 * back within one test.
 *
 * Lead matters beyond the settings page: it implies Portal User on save, and is
 * required to escalate a case past EL3. A run that dies between the two saves
 * leaves the account without it, so the restore runs from `finally`.
 */
export const SETTINGS_USER_INPUT = {
  projectType: ProjectType.SUBSCRIPTION,
  email: "paraparan@wso2.com",
  /** The role toggled off and back on. */
  role: "Lead",
  /**
   * The backend refuses role changes on an account whose supported email domains
   * are not configured, answering 500 with this message. Verified live on
   * staging, where every role save fails this way — so the edit test recognises
   * it and skips rather than reporting a product bug it cannot act on.
   */
  unconfiguredDomainsMessage:
    "supported email domains list for your account has not been defined",
} as const;

/**
 * Project types whose Deployments tab is reachable.
 *
 * The tab is gated on `permissions.hasDeployments`
 * (`filterProjectDetailsTabsByPermissions`), a per-project feature flag rather
 * than anything the type label decides — so this is recorded per project rather
 * than inferred.
 *
 * Cloud Support is deliberately absent: its deployment is hidden and locked to
 * primary production for case creation (`autoSelectsDeployment`), and the
 * deployment fixtures for it are empty, so there is nothing here to assert
 * against.
 */
export const DEPLOYMENT_ACCESS_PROJECTS: ProjectType[] = [
  ProjectType.SUBSCRIPTION,
  ProjectType.MANAGED_CLOUD_SUBSCRIPTION,
];

/**
 * Whether each project's dashboard carries the Outstanding Operations donut.
 *
 * The chart is gated on `showOutstandingOpsChart` (`hasSR || hasCR`) — service
 * request or change request access — not on the project type label. It happens
 * to line up with Managed Cloud Subscription on this tenant, but it is the
 * access that decides, so this is recorded per project rather than inferred
 * from the type.
 *
 * Mirrors the Operations row of SIDE_NAV_VISIBILITY, which the same access
 * governs. The two are kept separate because a chart and a nav item could
 * legitimately diverge; if they ever do, that is a finding rather than a bug in
 * the test.
 */
export const DASHBOARD_OPERATIONS_VISIBILITY: Record<ProjectType, boolean> = {
  [ProjectType.SUBSCRIPTION]: false,
  [ProjectType.MANAGED_CLOUD_SUBSCRIPTION]: true,
  [ProjectType.CLOUD_SUPPORT]: false,
};

/**
 * Project types whose cases lists are covered by the list-cases suite.
 *
 * Every project type reaches the lists the same way — Support Center →
 * Outstanding Cases → the footer buttons — so unlike the case-view sets below
 * this needs no per-project data beyond the project itself.
 *
 * The My Cases half asserts a non-empty list, so a project only belongs here
 * once the session's own account has created a case in it.
 */
export const CASE_LIST_PROJECTS: ProjectType[] = [
  ProjectType.SUBSCRIPTION,
  ProjectType.MANAGED_CLOUD_SUBSCRIPTION,
  ProjectType.CLOUD_SUPPORT,
];

export const CASE_VIEWS: CaseViewSet[] = [
  {
    projectType: ProjectType.SUBSCRIPTION,
    wso2CaseIdPattern: /^AUTOMATIONTESTCUSSUB-\d+$/,
    cases: [
      {
        severity: Severity.S1,
        caseId: "ce1502cf3bee4b103e1e088aa4e45a6d",
        subject: "subscription case S1",
        comment:
          "This is a test case for subscription project with severity S1",
      },
      {
        severity: Severity.S2,
        caseId: "d225c2473ba64b1091404c6aa5e45af0",
        subject: "subscription case S2",
        comment:
          "This is a test case for subscription project with severity S2",
      },
      {
        severity: Severity.S3,
        caseId: "12350ecf3bee4b103e1e088aa4e45a9b",
        subject: "subscription case S3",
        comment:
          "This is a test case for subscription project with severity S3",
      },
      {
        severity: Severity.S4,
        caseId: "62e442073ba64b1091404c6aa5e45a39",
        subject: "subscription case",
        comment: "This is a test case for subscription project",
      },
    ],
  },
  {
    projectType: ProjectType.MANAGED_CLOUD_SUBSCRIPTION,
    wso2CaseIdPattern: /^AUTOMATIONTESTCUSMSSUB-\d+$/,
    cases: [
      {
        severity: Severity.S1,
        caseId: "07962293ebeec310fcf5f5dabad0cdcd",
        subject: "MS subscription case S1",
        comment:
          "This is a test case for MS subscription project with severity S1",
      },
      {
        severity: Severity.S2,
        caseId: "67a6aedb3bea0f1091404c6aa5e45a8f",
        subject: "MS subscription case S2",
        comment:
          "This is a test case for MS subscription project with severity S2",
      },
      {
        severity: Severity.S3,
        caseId: "33b6661f3bea0f1091404c6aa5e45a7e",
        subject: "MS subscription case S3",
        comment:
          "This is a test case for MS subscription project with severity S3",
      },
      {
        // Unlike the other two projects, this project's S4 case follows the same
        // naming as its S1-S3 — it was raised through the case matrix rather than
        // from CASE_INPUT.
        severity: Severity.S4,
        caseId: "c4d62e93ebeec310fcf5f5dabad0cdbb",
        subject: "MS subscription case S4",
        comment:
          "This is a test case for MS subscription project with severity S4",
      },
    ],
  },
  {
    projectType: ProjectType.CLOUD_SUPPORT,
    wso2CaseIdPattern: /^AUTOMATIONTESTCUSCLSUB-\d+$/,
    cases: [
      {
        severity: Severity.S1,
        caseId: "d4e66e1f3bea0f1091404c6aa5e45ae0",
        subject: "Cloud support case S1",
        comment:
          "This is a test case for cloud support project with severity S1",
      },
      {
        severity: Severity.S2,
        caseId: "d8f626d3ebeec310fcf5f5dabad0cd98",
        subject: "Cloud support case S2",
        comment:
          "This is a test case for cloud support project with severity S2",
      },
      {
        severity: Severity.S3,
        caseId: "d407eed33baa4f103e1e088aa4e45a5c",
        subject: "Cloud support case S3",
        comment:
          "This is a test case for cloud support project with severity S3",
      },
      {
        severity: Severity.S4,
        caseId: "4a054a8f3bee4b103e1e088aa4e45a11",
        subject: "Cloud support case",
        comment: "This is a test case for cloud support project",
      },
    ],
  },
];

/** A security report analysis to open and inspect. */
export interface SraView {
  projectType: ProjectType;
  /** SRA sysid, as it appears in
   * `/security-center/security-report-analysis/<id>`. */
  sraId: string;
  /** Subject shown in the header.
   *
   * ⚠️ Pinned literally, and the date in it is the date the SRA was RAISED, not
   * today's. Security report subjects are generated as
   * `<deployment> - <product name> - YYYY-MM-DD` at creation time, so for an
   * existing record that string is fixed. Do not "fix" this by computing today's
   * date — that would break the test the next day.
   */
  subject: string;
  /** WSO2 Case ID format for this project. */
  wso2CaseIdPattern: RegExp;
}

/**
 * Existing security report analyses to open and assert. Read-only.
 *
 * Sysids and subjects are environment data, read off the live pages.
 */
export const SRA_VIEWS: SraView[] = [
  {
    projectType: ProjectType.SUBSCRIPTION,
    sraId: "5fc57257eba20710fcf5f5dabad0cd08",
    subject: "Production - WSO2 API Manager - 2026-08-12",
    wso2CaseIdPattern: /^AUTOMATIONTESTCUSSUB-\d+$/,
  },
  {
    projectType: ProjectType.MANAGED_CLOUD_SUBSCRIPTION,
    sraId: "92d57e1b3b6e4f103e1e088aa4e45a9b",
    subject: "Production - WSO2 Identity Server - 2026-08-12",
    wso2CaseIdPattern: /^AUTOMATIONTESTCUSMSSUB-\d+$/,
  },
];

/** A service request to open and inspect. */
export interface ServiceRequestView {
  projectType: ProjectType;
  /** Service request sysid, as it appears in
   * `/operations/service-requests/<id>`. */
  serviceRequestId: string;
  /** Subject shown in the header. For a service request this is the Request
   * Details value it was raised with, which is why it reuses
   * SERVICE_REQUEST_INPUT rather than repeating the string. */
  subject: string;
  /** WSO2 Case ID format for this project. */
  wso2CaseIdPattern: RegExp;
}

/**
 * Existing service requests to open and assert. Read-only.
 *
 * Sysids are environment data, read off the live pages.
 */
export const SERVICE_REQUEST_VIEWS: ServiceRequestView[] = [
  {
    projectType: ProjectType.MANAGED_CLOUD_SUBSCRIPTION,
    serviceRequestId: "7ef3c6d7eb2ac310fcf5f5dabad0cdcc",
    subject: SERVICE_REQUEST_INPUT.requestDetails,
    wso2CaseIdPattern: /^AUTOMATIONTESTCUSMSSUB-\d+$/,
  },
];

/**
 * Content the service-request comment and attachment suite adds.
 *
 * The comment text is fixed rather than stamped per run: comments cannot be
 * deleted, so the spec recognises its own earlier one and does not post again.
 * The attachment reuses the shared fixture file for the same reason the case
 * suite does — one upload that stays put.
 */
export const SERVICE_REQUEST_ACTIVITY_INPUT = {
  comment: "This is a test comment on a service request from Automation Test",
} as const;

/** Subject of the announcement published to all three automation projects. */
export const SHARED_ANNOUNCEMENT_SUBJECT =
  "[SECURITY Announcement] Lack of access control in the keymanager-operations " +
  "DCR endpoint (WSO2-2025-4483/CVE-2025-9152)";

/** An announcement to open and inspect. */
export interface AnnouncementView {
  projectType: ProjectType;
  /** Announcement sysid, as it appears in `/announcements/<id>`. */
  announcementId: string;
  /** Subject shown in the header. */
  subject: string;
  /** WSO2 Case ID format for this project. */
  wso2CaseIdPattern: RegExp;
}

/**
 * Existing announcements to open and assert. Read-only.
 *
 * The same advisory is published to all three projects, so they share a subject
 * but each carries its own case number and WSO2 id.
 */
export const ANNOUNCEMENT_VIEWS: AnnouncementView[] = [
  {
    projectType: ProjectType.SUBSCRIPTION,
    announcementId: "25fd83573ba28f103e1e088aa4e45a98",
    subject: SHARED_ANNOUNCEMENT_SUBJECT,
    wso2CaseIdPattern: /^AUTOMATIONTESTCUSSUB-\d+$/,
  },
  {
    projectType: ProjectType.MANAGED_CLOUD_SUBSCRIPTION,
    announcementId: "0cfdcb173ba28f103e1e088aa4e45ae8",
    subject: SHARED_ANNOUNCEMENT_SUBJECT,
    wso2CaseIdPattern: /^AUTOMATIONTESTCUSMSSUB-\d+$/,
  },
  {
    projectType: ProjectType.CLOUD_SUPPORT,
    announcementId: "20fd0f173ba28f103e1e088aa4e45ae0",
    subject: SHARED_ANNOUNCEMENT_SUBJECT,
    wso2CaseIdPattern: /^AUTOMATIONTESTCUSCLSUB-\d+$/,
  },
];

/** Expected side-menu visibility for a project: item label to whether it should
 * render. */
export type SideNavVisibility = Record<string, boolean>;

/**
 * Side-menu expectations per project type.
 *
 * Visibility is driven by the project's feature flags rather than its type label
 * (see SideBar.tsx), so these are recorded per project and verified live rather
 * than inferred. Only projects listed here are covered; add an entry to extend
 * the suite.
 *
 * Note "Settings" also renders for these projects but is deliberately not
 * asserted per-item — it is outside the gated set. It is accounted for by
 * SIDE_NAV_UNGATED_ITEMS when checking the list is exhaustive.
 */
/**
 * Sidebar items that render for every project, whatever its flags.
 *
 * "Settings" is a footer item with no permission filter in SideBar.tsx, so it is
 * not part of the gated table above — but it *is* part of the rendered list, and
 * an exhaustiveness check has to expect it or it would read as an unexpected
 * extra.
 */
export const SIDE_NAV_UNGATED_ITEMS: string[] = ["Settings"];

export const SIDE_NAV_VISIBILITY: Partial<
  Record<ProjectType, SideNavVisibility>
> = {
  [ProjectType.SUBSCRIPTION]: {
    Dashboard: true,
    Support: true,
    // Hidden: the Operations item needs service-request or change-request access,
    // which this project does not have.
    Operations: false,
    Updates: true,
    "Security Center": true,
    Engagements: true,
    "Usage & Metrics": true,
    "Project Details": true,
    Announcements: true,
  },
  [ProjectType.MANAGED_CLOUD_SUBSCRIPTION]: {
    Dashboard: true,
    Support: true,
    // Visible here, unlike Subscription: this project has service-request access.
    Operations: true,
    Updates: true,
    "Security Center": true,
    Engagements: true,
    "Usage & Metrics": true,
    "Project Details": true,
    Announcements: true,
  },
  [ProjectType.CLOUD_SUPPORT]: {
    Dashboard: true,
    Support: true,
    // The four hidden items each trace to a feature flag this project lacks:
    // Operations needs SR or CR access, Updates needs updates access, Security
    // Center needs SRA or component analysis, and Usage & Metrics needs both the
    // project flag and the portal-wide one.
    Operations: false,
    Updates: false,
    "Security Center": false,
    Engagements: true,
    "Usage & Metrics": false,
    "Project Details": true,
    Announcements: true,
  },
};

/** A comment to post on a case, and the case it goes on. */
export interface CaseCommentTarget {
  projectType: ProjectType;
  /** Case to comment on — each project's S1 case, also used by the view-case
   * suite. */
  caseId: string;
  text: string;
}

/**
 * Comments posted by the case-comment spec, one per project type.
 *
 * ⚠️ Comments cannot be deleted — there is no delete endpoint — so the spec posts
 * only when this exact text is not already on the case. The text is therefore the
 * idempotency key: changing it makes the next run post a new comment on every
 * project, permanently.
 *
 * The same text is used across projects deliberately: each comment lives on a
 * different case, so there is no ambiguity, and one string keeps the intent
 * obvious.
 */
export const CASE_COMMENTS: CaseCommentTarget[] = [
  {
    projectType: ProjectType.SUBSCRIPTION,
    caseId: "ce1502cf3bee4b103e1e088aa4e45a6d",
    text: "This is a test comment from Automation Test",
  },
  {
    projectType: ProjectType.MANAGED_CLOUD_SUBSCRIPTION,
    caseId: "07962293ebeec310fcf5f5dabad0cdcd",
    text: "This is a test comment from Automation Test",
  },
  {
    projectType: ProjectType.CLOUD_SUPPORT,
    caseId: "d4e66e1f3bea0f1091404c6aa5e45ae0",
    text: "This is a test comment from Automation Test",
  },
];

/** The two files the attachment specs use, shared by every project. */
export const ATTACHMENT_FILES = {
  /** Uploaded once and left in place, so the list, expand, collapse and download
   * tests always have something to work with. */
  kept: {
    /** Relative to the tests/e2e directory. Kept in-repo rather than pointing at
     * a developer's Documents folder, so the specs run anywhere. */
    path: "fixtures/files/screenshot.png",
    name: "screenshot.png",
    /** Size as the list formats it (807102 bytes → "788.2 KB"). Pinned so
     * replacing the fixture fails loudly rather than asserting nothing. */
    size: "788.2 KB",
  },
  /** Uploaded and then removed by the delete test, so it never accumulates. */
  transient: {
    path: "fixtures/files/deleteAttachment.png",
    name: "deleteAttachment.png",
    size: "788.2 KB",
  },
} as const;

/** Cases the attachment specs act on, per project. */
export interface AttachmentTarget {
  projectType: ProjectType;
  /** Case that keeps an uploaded file — used by list, expand, collapse and
   * download. */
  caseId: string;
  /** A different case for the upload-then-delete round trip. Kept separate so
   * the delete never removes the fixture the other tests depend on. */
  deleteCaseId: string;
}

export const ATTACHMENT_TARGETS: AttachmentTarget[] = [
  {
    projectType: ProjectType.SUBSCRIPTION,
    caseId: "07f53bef3b6e43503e1e088aa4e45a38",
    deleteCaseId: "d225c2473ba64b1091404c6aa5e45af0",
  },
  {
    projectType: ProjectType.MANAGED_CLOUD_SUBSCRIPTION,
    caseId: "07962293ebeec310fcf5f5dabad0cdcd",
    deleteCaseId: "c4d62e93ebeec310fcf5f5dabad0cdbb",
  },
  {
    projectType: ProjectType.CLOUD_SUPPORT,
    caseId: "d4e66e1f3bea0f1091404c6aa5e45ae0",
    deleteCaseId: "d417eed3ebeec310fcf5f5dabad0cde8",
  },
];

/** How the list renders an upload date, e.g. "Aug 13, 2026, 9:41 PM". Used on
 * runs that did not upload, where the row keeps its original date and today's
 * cannot be asserted. */
export const ATTACHMENT_DATE_PATTERN =
  /[A-Z][a-z]{2} \d{1,2}, \d{4}, \d{1,2}:\d{2} [AP]M/;

/** Formats shared across every project's cases. */
export const CASE_VIEW_EXPECTATIONS = {
  /** ServiceNow case number format, e.g. CS0441157. */
  caseNumberPattern: /^CS\d+$/,
} as const;

/** Deployment types offered by the Add Deployment modal. Verified live against
 * staging — note there is no plain "Production": the production-like option is
 * "Primary Production". */
export const DeploymentType = {
  DEVELOPMENT: "Development",
  QA: "QA",
  STAGING: "Staging",
  STRESS: "Stress",
  UAT: "UAT",
  PRIMARY_PRODUCTION: "Primary Production",
} as const;

export type DeploymentType =
  (typeof DeploymentType)[keyof typeof DeploymentType];

/** Content submitted by the add-deployment flow. */
export interface DeploymentInput {
  /** Name *prefix*. The spec appends a timestamp because the backend rejects a
   * duplicate name with 409 and there is no delete endpoint, so a fixed name
   * only ever works on the very first run. */
  namePrefix: string;
  /** Name of a deployment that already exists on the project, for exercising the
   * duplicate-name rejection. Durable as a fixture because deployments have no
   * delete endpoint, so once created this name cannot disappear. */
  existingName: string;
  type: DeploymentType;
  /** Goes in the field labelled "Description *" — the modal does not call it
   * "Deployment Description". */
  description: string;
  /** What the Edit Deployment modal changes the description to. */
  updatedDescription: string;
  /** A type other than the default, for editing the type and for creating a
   * deployment that is not production. */
  alternateType: DeploymentType;
  /** Appended to a deployment's name when renaming it. The base name is already
   * unique to the run, so this keeps the new one unique too. */
  renameSuffix: string;
}

/**
 * Deployment to create on the Managed Cloud Subscription project.
 *
 * ⚠️ Creates a permanent record on every run — `POST /projects/{id}/deployments`
 * has no delete counterpart, so these accumulate and cannot be cleaned up. The
 * timestamped name keeps them identifiable and ordered.
 */
export const DEPLOYMENT_INPUT: DeploymentInput = {
  namePrefix: "Automation Test Deployment",
  existingName: "Automation Test Deployment",
  type: DeploymentType.PRIMARY_PRODUCTION,
  description:
    "This is a test deployment for Automation Test MS Customer Project",
  updatedDescription:
    "Deployment description updated by automation test",
  alternateType: DeploymentType.DEVELOPMENT,
  renameSuffix: "renamed",
};

/** Content submitted by the create-security-report flow.
 *
 * A security report is a case raised at `/support/security-report/create`. The
 * form hides Issue Type and Severity and requires at least one attachment.
 *
 * There is no `title` here: for security reports CreateCasePage generates it
 * from the selected deployment, the product name and today's date, overwriting
 * anything typed (see the auto-fill effect in CreateCasePage.tsx). */
export interface SecurityReportInput {
  description: string;
}

/** Attachment used by every security report. Path is relative to the tests/e2e
 * directory, and the file is kept in-repo rather than pointing at a developer's
 * Downloads folder so the specs are portable to other machines and to CI. */
export const SECURITY_REPORT_ATTACHMENT = "fixtures/files/sraattachment.csv";

/**
 * Security report content per project type.
 *
 * ⚠️ Creates a permanent record on every run, for every project type covered.
 * Descriptions name their project so the records stay identifiable.
 */
export const SECURITY_REPORT_INPUT: Record<ProjectType, SecurityReportInput> = {
  [ProjectType.SUBSCRIPTION]: {
    description: "This is a test security report for subscription project",
  },
  [ProjectType.MANAGED_CLOUD_SUBSCRIPTION]: {
    description: "This is a test security report for MS subscription project",
  },
  [ProjectType.CLOUD_SUPPORT]: {
    description: "This is a test security report for cloud support project",
  },
};

/**
 * "Request Product Logs" service request, under the Information Request
 * catalog.
 *
 * This catalog item has several variables rather than the single free-text
 * field the Generic Requests item has, including two date/time inputs. The
 * dates are expressed as offsets so each run submits a valid, recent window
 * rather than a hardcoded date that drifts into the past.
 *
 * ⚠️ Creates a permanent record on every run.
 */
export interface ProductLogsRequestInput {
  catalog: string;
  catalogItem: string;
  /** Value for "Types of product log required". */
  logType: string;
  /** Start Time, as whole days before today. */
  startDaysAgo: number;
  /** End Time, as whole days before today. */
  endDaysAgo: number;
  purpose: string;
  description: string;
}

export const PRODUCT_LOGS_REQUEST_INPUT: ProductLogsRequestInput = {
  catalog: "Information Request",
  catalogItem: "Request Product Logs",
  logType: "Carbon",
  startDaysAgo: 3,
  endDaysAgo: 1,
  purpose: "This is a test Information Request SR for MS subscription project",
  description:
    "This is a test Information Request Description SR for MS subscription project",
};

/**
 * Additional severity coverage, Subscription project only.
 *
 * `CASE_INPUT` already covers S4 for every type; these exercise the rest of the
 * range on one project. Which severities a project actually offers comes from
 * its `acceptedSeverityValues`, so this list is deliberately scoped to the
 * Subscription fixture rather than applied to all types.
 *
 * ⚠️ Each entry creates its own permanent case on every run.
 */
export const SUBSCRIPTION_SEVERITY_CASES: CaseInput[] = [
  {
    title: "subscription case S1",
    description: "This is a test case for subscription project with severity S1",
    issueType: IssueType.QUESTION,
    severity: Severity.S1,
  },
  {
    title: "subscription case S2",
    description: "This is a test case for subscription project with severity S2",
    issueType: IssueType.QUESTION,
    severity: Severity.S2,
  },
  {
    title: "subscription case S3",
    description: "This is a test case for subscription project with severity S3",
    issueType: IssueType.QUESTION,
    severity: Severity.S3,
  },
];
