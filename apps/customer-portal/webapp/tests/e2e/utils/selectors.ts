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
// How the UI is addressed: accessible names, element ids, and test ids.
// Environment-specific data (project ids, form values) lives in
// `../config/testData.ts`.
//

/** Accessible name of the header control that starts the issue/case flow.
 * Rendered by GetHelpDropdown as a split button: this is the primary half
 * (aria-label="Get Help"), which goes straight to the flow. The other half
 * ("More help options") opens the Issue / Service Request / Security Report
 * menu. */
export const GET_HELP_BUTTON = "Get Help";

/** Case-creation form. Its field labels are sibling <Typography> nodes rather
 * than real <label for>, so `getByLabel` does not work here — the MUI Selects
 * are located by the placeholder text their `renderValue` emits, and the
 * inputs by their stable ids. */
export const CREATE_CASE = {
  heading: "Complete Case Details",
  submitButton: "Create Support Case",
  successMessage: "Case created successfully",
  /** Maximum title length enforced by CreateCasePage's handleSubmit and shown
   * by the Title field's counter. */
  titleMaxLength: 160,
  titleCounter: /^\d+\/160$/,
  titleTooLongError: "Title must be 160 characters or fewer.",
  /** Banner messages from `showError` when submit validation rejects. */
  validationErrors: {
    missingTitle: "Please enter a case title.",
    missingDescription: "Please enter a description.",
    missingDeployment: "Please select a deployment type.",
    missingProduct: "Please select a product version.",
  },
  placeholders: {
    deployment: "Select Deployment...",
    /** Reads "Select deployment first" until a deployment is chosen, and
     * "Select Product..." on Cloud Support projects. */
    productVersion: /Select Product Version|Select Product|Select deployment first/,
  },
  ids: {
    title: "#title",
    issueType: "#issue-type-select",
    severity: "#severity-level-select",
  },
  testIds: {
    /** Lexical rich-text editor — a contenteditable, not an <input>. */
    description: "case-description-editor",
  },
} as const;

/** Side navigation. Items are buttons inside the sidebar landmark; which ones
 * render depends on the project's feature flags (see SideBar.tsx). */
export const SIDE_NAV = {
  /**
   * The route each item opens, relative to `/projects/{id}/`.
   *
   * Taken from APP_SHELL_NAV_ITEMS, where every gated item's path is its label
   * slugified. Settings is not in that list — it is a footer link — and carries
   * its own path.
   */
  paths: {
    Dashboard: "dashboard",
    Support: "support",
    Operations: "operations",
    Updates: "updates",
    "Security Center": "security-center",
    Engagements: "engagements",
    "Usage & Metrics": "usage-metrics",
    "Project Details": "project-details",
    Announcements: "announcements",
    Settings: "settings",
  } as Record<string, string>,
  items: {
    dashboard: "Dashboard",
    support: "Support",
    operations: "Operations",
    updates: "Updates",
    securityCenter: "Security Center",
    engagements: "Engagements",
    usageMetrics: "Usage & Metrics",
    projectDetails: "Project Details",
    announcements: "Announcements",
    settings: "Settings",
  },
} as const;

/** Project details page (`/projects/:projectId/project-details`), reached from
 * the side nav. Its tabs are Overview, Deployments and Time Tracking. */
export const PROJECT_DETAILS = {
  navItem: "Project Details",
  pathSegment: "project-details",
  tabs: {
    overview: "Overview",
    deployments: "Deployments",
    timeTracking: "Time Tracking",
  },
} as const;

/** Overview tab of the project details page.
 *
 * Every entry below is a rendered label rather than a value: the values are
 * environment data (dates, tiers, hours) that change, so specs assert the
 * fields are present and populated rather than pinning them. */
export const PROJECT_OVERVIEW = {
  sections: {
    projectInformation: "Project Information",
    contactInformation: "Contact Information",
    serviceHoursAllocations: "Service Hours Allocations",
  },
  labels: {
    projectName: "Project Name",
    createdDate: "Created Date",
    supportTier: "Support Tier",
    goLiveDate: "Go Live Date",
    subscriptionPeriod: "Subscription Period",
    start: "Start",
    end: "End",
    remaining: "Remaining",
    accountManager: "Account Manager",
    queryHours: "Query Hours",
  },
  /** Rendered in place of a value the API did not return — see the `"--"`
   * fallbacks in ProjectInformationCard. */
  emptyValue: "--",
} as const;

/** Add Deployment modal, opened from the Deployments tab.
 *
 * Unusually for this app the fields have real ids and associated labels, so no
 * structural locators are needed here. */
export const ADD_DEPLOYMENT = {
  openButton: "Add Deployment",
  dialogTitle: "Add New Deployment",
  /** The modal's confirm control carries the same name as the button that opens
   * it, so it must be scoped to the dialog. */
  submitButton: "Add Deployment",
  ids: {
    name: "#deployment-name",
    type: "#deployment-type",
    description: "#deployment-description",
  },
} as const;

/** Settings page (`/projects/:projectId/settings`), reached from the side nav's
 * footer item.
 *
 * Which tabs render depends on the signed-in user's ServiceNow role: AI
 * Assistant is admin-only, as are the Add User button and the per-row actions
 * (`canAddOrRemoveUsers`). */
export const SETTINGS = {
  navItem: "Settings",
  pathSegment: "settings",
  tabs: {
    userManagement: "User Management",
    aiAssistant: "AI Assistant",
    registryTokens: "Registry Tokens",
    display: "Display",
  },
  userManagement: {
    /** Column headers of the user list. */
    headers: ["User", "Role", "Status", "Actions"],
    searchPlaceholder: "Search users by name, email, or role...",
    addUserButton: "Add User",
    editUserButton: "Edit user",
    removeUserButton: "Remove user",
    editModal: {
      title: "Edit User Roles",
      saveButton: "Save Changes",
      cancelButton: "Cancel",
      /** Role checkboxes, by their rendered label. */
      roles: {
        admin: "Admin",
        lead: "Lead",
      },
    },
  },
  display: {
    heading: "Display Preferences",
    fontSizeTitle: "Font Size",
    /** Each option's control is labelled "Font size: <label>", and choosing one
     * sets `document.documentElement.style.fontSize` to the matching value. */
    fontSizeOption: (label: string) => `Font size: ${label}`,
    fontSizes: [
      { label: "Small", px: "13px" },
      { label: "Default", px: "16px" },
      { label: "Large", px: "18px" },
      { label: "Extra Large", px: "20px" },
    ],
  },
} as const;

/** Updates page (`/projects/:projectId/updates`), reached from the side nav.
 *
 * The page is the All Updates tab: a filter row over update levels, plus results.
 * The four selects cascade — version needs a product, the start level needs a
 * version, the end level needs a start — and Search stays disabled until all
 * four are set. */
export const UPDATES = {
  navItem: "Updates",
  pathSegment: "updates",
  sectionTitle: "Search Update Levels",
  labels: {
    product: "Product Name *",
    version: "Product Version *",
    startLevel: "Starting Update Level *",
    endLevel: "Ending Update Level *",
  },
  ids: {
    product: "all-updates-product",
    version: "all-updates-version",
    startLevel: "all-updates-start",
    endLevel: "all-updates-end",
  },
  searchButton: "Search",
  /** Disabled until something is selected or a search has been run
   * (`canClear`). Clearing resets the filters, the search and the URL. */
  clearFiltersButton: "Clear Filters",
  /** The placeholder MenuItem's text. Only rendered inside the open menu — a
   * closed select with no value displays a zero-width space, not this, so it
   * cannot be used to assert that a select has been reset. Verified live. */
  placeholders: {
    product: "Select Product",
    version: "Select Version",
    level: "Select Level",
  },
  /** Shown before a search has been run. */
  idleHint:
    "Select product, version, and update level range, then click Search to view updates.",
  /**
   * The results table a search renders (PendingUpdatesList), and the copy shown
   * instead when the range holds nothing.
   *
   * The table replaces a skeleton, so a caller has to wait for one of these two
   * states rather than reading straight after the response.
   */
  results: {
    headers: ["Update Level", "Update Type", "Details"],
    emptyMessage: "No update levels found for the selected criteria.",
    /** The Details column's control, one per row. */
    viewButton: "View",
  },
  /**
   * The update level details page
   * (`/projects/:id/updates/pending/level/:level`), opened by a row's View.
   *
   * Carries the searched product forward as query parameters — note
   * `productBaseVersion` here where the search used `pv`.
   */
  levelDetails: {
    pathSegment: "updates/pending/level",
    summaryLabels: ["Product Name", "Product Version", "Released Update Level"],
    /** Filters over the listed updates. */
    filterButtons: ["All", "Security", "Regular"],
    /** Each listed update shows its number and, when it has one, a description
     * section. The number is rendered inline as "Update Number: <n>". */
    updateNumberPrefix: "Update Number:",
    descriptionHeading: "Description",
    /** Returns to whatever opened this page — it navigates history back
     * (ROUTE_PREVIOUS_PAGE = -1) rather than to a fixed route. */
    backButton: "Back",
  },
  /**
   * The report download, offered beside Search once a search has produced
   * results.
   *
   * There is no intermediate dialog: the button builds the PDF in the browser
   * with jsPDF and saves it as
   * `Update-Summary-<product>-<version>-<start>-<end>.pdf`, which the browser
   * surfaces as a download. (`UpdateLevelsReportModal` exists in the codebase but
   * nothing renders it — only its unit test refers to it.)
   */
  report: {
    downloadButton: "Download Report",
    /** The label while the PDF is being produced; the button is disabled then. */
    generatingButton: "Generating...",
    fileName: (
      product: string,
      version: string,
      startLevel: string,
      endLevel: string,
    ) => `Update-Summary-${product}-${version}-${startLevel}-${endLevel}.pdf`,
  },
  /** Query parameters a search writes onto the URL (replace, not push). */
  urlParams: {
    product: "pn",
    version: "pv",
    startLevel: "sl",
    endLevel: "el",
  },
} as const;

/** Announcements list (`/projects/:projectId/announcements`), reached from the
 * side nav. Announcements are cases underneath, so their rows carry a "CS"
 * number and the detail page reuses the case header. */
export const ANNOUNCEMENTS_LIST = {
  navItem: "Announcements",
  pathSegment: "announcements",
  title: "Announcements",
  description: "View and manage announcements for your project",
  searchPlaceholder: "Search announcements...",
  /** ListResultsBar with `entityLabel: "announcements"`. */
  resultsCountPattern: /Showing (\d+) of (\d+) announcements/,
  emptyMessage: "No announcements yet.",
  /** Row number, matched within the card's whole text — so unanchored, for the
   * same reason the change-request pattern is. */
  numberPattern: /CS\d+/,
  /** Opens the filter panel; becomes "Clear Filters (n)" once one is applied. */
  filtersButton: "Filters",
  clearFiltersButton: (activeCount: number) => `Clear Filters (${activeCount})`,
  /** The only filter announcements offer (ANNOUNCEMENT_FILTER_DEFINITIONS), and
   * it is multi-select. Its element id comes from the definition's `id`, its
   * label from `deriveFilterLabels`. */
  statusFilter: {
    selectId: "status",
    label: "Status",
  },
  /** Sort controls, from the shared ListResultsBar. The order labels depend on
   * the field: a chronological sort reads "Newest first"/"Oldest first", an
   * ordinal one "Descending"/"Ascending" — Updated date is chronological, Status
   * is ordinal. */
  sort: {
    fieldSelectId: "list-sort-field",
    orderSelectId: "list-sort-order",
    fields: {
      updatedDate: { label: "Updated date", value: "updatedOn" },
      status: { label: "Status", value: "state" },
    },
    orders: {
      chronologicalDesc: { label: "Newest first", value: "desc" },
      chronologicalAsc: { label: "Oldest first", value: "asc" },
      ordinalDesc: { label: "Descending", value: "desc" },
      ordinalAsc: { label: "Ascending", value: "asc" },
    },
  },
  /** ListPagination is given no explicit page sizes here, so it uses its own
   * defaults; the list starts at ANNOUNCEMENTS_PAGE_SIZE. */
  defaultRowsPerPage: 10,
  rowsPerPageOptions: [5, 10, 25, 50],
} as const;

/** Change requests list (`/projects/:projectId/operations/change-requests`),
 * reached from the Operations hub or the dashboard's operations donut.
 *
 * The title depends on router *state* rather than the URL — `outstandingOnly`,
 * `actionRequired` and `scheduledOnly` all land on this same path — so a direct
 * navigation always gets the unfiltered "All Change Requests" view. */
export const CHANGE_REQUESTS_LIST = {
  pathSegment: "operations/change-requests",
  titles: {
    all: "All Change Requests",
    outstanding: "Outstanding Change Requests",
    actionRequired: "Action Required Change Requests",
    scheduled: "Upcoming Change Requests",
  },
  descriptions: {
    outstanding: "Manage and track outstanding change requests",
    scheduled: "Upcoming scheduled change requests",
  },
  /** The list/calendar switch (CHANGE_REQUESTS_VIEW_TABS_CONFIG), rendered by
   * TabBar as `role="tab"` with `aria-selected`. */
  viewTabs: {
    list: "List View",
    calendar: "Calendar View",
  },
  /** Shown when the list has nothing to show — the second only once a search or
   * filter has been applied. */
  emptyMessage: "No change requests yet.",
  emptyRefinedMessage:
    "No change requests found. Try adjusting your filters or search query.",
  /**
   * A change request's number, as the row and the detail header render it.
   *
   * Deliberately unanchored: it is used with `hasText` to pick rows out, and a
   * row's text is the whole card — "Medium Add/Remove Certificates CHG0038759
   * Scheduled | SR: CS0441440 …" — so an anchored pattern matches nothing and the
   * list reads as empty.
   */
  numberPattern: /CHG\d+/,
} as const;

/** MUI TablePagination's default labels.
 *
 * Every paginated list in the portal renders the same control, so the strings
 * are identical — the dashboard's cases table records its own copy under
 * `DASHBOARD.casesTable.pagination` because it also pins that table's page
 * sizes. */
export const MUI_PAGINATION = {
  rowsPerPageLabel: "Rows per page:",
  nextPageButton: "Go to next page",
  previousPageButton: "Go to previous page",
  /** "1–10 of 87". The separator is an en dash; a hyphen is accepted too so a
   * locale or MUI change does not break the match. */
  displayedRowsPattern: /(\d+)[–-](\d+) of (\d+)/,
} as const;

/** The deployments list on the Deployments tab. */
export const DEPLOYMENTS_LIST = {
  /** `rowsPerPage` starts at 10 in ProjectDeployments. */
  defaultRowsPerPage: 10,
  /** The page sizes ListPagination is given here — fewer than its own default
   * set, which includes 50. */
  rowsPerPageOptions: [5, 10, 25],
  /** Shown in place of the list when a project has no deployments. */
  emptyMessage: "It seems there are no deployments associated with this project.",
} as const;

/** Edit Deployment modal, opened from a deployment card's toolbar.
 *
 * The toolbar control is per card but its accessible name is not — every card
 * offers an identically named "Edit deployment" — so it has to be reached
 * through the card it belongs to. */
export const EDIT_DEPLOYMENT = {
  openButton: "Edit deployment",
  dialogTitle: "Edit Deployment",
  dialogDescription: "Update deployment name, type, and description.",
  submitButton: "Update",
  cancelButton: "Cancel",
  ids: {
    name: "#edit-deployment-name",
    type: "#edit-deployment-type",
    description: "#edit-deployment-description",
  },
} as const;

/** Delete-deployment confirmation, opened from a deployment card's toolbar.
 *
 * "Delete" is a deactivation, not a removal: confirming PATCHes
 * `{ active: false }` to the same endpoint the edit modal uses, and the
 * deployment drops out of the list. Like the edit control, the toolbar button is
 * named identically on every card, so it has to be reached through its card. */
export const DELETE_DEPLOYMENT = {
  openButton: "Delete deployment",
  dialogTitle: "Confirm Action",
  confirmButton: "Confirm",
  goBackButton: "Go Back",
  /** The dialog names the deployment it is about, which is what tells the right
   * card's dialog from another's. */
  confirmMessage: (name: string) =>
    `Are you sure you want to delete the deployment "${name}"? This action cannot be undone.`,
} as const;

/** Add WSO2 Product modal, opened from an expanded deployment on the
 * Deployments tab.
 *
 * The deployment cards are MUI Accordions with `unmountOnExit`, so the Add
 * Product button does not exist until the card is expanded. Like the Add
 * Deployment modal, the fields carry real ids. */
export const ADD_PRODUCT = {
  openButton: "Add Product",
  dialogTitle: "Add WSO2 Product",
  dialogDescription: "Add a WSO2 product to this deployment environment.",
  /** The modal's confirm control repeats the name of the button that opened it,
   * so it must be scoped to the dialog. */
  submitButton: "Add Product",
  cancelButton: "Cancel",
  ids: {
    productName: "#product-name",
    version: "#product-version",
    cores: "#product-cores",
    tps: "#product-tps",
    description: "#product-description",
  },
  labels: {
    productName: "Product Name *",
    version: "Version *",
    cores: "Core Count",
    tps: "TPS (Transactions Per Second)",
    description: "Description",
  },
  /** Each listed product carries a per-row edit control whose accessible name
   * embeds the product's label — the reliable marker that it is listed, since
   * the row's own text has no id or test id. */
  rowEditButton: (productLabel: string) => `Edit ${productLabel}`,
} as const;

/** Delete-product confirmation, opened from a listed product's delete control.
 *
 * Like deleting a deployment, this is a deactivation: confirming PATCHes
 * `{ active: false }` to the same endpoint the Manage Product modal saves
 * through, and the product drops out of the list. */
export const DELETE_PRODUCT = {
  /** The row control carries the product's label, unlike the deployment
   * toolbar's, so it needs no scoping to a card. */
  openButton: (productLabel: string) => `Delete ${productLabel}`,
  dialogTitle: "Confirm Action",
  confirmButton: "Confirm",
  goBackButton: "Go Back",
  /** The dialog names the product with its version — "WSO2 API Manager (4.4.0)"
   * — which is what tells the right product's dialog from another's. */
  confirmMessage: (productLabel: string, version: string) =>
    `Are you sure you want to delete "${productLabel} (${version})"? This action cannot be undone.`,
} as const;

/** Manage Product modal, opened from a listed product's edit control.
 *
 * Two tabs over one record: Product Details saves through "Save Changes", which
 * closes the modal, while Update History saves through "Add Update" in the same
 * footer. Both go to `PATCH /deployments/{id}/products/{id}`, but the update
 * save sends only `{ updates }` — it does not carry a pending description edit,
 * so the two have to be saved separately. */
export const MANAGE_PRODUCT = {
  dialogTitle: "Manage Product",
  dialogDescription: "Update product details and manage update history",
  tabs: {
    details: "Product Details",
    history: "Update History",
  },
  ids: {
    description: "#manage-product-description",
    cores: "#manage-product-cores",
    tps: "#manage-product-tps",
    /** Add New Update section, on the Update History tab. */
    updateLevel: "#new-update-level",
    appliedOn: "#new-applied-on",
    updateDescription: "#new-update-description",
  },
  /** Per-entry controls on the Update History tab. The level is rendered with a
   * "U" prefix — "U12" — in both the row's controls and the current-level
   * readout. */
  updateRow: {
    editButton: (level: string) => `Edit update U${level}`,
    deleteButton: (level: string) => `Delete update U${level}`,
    /** The inline edit form's fields carry no ids, but their labels differ from
     * the Add New Update section's — "Update Level" vs "Update Level *",
     * "Date" vs "Applied On *", "Description" vs "Description (Optional)" — so
     * an exact label match tells them apart. */
    levelLabel: "Update Level",
    dateLabel: "Date",
    descriptionLabel: "Description",
    saveButton: "Save",
    cancelButton: "Cancel",
  },
  /** Readout above the Add New Update section. */
  currentLevelLabel: "Current Update Level:",
  currentLevelValue: (level: string) => `U${level}`,
  /** Footer controls. Which of the two save buttons renders depends on the tab.
   * The Update History tab has an in-tab Add Update button too, but only when it
   * manages its own form — inside this modal it does not, so there is exactly
   * one. */
  saveButton: "Save Changes",
  addUpdateButton: "Add Update",
  closeButton: "Close",
  updateAddedMessage: "Update history entry added successfully.",
} as const;

/** Get Help dropdown menu items (the arrow half of the split button). */
export const GET_HELP_MENU = {
  trigger: "More help options",
  items: {
    issue: "Issue",
    serviceRequest: "Service Request",
    securityReport: "Security Report",
  },
} as const;

/** Create-service-request form
 * (`/projects/:projectId/support/service-requests/create`).
 *
 * Deployment and Product reuse the same controls as the case form. The Request
 * Details fields below are rendered from the selected catalog item's ServiceNow
 * variables (VariableFormFields.tsx), so they have no stable ids and no
 * `<label for>` — see ServiceRequestCreatePage for how they are addressed. */
export const CREATE_SERVICE_REQUEST = {
  heading: "New Service Request",
  requestTypeHeading: "Select Request Type",
  detailsHeading: "Request Details",
  submitButton: "Create Service Request",
  successMessage: /Service request .*created successfully/,
  /** URL segment a created service request's detail page carries. */
  detailPathSegment: "service-requests",
  testIds: {
    /** The shared rich-text Editor hardcodes this id, so the description field
     * carries the same test id here as on the case form. */
    description: "case-description-editor",
  },
} as const;

/** Create-security-report form
 * (`/projects/:projectId/support/security-report/create`).
 *
 * Rendered by the same CreateCasePage as a normal case, with `isSecurityReport`
 * derived from the path. That flag hides Issue Type and Severity, adds the
 * attachment section, and forces skipChat — so unlike the case form, this route
 * can be opened directly by URL. */
export const CREATE_SECURITY_REPORT = {
  submitButton: "Submit Security Report",
  attachSectionLabel: "Attach Security Report",
  /** The dropzone that opens the upload modal. */
  uploadDropzone: "Upload files",
  /** Shown when submitting with no attachment. */
  missingAttachmentError:
    "Please attach at least one security report file.",
  /** The shared UploadAttachmentModal, opened by the dropzone. Its confirm
   * button reads "Add" (not "Upload") because CreateCasePage passes `onSelect`,
   * so the file is held locally until the report itself is submitted. */
  uploadModal: {
    title: "Upload Attachment",
    confirmButton: "Add",
    nameField: "Attachment name",
  },
} as const;

/** Security Center (`/projects/:projectId/security-center`), whose default tab is
 * the Security Report Analysis list. */
export const SECURITY_CENTER = {
  pathSegment: "security-center",
  /** The list search covers case number, title AND description — which is what
   * lets a report be found by its stable description rather than by its
   * date-stamped generated title. */
  searchPlaceholder: /Search reports/,
  emptyMessage: "No reports found.",
} as const;

/** The comment box on a case's Activity tab (ActivityCommentInput). */
export const CASE_COMMENT_INPUT = {
  /** Placeholder of the rich-text editor. */
  placeholder: "Write a comment...",
  /** The send control's accessible name — it carries an aria-label as well as a
   * matching tooltip. */
  sendButton: "Send comment",
} as const;

/** The case detail page's Details tab (CaseDetailsDetailsPanel).
 *
 * Every label below is copied verbatim from the component — including
 * "Production Version", which really is spelt that way. The overview ID label is
 * dynamic ("Case ID" for a case, "Service Request Overview" / "Security Report
 * Analysis ID" / "Engagement ID" for the other kinds), so only the case value
 * appears here. */
export const CASE_DETAILS_PANEL = {
  tab: "Details",
  sections: {
    caseOverview: "Case Overview",
    escalationLevels: "Escalation Levels",
    productEnvironment: "Product & Environment",
    customerInformation: "Customer Information",
    watchList: "Watch List",
  },
  fields: {
    caseOverview: [
      "Case ID",
      "WSO2 Case ID",
      "Status",
      "Severity",
      "Category",
      "Created by",
      "Created Date",
      "Last Updated",
    ],
    productEnvironment: ["Product Name", "Production Version"],
    customerInformation: ["Organization", "Project"],
  },
} as const;

/** The Attachments tab of a case (CaseDetailsAttachmentsPanel).
 *
 * Unlike cases, comments and deployments, attachments CAN be deleted — there is a
 * delete affordance per row plus a confirmation dialog — so the attachment spec
 * cleans up after itself instead of accumulating records. */
export const CASE_ATTACHMENTS = {
  /** The tab label carries a live count, e.g. "Attachments (0)". */
  tab: /^Attachments/,
  uploadButton: "Upload Attachment",
  emptyMessage: "No attachments found.",
  uploadModal: {
    title: "Upload Attachment",
    /** Opens the OS file picker via the dropzone's hidden input. */
    chooseFileButton: "Choose file",
    /** Reads "Upload" here — the "Add" variant only appears where the caller
     * passes `onSelect` to hold the file locally, which this panel does not. */
    confirmButton: "Upload",
    nameField: "Attachment name",
  },
  /** Each row reads "<name> | <size> | • | Uploaded by <email> | • | <date>". */
  row: {
    /** formatFileSize output, e.g. "788.2 KB", "30 B", "1.2 MB". */
    sizePattern: /\d+(\.\d+)? (B|KB|MB)/,
    uploadedByPrefix: /Uploaded by \S+/,
    /** formatDateTime output, e.g. "Aug 13, 2026, 9:41 PM". */
    datePattern: /[A-Z][a-z]{2} \d{1,2}, \d{4}, \d{1,2}:\d{2} [AP]M/,
  },
  /** Image rows carry a preview toggle; the label flips once expanded. */
  expandImageButton: "Expand image",
  collapseImageButton: "Collapse image",
  /** The tab label carries the count, e.g. "Attachments (1)". */
  tabCountPattern: /^Attachments \((\d+)\)/,
  deleteModal: {
    title: "Confirm Action",
    confirmButton: "Confirm",
    cancelButton: "Go Back",
  },
} as const;

/** Project dashboard (`/projects/:projectId/dashboard`), the side nav's first
 * item and the landing page for a project. */
export const DASHBOARD = {
  navItem: "Dashboard",
  pathSegment: "dashboard",
  /**
   * The four stat cards across the top (DASHBOARD_STATS), in render order.
   *
   * `clickable` records which ones open a list: Avg. Response Time is named in
   * the grid's `nonClickableKeys`, so it is a plain card while the other three
   * are buttons. Unlike the Support Center cards, none of these carry their
   * value in the accessible name, so they cannot be counted as "buttons
   * starting with a digit".
   */
  statCards: [
    { label: "Action Required", clickable: true },
    { label: "Outstanding", clickable: true },
    { label: "Closed (Last 30d)", clickable: true },
    { label: "Avg. Response Time (Last 30d)", clickable: false },
  ],
  /**
   * Where each clickable stat card leads (DashboardPage's `onStatClick`).
   *
   * The item pages are nested *under* the dashboard route, so their paths carry
   * the `dashboard/` segment — `navigate("action-required")` from the dashboard
   * resolves against the route tree, not the bare project path.
   *
   * Card label and destination title differ on every one of these: the card
   * reads "Closed (Last 30d)" while the page it opens is titled "Closed items
   * (last 30d)", so both are recorded.
   *
   * Avg. Response Time is absent on purpose — it is in the grid's
   * `nonClickableKeys` and opens nothing.
   */
  statCardTargets: [
    {
      label: "Action Required",
      pathSegment: "dashboard/action-required",
      title: "Action Required Items",
      description: "Items awaiting your response",
      emptyMessage: "No action required items.",
    },
    {
      label: "Outstanding",
      pathSegment: "dashboard/outstanding-interactions",
      title: "Outstanding Items",
      description: "View all currently active and unresolved items",
      emptyMessage: "No outstanding items.",
    },
    {
      label: "Closed (Last 30d)",
      pathSegment: "dashboard/closed-last-30d",
      title: "Closed items (last 30d)",
      description:
        "Successfully closed and resolved items during the last 30 days",
      emptyMessage: "No closed items in the last 30 days.",
    },
  ],
  /** The one card that opens nothing. */
  nonClickableStatCard: "Avg. Response Time (Last 30d)",
  /** Sections below the stat cards. */
  sections: {
    /** Used twice on the page — the severity donut and the cases table beneath
     * it carry the same title (CASES_TABLE_HEADER_TITLE) — so specs assert it is
     * present rather than matching a single element.
     *
     * The table is also withheld on mid-size touch viewports, where the layout
     * is too cramped for it; a desktop browser gets both. */
    outstandingCases: "Outstanding Support Cases",
    /** Gated on `permissions.hasEngagements`, the same flag as the Engagements
     * nav item — which SIDE_NAV_VISIBILITY records as on for all three fixture
     * projects. */
    outstandingEngagements: "Outstanding Engagements",
    /** Gated on `showOutstandingOpsChart` (`hasSR || hasCR`) — the same access
     * that governs the Operations nav item, so it is present for some projects
     * and withheld for others. Expected visibility per project lives in
     * DASHBOARD_OPERATIONS_VISIBILITY. */
    outstandingOperations: "Outstanding Operations",
  },
  /**
   * The Outstanding Support Cases table at the foot of the dashboard.
   *
   * Shares its title with the severity donut above it, so the subtitle is what
   * identifies this card specifically.
   *
   * Each row is a `role="row"` carrying the case's number as "ID: CS…" — not as
   * a bare "CS…", which is how the case detail page renders it. Clicking a row
   * opens that case.
   */
  casesTable: {
    subtitle: "Track and manage all active support tickets",
    /**
     * The table's own My Cases / All Cases switch (DASHBOARD_CASES_VIEW_TABS).
     *
     * These are tabs, not links: choosing one re-queries the table in place with
     * `createdByMe` set, leaving the dashboard URL untouched — unlike Support
     * Center's "View my cases", which navigates to a filtered list page. So the
     * request body and the rows are the only evidence the switch took.
     *
     * TabBar renders each as `role="tab"` with `aria-selected`.
     */
    viewTabs: {
      myCases: "My Cases",
      allCases: "All Cases",
    },
    /** Prefix the Created by column puts before each row's creator. */
    createdByPrefix: "Created by",
    /** Opens the filter panel. Becomes "Clear Filters (n)" once a filter is
     * applied, so this label only matches while none is. */
    filtersButton: "Filters",
    /** The same button once a filter is applied — it clears rather than toggles
     * (`formatCasesTableClearFiltersLabel`). The count makes the label change
     * with the number of active filters. */
    clearFiltersButton: (activeCount: number) =>
      `Clear Filters (${activeCount})`,
    /**
     * Filters in the panel. Each Select takes its element id from the field's
     * `filterKey` and its label from `deriveFilterLabels`, so the severity
     * control is `#severityIds` labelled "Severity".
     */
    filters: {
      severity: {
        label: "Severity",
        selectId: "severityIds",
        /** The severity option to choose, and the id it sends. Option labels are
         * display names (`mapCasesTableFilterOptionLabel`), not the API's own
         * labels — "Low (P4)" is shown as "S4(Query)" — while the request
         * carries the numeric id. */
        option: { label: "S4(Query)", id: 13 },
      },
    },
    /** MUI TablePagination at the foot of the card. The table starts at 5 rows
     * (`useState(5)` in CasesTable) and offers 5, 10, 25 and 50. */
    pagination: {
      rowsPerPageLabel: "Rows per page:",
      defaultRowsPerPage: 5,
      options: [5, 10, 25, 50],
      /** MUI's default `getItemAriaLabel` strings. */
      nextPageButton: "Go to next page",
      previousPageButton: "Go to previous page",
      /** MUI's `labelDisplayedRows`: "1–5 of 36". The separator is an en dash,
       * but a hyphen is accepted too so a locale or MUI change does not break
       * the match. */
      displayedRowsPattern: /(\d+)[–-](\d+) of (\d+)/,
    },
    /** Marks a data row; the header row has no case id. */
    rowIdPattern: /ID: CS\d+/,
    /** Captures the case number out of a row's text. */
    rowCaseNumberPattern: /ID: (CS\d+)/,
  },
  /**
   * The legend of the Outstanding Operations donut. Each entry opens the
   * matching operations list, narrowed to outstanding.
   *
   * Only rendered where the Operations chart is — see
   * DASHBOARD_OPERATIONS_VISIBILITY.
   *
   * `outstandingOnly` travels in router state rather than the URL, so the path
   * alone cannot tell this apart from the unfiltered list. The "Outstanding …"
   * title is what proves the state was handed over, which is why it is asserted
   * as well as the path.
   */
  operationsLegend: [
    {
      label: "Service Requests (SR)",
      pathSegment: "operations/service-requests",
      title: "Outstanding Service Requests",
      description: "Manage and track outstanding service requests",
    },
    {
      label: "Change Requests (CR)",
      pathSegment: "operations/change-requests",
      title: "Outstanding Change Requests",
      description: "Manage and track outstanding change requests",
    },
  ],
  /**
   * The donut itself, whose slices are clickable in the same order as the
   * legend below it.
   *
   * Recharts renders each slice as an SVG `path.recharts-sector` with no
   * accessible name, so they can only be addressed structurally and by position.
   *
   * How many are drawn depends on the data, and whether a series with a count of
   * zero gets a sector is not something these specs establish — so a slice's
   * index must not be taken to mean the legend entry at the same index. Specs
   * read the severity back from the list a slice opens instead. Every fixture
   * project currently has cases at all four severities, so the question has not
   * arisen in practice.
   */
  severityChartSlice: "path.recharts-sector",
  /**
   * The severity legend of the Outstanding Support Cases donut. Each entry
   * opens the cases list filtered to that severity.
   *
   * `severityId` is the numeric id the URL carries
   * (SEVERITY_LEGEND_KEY_TO_ID); `title` and `description` are what the
   * destination renders, both derived from that id rather than from the data,
   * so they hold even for a severity with no outstanding cases.
   *
   * Entries are plain text, not buttons — the whole legend row is the click
   * target.
   *
   * S0 - Catastrophic is deliberately absent: it renders only where
   * `acceptedSeverityValues` includes P0, which is a per-project policy rather
   * than something every project shows.
   */
  severityLegend: [
    {
      label: "S1 - Critical",
      severityId: "10",
      title: "Outstanding S1 Cases",
      description: "Manage and track S1 outstanding support cases",
    },
    {
      label: "S2 - High",
      severityId: "11",
      title: "Outstanding S2 Cases",
      description: "Manage and track S2 outstanding support cases",
    },
    {
      label: "S3 - Medium",
      severityId: "12",
      title: "Outstanding S3 Cases",
      description: "Manage and track S3 outstanding support cases",
    },
    {
      label: "S4 (Query) - Low",
      severityId: "13",
      title: "Outstanding S4 Cases",
      description: "Manage and track S4 outstanding support cases",
    },
  ],
} as const;

/** Calls tab of a case detail page, and its Request Call modal. */
export const CASE_CALLS = {
  /** Tab label carries a live count ("Calls (0)"), so it is matched on the
   * prefix — same as the Attachments tab. */
  tab: /^Calls/,
  /** A tab every case carries whatever its status — the marker that the tab
   * strip has rendered, so an absent Calls tab means withheld rather than not
   * yet drawn. */
  alwaysPresentTab: "Activity",
  /** Statuses that allow call scheduling (CALL_SCHEDULABLE_CASE_STATUSES). A
   * case outside these — a new one, or a closed one — has no Calls tab at all. */
  schedulableStatuses: [
    "Work In Progress",
    "Awaiting Info",
    "Waiting on WSO2",
    "Solution Proposed",
    "Reopened",
  ],
  requestButton: "Request Call",
  modal: {
    title: "Request Call",
    description: "Schedule a call with our support team.",
    /** `<input type="datetime-local">`, so it takes a "YYYY-MM-DDTHH:mm" value.
     * Addressed by id: the label carries a required marker and MUI does not tie
     * it to the input with `for`. */
    preferredTimeInputId: "preferred-time-0",
    durationLabel: "Meeting Duration *",
    reasonLabel: "Reason *",
    /** Same wording as the button that opened the modal, so any locator for it
     * must be scoped to the dialog. */
    submitButton: "Request Call",
    cancelButton: "Cancel",
    /** Adds another preferred time. Disabled at the maximum. */
    addTimeButton: "Add preferred time",
    /** Removes one, by 1-based position. The first row's is always disabled —
     * a request must keep at least one preferred time. */
    removeTimeButton: (position: number) => `Remove preferred time ${position}`,
    /** MAX_PREFERRED_TIMES in RequestCallModal. */
    maxPreferredTimes: 3,
    /** Every duration on offer, in the order the select lists them. */
    durationOptions: ["15 minutes", "30 minutes", "45 minutes", "1 hour"],
  },
  /** A call request as CallRequestCard renders it. */
  card: {
    title: "Call Request",
    preferredTimesLabel: "Preferred Times",
    durationLabel: "Duration",
    /** The card always renders raw minutes, whatever the modal called the
     * option ("1 hour" is shown as "60 minutes"). */
    durationPattern: /\d+ minutes/,
    reasonLabel: "Reason / Notes",
    /** Rendered in place of a value the request does not carry. */
    emptyValue: "--",
    /** State a newly created request opens in. */
    pendingState: "Pending on WSO2",
    /** Opens the same modal in edit mode. Disabled once the request reaches a
     * terminal state, so a request an engineer has closed cannot be edited. */
    rescheduleButton: "Reschedule",
    /** Opens the cancellation dialog. Disabled on a terminal request, which is
     * what makes cancelling a one-way move. */
    cancelButton: "Cancel",
  },
  /** Confirmation dialog behind a card's Cancel button. Its own Reason is
   * required and is separate from the reason the call was requested for. */
  cancelModal: {
    title: "Confirm Action",
    reasonInputId: "cancel-call-reason",
    confirmButton: "Confirm",
    goBackButton: "Go Back",
  },
  /** The same modal in edit mode, reached from a card's Reschedule button.
   *
   * It offers only the preferred times and the duration — the Reason field is
   * not rendered when editing (`{!isEdit && …}` in RequestCallModal), so a
   * reschedule cannot change why the call was asked for. */
  editModal: {
    title: "Edit Call Request",
    description:
      "Update preferred times and meeting duration for this call request.",
    submitButton: "Update Call Request",
  },
  /** Shown *instead of* the request modal when the profile has no time zone
   * (CallsPanel.handleOpenModal returns early). A prerequisite of the account,
   * not something a spec can set up, so specs name it in their failure message
   * rather than trying to work around it. */
  timeZoneDialogTitle: "Time Zone Not Set",
} as const;

/** Support Center landing page (`/projects/:projectId/support`), reached from
 * the side nav. Its Outstanding Cases card is the only entry point to the
 * filtered "my cases" list — the URL is otherwise undiscoverable in the UI. */
export const SUPPORT_CENTER = {
  navItem: "Support",
  pathSegment: "support",
  outstandingCases: {
    title: "Outstanding Cases",
    /** Footer buttons of the card, in render order. */
    myCasesButton: "View my cases",
    allCasesButton: "View all cases",
  },
  /** Every list reached from Support Center offers this, because the card sets
   * `returnTo` on navigation. The cases list falls back to a plain "Back" when
   * it is opened any other way, so this label doubles as a check that the
   * card wired `returnTo` up. */
  backButton: "Back to Support Center",
  /**
   * The four stat cards across the top (SUPPORT_STAT_CONFIGS), in render order,
   * each with the list it opens (SupportPage's `handleStatClick`).
   *
   * `label` is the card; `title` is the heading of the page it opens. They are
   * not always the same string — the "Resolved via Chat" card is parenthesised
   * and its destination heading is not.
   *
   * A card navigates whatever its count, including zero, so specs assert the
   * destination page rather than its rows.
   */
  statCards: [
    {
      label: "Outstanding Cases",
      pathSegment: "support/cases",
      query: "statusFilter=active",
      title: "Outstanding Cases",
      description: "Cases that are currently in progress",
    },
    {
      label: "Active Chats",
      pathSegment: "support/conversations",
      query: "statusFilter=active",
      title: "Active Chats",
      description: "Conversations currently in progress",
    },
    {
      label: "Resolved Cases (Last 30d)",
      pathSegment: "support/cases",
      query: "statusFilter=resolved",
      title: "Resolved Cases (Last 30d)",
      description: "Cases that have been resolved during the last 30 days",
    },
    {
      label: "Resolved via Chat (Last 30d)",
      pathSegment: "support/conversations",
      query: "statusFilter=resolvedViaChat",
      title: "Resolved via Chat Last 30d",
      description:
        "Conversations that were resolved via chat during the last 30 days",
    },
  ],
} as const;

/** Cases list (`/projects/:projectId/support/cases`).
 *
 * One page serves several lists: `?createdByMe=true` makes it My Cases, and its
 * heading, description and Created By filter all change with it (AllCasesPage). */
export const CASES_LIST = {
  pathSegment: "support/cases",
  /** Default placeholder of ListSearchPanel's input. */
  searchPlaceholder: /Search cases/,
  /** Query string "View my cases" navigates to. */
  myCasesQuery: "createdByMe=true",
  /** ListPageHeader copy, selected by the query string. */
  myCases: {
    title: "My Cases",
    description: "Manage and track your support cases",
  },
  allCases: {
    title: "All Cases",
    description: "Manage and track all your support cases",
  },
  /** ListResultsBar renders "Showing X of Y cases". */
  resultsCountPattern: /Showing (\d+) of (\d+) cases/,
  /** Prefix ListCard puts before the creator on every row that has one. */
  createdByPrefix: "Created by ",
  /** Opens the filter panel (ListSearchBar). The panel is collapsed by default,
   * so no filter label is in the DOM until this is clicked. Its label becomes
   * "Clear Filters (n)" once any filter is applied — these specs apply none. */
  filtersButton: "Filters",
  /** Label of the Created By filter (deriveFilterLabels("createdBy")), which is
   * withheld on My Cases since the list is already narrowed to one creator. */
  createdByFilterLabel: "Created By",
  /** A filter offered on every cases list, whatever the query string. Used to
   * prove the panel is open, so "Created By is absent" cannot pass merely
   * because nothing is rendered. */
  severityFilterLabel: "Severity",
} as const;

/** Case detail page (`/projects/:projectId/support/cases/:caseId`).
 *
 * The state-change buttons come from `getAvailableCaseActions(status)`: an open
 * case offers the "Closed" action, rendered in present tense as "Close" by
 * `toPresentTenseActionLabel`. Clicking it opens a confirmation dialog rather
 * than closing outright. */
export const CASE_DETAIL = {
  /** URL segment every case detail page carries. */
  pathSegment: "support/cases",
  /** URL segment for a security report analysis, which reuses the same header. */
  sraPathSegment: "security-center/security-report-analysis",
  /** URL segment for a service request, which reuses the same header too. */
  serviceRequestPathSegment: "operations/service-requests",
  /** URL segment for an announcement — also the same header. */
  announcementPathSegment: "announcements",
  /** The app's <main> region (AppShellLayout). Actions must be scoped to it:
   * the promo banner outside it renders its own dismiss control also named
   * "Close", which otherwise makes the locator ambiguous. */
  mainTestId: "app-main",
  closeButton: "Close",
  /** The comment editor's test id. The shared Editor hardcodes this id, so the
   * Activity tab's comment box carries the same one as the case form's
   * description field. */
  commentEditorTestId: "case-description-editor",
  /** Status shown in the header chip once the case is closed. */
  closedStatus: "Closed",
  /** The header row renders, in order: WSO2 case id, case number, a status chip,
   * a severity chip, then the subject. None carry ids or test ids, so they are
   * addressed positionally within the header — see CaseDetailPage. */
  header: {
    /** Case states the header may show, for asserting the value is a real one
     * rather than a placeholder. Mirrors CaseStatus in supportConstants.ts. */
    states: [
      "Open",
      "Work In Progress",
      "Awaiting Info",
      "Waiting On WSO2",
      "Solution Proposed",
      "Reopened",
      "Closed",
    ],
  },
  /** Label on the Details tab. The header shows the same value, so the view spec
   * asserts it there rather than switching tabs. */
  wso2CaseIdLabel: "WSO2 Case ID",
  confirmDialog: {
    title: "Confirm State Change",
    confirmButton: "Confirm",
    cancelButton: "Cancel",
  },
} as const;
