# Outages and the cloud status page — requirements

**Status:** describes what the service does today, not what anyone planned.
Items marked **Confirm** are things the code does that a product owner should check are
intended; they are collected in section 11.

**Audience:** product owners, SRE and CS leads, QA, and engineers new to the service.

---

## 1. Purpose

When one of the cloud products has an outage, a degradation or a planned maintenance, the SRE
team records it. From that one record the service:

- keeps the outage's details and its running commentary;
- tells **internal stakeholders** when it is declared, updated and resolved;
- tells the **SRE team** when it is declared and resolved;
- keeps the **public cloud status page** in step: the status of each affected component, the
  incident history, and the availability figures.

The service decides **what** to say and **when**, and records that it has said it. The
scheduled-tasks service and the notification service deliver the emails and the status-page posts.

## 2. Who is involved

| Who | What they do |
|---|---|
| SRE engineer | Declares, updates, ends and comments on outages in the CSM portal. |
| Internal stakeholders | Receive the internal notices. |
| The SRE team | Receives the declaration and resolution mails. |
| The public status page | Reads components, incidents and availability, and receives webhooks. |
| The scheduled-tasks and notification services | Ask what is owed, send it, and report back. |

## 3. Outage records

An outage has a **type** (outage, degradation, or planned), a start, an optional end, a short
description, and optionally the service offering it affects, further affected items, a linked
incident, public and internal notes, an impact and a state label. It is **in progress** until it
has an end, and **resolved** after.

**CUS-601** The service shall let a person create an outage. A type, a start time and a short
description are required. The short description must not be blank and is at most 160 characters.
The impact and state labels are optional and at most 40 characters each.

**CUS-602** Times may be given either in the standard date-and-time form or as
"YYYY-MM-DD HH:mm:ss". The end, when given, shall not be before the start. For a change, the
end is checked against the start that is **stored**, not only the fields sent.
*Check:* send only an end earlier than the stored start; refused.

**CUS-603** The two optional switches, **notify internal stakeholders** and **send outage
communication to the SRE team**, are off unless the person turns them on.

**CUS-604** The service shall let a person change an outage: only the fields sent change. An end
set to empty **reopens** the outage. The short description may not be changed to blank. An unknown
outage is "not found".

**CUS-605** If the service offering chosen for the outage, or any affected item **added** by a
change, belongs to a cloud shown on the public status page, the request shall be refused as a
conflict unless the person has explicitly agreed that the outage will be **publicly visible**.
A change that does not add or change such an item does not ask again, so closing an outage never
needs the agreement a second time.
*Check:* create an outage for a monitored cloud without the agreement; refused as a conflict, not
as a bad request, so the portal can show its warning.

**CUS-606** The service shall let a person open one outage with its details, its affected items,
and whether it publishes to the status page. The linked incident's number and subject are shown
only to a person who may see that incident; anyone else sees them blank.

**CUS-607** The service shall record who made each change: the signed-in person, otherwise the
calling service, otherwise the service itself.

**CUS-608** The service shall let a person search outages by type, status (in progress or resolved),
service offering, linked incident, and a start-from time; sorted by start, end, number, created or
updated, ascending or descending; in pages (20 by default). Anything else is refused.

**CUS-609** When no start-from time is given, the search shall look back **six months**, and the
answer shall say which start-from time was used and that it was a default.

**CUS-610** An affected item listed twice shall count once.

### Commentary on an outage

**CUS-615** The service shall let a person add an entry to an outage's log, on one of three
channels: **external**, **internal** or **additional**. The body is required.

**CUS-616** An **external** entry on an outage that publishes to the status page shall be refused
as a conflict unless the person agrees it will be publicly visible.

**CUS-617** The service shall let a person list an outage's entries, optionally by channel, in pages
(50 by default).

**CUS-618** The service shall provide the choices a form needs: the types, the statuses, the
channels (with the external one marked public), and the clouds on the status page.

## 4. Internal stakeholder notices

**CUS-620** The service shall, when asked by an **internal** caller (in practice the
scheduled-tasks service), decide which notices are owed for every outage that has the "notify
internal stakeholders" switch on, and return them for sending. A caller who is not internal, such as
a customer, is refused.

**CUS-621** For one outage, at most **one** notice is owed per check, chosen in this order:
1. **Declared**, if nothing has been said yet.
2. **Resolved**, if the outage has ended and a "resolved" notice has not been sent.
3. **Update**, if it has been declared, has not ended, and has **changed since the last thing said**.
Otherwise nothing.

**CUS-622** An outage that has already ended when notification is first switched on gets its
"declared" notice first and its "resolved" notice at the **next** check, so both are sent, in order.

**CUS-623** An update shall be sent **once per change**, not once per check: an unchanged outage
mails nobody again.

**CUS-624** The service shall record each notice as sent **before** returning it for sending, so a
crash loses a notice rather than repeating it. A notice whose recording fails is not returned.
(**Confirm**, see 11.1)

**CUS-625** Outages that were already announced by the legacy case system are treated as already
announced, so switching over does not announce them again.

**CUS-626** Only one check runs at a time; a second one that finds the first running reports
nothing owed.

**CUS-627** Each notice has the subject "[Outage] *number* - Declared / Update / Resolved" and a
single fixed sentence ("Outage *number* declared.", "...resolved.", "...update."). It carries no
other detail. (**Confirm**, see 11.2)

**CUS-628** An internal caller shall be able to read what has been sent for one outage.

## 5. Mails to the SRE team

**CUS-630** The service shall decide the SRE team's mails for every outage that has the SRE
communication switch on **and** a start time.

**CUS-631** A **declaration** is owed only for an outage of type **outage** that has no end and has
not been declared. A degradation or a planned outage is **never** declared. An outage that arrived
already ended is never declared.

**CUS-632** A **resolution** is owed once the outage has an end, has been declared, and has not been
resolved.

**CUS-633** The mail's subject starts with a fixed product word and the outage's number and short
description; the body gives the number, title, start time, impact, current state and a next-steps
paragraph, and is signed "SRE". (**Confirm**, see 11.3)

**CUS-634** As for stakeholder notices, each mail is recorded in the communication log **before** it is
returned, and only one check runs at a time. The log is read by outage **number**.

**CUS-635** Only an internal caller may ask what is owed or read the log.

## 6. The public status page

The status page covers a configured set of services (the "in scope" services). Its components are
the monitors, and each belongs to a **cloud** from the cloud registry.

**CUS-640** The service shall treat an outage as relevant to the status page when it is on an in-scope
service. With none configured, it does nothing and warns.

**CUS-641** For each in-scope outage the service shall record a **begin** event when it is in
progress and an **end** event when it has ended, **once** per cloud, and shall hand each recorded
event to the delivery step as a webhook.

**CUS-642** For an outage in progress the clouds told are the outage's own cloud **plus** every
in-scope cloud of its affected items. For an ended outage only its own cloud is told, so no
all-clear goes to a status page that was never told.

**CUS-643** A cloud with no active place in the registry is skipped and counted, never silently lost.

**CUS-644** On every check the service shall put each affected monitor into the status the outage
implies: while in progress, **planned** shows as maintenance, **degradation** as degraded, **outage**
as partial outage; once ended, back to operational. A monitor already showing that status is left alone.
An in-progress outage with no usable type falls back to a visible default and is counted.

**CUS-645** When an outage is created or changed through the service, the status page shall be told at
once, in the same request: the same decision is made and an "outage status page due" event is
published. A failure is logged and never fails the outage; the next check catches it.

**CUS-646** Delivery shall be safe: a webhook is claimed before it is posted, so it cannot be posted
twice at once. A claim lasts 15 minutes. A webhook is tried at most **5** times, and one that runs
out stays recorded and undelivered so the failure is visible. A report of an attempt that is not the
current one is refused as a conflict.

**CUS-647** The service shall record each attempt's outcome: delivered, failed with an error, or
**unknown**. An unknown outcome is never re-sent.

**CUS-648** Claiming webhooks is refused while the cloud registry holds no active cloud.

## 7. What the status page reads

**CUS-650** The service shall answer five reads, each for one cloud named by its short name: the
**monitors** (each with its status, availability and any ongoing outage), the **incident history**,
the **availabilities**, the **availability history**, and one **incident's detail**. The short names
come from the cloud registry (seven are seeded; adding a cloud is data, not a release). An unknown
or inactive cloud is refused, listing the valid ones.

**CUS-651** The incident history shall always carry **six months**, whether or not they have
incidents. An incident that does not belong to the named cloud is "not found".

## 8. Availability figures

**CUS-660** The service shall recompute availability for every component that has a commitment,
replacing the stored rows, so the dashboard's figures keep advancing. Only an **internal** caller may
run it.

**CUS-661** Day boundaries for availability shall follow a configured time zone, defaulting to
Asia/Colombo. An unknown zone stops the service from starting rather than quietly dating every row
wrongly.

## 9. Switches and settings

**CUS-670** The in-scope services are set by configuration; adding a cloud to the status page is a
setting and a registry entry, not a release.

**CUS-671** The status-page publishing needs the operations event topic and the in-scope services;
without either, the scheduled task posts on its own tick as before.

## 10. What this covers

Who receives the stakeholder notices, and how mail and posts are delivered, is decided by the
scheduled-tasks and notification services, not here.

## 11. Open questions

1. **Notices can be lost.** Because each notice is recorded before it is sent, a crash between the
   two means the notice is never sent. This was chosen deliberately over the risk of a duplicate.
2. **The stakeholder mails say almost nothing.** They are one fixed sentence with no outage detail.
   Making them useful is a product decision not yet taken.
3. **The SRE mail names the same product word for every cloud**, and degradations and planned work are
   never declared to the SRE team. Should the subject name the affected cloud, and should other types
   be declared?
4. **Overlapping outages.** When one outage ends, its components go back to operational even if a
   second outage on them is still running.
5. **A type of "outage" shows as a partial outage**, never a major outage; the highest level is never
   used.
6. **Short outages can be missed.** One that begins and ends between two checks produces only an end,
   so it never appears on the public page.
7. **Access is not restricted for outage records.** Creating, changing and commenting on outages, and
   the status page's five reads and its delivery routes, carry no internal-only check in this service.
   Only the stakeholder and SRE checks and the availability run do. Is the gateway meant to be the only
   guard?

## 12. Where this is implemented

| Area | Where (entity-service) |
|---|---|
| Outage records, validation | `internal/service/pg_outage_service.go`, `internal/repository/outage_repo.go` |
| Status page after a write | `internal/service/outage_cloud_status.go` |
| Stakeholder notices | `internal/service/outage_notification_service.go`, `internal/service/outage_notification_decision.go` |
| SRE mails | `internal/service/outage_communication_service.go`, `internal/service/outage_communication_decision.go` |
| Status page webhooks, monitors | `internal/service/cloud_status_service.go`, `internal/domain/cloud_status.go` |
| Status page reads, registry | `internal/service/cloud_status_api_service.go`, `internal/service/cloud_registry.go` |
| Availability | `internal/service/availability_service.go`, `availability_calc.go` |
| Routes | `/outages`, `/outages/{id}`, `/outages/{id}/communications`, `/outages/metadata`, `/outage-notifications/sweep`, `/outage-communications/sweep`, `/cloud-status/*`, `/internal/cloud-status/*`, `/internal/availability/sweep` |
| Tables | migrations `0079`, `0083`, `0090`, `0115`, `0116`, `0173`, `0184`, `0186`, `0213`, `0282` |
| Tests | `outage_*_test.go`, `cloud_status_*_test.go`, `availability_*_test.go` |
