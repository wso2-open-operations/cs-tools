# IT services and service offerings — requirements

**Status:** describes what the service does today, not what anyone planned.

**Audience:** product owners, CS leads, QA, and engineers new to the service.

**Related documents:** [projects.md](projects.md). The knowledge base is in
[knowledge-base-and-runbooks.md](knowledge-base-and-runbooks.md); the lookup used to pick what an
outage affects is in [reference-data-and-search.md](reference-data-and-search.md).

---

## 1. Purpose

**IT services** (and their **offerings**) are reference data that other features pick from, for
example when raising an incident or an outage.

## 2. Who can use these routes

**PRJ-412** The service and offering lists are reference data and carry no extra checks.

## 3. Searching

**PRJ-470** The service shall search **IT services** by text, with paging (20 per page, at
most 50). Each result has the service name, its class, its business criticality (most
critical, somewhat critical, less critical, not critical) and its support group.

**PRJ-471** Text shall match the service name or number, ignoring letter case, and results
shall be ranked: an exact name match first, then names that start with the text, then names
where a word or a hyphenated part starts with the text, then everything else; ties newest
first.
*Check:* typing "web" lists a service called "Web" before "Intranet Web".

**PRJ-472** The service shall search **service offerings** by text on the offering name,
optionally limited to the offerings of given services (valid UUIDs), by name. Each result
shows the offering and the service it belongs to, if any.

## 4. Where this is implemented

| Area | Where (entity-service) |
|---|---|
| IT services | `internal/service/it_service_service.go`, `internal/repository/it_service_repo.go` |
| Service offerings | `internal/service/service_offering_service.go`, `internal/repository/service_offering_repo.go` |
| Routes | `POST /services/search`, `POST /service-offerings/search` |
| Tests | `it_service_repo_integration_test.go` |
