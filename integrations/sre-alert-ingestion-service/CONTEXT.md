# Alert Ingestion

Normalizes monitoring webhooks from many sources into one canonical alert shape.

## Language

**Alarm alert**:
An AWS alert raised by a CloudWatch alarm changing state; it can recover (state OK).
_Avoid_: metric alert

**Alarm overrides**:
Field values (service, environment, severity, category, assignment group) an alarm owner writes into the alarm's description to steer how its alerts are classified and routed.

**Budget alert**:
An AWS alert raised when spend crosses a threshold set on an AWS Budget. Critical and a service interruption unless its **Budget tags** say otherwise; it has no recovery.
_Avoid_: billing alarm, cost alarm

**Budget tags**:
The tags on an AWS Budget that supply a **Budget alert**'s service, environment, severity, category and assignment group, standing in for **Alarm overrides**, which budgets do not have.

## Relationships

- An AWS alert is either an **Alarm alert** or a **Budget alert**
- An **Alarm alert** is classified by its **Alarm overrides**; a **Budget alert** by its **Budget tags**

## Flagged ambiguities

- "metrics alerts" was used for what is here an **Alarm alert**: CloudWatch alarms can watch logs, composites and anomalies too, so the term is the alarm, not the metric.
