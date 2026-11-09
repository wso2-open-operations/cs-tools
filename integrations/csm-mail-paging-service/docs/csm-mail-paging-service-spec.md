# CSM Mail Paging Service — Specification

## 1. Purpose

Watches one Gmail mailbox for alert emails sent to a configured group address and publishes one event per
matching email to the event bus. It places no calls and holds no paging logic. The calls are made by
csm-notification-service when it consumes `mail.alert.received` (see `MAIL_ESCALATION_CONFIG` there:
levels R1, R2, … of people, called a wait apart, keyed by this service's rule `name`).

```
Gmail (group mail) -> poll -> rules -> event bus (Envelope) -> csm-notification-service -> phone calls
```

## 2. How a message is handled

Every poll interval (`MAIL_POLL_INTERVAL`, default 30s):

1. List inbox messages addressed (To, Cc or Delivered-To) to any rule's `recipient`, newer than
   `MAIL_LOOKBACK` (default 2d), that do not carry the processed label. Oldest first.
2. Read only the headers (From, To, Cc, Subject, Message-ID). Bodies are never read.
3. For each rule the message matches, publish `{type: eventType, entityId: <Message-ID>, payload}`.
4. Once every publish for the message succeeded, add the processed label. A message that matched no rule is
   labelled too, so it is not read again.

If a publish fails, the message stays unlabelled and the next pass retries it, so delivery is at-least-once.
Retries stop when the message ages out of `MAIL_LOOKBACK`; alert on the "mail message failed; will retry" log
line.

The Gmail label is the only record of "handled". There is no database or Redis.

## 3. Rules (`MAIL_TRIGGER_RULES`)

A JSON array; at least one entry; names must be unique.

| Field | Meaning |
|---|---|
| `name` | Unique rule name, carried in the payload and used by csm-notification-service to pick the ladder |
| `recipient` | Group or mailbox address the email was sent to (case-insensitive) |
| `senderRegex` | Optional: only emails whose From matches |
| `eventType` | Event type published on a match |

One email matching two rules produces two events.

## 4. Event

Envelope `{type, entityId, payload}`, keyed by the Message-ID. Payload fields: `ruleName`, `messageId`,
`dedupeKey` (`<Message-ID>/<rule name>`, stable across retries and distinct per rule), `subject`, `from`,
`receivedAt`. Consumers dedupe on `dedupeKey`, not on the Message-ID alone. Only `mail.alert.received` has a
consumer today.

## 5. Google access

An OAuth refresh token for one dedicated mailbox user who is a member of the group (no domain-wide
delegation, so a leaked token exposes that mailbox only). The token is obtained once with
`go run ./cmd/server gmail-auth`. The only scope used is the one that allows list, read headers and label.

## 6. Deployment

Single replica: two replicas polling one mailbox can both publish a message before either labels it. The only
HTTP endpoint is `GET /health` (liveness; it never reports Gmail or event-bus state).

## 7. Limits

- A message unpublished after `MAIL_LOOKBACK` is no longer listed.
- Duplicates are possible (at-least-once); the consumer's `dedupeKey` handling is what absorbs them.
