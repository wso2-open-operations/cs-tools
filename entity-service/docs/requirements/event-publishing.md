# Event publishing — requirements

**Status:** describes what the service does today, not what anyone planned. Items marked
**Confirm** are things the code does that a product owner should check are intended; they
are collected in section 8.

**Audience:** product owners, CS leads, QA, and engineers new to the service.

Related: [salesforce-ingest.md](salesforce-ingest.md) (the invitation, welcome and
"back to open" events), [case-escalation.md](case-escalation.md) (the escalation email),
[platform-and-operations.md](platform-and-operations.md) (the settings).

---

## 1. Purpose

Most emails, chat cards and phone calls about cases, incidents, projects and outages are
sent by a separate notification service. This service does not send them. Instead, when
something happens, it **publishes an event** to a message hub, already decided and worded
(who should be told, and what about), and the notification service sends the message.
Splitting it this way keeps the rules about *who and what* next to the data, and the
delivery (emails, chat, calls, retries) in one place.

## 2. General rules

**INT-200** Publishing shall be off unless **both** a message-hub address is configured and
publishing is explicitly switched on (the word `true`, exactly). A deployment can therefore
have the hub fully configured and still publish nothing until someone turns it on. With
publishing off, every action still works except the admin "Recalculate SLA" request, which
answers "not available"; nothing is published and nothing is recorded as failed.
*Check:* configure the hub but leave the publishing switch off; create a case; the case is
created and no event is sent.

**INT-201** The hub address, connection secret and shared topic name shall be set together
or not at all. A partial set shall stop the service from starting, so a half-configured
deployment cannot look healthy while every publish fails.

**INT-202** Every event shall be an envelope of three things: an event type, an entity id
(the case, incident, project or other record the event is about), and a payload. The entity
id is also the ordering key: every event about the same record goes to the same partition
and is processed in the order published.

**INT-203** Publishing shall be a **best-effort side effect**. The action that caused it
(creating a case, writing a comment, escalating) has already succeeded by then, and a
publishing failure shall never fail or undo it. A failure is logged and recorded
(section 4), and the caller is told the action worked.

**INT-204** Most publishes shall run in line with the request but be bounded to a short time
(five seconds for case, incident, escalation and invitation events) so a slow hub cannot hold
a request for long. The "back to open" notice, which runs under a database lock, is bounded to
thirty seconds.

**INT-205** Events shall follow the **write**, not the data source: an event is published
whenever the CSM database recorded the change, so a later data-source change cannot silently
stop notifications. Where a change is also mirrored to a legacy system, only one of the two
paths shall publish, so the same change is not announced twice.

**INT-206** An event that exists to email people (a case created, a comment, a status change,
an assignment, a severity change) shall carry its list of recipients, and shall be **skipped**
(logged, not published) when the list is empty, because the notification service rejects an
event with no recipients. A case that has no watchers yet is an expected state, not an error.

**INT-207** A case event shall be skipped when the case has no project, because every case
event must name its project. (This happens for a service request raised from an external
issue that has no project yet.)

**INT-208** A publish error does not prove the hub refused the event: the hub can have kept it
while only the acknowledgement was lost. An event may therefore, rarely, be delivered twice if
someone re-sends it from a failure record. The service has no event id a consumer could use to
detect a duplicate. (**Confirm**, see 8.1.)

## 3. Topics

Events go to different topics so one volume cannot delay another, and each can be watched
separately.

| Topic | Carries |
|---|---|
| Shared case topic | Case events, case escalation, incident events (unless they have their own topic), knowledge-article published, SLA recalculation requested, paging test call. |
| Project topic (default `project-events`) | Project invitation, project welcome, project "back to open". |
| Incident topic (optional) | Incident events. When unset, incident events use the shared case topic. |
| Operations topic (optional, `SRE_EVENT_HUB_TOPIC`) | Service-request events, incident special-operations alert, cloud status page. When set it also becomes the topic for the change-request and outage notices, overriding their own settings. |
| Change-request topic (default `cr-events`) | Change-request notices. |
| Outage topic (default `outage-events`) | The two outage emails. |
| Salesforce write topics (defaults `salesforce-writes`, `salesforce-write-results`, and a dead-letter topic) | The deferred membership write request this service sends, and the answers it listens for. |

**INT-210** Every topic shall use the same hub address and secret. Only the topic differs.

**INT-211** Service-request events shall be published only when an operations topic is
configured, publishing is on, and there is a database. Without them the acknowledgement of a
new request would be posted with no card announcing it.

## 4. When publishing fails

**INT-220** When the hub does not acknowledge a publish, the service shall store a **failure
record**: the event type, the entity id, the full payload, and the error. It shall be written
with its own ten-second limit, and if even that fails, both errors are logged. This applies to
every publish except those made by the back-to-open notice job (INT-224), which leave no failure
record, so those attempts will not appear in the failure-record list.

**INT-221** With no database configured there is nowhere to record a failure; the service
shall log the failure and return the original error.

**INT-222** Operators and other services shall be able to read, create and resolve failure
records:
- **Create** requires an event type, an entity id, a payload that is a JSON object, and an
  error; anything missing or malformed is a bad request.
- **Search** returns records newest first, can be limited to resolved or unresolved ones, and
  uses standard paging (PLT-050 in platform-and-operations.md).
- **Resolve** marks one record as handled, by its id.

**INT-223** The service shall not re-send failed events by itself. A person or tool reads the
unresolved records, re-sends, and marks them resolved. (**Confirm**, see 8.2.)

**INT-224** The "back to open" notice job, which retries its own work, shall not also leave a
failure record for each attempt, because a person re-sending that record while the job still
retries would send the event twice. The job keeps its own retry state (section 6).

**INT-225** These endpoints are available only when there is a database. Without one they are
not registered. The service does not check the caller. (**Confirm**, see 8.3.)

## 5. The events

Who receives each event and what it does is described by the notification service. This table
says when this service publishes each one.

### Cases

| Event | When it is published |
|---|---|
| Case created | After a case, service request, engagement, security report analysis or announcement is created, if it has a project and recipients. A "case" type also needs a severity. Recipients are the case's watchers plus the account's default watchers; an announcement goes to the project contacts holding the security-contact (security announcements) or portal-user role, falling back to the watchers when nobody holds it. |
| Comment added | After a comment or work note is written. A work note goes only to addresses in the company's own domain and is marked internal. The author's display name is included; the event is skipped if there are no watchers. |
| Status changed | When a case's state actually changes. Skipped with no project or no watchers. |
| Severity changed | When a case's severity changes, for the case type only. Skipped with no watchers. |
| Assigned | When a case's assignee changes, naming the new assignee; recipients are the watchers. Skipped with no watchers. |
| Acknowledged | The first time a case is acknowledged, never for a repeat. Goes to chat only, so it has no recipients. |
| Workaround provided | When a case's "workaround provided" is set to true. A tracking signal with only the case id. |
| Type changed | When a case-type transfer revises the case's SLA clocks, so the notification service's own clock tracking re-syncs. |
| Escalated | When a case is escalated (see case-escalation.md). Never for a de-escalation. |

### Incidents

| Event | When it is published |
|---|---|
| Incident created | After an incident is created; starts the call-escalation ladder. |
| Priority elevated | On the legacy data source only, when an incident's priority is raised; starts the ladder for the new priority. The native data source does not publish it. |
| Acknowledged | When an incident leaves the new state; stops the ladder started by creation. |
| Comment added | On the legacy data source only, after a comment or work note on an incident; says whether it was public, because a public comment is what stops a ladder started by an elevation. The native data source does not publish it. |
| Assigned | When an engineer is set as the incident's assignee; stops the on-call ladder. |
| Special-operations alert | When an incident's assignment group changes to a Special Ops team's group, however it changed. Needs the operations topic and a valid hand-off configuration. |

### Service requests (operations topic)

| Event | When it is published |
|---|---|
| Created | For every service request created when the CSM database is the only system, and for one raised from an external issue in every mode. A service request a portal creates first in a legacy system (dual-write) is announced by that system's own flow instead. |
| Acknowledged | When the request's account SRE team is in the configured list: the request is first assigned to that team, gets the standard acknowledgement comment, and moves to Open. |
| Comment added | For every comment or work note on a service request that has an author email. |

### Projects and people

| Event | When it is published |
|---|---|
| Invitation | After a membership is written as invited or re-invited by a notice that moved it into that state (see salesforce-ingest.md). Also when a portal invites or re-sends. |
| Welcome | When an existing membership moves from invited or re-invited to registered. |
| Back to open | After a suspended project is reopened (see salesforce-ingest.md), through a queue (section 6). |
| Paging test call | When someone asks for a test call to a person's paging phone. The result is reported back by the notification service. |
| Deferred membership write | When a portal invitation could not reach Salesforce and was saved locally instead. The answers (completed or failed) come back on a results topic this service reads. |

### Other

| Event | When it is published |
|---|---|
| Knowledge article published | When an article's state changes **to** published (not for later saves). |
| SLA recalculation requested | Once per click of the CSM admin "Recalculate SLA" action, with a fixed key because there is no single record it is about. If no publisher is configured the action answers "not available" instead of doing nothing. |
| Change-request notices | Approval requested, and plan-date notices (customer proposed, accepted, rejected) — see change-requests.md. |
| Outage emails | The internal-stakeholder notice and the outage communication — see section 6. |
| Cloud status page | When an outage begins or ends on a cloud the public status page shows — see section 6. |

**INT-230** Every event listed above shall go to the topic given in section 3 and be keyed by
the entity id given in section 2.

**INT-231** An event shall be published only once the data it describes has been committed, so
a consumer never hears about a change the database cannot yet show.

## 6. Notices that are queued first

Some notices are decided from a **before and after** comparison that only the database can see,
or must survive a restart. These are written to a database queue when the change is made and
published by a background job.

| Notice | Background job | Runs when | Interval |
|---|---|---|---|
| Change-request notices | Change-request notice job | `CR_NOTICES_ENABLED` is exactly `true`, a database, and publishing are all present | 5 seconds when idle; runs without waiting while a backlog exists |
| Outage emails | Outage notice job | A database (not the legacy data source), publishing configured, and at least one outage recipient list set | Woken by each outage change, with a 60-second fallback |
| Cloud status | Cloud status job | `CLOUD_STATUS_DRAINER_ENABLED` is exactly `true`, a service list is set, not the legacy data source | 10 seconds |
| "Back to open" | Reactivation job | Publishing configured | 5 seconds |
| Incident report tasks | Incident report job | A database is present | 5 seconds |

**INT-240** A change-request notice shall be on only when explicitly switched on. It is off by
default because the legacy system still sends those emails; turning it on is a paired change
with turning them off there, or every approver is emailed twice.

**INT-241** An outage email shall be on only when its recipient list is set; an empty list
means that email is never sent. Both lists empty turns the job off and logs why.

**INT-242** The "back to open" notice is written in the same all-or-nothing step as the reopen
and published later. A failed publish shall be retried with growing delay (30 seconds,
doubling, at most an hour) and set aside after 20 attempts, and it shall not leave a failure
record per attempt. An operator re-sends a set-aside notice by resetting its queue row.
Delivery is at least once.

**INT-243** The incident report job shall run whenever there is a database, with no switch,
because the database records every incident change and a stopped job would let them pile up and
later be replayed as stale tasks. Each row is applied and marked done in one step; a failed row
is retried with backoff (30 seconds doubling, at most an hour) and parked after ten attempts.
It writes only to the CSM database and publishes nothing except the special-operations alert.

**INT-244** A cloud status event shall carry a claim token the consumer must present to
reserve it before posting and to report the outcome. The scheduled-tasks service remains the
retry path for a post that fails.

## 7. What the notification service relies on

**INT-250** The shape of every event shall stay in step with the notification service's own
copy. The notification service reads events strictly: a new field is accepted only after that
service knows it, and an unknown event type is rejected and retried. When a new event type or
field is added, the notification service shall be deployed **first**.

**INT-251** Email events shall carry final recipients and wording, and the ids of what they are
about, but no links. The notification service builds each recipient's portal link from those ids,
adds its own layout and any copy list, and shall not decide who is told. A publisher shall not add
a link field, because the notification service rejects fields it does not know (INT-250).

## 8. Open questions for the product owner

1. **Duplicates are possible and undetectable.** There is no event id for a consumer to
   deduplicate on, and a re-sent failure record can duplicate an event that did reach the hub.
2. **Nothing re-sends failed events automatically.** Operators must read the failure records
   and re-send by hand. Is that acceptable, or should there be a retry job?
3. **No caller check** on creating, searching or resolving failure records inside this service.
4. **An SLA clock-register event type exists but is never published.** Its payload and name are
   defined, but nothing in this service publishes it. Is it retired?
5. **Double emails during the migration.** While the legacy system still sends its own flows,
   turning on change-request notices or escalation emails here emails everyone twice unless the
   legacy flow is switched off at the same time.
6. **Operations topic overrides other topics.** Setting it silently moves change-request and
   outage notices onto it as well. Is that always intended?
7. **Events skipped for lack of recipients** are only logged. A case nobody watches gets no
   email and leaves no record that one was skipped.

## 9. Where this is implemented

| Area | Where (entity-service) |
|---|---|
| Publishing and failure records | `internal/service/event_publisher_service.go`, `event_publish_failure_service.go`, `internal/repository/event_publish_failure_repo.go`, `internal/eventbus` |
| Event shapes | `internal/events/*.go` |
| Case events | `internal/service/sn_case_service.go`, `case_service.go`, `case_type_transfer_service.go` |
| Incident events | `internal/service/sn_incident_service.go`, `sn_comment_service.go`, `incident_report_service.go` |
| Service-request events | `internal/service/sr_notice_service.go` |
| Project and membership events | `internal/service/salesforce_membership_ingest.go`, `project_membership_write_service.go`, `acp_reactivation_drainer.go`, `project_membership_deferred_write.go` |
| Other events | `kb_article_service.go`, `sla_status_service.go`, `paging_phone.go` |
| Queued notices | `cr_notice_drainer.go`, `outage_notice_drainer.go`, `cloud_status_drainer.go`, `cmd/api/main.go` |
| Wiring of topics | `internal/server/routes.go`, `internal/config/config.go` |
| Routes | `POST /event-publish-failures`, `POST /event-publish-failures/search`, `POST /event-publish-failures/{id}/resolve` |
| Settings | `EVENT_HUB_*`, `EVENT_PUBLISHING_ENABLED`, `*_EVENT_HUB_TOPIC`, `CR_NOTICES_ENABLED`, `CASE_ESCALATION_NOTICES_ENABLED`, `OUTAGE_*`, `CLOUD_STATUS_*`, `SR_ALERT_SRE_TEAM_IDS` |
| Tests | `event_publisher_service_test.go`, `sn_case_service_*_test.go`, `sr_notice_service_test.go`, `outage_notice_drainer_test.go` |
