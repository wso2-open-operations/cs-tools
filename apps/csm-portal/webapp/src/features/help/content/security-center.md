# Security Center

Security Center is where security-related work for customer deployments lives, split into
two tabs: **Security reports** and **Vulnerabilities**.

## Security reports

Security reports are a case type (Security Report Analysis) filed against a specific project,
deployment, and deployed product. The list shows the same table used elsewhere for cases, but
locked to this case type: the case type column and severity column are hidden since every row
is the same type.

To file a new report, use **New security report**. The form asks for:

- **Project**: searchable, or pre-filled and locked if you opened the form from a project's
  page.
- **Deployment**: the environments available under the selected project.
- **Deployed product**: the products available under the selected deployment.
- **Subject**: auto-generated as `{Deployment} - {Product} - {date}` once a product is
  chosen; edit it and the auto-generation stops so your text isn't overwritten later.
- **Description**: required; describe what needs security analysis.
- **Attachments**: optional; uploaded after the report is created, so a failed upload never
  blocks report creation. If any attachment fails, you'll see a warning and can retry it from
  the report page.

Once created, the report opens like any other case, and its Back button returns you to
wherever you started the form from (the Security Center list, or the originating project page).

## Vulnerabilities

The Vulnerabilities tab lists known product vulnerabilities across customer deployments,
independent of any single case. Each row shows:

- **CVE / Vulnerability ID**: the CVE identifier and/or internal vulnerability ID.
- **Component**: the affected component and its version.
- **Product**: the affected product and its version.
- **Priority**: Critical, High, Medium, Low, Info, or Unknown, shown as a colored chip
  (Critical/High in red, Medium in amber, Low in blue).
- **Type**: the vulnerability type.
- **Update level**: the update level the fix applies to.

Use the search box to filter by free text, or the **Priority** filter to narrow to a single
priority level. The list is paginated server-side; use **Refresh** to re-run the search and
pick up any changes.

Click a row to open the vulnerability's detail page, which shows the same overview fields plus
any available long-form text: **Use case**, **Justification**, and **Resolution**. These
sections only appear when the underlying data has content. The detail page's Back button
returns you to the Vulnerabilities list (or, if you reached the vulnerability from elsewhere,
back to that page).
