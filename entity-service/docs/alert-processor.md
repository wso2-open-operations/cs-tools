# SRE alerts v2: queue consumer and alert processor

How alerts get from the cloud queues to CSM incidents inside entity-service, why it is built this way, and what phase 2
adds before v2 replaces alert-ingestion and alert-core. The operational detail (settings, columns, statuses) is in
`entity-service/CLAUDE.md`, sections "SRE alert consumer" and "Alert processor".

## Phase 1 (built)

```
gateways ──► priority queues (Service Bus, SQS) ─┐
         └─► standard queues (Service Bus, SQS) ─┴─► AlertConsumer ──► alert (raw body + normalised alert)
                                                                            │
                                         AlertProcessor (claim, lock, decide)┘──► IncidentService.CreateIncident
```

| Requirement | How |
|---|---|
| Any number of clouds and queues | Queues are configuration per tier and cloud; one adapter per provider type |
| Priority before standard | Strict priority by default in the consumer (`ALERT_CONSUMER_PRIORITY_WEIGHT=0`) and in the processor's claim (priority tier first) |
| Raw body kept | `alert.raw_body` on every message's first row; a message that cannot be normalised is an error row (`error_reason`, status `ERROR`), dead-lettered only if even that row is refused |
| Never lose an alert | ACK only after commit; dedup on `(message_id, message_index)`; claims on a lease; retries, then FAILED |
| One incident : many alerts | `alert.incident_id`; an open incident of the fingerprint takes later alerts |
| No duplicate incident | Per-fingerprint advisory lock; exact correlation-id lookup before every create |
| Incidents like alert-core's | Same request mapping, correlation tag and creation note; same mute-rule matcher; same fingerprint |
| Safe rollout | `hybrid` records decisions (SHADOWED) and raises nothing; `v2` raises |

### Decisions and why

- **Postgres is the queue, woken by `pg_notify`; in-process create.** The team asked for the processor to act on an
  alert as soon as it is stored. The consumer's insert notifies `alert_stored` in the same transaction (delivered only on
  commit) and a LISTEN connection wakes the workers at once; a 5s poll is the backstop. An expert review from four angles
  (reliability, Postgres, messaging, delivery) preferred this to publishing `alert.created` through an outbox to Event
  Hubs: nothing else reads the alerts, Postgres already gives exact priority (`ORDER BY tier DESC`), retry timing,
  dead letter (FAILED) and idempotency, and an Event Hubs hop adds a second at-least-once hop with no per-message ack (it
  needs a stale sweep), a hard dependency, and as first built about 1s per publish. One table, no `alert_outbox`: the
  queue state lives on the alert row. The create goes through the same service as `POST /incidents`, so assignment,
  `incident.created` and SLA clocks are not reimplemented.
- **Store, commit, ACK, then decide separately.** Creating the incident before the queue ACK would tie intake to the
  incident path: a slow create expires the queue lock and is created twice, an incident outage uses up the delivery count
  and dead-letters alerts mid-outage, and a half-failed multi-alert message re-raises what already worked.
- **Open incident found by correlation tag, not only by link.** alert-core's open incidents carry the same
  `[fp:<12>:<ms>]` tag, so during the overlap at cutover a v2 alert joins the incident v1 opened instead of paging
  again, and an alert whose link was lost still finds its incident.
- **Attach for as long as the incident is open, capped by `ALERT_PROCESSOR_MAX_INCIDENT_AGE` (24h).** alert-core opens a
  new incident 5 minutes after the first alert; that pages repeatedly for one outage. With no cap, an incident left open
  for days would swallow a new outage of the same fingerprint, so it would not page. 24h is the starting value; gate 5b
  of the shadow run (below) measures whether it is right.
- **Muting fails open**, as alert-core's does: if the rules cannot be read for 5 minutes, alerts page.
- **Separate `alert` table** (not alert-core's `alerts`): v1 is never touched, needs no fence during the shadow run, and
  entity-service owns the table's migrations.
- **In entity-service, not csm-scheduled-tasks.** csm-scheduled-tasks is a Choreo Scheduled Task: it starts, runs one
  tick and exits, hourly by default, with no database of its own. The consumer needs a process that stays up to hold
  long polls and renew Service Bus locks, and the processor needs the database and the in-process `IncidentService`;
  an hourly tick would also add up to an hour to every page. Daily jobs around alerts (retention purge, shadow-run
  gates) do fit csm-scheduled-tasks, calling an entity-service endpoint as the audit-log upkeep does. If alert work ever
  needs its own scaling or blast radius, run the same entity-service image as a separate deployment with only the
  alert workers on, rather than a new service.
- **Provider adapters** (`internal/servicebus`, `internal/sqsqueue`): each cloud's SDK receives, acknowledges, renews
  locks and dead-letters differently. One adapter per provider type puts those behind one interface, so the scheduler,
  workers and store are written once and a new cloud is one adapter plus configuration.
- **Per-fingerprint lock** (`pg_advisory_xact_lock` on the fingerprint, for the decision's transaction): workers and
  replicas run in parallel, and `CreateIncident` does not de-duplicate. Without the lock, two alerts of one outage on two
  workers both find no open incident and both create one, and both page. With it, the second waits, then finds the
  incident the first created and attaches.

### Known limits of phase 1

- An alert that joins an open incident adds nothing to it: no Duplicate / OK work note, and no severity escalation.
- A muted alert is not re-checked when its rule ends; a source that does not fire again stays muted.
- Nothing pages on a FAILED alert or a dead-lettered message beyond the ERROR log and the gateway's DLQ notifier.
- Alerts of one fingerprint are serialized but not strictly ordered: under concurrency a later alert can be decided
  first.
- A message SQS handed to a long poll that a stopping replica had open stays hidden for the 60 second visibility timeout,
  then arrives (seen in a local run): delayed, not lost.
- One AWS region and one Service Bus namespace.
- `CreateIncident` runs inside the fingerprint lock's transaction, so a worker can hold two pool connections; the
  pool-budget check counts them. The LISTEN connection is one more Postgres connection, outside the pool.
- If production reaches Postgres through a pooler in transaction mode, LISTEN receives nothing and decisions wait for
  the 5s backstop; confirm before v2.

## Phase 2 (to do before v2 replaces v1)

In priority order. Each item names what done looks like.

1. **Paging for what was not raised.** Alert on any FAILED row and any dead-lettered message (both clouds), and on
   error rows (`error_reason`) per source. Done: a FAILED row reaches the SRE on-call within 5 minutes.
2. **Work notes on the incident.** Port alert-core's Duplicate / OK notes (`BuildWorkNote`) through `UpdateIncident`,
   batched per incident like its delivery sweep. Done: every linked alert shows on its incident.
3. **Severity escalation.** A more severe duplicate raises the incident's impact and urgency (alert-core's
   `MoreSevere`). Done: a warning incident that turns critical carries critical priority.
4. **Mute rule end.** Re-check MUTED fingerprints when their rule ends or is ended early; raise the incident if the last
   alert is still firing, as alert-core's `holdMuted` does.
5. **Observability.** Metrics per tier: queue depth, oldest message age, claim lag (`processed_on - created_on`),
   counts by status. Alerts on standard-tier oldest age (starvation under strict priority) and on claim lag.
6. **Chat fallback.** When a create keeps failing, post alert-core's fallback Chat card so the alert is seen.
7. **Shadow-run gates (in `hybrid`), automated daily**, because alert-core purges its rows after 7 days:
   - gate 1: every fingerprint alert-core stored in a day, v2 stored too (allowing for the listed fingerprint
     differences: AWS budget identifiers, metric name defaults);
   - gate 2: error rows explained per source;
   - gate 3: zero unexplained dead letters;
   - gate 4: p99 of `processed_on - received_on` per tier within target;
   - gate 5: `WOULD_CREATE` count against alert-core's incidents, difference explained;
   - gate 5b: `WOULD_ATTACH` to incidents older than 24h, which sets the final `ALERT_PROCESSOR_MAX_INCIDENT_AGE`.
   Exit: 7 consecutive green days within 14, including a weekend, signed off by the SRE lead.
8. **Cutover and rollback runbook.** Set `v2`, disable (not delete) the v1 feeds, let alert-core drain, stop v1. Keep
   v1 deployable for 14 days; rollback is `hybrid` plus re-enabling the v1 feeds.
9. **Retention.** Decide how long `alert` rows are kept, and add a purge (alert-core keeps 7 days).
10. **More regions and namespaces.** Key SQS clients by region and Service Bus clients by namespace, so a second one is
    configuration.

## Open questions

- Latency target per tier (gate 4).
- Who is paged for FAILED rows and dead letters (phase 2 item 1).
- Retention period (item 9).
- Whether the gateways' `source` values equal alert-ingestion's, or gate 1 needs a mapping.
