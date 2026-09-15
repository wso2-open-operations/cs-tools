# Event fixtures

Hand-written envelopes for `cmd/replay` (match only, no I/O) and `cmd/dryrun`
(runs the flow against the real database, publishes nothing).

The `entity.changed` fixtures mirror what the outbox trigger actually writes,
which is narrower than it looks: `snapshot` is `to_jsonb(NEW)` of the table that
changed, so it holds **that table's own columns, under their database names**.
A `change_request` row change carries `id`, `state` and `git_reference` — in
snake_case — and nothing else.

It does **not** carry the number (that is `work_item.number`), the project, or
the requester. A flow needing those reads the record; see
`flows.ChangeRequests`. Adding camelCase keys here to make a fixture more
convenient would be inventing a payload the database never produces, and the
flow would then pass its tests and fail in staging.

The two CR fixtures use the same id as `setup-cr-flow-test.sql`, so a dry run
against staging resolves real rows.
