# Product catalogue

The product catalogue is every WSO2 product, its versions and their support dates. It is the data
published on the public **Support Matrix** and **Release Matrix**, so a change here reaches those
pages as soon as it is saved. Open it from **Settings → Product catalogue**. It has two tabs: **Products** and
**Release lines**.

The Product catalogue tab is shown only to people with the product catalogue manager role, who
can also edit it.

## Products

The Products tab is one table of product versions. Each row starts with the product code (hover a
code to see the full product name), then the version, its support status, release date, the
earliest possible end of life, and the end-of-life announcement and end-of-life dates. Rows are
sorted by product code, newest release first; click a date column's heading to sort by that date
(click again to reverse it; versions without the date stay at the bottom). Type in the search box to
find versions by product code or name, or by version, and narrow the list with the **Product**,
**Status** and **EOL** filters (past EOL, EOL within 1, 3 or 6 months, or no EOL date). **Clear** resets the
search and the filters.

A version's deployment profiles share their status and dates, so they are one row. When a product
publishes one version as several listings (API Manager 4.5.0 and later list the Control Plane,
Universal Gateway and Traffic Manager separately), each listing is its own row with its public
name under the version.

WSO2's cloud products (Choreo, Devant, Bijira, Asgardeo and the like) are not listed, and neither
are retired or discontinued versions, or a product whose versions have all been retired or
discontinued.

Managers have two buttons above the table:

- **Add product** adds a new product: its name and class, plus an optional code, unit, business
  unit, manufacturer and description.
- **Add product version** adds a version to a product. Enter the version, one or more
  deployment profiles (each becomes its own row, all in one public listing, added together or not
  at all) and its support status and dates. The publishing settings — public name and abbreviation, whether it is on the
  Support Matrix and the Release Matrix, release line, tags, release links and description — are
  copied from the product's most recent version, so the new version publishes like the ones before
  it. They are shown under **Publishing**, with a note saying which version they came from, and
  you can change any of them before adding.

The pencil on a row opens **Edit**. The support status and end-of-life dates are shown first;
the collapsed **Advanced** section holds the release date and the publishing settings (public name
and abbreviation, Support Matrix and Release Matrix, release line, tags, release links, description
and Carbon platform). The version number and deployment profiles cannot be changed here.
**Save** applies the change at once to every deployment profile of the version; the dialog names
the profiles it will move. To take a version off the public feeds, use **Retire this version** at
the bottom of the Edit dialog; it asks you to confirm, retires all of its deployment profiles, and
the row drops out of the catalogue. If a change breaks a rule (for example
an end-of-life date before the release date), nothing is saved and the dialog says why.

## Release lines

Release lines group versions that are released together; retired ones are not listed. Managers
can create a release line, rename it, change its start date or status, and retire it from its edit
dialog.
