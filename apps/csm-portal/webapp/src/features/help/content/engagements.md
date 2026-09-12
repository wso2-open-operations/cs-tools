# Engagements

Engagements track professional-services work for a customer (migrations, consultancy,
onboarding, follow-ups, and new-feature/improvement work), separately from support cases.
Under the hood an engagement is a case of type **Engagement**, so it reuses the same list,
detail page, comments, activity feed, and time-tracking as a regular case, just scoped to
this one type and without a severity concept (severity isn't meaningful for this kind of
work, so the severity column and chip are hidden throughout).

## List view

The Engagements list is the same table used for cases, locked to engagement-type rows: the
case-type column is hidden (every row is the same type) and a dedicated **Engagement type**
filter lets you narrow by Migration, Consultancy, New feature / improvement, Follow up, or
Onboarding. Search, sort, and filter by project, deployment, product, assignee, and state the
same way you would on the Cases list, and export the filtered results to CSV with the same
export button.

Click a row to open that engagement's detail page. Use **Create engagement** to start a new
one.

## Creating an engagement

The creation form asks for:

- **Project**: searchable, or pre-filled and locked if you opened the form from a project's
  page.
- **Deployment**: the environments available under the selected project.
- **Deployed product**: the products available under the selected deployment.
- **Engagement type**: Migration, Consultancy, New feature / improvement, Follow up, or
  Onboarding.
- **Subject**: a short title, up to 200 characters.
- **Description**: required; a rich-text field describing the engagement.

Project, deployment, and deployed product are dependent selects: choosing a project loads its
deployments, and choosing a deployment loads its deployed products. If any of those dropdowns
fail to load, a **Retry** control appears instead of a silent empty list.

Once created, the engagement opens on its own detail page, and its Back button (like the
form's Cancel/Back buttons) returns you to wherever you started: the Engagements list, or the
originating project page.

## Detail view

An engagement's detail page is the same case detail page used for support cases, cases,
service requests, and security reports, so it carries the same building blocks:

- **Header**: case/engagement number, engagement-type chip, lifecycle state chip (Open, Work
  in progress, Solution proposed, Awaiting info, Waiting on WSO2, Closed, Reopened), and, while
  in progress, a sub-state chip (ongoing/paused) plus an auto-closure hold indicator if
  applicable. No severity chip is shown.
- **Overview**: project, deployment, deployed product, and other case metadata.
- **Comments and activity feed**: the same threaded comment and history view used on cases.
- **Time cards**: engineers can log time against an engagement the same way they log time
  against any other case, from the time cards panel on the detail page; that logged time then
  shows up in Time cards under this engagement.

Because an engagement is a case under a different type, it can also be transferred to or from
another case type from the detail page, the same way a case can be reclassified.
