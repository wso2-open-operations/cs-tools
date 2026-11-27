# SRE alerts v2, phase 1: queues to incidents

How an SRE alert gets from the cloud queues to a CSM incident inside entity-service, why it is built this way, and
what phase 2 adds before v2 replaces the v1 services (`integrations/sre-alert-ingestion-service`,
`integrations/sre-alert-core-service`). Settings and file names are in `entity-service/CLAUDE.md`, "SRE alert
pipeline (v2)".

## Flow

```
gateways ──► priority queues (Service Bus, SQS) ─┐
         └─► standard queues (Service Bus, SQS) ─┴─► AlertConsumer ──► alert table (raw body + canonical alert)
                                                                          │ pg_notify on commit
                                       AlertRelay (priority first, backpressure)┘──► Event Hub ALERT_EVENT_HUB_TOPIC
                                                                                       │ alert.created
                                         AlertProcessor (claim, lock, decide) ◄───────┘──► IncidentService.CreateIncident
```

| Requirement | How |
|---|---|
| Any number of clouds and queues | Queues are configuration per tier and cloud; one adapter per provider type (`internal/alertqueue`) |
| Priority before standard | The consumer's free worker takes a priority message first; the relay publishes priority first and holds a backlog in the table, so at most `ALERT_RELAY_MAX_IN_FLIGHT` alerts are ahead of a priority one on the topic |
| Never lose an alert | ACK only after commit; dedup on `(message_id, message_index)`; the row is the outbox and a sweep republishes what nobody acted on; claims on a lease; retries, then FAILED |
| Raw body kept | `alert.raw_body` on every message's first row; a message that cannot be read is an error row, dead-lettered only if even that row is refused |
| One incident : many alerts | `alert.incident_id`; an open incident of the fingerprint takes later alerts |
| No duplicate incident | Per-fingerprint advisory lock; exact correlation-id lookup before every create |
| Incidents like v1's | Same transforms and fingerprint (ported from alert-ingestion), same request mapping, correlation tag and creation note (from alert-core) |
| Safe rollout | `hybrid` records decisions (SHADOWED) and raises nothing; `v2` raises |

## Decisions and why

- **Store, commit, acknowledge, then decide separately.** Creating the incident before the queue acknowledgement
  would tie intake to the incident path: a slow create expires the queue lock and is created twice, an incident
  outage uses up the delivery count and dead-letters alerts mid-outage, and a half-failed multi-alert message
  re-raises what already worked.
- **Queued hand-off over Event Hubs, the alert row as the outbox.** The processor consumes `alert.created` instead of
  polling. The conditions that keep the hop fast and safe:
  - one topic, with relay backpressure, so the backlog stays in the priority-ordered table and no second topic or wait
    gate is needed;
  - a 5 ms producer batch wait (`eventbus.PromptBatchTimeout`; kafka-go's default holds a lone record for a second). The
    producers that carry `incident.created` use it too, since every create publishes it inside the decision: with the
    default, a consumer raised about one new incident a second and a priority alert waited ~7 s in a 40-alert storm;
    with it, the whole storm was decided within 0.1 s.
  - no row lock held while publishing: mark and commit, publish, and give back on failure;
  - a stale sweep, because Event Hubs has no per-message acknowledgement or dead letter.

  The event carries ids only: the processor decides from the row, which stays the record, and an alert can hold
  customer details the topic would otherwise keep for its retention. One table, no separate outbox: the outbox state
  (`published_on`) lives on the alert row.
- **Transforms ported, not shared.** The source transforms are a copy of alert-ingestion's (`internal/alertsource`),
  so a v2 alert has v1's canonical fields and fingerprint during the shadow run. Keep the two in step until v1 is
  retired.
- **Error rows, not retries, for unreadable messages.** A message with no source, an unknown source, a body its
  transform refuses or no alerts is stored as an ERROR row with its raw body and completed: it is kept for whoever
  looks into the source, and never blocks or loops the queue.
- **Open incident found by correlation tag.** alert-core's open incidents carry the same `[fp:<12>:<ms>]` tag, so
  during the overlap at cutover a v2 alert joins the incident v1 opened instead of paging again.
- **Attach while the incident is open, capped by `ALERT_PROCESSOR_MAX_INCIDENT_AGE` (24h).** Without a cap an incident
  left open for days would swallow a new outage of the same fingerprint.
- **In entity-service.** The consumer needs a long-running process to hold long polls and renew locks, and the
  processor needs the database and the in-process incident service. A separate deployment of the same image with only
  the alert workers on is the way to scale or isolate it later.

## Known limits of phase 1

- **No mute rules.** Alert mute rules are not applied; a muted alert in v1 raises an incident here. Phase 2.
- An alert that joins an open incident adds nothing to it: no Duplicate / OK work note, no severity escalation.
- Nothing pages on a FAILED alert or a dead-lettered message beyond the ERROR log.
- Alerts of one fingerprint are serialized, not strictly ordered: under retries a later alert can be decided first.
- One AWS region and one Service Bus namespace.
- The backpressure count is not locked: relays on several replicas can go a batch over the limit for a moment.
- If production reaches Postgres through a pooler in transaction mode, LISTEN receives nothing and publishing waits
  for the 5 s backstop.

## Phase 2 (before v2 replaces v1)

1. **Mute rules** once they are on main: hold back matching alerts (MUTED) and re-check them when the rule ends.
2. **Paging for what was not raised:** FAILED rows, dead-lettered messages and error rows per source.
3. **Work notes and severity escalation** on an attached incident (alert-core's `BuildWorkNote`, `MoreSevere`).
4. **Observability:** alerts in flight, publish and decision lag, oldest waiting alert per tier, counts by status.
5. **Shadow-run gates** in `hybrid`, daily (v1 purges after 7 days): every v1 fingerprint stored by v2, error rows
   explained per source, no unexplained dead letters, decisions matching v1's incidents.
6. **Cutover runbook and retention** for the alert table.
