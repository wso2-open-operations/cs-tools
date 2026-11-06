# csm-mail-paging-service

Polls Gmail and publishes one event per rule-matching email. No Twilio and no
paging logic lives here: calls are placed by csm-notification-service's
`internal/mailpaging` (`mail.alert.received`, levels R1, R2, ... of people (by email,
numbers come from entity-service) called a wait apart, configured there by
`MAIL_ESCALATION_CONFIG`, keyed by this service's rule `name`). Set this service's
`EVENT_BUS_TOPIC` to the topic that service reads (`EVENT_HUB_TOPIC`). Do not add a second call ladder to this service.

- **Gmail label = idempotency.** A message is labelled processed only after all
  its events were published. Do not move that earlier, and do not add a second
  state store for correctness.
- **Headers only.** The client requests `format=metadata`; never read bodies.
- **One `main` package only** (`cmd/server`). The Choreo buildpack cannot choose between several and fails with "no Go files"; the consent helper is the `gmail-auth` subcommand instead.
- **Auth is an OAuth refresh token for one mailbox user, not a service account.**
  Do not reintroduce domain-wide delegation: a leaked delegated key exposes every
  mailbox in the domain, a leaked refresh token only this one. The token comes from
  a one-time `go run ./cmd/server gmail-auth`; the service never sees a password or 2FA.
- **Config is flat env strings** (`MAIL_TRIGGER_RULES` is one JSON string).
  Never commit credentials or real group addresses; tests use example.com.
- **`eventbus` and `poller.Envelope` mirror csm-notification-service** (separate
  Go modules, kept in sync by hand).
- Run `gosec -fmt=text ./...` and `govulncheck ./...` before a PR.
- **Choreo:** `.choreo/component.yaml` + `openapi.yaml` expose only `GET /health` (liveness, like csm-notification-service). Keep it single-replica.
