# Usage and metrics data: source of truth and shape

This document describes where the usage and metrics data shown for a
deployment's products (core counts, JDK version, usage counters such as
transaction counts and user counts) comes from, and how the pieces of that
data are tied together. It intentionally does not reference how any upstream
system used to supply this data, or how it got into these tables — only the
tables themselves and how a reader should interpret them.

This describes the default operating mode. In that mode, all of the data
described here is read directly from the application's own Postgres
database, with no live external call involved in producing any of the
responses described below. One alternate operating mode (used in a small
number of deployments, and not covered by this document) still reads this
same data from a different source instead.

The environment variable `PRODUCT_CONSUMPTION_DATA_READ_MODE` selects the read routing under the
dual-write data source: `postgres` (the default, as described here) or
`legacy`, which restores the previous routing in which these reads are served
from the other system the dual-write mode mirrors to. It exists while some
deployments are not yet ready for the Postgres-only read path. It has no
effect under the other data sources, and an invalid value fails startup.

## The five tables

| Table | What one row represents |
|---|---|
| `deployment_node` | One running instance ("node") of a deployed product. Carries its own node identifier, which project and deployment it was reported under, and which product version it is running. |
| `deployment_information` | One point-in-time snapshot of a node's runtime facts: core count, JDK version, and a raw metadata payload. A node can have many of these over time — one per reporting cycle. |
| `hourly_usage_summary` | One usage count for one node, for one hour, for one usage type (for example, how many transactions or how many users were recorded in that hour). |
| `daily_usage_summary` | The same shape as `hourly_usage_summary`, but rolled up to a day, and additionally tagged with how the data for that day was supplied (two possible tags — see below). |
| `monthly_usage_summary` | A further roll-up to a calendar month. Not currently read by any of the endpoints below; kept for completeness. |

A sixth table, `product_usage_map`, holds a lookup of product subscription
codes to their entitlement unit and limit (for example, a given subscription
code entitles a customer to a certain number of cores). It is reference data
for entitlements, not usage data, and is not part of the flows below.

## How a node is tied to a project, deployment and product

`deployment_node` does not hold a direct foreign key to the `deployment` or
`project` tables. Instead, each node row carries two plain text fields that
have to be matched against those tables' own natural keys:

- the node's reported project key is matched against a project's own unique
  key;
- the node's reported deployment number is matched against a deployment's
  own unique number, **but only among deployments that belong to the
  project already resolved in the step above** — a deployment number alone
  is not guaranteed to be unique across the whole system, only within a
  single project;
- the node's product version reference is a real foreign key into the
  product-version catalog, so the product a node is running is always
  resolved directly;
- the specific deployed-product record a node belongs to is found by
  matching both the resolved deployment and the node's product version
  together, since a deployment number alone does not uniquely identify one
  deployed product.

A node that cannot be matched to a project this way will never appear
anywhere in these endpoints. A node that matches a project but not a
deployment (for example, because the reported deployment number doesn't
exist, or belongs to a different project) still appears in a plain instance
listing, but with no deployment or deployed-product information attached,
and cannot be resolved by anything that is scoped to one specific deployed
product.

**For usage/metrics data belonging to one specific deployed product**
(rather than a plain, unscoped list of instances) to show up at all, every
one of the following has to be true at once:

1. the node's reported deployment number must equal the real number of the
   deployment the product is deployed under;
2. the node's product version must equal the deployed product's own
   recorded product version;
3. the deployed product's own record must itself have its product
   reference set (not just its product-version reference) — a deployed
   product with a product version but no linked product is treated as
   having no products at all by anything that counts "how many products
   does this deployment have," which in turn decides whether that
   deployment is even offered as a choice in the first place.

## What each capability reads

- **Listing instances** (optionally narrowed to one project, one
  deployment, or one deployed product, and optionally to a date range) reads
  `deployment_node`, enriched with each node's single most recent
  `deployment_information` row as its current snapshot.

- **Metrics history** for a set of instances reads every
  `deployment_information` row for those nodes inside the requested date
  range, newest to oldest — this is the raw time series of core count / JDK
  version / metadata snapshots.

- **Usage history** for a set of instances reads `hourly_usage_summary` for
  those nodes inside the requested date range, grouped into one entry per
  (instance, calendar day), with every usage type recorded for that node on
  that day collected together.

- **A single deployed product's core-count metrics** (as opposed to the
  generic metrics history above) read `hourly_usage_summary` instead of
  `deployment_information` — specifically, only the rows whose usage type is
  the core-count type, for that deployed product's own resolved nodes. This
  means a deployed product can have plenty of `deployment_information`
  snapshots and still show nothing here, if none of its nodes have a
  core-count row in `hourly_usage_summary` for the requested range — the two
  are genuinely separate facts about a node, not two views of the same data.

- **A single deployed product's usage counts** read `hourly_usage_summary`
  for that deployed product's own resolved nodes, across every usage type,
  the same way the generic usage history above does.

Resolving "this deployed product's own nodes" for either of the two
deployed-product-scoped reads above requires the deployed product's record
to have its product reference set, not only its deployment and
product-version references — the same requirement the picker already
applies (see point 3 under "How a node is tied to a project, deployment and
product" above). A deployed product missing that reference resolves to no
nodes at all for these two reads, exactly as if it did not exist.

- **Aggregated metrics ("stats")** for a set of instances takes, for each
  calendar day in the requested range, only the single latest
  `deployment_information` snapshot per node that day, and sums the core
  counts of those latest-per-day snapshots across every matching node. This
  produces one total-cores figure per day, plus the minimum, maximum and
  average of those daily totals across the whole range. A node that reports
  more than once in a day must only ever contribute its last report for
  that day to this total — never every report.

- **Aggregated usage ("usage stats")** for a set of instances reads
  `daily_usage_summary` instead of the hourly table (because this is the
  one table that also records which of the two supply tags below applies),
  sums every node's value for a given day and usage type together, and can
  optionally be narrowed to just one of those two tags.

  The two supply tags `daily_usage_summary` can carry are: data that arrived
  as a direct reported call, and data that arrived as an uploaded file. A
  caller may ask for usage summed across both (the default), or narrowed to
  just one of the two.

## What is required for a deployed product's usage/metrics pages to show data

Given the resolution rules above, for a specific deployed product's pages to
show anything at all:

- the deployed product's own record must have its product reference set,
  alongside its deployment and product-version references (see above) —
  without it, these two reads resolve no nodes at all, independent of
  whatever `deployment_node`/`hourly_usage_summary` data otherwise exists;
- at least one `deployment_node` row must exist whose reported project key,
  deployment number and product version correctly resolve to that exact
  deployed product (see "How a node is tied to a project, deployment and
  product" above);
- for the metrics page: that node needs at least one `hourly_usage_summary`
  row of the core-count type within the requested date range — not a
  `deployment_information` row, which backs the separate, instance-scoped
  metrics history instead;
- for the usage page: that node needs at least one `hourly_usage_summary`
  row (any usage type) within the requested date range;
- for the deployment to be offered as a choice at all in a product/deployment
  picker: the deployed product record under it must have both its product
  reference and its product-version reference set, and must not be marked
  inactive — the same requirement the two reads above enforce directly, not
  only through the picker.

A node with none of the above still exists as a row and can still show up
in a plain, unscoped instance listing — it simply contributes nothing to any
view that is scoped to a specific deployed product, or to any aggregate
figure.

## No personal data

None of the tables described here store any end-user personal information.
They describe infrastructure (nodes, deployments, product versions) and
aggregate usage counts, not individual people or their activity.
