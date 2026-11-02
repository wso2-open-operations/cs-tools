# Products

Products is the product catalogue: every WSO2 product, its versions and their support dates. It is
the data published on the public **Support Matrix** and **Release Matrix**, so a change here
reaches those pages as soon as it is saved. It has two tabs: **Catalogue** and **Release lines**.

Anyone with staff access can read the catalogue. Editing it needs the product catalogue manager
role (or admin); everyone else sees the same pages read-only.

## Catalogue

The Catalogue tab is one table of product versions. Each row starts with the product code (hover a
code to see the full product name), then the version, its support status, release date, the
earliest possible end of life, and the end-of-life announcement and end-of-life dates. Rows are
sorted by product code, newest release first. Type in the search box to find versions by product
code or name, or by version.

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
  it. The collapsed **Advanced configuration** section shows what will be used, and you can change
  any of it there.

The pencil on a row opens **Edit**, which changes only its support status and end-of-life
dates. The release date is set when the version is added and is not edited here.
**Save** applies the change at once to every deployment profile of the version; the dialog names
the profiles it will move. To take a version off the public feeds, use **Retire this version** at
the bottom of the Edit dialog; it asks you to confirm, retires all of its deployment profiles, and
the row drops out of the catalogue. If a change breaks a rule (for example
an end-of-life date before the release date), nothing is saved and the dialog says why.

## Release lines

Release lines group versions that are released together; retired ones are not listed. Managers
can create a release line, rename it, change its start date or status, and retire it from its edit
dialog.
