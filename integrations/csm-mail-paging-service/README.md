# csm-mail-paging-service

Polls a Gmail mailbox and publishes an event to the event bus for every email
that matches a configured rule. It does not place calls. csm-notification-service
consumes `mail.alert.received` and calls the rule's escalation levels (R1, R2, ...,
one wait apart) by phone, looking each person's number up by email; the ladder is
configured there (`MAIL_ESCALATION_CONFIG`, keyed by the rule `name`), and is off
until that is set. Point `EVENT_BUS_TOPIC` at the topic that service reads
(its `EVENT_HUB_TOPIC`). Any other `eventType`
is published but has no consumer yet.

```
Gmail (group mail) -> poll -> rules -> event bus (Envelope) -> consumer
```

## How it works

1. Every `MAIL_POLL_INTERVAL` it lists messages in the mailbox user's inbox that
   were sent to a rule's `recipient` (To, Cc or Delivered-To), are newer than
   `MAIL_LOOKBACK` and do not carry the processed label.
2. It reads only the headers (From, To, Cc, Subject, Message-ID), never the body.
3. For each matching rule it publishes `{type: eventType, entityId: <Message-ID>, payload}`.
4. Only after every publish succeeded does it add the processed label. A failed
   publish leaves the message unlabelled, so the next pass retries it
   (at-least-once). Consumers dedupe on the payload's `dedupeKey`
   (`<Message-ID>/<rule name>`), not on the Message-ID alone: one email that
   matches two rules produces two events.

   Retries are bounded by `MAIL_LOOKBACK` (default `2d`): a message still
   unpublished after that window is no longer listed. Alert on the
   "mail message failed; will retry" log line.

The Gmail label is the durable record of "handled", so no database or Redis is
needed for correctness.

## Rules (`MAIL_TRIGGER_RULES`)

| Field | Description |
|-------|-------------|
| `name` | Unique rule name, carried in the payload |
| `recipient` | Group or mailbox address the email was sent to |
| `senderRegex` | Optional: only emails from matching senders |
| `eventType` | Event type published on a match |

A new source is another entry in the array.

## Google setup

No domain-wide delegation. The service uses an OAuth refresh token for one
dedicated mailbox user that is a member of the group, so a leaked token exposes
that mailbox only.

1. In Google Cloud, enable the Gmail API and create an OAuth client of type
   "Desktop app". Set the consent screen to "Internal" so the refresh token
   does not expire after 7 days.
2. Run `GMAIL_OAUTH_CLIENT_ID=... GMAIL_OAUTH_CLIENT_SECRET=... go run ./cmd/server gmail-auth`,
   log in as the mailbox user (password and 2FA happen in the browser), and
   copy the printed refresh token.
3. Store the client id, secret and refresh token as secrets (see `.env.example`).

## Deployment (Choreo)

`.choreo/component.yaml` and `openapi.yaml` follow csm-notification-service: the
only endpoint is `GET /health` on `PORT` (default 8080), a liveness probe that
never reports Gmail or event-bus state. Run a **single replica**: two replicas
polling the same mailbox can both publish a message before either labels it.

## Commands

```bash
go test -race ./...
go run ./cmd/server
```
