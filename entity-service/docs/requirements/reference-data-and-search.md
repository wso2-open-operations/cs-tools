# Reference data and service lookups — requirements

**Status:** describes what the service does today, not what anyone planned.
Items marked **Confirm** are things the code does that a product owner should check are
intended; they are collected in section 7.

**Audience:** product owners, CS leads, QA, and engineers new to the service.

---

## 1. Purpose

Both portals need lists to fill their menus (time zones, case states, severities), to know what
a project is allowed to do. They also
need to pick a service or service offering when raising an incident or an outage.

This document covers those read-only features: system and project metadata and the service
lookups. The search that finds projects and cases together is in [dashboard-reports.md](dashboard-reports.md). Outage-specific metadata is in [outages-and-cloud-status.md](outages-and-cloud-status.md).

## 2. Who is involved

| Who | What they do |
|---|---|
| Customer user | Uses the menus and the project's feature switches. |
| CS engineer | Same, across every project; picks services and service offerings. |

## 3. System metadata

**CUS-301** The service shall return the system-wide lists in one response: the **project
types**, the **time zones**, and the **feedback emojis**.

**CUS-302** Time zones shall be listed by their value, each with a display label.

**CUS-303** The feedback emojis shall be the five-point satisfaction scale: Very Dissatisfied (1),
Dissatisfied (2), Neutral (3), Satisfied (4), Very Satisfied (5). Each carries its selected and
unselected picture and the list of reasons ("chips") a person can tick, in the configured display
order. Only active ratings are returned. A rating whose stored name is not one of the five
known labels is left out rather than shown with a guessed label.
*Check:* the response holds up to five emojis, each with its own reasons.

## 4. Project metadata

**CUS-310** The service shall return, for one project, the choice lists a portal needs to
build that project's screens: case states, call request states, change request states,
conversation states, time card states, change request impacts, severities, issue types,
deployment types, case types, engagement types, engagement payment types, resolution codes
and causes.

**CUS-311** The call request states, resolution codes and causes shall be offered in the form
the matching write operations accept, so a value picked from those lists is never refused for
being in the wrong form. A stored value with no matching form (or, for causes, one that cannot
be written back) is left out of the list. The other lists carry the stored labels as they are.

**CUS-312** The response shall also say what the project may do (its **feature switches**),
taken from the project's type: read and write access to service requests, SRA, deployments;
read access to change requests, engagements, updates, time logs, component analysis and usage
metrics.

**CUS-313** The accepted severities shall be the ones the project's type allows, each shown with
its display label (for example "Critical (P1)"), and the default case and service-request
product categories shall be listed in lower case.

**CUS-314** For a project of the Cloud Support or Cloud Evaluation Support type that is on a
PDP subscription, the service shall switch service-request read and write access **on**,
whatever the type says.

**CUS-315** An unknown project shall be "not found", and a malformed project id a bad request.

**CUS-316** A project with no type shall still return the lists, with every feature switch off.

## 5. Searching projects and cases together

The search that finds projects and cases together is described once, in
[dashboard-reports.md](dashboard-reports.md) (section 5, CSR-130 to CSR-138).

## 6. Service lookups

Searching **IT services** and **service offerings** is described in
[services-and-offerings.md](services-and-offerings.md) (PRJ-470 to PRJ-472). The one lookup
specific to outages is below.

**CUS-332** The service shall let a person search **configuration items** by text, in pages.
Only service offerings are returned (never other kinds of item), alphabetical, each with its
parent service shown as its description. This is the list an outage's "what is affected" picker
uses, so it offers only items an outage can be created for.
*Check:* searching returns the same names the outage form accepts.

**CUS-333** Search text for IT services and service offerings may be at most 200 characters, and
filter ids must be UUIDs.

## 7. Open questions

1. **Project metadata is not scoped to the caller.** Any caller who can reach the service can
   ask for any project's metadata, and learns that the project exists. Is that acceptable,
   or should it follow the same project-membership rule as search?
2. **System metadata and the lookups are open to any caller.** There is no internal-only check.
   Confirm the lists hold nothing sensitive (project type names, service names).
3. **Configuration-item search has no length limit** on its text, unlike the other two lookups.
4. **PDP service-request access is decided by project type name.** The two names are written
   into the service, so a renamed type silently loses the exception. Should it be data?
5. **Severity ids in project metadata are fixed numbers** that map to the display labels
   (P0 to P4); a new severity needs a code change.

## 8. Where this is implemented

| Area | Where (entity-service) |
|---|---|
| System metadata | `internal/service/global_service.go` |
| Project metadata | `internal/service/project_metadata_service.go`, `internal/repository/reference_data_repo.go` |
| Feedback emojis | `internal/repository/reference_data_repo.go`, `internal/repository/case_feedback_repo.go` |
| IT services, service offerings | `internal/service/it_service_service.go`, `internal/service/service_offering_service.go` |
| Configuration items | `internal/service/pg_configuration_item_service.go` |
| Routes | `GET /metadata`, `GET /projects/{id}/metadata`, `POST /services/search`, `POST /service-offerings/search`, `POST /configuration-items/search` |
| Tests | `project_metadata_service_test.go` |
