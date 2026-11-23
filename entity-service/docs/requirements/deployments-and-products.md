# Deployments, products and instances — requirements

**Status:** describes what the service does today, not what anyone planned.
Items marked **Confirm** are things the code does that a product owner should check are
intended; they are collected in section 8.

**Audience:** product owners, CS leads, QA, and engineers new to the service.

**Related documents:** [projects.md](projects.md), [../specs/usage-metrics.md](../specs/usage-metrics.md)
(where usage and metrics data comes from and how a node is tied to a deployed product; this
document does not repeat it).

---

## 1. The model in one paragraph

A **project** has one or more **deployments** (an environment such as production, staging or
QA). A deployment holds **deployed products**: one entry per WSO2 product and version the
customer runs there, with how many cores it has, its update history and whether it is still
in use. The list of products and versions that exist comes from the **product catalogue**.
Running **instances** report usage back, and the **vulnerability** list tells staff and
customers which known issues affect which product versions.

## 2. Who can see and change what

**PRJ-210** A deployment or deployed product shall be visible only to internal staff and to
members of the project it belongs to. Anyone else shall get "not found" or an empty list.

**PRJ-211** A project member, and internal staff, shall be able to create and change
deployments and deployed products in their project, **including deactivating them**
(deactivating is a change, see 3.2 and 4.2). Only internal staff could ever delete one
outright, and no route does: nothing is deleted, items are deactivated instead. So a project
member can deactivate their project's items (see 8.3).

**PRJ-212** A deployed product shall always agree with its deployment: it cannot be saved
with one project's id beside a different project's deployment.

**PRJ-213** A change to a deployment or deployed product shall be attributed to the
signed-in person. A change with no person behind it shall be refused.

Rows left behind without a project are visible to internal staff only.

## 3. Deployments

### 3.1 Creating a deployment

**PRJ-220** The service shall let a caller create a deployment by giving a project, a name,
a type and a description. All four are required.
*Check:* leave out the description; refused with "description is required".

**PRJ-221** The type shall be one of: primary production, staging, QA, stress, UAT,
development. Letter case does not matter. Anything else shall be refused.

**PRJ-222** A new deployment shall be active and shall get the next deployment number,
written as `DEP` followed by nine digits (for example `DEP000002442`). Numbers shall never
repeat.

**PRJ-223** A project that does not exist shall be refused ("projectId does not exist"). A
caller who is not a member of the project shall not be able to create the deployment.

### 3.2 Changing a deployment

**PRJ-230** The service shall let a caller change a deployment's name, type or description.
At least one of name, type, description or active must be given.

**PRJ-231** The description can be cleared by sending an explicit empty (null) value; leaving
it out changes nothing.

**PRJ-232** A deployment can be **deactivated** but never reactivated through this route:
"active" can only be set to false. Deactivating cannot be combined with changing the name,
type or description in the same request.

**PRJ-233** A deployment is **never deleted**. Deactivating it hides it from the default list
(3.3). Its deployed products are not changed. **Confirm**, see 8.1.

**PRJ-234** An unknown deployment, or one the caller cannot see, shall be "not found".

### 3.3 Finding deployments

**PRJ-240** The service shall search deployments, newest first, with paging (20 per page, at
most 50) and a text search on the name, returning each deployment's number, name, type,
description, project, whether it is air-gapped, and the number of **active** deployed
products it holds.

**PRJ-241** The search shall support these filters, all combining (AND): project ids,
deployment ids, deployment numbers, Salesforce ids of the project, deployment types, and
active state.

**PRJ-242** By default only active deployments shall be listed. A caller can ask for
deactivated ones only (active = false), or for both (include inactive). Asking for a
specific active state **and** include inactive shall be refused as contradictory.

**PRJ-243** At most 50 deployment numbers may be given, each 1 to 50 characters.
Project and deployment ids shall be valid UUIDs, and Salesforce ids shall have no leading or
trailing spaces.

**PRJ-244** A deployment with no project shall never be listed.

**PRJ-245** A deployment's count of deployed products shall count only **active** deployed
products that have a product. Deployed products without a product are not counted.

### 3.4 Air-gapped deployments

**PRJ-250** A deployment can be marked **air-gapped** (cannot reach WSO2's services) or not.
New deployments are not air-gapped.

**PRJ-251** Only **internal callers** shall be able to change this, because it decides what
licence the customer's products get. The portal that exposes it limits it further to the
staff group allowed to.

**PRJ-252** The request shall say true or false explicitly; a missing value shall be refused.
Setting the value it already has still records who confirmed it and when.

**PRJ-253** An unknown deployment shall be "not found".

## 4. Deployed products

### 4.1 Creating a deployed product

**PRJ-260** The service shall let a caller add a product to a deployment by giving the
project, the deployment, the product and the product version. Optionally: the number of
cores, the transactions per second, a description and a category.

**PRJ-261** The deployment shall belong to the project given, and the version shall be a
version of the product given. Otherwise the request shall be refused with a message saying
which.
*Check:* give a version of a different product; refused with "versionId is not a version of
productId".

**PRJ-262** The category, when given, shall be one of `pdp`, `ms`, `ps`, `cl`, `pc`, in any
letter case.

**PRJ-263** A new deployed product shall be active and shall get the next reference number,
written as `IBITM` followed by digits. If two people are numbered at the same moment the
service shall try once more, and if that also fails refuse with "please try again".

**PRJ-264** Any id that does not refer to an existing record shall be refused, with the
field named.

### 4.2 Changing a deployed product

**PRJ-270** The service shall let a caller change cores, transactions per second,
description, category and the **update history**, in one request or any combination.

**PRJ-271** The update history is a list of entries, each with an update level (zero or
more), a date (`YYYY-MM-DD`) and optional details. A new list **replaces** the whole
existing one; sending an empty list clears the history; leaving it out changes nothing. An
invalid entry shall refuse the whole request, naming the entry.

**PRJ-272** The description can be cleared with an explicit empty (null) value. A value that
is neither text nor empty shall be refused.

**PRJ-273** A deployed product can be **deactivated**, never reactivated through this route.
Deactivating cannot be combined with the other changes.

**PRJ-274** A caller may state the deployment the product is expected to be in. If the
product is not in that deployment the request shall be "not found for the given
deployment" and nothing changes. This does not move a product between deployments.

### 4.3 Finding deployed products

**PRJ-280** The service shall search deployed products with paging, returning for each its
deployment (with type), product, version (with release date and support end date), cores,
transactions per second, category, description, active state, update history and project's
Salesforce id.

**PRJ-281** Filters: deployment ids, product categories, Salesforce ids of the project, and
active state. By default only active products are listed (a product with no active value
counts as active); active = false lists deactivated ones only. Items with no deployment or
no product are never listed.
A deployed product with **no category** shall match **every** category filter, so that
products nobody has categorised yet are not hidden from category-limited lists.
**Confirm**, see 8.8.

**PRJ-282** The service shall list **the projects running a given product version**, by
name, with paging, to find who is affected by an end-of-life announcement. The list shall
always leave out projects that are **Restricted** or **Suspended**, projects of the Cloud
Support and Cloud Evaluation Support types, and projects whose end date has passed (the
project counts as ended after the last millisecond of its end date). This exclusion cannot be
turned off by the caller.
*Check:* a project suspended yesterday is not in the list for a version it runs.

**PRJ-283** Usage metrics and usage counts for one deployed product shall be read from the
data described in [../specs/usage-metrics.md](../specs/usage-metrics.md). The deployment, a start date and
an end date shall be given, the start date shall be before the end date, and the range shall
not exceed one year.

## 5. Product catalogue

**PRJ-290** The service shall search products by name, optionally by class (software or
service), newest first, with paging. An unknown class shall be refused.

**PRJ-291** The service shall search the versions of one product, by version text and
optionally by deployment profile (ignoring letter case, at most 255 characters), newest
release first. Each version shows its release date, support end-of-life date, earliest
possible end-of-life date, current support status and deployment profile.

**PRJ-292** The service shall find the **source code repository** for a product name. It
tries, in order: an exact product name; the name without a trailing bracketed note; an exact
product abbreviation; then the longest product name that the given text starts with, followed
by a space (a name carrying a version). If nothing matches it shall say no repository is
mapped. An empty name shall be refused. Only active mappings count.

## 6. Instances and usage

Instances are the running copies of a product that report back. How they are stored and tied
to a project, deployment and product is described in
[../specs/usage-metrics.md](../specs/usage-metrics.md); only the rules around them are listed here.

**PRJ-300** The service shall list instances and their metrics and usage, by project,
deployment or deployed product, over a date range. **At most one** of project, deployment and
deployed product may be given at a time. Reads respect the same project membership as
deployments (2).

**PRJ-301** A date range for instance reads shall be in `YYYY-MM-DD` form, and its start
shall not be after its end. For metrics and usage totals the start shall be before the end,
and the range shall not exceed one year.

**PRJ-302** Reporting data **into** the service (instance details, usage figures, daily
usage batches, roll-ups) shall be possible for **internal callers only**.

**PRJ-303** A daily usage batch shall hold at most 5,000 entries. Node ids, project keys,
deployment numbers and other text shall be refused when longer than their column allows,
with the field named.

**PRJ-304** Internal callers shall be able to list products that were reported but could not
be matched to a catalogue product (the last 24 hours and 500 entries by default, at most
5,000), and to list recent usage report imports (100 by default, at most 1,000).

**PRJ-305** Internal callers shall be able to read a project's secret key and a deployment's
consumption secret, by project key and deployment number.

## 7. Product vulnerabilities

**PRJ-310** The service shall search the vulnerability list, newest first, with paging.
Each entry has the CVE, the vulnerability id, a priority, the product name and version, the
affected component (name, version, type), the update level, how the product is used, the
justification and the resolution.

**PRJ-311** Filters: free text (matched against the CVE, the vulnerability id or the
component name), priority (info, low, medium, high, critical or unknown), product name,
product version, and CVE. The CVE filter matches **one CVE exactly**, ignoring letter case
and surrounding spaces, and may be at most 100 characters. A priority that is not one of the
six shall be refused.

**PRJ-312** The service shall return one entry by id, and a list of the severities and of
every product with its versions, for use in filter dropdowns. When a cache is configured the
product list is kept for a while (24 hours by default) and is refreshed whenever the list
is synchronised.

**PRJ-313** The vulnerability list shall be **synchronised as a full replacement**: the
caller sends the complete current list, entries not in it are deleted, entries in it are
added or updated. Only a **configured machine client** may do this. A caller with no client
id, or one not on the list, shall be refused, and an unauthenticated caller shall be
unauthorised.

**PRJ-314** A synchronisation shall be refused if the list is empty (it would delete
everything), if an entry has no id of its own, or if two entries share an id. Text over its
limit (for example 255 characters for the id and product name, 500 for the component name,
100 for versions and CVE) and text containing a NUL character shall be refused, with the
entry and field named.

**PRJ-315** The severity of an entry may be sent as a name, a number or an object, and an
absent severity shall be stored as unknown. A severity that is not recognised shall refuse
the entry.

**PRJ-316** The result shall say how many entries were added, updated and deleted. The change
shall be all-or-nothing, and duplicate or keyless old rows shall be removed.

## 8. Open questions for the product owner

1. **Deactivating a deployment leaves its products active.** They stay visible through
   product search and counts until deactivated one by one. Is that intended?
2. **No reactivation.** A deployment or deployed product that is deactivated by mistake
   cannot be turned back on through the service.
3. **Who may deactivate.** Any project member can deactivate a deployment or product of their
   own project through this service; the portals decide whether to offer it.
4. **Affected-projects list ignores whether the product is still active.** A deactivated
   deployed product still puts its project in the list for that version, and products with no
   project link of their own are not found at all.
5. **The vulnerability list is readable by anyone who can call the service.** Each entry
   carries an "approved by security team" flag that is stored but not used to hide
   unapproved entries. Should customers see unapproved entries?
6. **Internal callers can read secrets.** The project secret key and a deployment's
   consumption secret are returned to any internal caller. Should reading them be limited
   further?
7. **Category codes are cryptic.** `pdp`, `ms`, `ps`, `cl`, `pc` have no description in the
   service. A written meaning belongs here once confirmed.
8. **Uncategorised products match every category.** Most existing deployed products have no
   category, so category filters rarely narrow anything. Is that acceptable until the data
   is backfilled?

## 9. Where this is implemented

| Area | Where (entity-service) |
|---|---|
| Deployments | `internal/service/deployment_service.go`, `internal/repository/deployment_repo.go` |
| Air-gapped | `internal/service/deployment_air_gap_service.go`, `internal/repository/deployment_air_gap_repo.go` |
| Deployed products | `internal/service/deployed_product_service.go`, `internal/repository/deployed_product_repo.go` |
| Catalogue | `internal/service/product_service.go`, `internal/service/product_version_service.go`, `internal/service/product_repo_mapping.go` |
| Instances and usage | `internal/service/instance_service.go`, `internal/service/instance_ingestion_service.go`, [../specs/usage-metrics.md](../specs/usage-metrics.md) |
| Vulnerabilities | `internal/service/product_vulnerability_service.go`, `internal/handler/product_vulnerability_handler.go`, `internal/service/cached_product_vulnerability_service.go` |
| Row-level security | migration `100017` (deployments and deployed products), `0246` (air-gapped), `0222` (deployment numbers) |
| Routes | `POST /deployments`, `POST /deployments/search`, `PATCH /deployments/{id}`, `PUT /deployments/{id}/air-gapped`, `POST /deployed-products`, `POST /deployed-products/search`, `POST /deployed-products/projects/search`, `PATCH /deployed-products/{id}`, `POST /deployed-products/{id}/metrics/search`, `POST /deployed-products/{id}/metrics/usage-counts/search`, `POST /products/search`, `POST /products/{id}/versions/search`, `GET /products/github-repo`, `POST /instances/...`, `POST /products/vulnerabilities/search`, `GET /products/vulnerabilities/{id}`, `GET /products/vulnerabilities/meta`, `POST /products/vulnerabilities/sync` |
| Tests | `deployment_service_test.go`, `deployed_product_service_test.go`, `deployment_air_gap_service_test.go`, `product_service_test.go`, `product_version_service_test.go`, `product_repo_mapping_test.go`, `product_vulnerability_service_test.go`, `instance_ingestion_test.go` |
