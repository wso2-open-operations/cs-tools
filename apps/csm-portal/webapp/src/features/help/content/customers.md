# Customers

Customers is where you look up an account or a project directly, rather than arriving at one
through a case or dashboard widget. It has two tabs: **Accounts** and **Projects**.

## Accounts

The Accounts tab lists every customer account, searchable by account name or Salesforce ID.
Clicking a row opens that account's detail page, which shows:

- **Overview**: tier, region, Salesforce ID, the account's Account Manager, Renewal Account
  Manager, and Technical Owner, its CRE/SRE team assignment, activation and deactivation
  dates, and whether the AI chat assistant and Smart KB suggestions are enabled for this
  account.
- **Projects**: every project tied to this account (an account can have several), with each
  project's key, subscription type, and end date. Click through to a project's own detail page
  from here.

An account with a deactivation date in the past is flagged with a **Deactivated** chip next to
its name.

## Projects

The Projects tab lists every project across all accounts, searchable by project name, project
key, or subscription type. A project is tied to a subscription, and its row shows its closure
state and start/end dates.

Opening a project takes you to its detail page, with four tabs:

- **Overview**: project key, closure state, subscription type, a link back to the owning
  account, Salesforce ID, and the created/updated/start/end dates. An **Onboarding Owner**
  field appears here too, but only for onboarding-enabled projects — it names the onboarding
  consultant assigned to the project. It's absent for projects with no onboarding engagement.
- **Deployments**: see below.
- **Project contacts**: see below.
- **Work items**: cases, service requests, and other work items filed against this project.

From a project's detail page, the **Create** menu lets you file a new case, service request
(managed-cloud projects only), engagement, or security report already scoped to this project,
so it can't end up filed against the wrong one.

### Deployments

A deployment is an environment within a project (for example, a production or staging
environment), each with its own type, description, and the products deployed to it. The
Deployments tab lists a project's deployments and lets you create one, edit its details, or
deactivate it. Deactivating a deployment marks it inactive rather than deleting it.

Opening a deployment shows its deployed products and attachments:

- **Deployed products**: create, edit, or deactivate a product deployed to that
  environment. Editing a deployed product has two tabs, each saving independently:
  **Details** (cores, TPS, description), saved with a **Save changes** action, and
  **Update History**, where each add, edit (of the latest entry), or delete of a
  version-update entry (update level, date, and optional details) saves immediately.
- **Attachments**: upload, list, edit the name/description of, download, or delete
  documents attached to the deployment.

### Project contacts

The Project contacts tab lists the people registered on a project (name, email, roles, and
registration state), plus an **Access** column showing whether each row actually grants that
person visibility into the project's cases in the customer portal, not just whether they're
listed. A row flagged **No access** (or **Orphaned**) carries an inline reason: either the
invite never completed (no linked contact record), or the row was invited under an address
that doesn't match its linked contact's own. Both are worth chasing directly, since either one
means that person can't see any of this project's cases.

This tab is a per-project list. To look up what access a *specific person* has across all
their projects, or to dig into their contact record in more detail, see the **People & project
access** Help topic.
