<!--
Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).

WSO2 LLC. licenses this file to you under the Apache License,
Version 2.0 (the "License"); you may not use this file except
in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing,
software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
KIND, either express or implied.  See the License for the
specific language governing permissions and limitations
under the License.
-->

# CRE escalation ladder — manual test runbook

Everything below runs locally and **dials nobody**: `--channel log` makes the
real engine run the whole ladder and write a line per rung naming who it would
have reached. The authority for every expected value is the user's own
`[Updated] CRE Notification Escalation rules .xlsx` (`Rules`, `Twillio calls`
and `Escalation ladder` tabs). That workbook is read-only — report a
disagreement, do not edit it.

## 0. Prerequisites

```bash
cd <repo>/sourcecode/cs-tools

# a. the stack (postgres, kafka, redis, mock-oidc, entity-service)
docker compose up -d postgres kafka redis mock-oidc entity-service

# b. migrations + roster + roles + alert-duty nominees. Safe to re-run.
docker compose up migrate

# c. confirm LEVEL_0 has somebody to call: expect 3 for all nine ABTs
docker exec csm-platform-postgres-1 psql -U postgres -d csm_platform -tAc \
  "select t.key||' = '||count(tm.alert_tier) from team t
     left join team_member tm on tm.team_id=t.id and tm.alert_tier is not null
    where t.type in ('cre-abt','sre-abt') group by t.key order by t.key;"
```

**Use a Redis the compose stack is not also using.** The running
csm-notification-service has its own escalation engine on `6379` and works the
same ladder keys, so both place calls — 62 alerts for 21 planned calls, the
first time this was got wrong.

```bash
docker run -d --name cre-test-redis -p 127.0.0.1:16390:6379 redis:7-alpine
```

## 1. The commands

Named scenarios, from the repo root -- these start and stop their own Redis, so
nothing collides with the compose stack:

```bash
USE_TEAM_SCHEDULE=1 ./scripts/csm-compose/test-cre-ladder.sh p0
USE_TEAM_SCHEDULE=1 ./scripts/csm-compose/test-cre-ladder.sh all        # ~1 min
USE_TEAM_SCHEDULE=1 ./scripts/csm-compose/test-cre-ladder.sh realtime   # ~16 min
USE_TEAM_SCHEDULE=1 TEAM=castor ./scripts/csm-compose/test-cre-ladder.sh p0
```

For a scenario the script does not name, run the harness directly. `ladder` is
a **shell function you must define first**, and it only works from the service
directory -- paste this block, then call it:

```bash
cd integrations/csm-notification-service

ladder() {
  USE_TEAM_SCHEDULE=1 CUSTOMER_ENTITY_BASE_URL=http://localhost:8081 \
  OAUTH2_CLIENT_ID=csm-notification-service-dev-client \
  OAUTH2_CLIENT_SECRET=dev-secret \
  OAUTH2_TOKEN_URL=http://localhost:9100/oauth2/token \
  go run ./cmd/escalation-local --channel log --redis 127.0.0.1:16390 \
    --minute 150ms --tick 50ms --max-calls 80 "$@"
}

ladder --priority P0 --shift LK --team vega
```

If you see `zsh: command not found: ladder`, the function was not defined in
this shell -- paste the block again, or use the script above instead.

The client id matters. entity-service reads the caller's identity from
`x-jwt-assertion` and checks it against `AUTH_INTERNAL_CLIENT_IDS`; the
service's own `.env` carries the real Asgardeo id, which the local container
does not trust, so every rung comes back `RESOLVE_FAILED` with nothing saying
why. mock-oidc mints a token for the dev id that it does accept.

`--minute 150ms` compresses a ladder minute. For `--cancel-at` that is fine;
for `--cancel-after` use `--minute 1m`, because this tool's own startup is
spent in ladder minutes (measured: 5s of startup = 12 ladder minutes at
`--minute 400ms`, which put an ack aimed at LEVEL_2 after LEVEL_4).

## 2. Reading the output

```
the ladder the engine scheduled (37 calls):
  +0s     LEVEL_0  #1  Apollo Engineer 05     ← three lines at one timestamp
  +0s     LEVEL_0  #1  Artemis Engineer 10       = three PEOPLE
  +1m0s   LEVEL_1  #1  Apollo Engineer 01
  +2m0s   LEVEL_1  #2  Apollo Engineer 01     ← #1/#2/#3 = the same rung
  +3m0s   LEVEL_1  #3  Apollo Engineer 01        dialled again
```

- **lines sharing a timestamp** are different people on that rung
- **`#1 #2 #3`** is the attempt index — a repeat call because nobody answered
- **calls = people × attempts.** P0 attempts are `[1,3,3,3,3]` per rung, P1
  `[2,3,3,3,3]`, from `DefaultPolicy`

## 3. Known harness limitations — read before judging a result

| Limitation | Effect on the output | Status |
|---|---|---|
| `escalation-local` does **not** read `INCIDENT_ESCALATION_CONFIG` | LEVEL_2 calls **every** team lead, not the configured 3; and R3's LEVEL_0 spans all nine ABTs including the SRE ones (apollo, artemis) rather than the seven CRE ones | open |
| The stand-in roster path has no rule table | without `USE_TEAM_SCHEDULE=1` the shift makes no difference to who is called, and the reported rule is from the retired fourteen-row model | by design |
| P2–P4 timings | verified against the sheet by unit test only, not by a real run | open |

So: **LEVEL_0, LEVEL_1, LEVEL_3, LEVEL_4 and all timings are testable now.
LEVEL_2's count is not** until the harness loads the config.

## 4. P0 scenarios

P0 = the sheet's **S0** row: rungs open at **+0, +1, +4, +8, +12**, ladder ends
+16. Attempts `[1,3,3,3,3]`.

| # | Scenario | Command | Expect |
|---|---|---|---|
| P0-1 | Business hours, on an ABT → **R2** | `ladder --priority P0 --shift LK --team vega` | `rule=R2`; L0 = **3** people, vega's T1→T2→T3 **in that order**; L1 = 1 (vega's lead) |
| P0-2 | Business hours, no ABT → **R3** | `ladder --priority P0 --shift LK --team "Unmapped Group"` | `rule=R3`; L0 = **7**, one nominee per ABT; L1 = **1** from the lead pool |
| P0-3 | Morning rota → **R1a** | `ladder --priority P0 --shift LK_MORNING --team vega` | `rule=R1a`; L0 = everyone on the morning rota (sheet describes 2) |
| P0-4 | Weekend rota → **R1b** | `ladder --priority P0 --shift LK_WEEKEND --team vega` | `rule=R1b`; L0 = the weekend rota (sheet describes 3) |
| P0-5 | Evening, on an ABT → **R4a** | `ladder --priority P0 --shift LK_EVENING --team vega` | `rule=R4a`; L0 = **2** — vega's own rota member **first**, then one other |
| P0-6 | Evening, no ABT → **R4b** | `ladder --priority P0 --shift LK_EVENING --team "Unmapped Group"` | `rule=R4b`; L0 = the evening rota (sheet describes 7) |
| P0-7 | Night → **R5** | `ladder --priority P0 --shift USA --team vega` | `rule=R5`; L0 = **3** Americas nominees; L1 = **3** Americas team leads; L2 = **1** Americas lead |
| P0-8 | Weekend night → **R6** | `ladder --priority P0 --shift USA_WEEKEND --team vega` | `rule=R6`; L0 = **4** — 1 rota member + 3 Americas nominees |

Every row also expects `L3 = 1` CRE Head and `L4 = 1` CS Head, and
`levels="[LEVEL_0 LEVEL_1 LEVEL_2 LEVEL_3 LEVEL_4]"` — a missing level means a
rung resolved to nobody, which is the defect class to watch for.

## 5. P1 scenarios

P1 = the sheet's **S1** row: rungs open at **+6, +9, +18, +28, +38**, ends +48.
Attempts `[2,3,3,3,3]` — note L0 gets **two** calls at P1 where P0 gets one.

Run the same eight rows with `--priority P1`. The recipients are identical; only
the clock and L0's attempt count change. Worth running at least P1-1 and P1-2:

```bash
ladder --priority P1 --shift LK --team vega             # R2, L0 opens at +6m, twice
ladder --priority P1 --shift LK --team "Unmapped Group" # R3, 7 people × 2 attempts
```

## 6. Acknowledgement

A CRE ladder stops on **both** gestures — a move out of NEW **and** a public
comment. Either alone is recorded and it keeps climbing, which is the point: a
dispatcher moving a status while triaging has not picked the incident up.

| # | Scenario | Command | Expect |
|---|---|---|---|
| ACK-1 | Acknowledged at each rung | `ladder --priority P0 --shift LK --team vega --cancel-at LEVEL_0` (then `LEVEL_1`, `LEVEL_2`, `LEVEL_3`) | `ACKNOWLEDGED by a move out of NEW AND a public comment`, then `N call(s) cancelled`; no call above that rung |
| ACK-2 | Status move alone | `ladder --priority P0 --shift LK --team vega --cancel-at LEVEL_1 --cancel-by status` | `half acknowledged; the ladder keeps climbing … stillNeeds="a public comment"`, and rungs above still fire |
| ACK-3 | Public comment alone | `… --cancel-at LEVEL_1 --cancel-by comment` | same — half acknowledged, still climbing |
| ACK-4 | Elevation replaces a ladder | `ladder --priority P0 --shift LK --team vega --kind elevated` | the running ladder is retired and a new one starts from LEVEL_0 |

Ignore the `Acknowledged : N call(s) cancelled` line at the very end of a run
that was **not** acknowledged — that is the harness retiring its own ladder on
exit so a later run does not resume it, not the engine accepting a gesture.

## 7. Conformance checks that need no stack

```bash
go test -count=1 ./internal/escalation/          # ~1s
go test -race -count=1 ./internal/escalation/
```

## 8. Teardown

```bash
docker rm -f cre-test-redis
```

Leaving it running is fine — pass `KEEP_REDIS=1` to `test-cre-ladder.sh`, which
wraps several of the above as named scenarios (`p0`, `timings`, `ack`,
`half-ack`, `shifts`, `not-abt`).

## 9. What a failure looks like

| Symptom | Almost certainly |
|---|---|
| every rung `RESOLVE_FAILED` | the token's client id is not in entity-service's `AUTH_INTERNAL_CLIENT_IDS` — use the `ladder()` wrapper above |
| a level missing from `levels=[…]` | that rung resolved to nobody; check `alert_tier` (step 0c) and the team's `lead` row |
| more calls than planned, repeated rungs | pointed at the compose stack's Redis; two engines are working one ladder |
| `rule=R13` or any R7–R14 | the stand-in roster answered — `USE_TEAM_SCHEDULE=1` was not set |
| `this plan is N calls, more than --max-calls` | raise `--max-calls`; R2 at P0 is 33, R3 is 37 |
