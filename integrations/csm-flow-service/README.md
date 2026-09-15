# CSM Flow Service

The Go service that replaces WSO2 CSM's ServiceNow automation ("flows"). It
consumes domain events off the shared event bus and runs the ported flows as
Go code.

> **Status: scaffold.** The event consumer, flow registry, and the packages it
> reuses are in place; **no flow is registered yet**. Flows are added one at a
> time, each paired with disabling its ServiceNow counterpart in the same
> change. See [`docs/cutover-porting-plan.md`](docs/cutover-porting-plan.md) for
> the backlog and priority bands, and
> [`docs/flow-porting-specs.md`](docs/flow-porting-specs.md) for each flow's
> real trigger, condition and actions.

## Testing a flow against a real Event Hub

`cmd/replay` runs flows in-process with no broker; `cmd/publish` is its producing
counterpart, putting one crafted event onto the real topic. Neither is deployed.

```bash
# 1. register the flow locally — All() is empty by design, so nothing matches until you do
#    (do NOT commit this: TestCRApprovalNotice_NotRegistered fails if you do)

# 2. put an event on the topic
EVENT_HUB_BROKER=… EVENT_HUB_CONNECTION_STRING=… EVENT_HUB_TOPIC=… \
  go run ./cmd/publish cmd/replay/testdata/cr_approval_assess.json

# 3. in another shell, consume it
EVENT_HUB_CONSUMER_GROUP=dev-<your-name> \
EMAIL_DEBUG_RECIPIENTS=you@wso2.com \
  go run ./cmd/consumer
```

**Use a non-production namespace.** Publishing writes a real record that every
consumer group on that topic sees.

**Give yourself your own `EVENT_HUB_CONSUMER_GROUP`.** Sharing one with a deployed
service means you and it compete for partitions, and each record reaches only one
of you.

**Set `EMAIL_DEBUG_RECIPIENTS`** so anything a flow requests goes to you rather
than a real approval group or customer contact.


## Why this shape

ServiceNow is being switched off. This service is the **pragmatic hand-port**
(plan §2): the live flows become Go handlers wired to the event stream and
`csm-notification-service`, rather than a configurable rule engine. The
configurable engine remains the long-term target — its foundations
(`internal/spec`, `internal/eval`) live here already — but it is not the cutover
vehicle.

## Layout

```
cmd/consumer/         event consumer + health server (the running process)
internal/config/      env configuration (flat single-line strings only)
internal/eventbus/    COPY of csm-notification-service's package (Event Hub)
internal/events/      wire schema of the bus (Envelope + event Type catalog)
internal/entity/      entity-service client (Config/Client/NewClient/do)
internal/flows/       the ported flows + the Registry that routes events
internal/apierror/    typed upstream error (copy)
internal/middleware/  correlation (cfs-), logger, security headers (copy)
internal/spec/        flow-as-tree model — for the future configurable engine
internal/eval/        pure condition evaluator — reusable by hand-ported flows
internal/porting/     snquery: ServiceNow encoded-query → condition (migration aid)
docs/                 architecture, plan, specs, ServiceNow discovery pack
```

`eventbus`, `events`, `entity`, `apierror` and `middleware` are copied from
`csm-notification-service` (separate Go modules, neither imports the other by
design). The originals are the source of truth; keep these in sync by hand and
do not edit the originals from here.

## Run locally

```bash
cp .env.example .env   # fill in EVENT_HUB_* (and CUSTOMER_ENTITY_*/OAUTH2_* for flows that need entity-service)
make run               # go run ./cmd/consumer
curl localhost:8080/health
```

With no flows registered the consumer connects, reads records, matches nothing,
and commits — the health endpoint reports `registeredFlows: 0`.

## Adding a flow

1. Read the flow's row in `docs/flow-porting-specs.md` (dump it with the
   discovery scripts first if it's snapshot-only).
2. Copy `internal/flows/example_template.go` to a new file; implement `Key`,
   `Match` (pure), and `Run` (idempotent).
3. Add table-driven tests using the flow's real conditions.
4. Register it in `internal/flows/register.go` **and** disable its ServiceNow
   (and any hardcoded Go) counterpart in the same commit — the double-fire
   guard (plan §7).

## Testing a flow locally

No broker or deployed dependency is needed — the consumer and flows are designed
to be exercised in-process.

First, once per checkout (this fetches modules and finalizes `go.mod`/`go.sum`):

```bash
go mod tidy
```

**Unit tests — the primary way.** `Match` is pure; `Run` takes fake `Deps`. See
`internal/flows/registry_test.go` for the routing tests and the fake-flow shape
a real flow's own test copies.

```bash
go test ./...            # everything
go test ./internal/flows/... -run Registry -v
```

**Replay harness — fire a crafted event through the registry, no broker.**

```bash
go run ./cmd/replay cmd/replay/testdata/comment_added.json
# reports which registered flows match; add -run to execute them (empty Deps,
# so a flow that calls entity-service or publishes will error — expected).
```

Edit the JSON under `cmd/replay/testdata/` (or pipe your own on stdin) to try
different events. With no flow registered yet it reports "no flows registered";
register one and it shows up.

**Full end-to-end (optional, later).** Only worth it once a flow does real I/O:
run a local Kafka (e.g. Redpanda in Docker) as the bus and a stub HTTP server as
entity-service, point `.env` at them, `make run`, and publish an event.

## CI gates

`go vet`, `go test -race ./...`, `gosec -fmt=text ./...` at 0 issues,
`govulncheck` on `go.mod` changes, `openapi.yaml` kept current. Wired from the
first PR (`make ci`).
