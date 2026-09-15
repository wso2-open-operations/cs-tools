// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

// Command dryrun is a DEV-ONLY harness that runs a ported flow end to end
// against the REAL database but publishes nothing and sends no mail.
//
// It exists because the last two hops of the chain need infrastructure a
// developer may not have: the bus needs a consumer group, and the mail needs
// csm-notification-service plus a mail credential. Everything BEFORE that —
// the outbox row, the trigger condition, the recipient resolution, the exact
// event that would be published — needs only DATABASE_URL, and that is the part
// where a port is most likely to be wrong.
//
//	# take the oldest unclaimed change_request row and run the flow on it
//	DATABASE_URL=… go run ./cmd/dryrun -flow cr_approval_notice
//
//	# run the same flow against a crafted event instead of the database
//	DATABASE_URL=… go run ./cmd/dryrun -flow cr_approval_notice \
//	    -event cmd/replay/testdata/cr_approval_assess.json
//
//	# write the event it would publish, to feed the notification service's preview
//	DATABASE_URL=… go run ./cmd/dryrun -flow cr_approval_notice -out notice.json
//
// Recipient resolution is REAL: it runs the same queries the deployed consumer
// would, against whatever database DATABASE_URL points at. Only the publish is
// stubbed. That is the point — a flow that resolves nobody is the single most
// common way a port looks fine and notifies no one.
//
// It runs a flow from flows.Catalogue(), not flows.All(), so a flow still
// gated behind the double-fire guard can be exercised without enabling it.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-flow-service/internal/config"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-flow-service/internal/eventbus"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-flow-service/internal/events"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-flow-service/internal/flows"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-flow-service/internal/outbox"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-flow-service/internal/store"
)

const dryRunTimeout = 30 * time.Second

func main() {
	var (
		flowKey   = flag.String("flow", "", "flow key to run (see -list)")
		eventFile = flag.String("event", "", "run against this event JSON instead of claiming an outbox row")
		out       = flag.String("out", "", "write the event that would be published to this file")
		debugTo   = flag.String("debug-recipients", "", "comma-separated override for the resolved audience")
		batch     = flag.Int("batch", 1, "how many outbox rows to claim")
		list      = flag.Bool("list", false, "list ported flow keys and exit")
	)
	flag.Parse()

	if *list {
		for _, k := range flows.CatalogueKeys() {
			fmt.Println(k)
		}
		return
	}
	if *flowKey == "" {
		fail("missing -flow (try -list); ported flows: %s", strings.Join(flows.CatalogueKeys(), ", "))
	}
	flow, ok := flows.ByKey(*flowKey)
	if !ok {
		fail("no ported flow %q; ported flows: %s", *flowKey, strings.Join(flows.CatalogueKeys(), ", "))
	}

	config.LoadDotEnv(".env")

	dsn := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if dsn == "" {
		fail("missing DATABASE_URL — recipient resolution is real, so this tool has nothing to resolve against without it")
	}

	ctx, cancel := context.WithTimeout(context.Background(), dryRunTimeout)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		fail("database: %v", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		fail("database unreachable: %v", err)
	}
	csmStore := store.New(pool)

	pub := &capturingProducer{}
	registry := flows.NewRegistry(flows.Deps{
		Recipients:           csmStore,
		ChangeRequests:       csmStore,
		Producer:             pub,
		EmailDebugRecipients: splitList(*debugTo),
	}, flow)

	fmt.Printf("flow    : %s (from the catalogue; enabled in All()=%v)\n", flow.Key(), isEnabled(flow.Key()))
	fmt.Printf("database: connected\n\n")

	var claimed []int64
	if *eventFile != "" {
		claimed = nil
		if err := runFromFile(ctx, registry, *eventFile); err != nil {
			fail("%v", err)
		}
	} else {
		claimed, err = runFromOutbox(ctx, registry, csmStore, flow, *batch)
		if err != nil {
			fail("%v", err)
		}
	}

	report(pub, *out)
	if len(claimed) > 0 {
		fmt.Printf("\nThose outbox rows are now marked published. To replay them:\n")
		fmt.Printf("  UPDATE event_outbox SET published_on = NULL WHERE id IN (%s);\n", joinIDs(claimed))
	}
}

// runFromOutbox drains real rows — the path the deployed consumer takes.
func runFromOutbox(ctx context.Context, registry *flows.Registry, csmStore *store.Store, flow flows.Flow, batch int) ([]int64, error) {
	types := flows.TriggerEntityTypes([]flows.Flow{flow})
	if len(types) == 0 {
		return nil, fmt.Errorf("flow %s is not row-triggered — it reacts to bus events, so use -event", flow.Key())
	}

	// Claim through the store directly first, so the ids can be reported for
	// replay; the drainer's own claim would swallow them.
	changes, err := csmStore.ClaimChanges(ctx, types, batch)
	if err != nil {
		return nil, fmt.Errorf("claim outbox rows: %w", err)
	}
	if len(changes) == 0 {
		fmt.Printf("no unclaimed event_outbox rows for entity_type in %v.\n", types)
		fmt.Println("Make a change the flow reacts to, or replay a claimed row with:")
		fmt.Println("  UPDATE event_outbox SET published_on = NULL WHERE id = <id>;")
		return nil, nil
	}

	ids := make([]int64, 0, len(changes))
	for _, c := range changes {
		ids = append(ids, c.ID)
		fmt.Printf("outbox  : id=%d entityType=%s entityId=%s\n", c.ID, c.EntityType, c.EntityID)
		fmt.Printf("changes : %s\n", compact(c.Changes))
	}

	drainer := &outbox.Drainer{
		Claimer:     preClaimed(changes),
		Dispatcher:  registry,
		EntityTypes: types,
		BatchSize:   batch,
	}
	if _, err := drainer.DrainOnce(ctx); err != nil {
		return ids, fmt.Errorf("dispatch: %w", err)
	}
	return ids, nil
}

// runFromFile feeds one crafted envelope through the same registry, for a flow
// whose triggering row has already been claimed (or does not exist yet).
func runFromFile(ctx context.Context, registry *flows.Registry, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read event: %w", err)
	}
	var env events.Envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return fmt.Errorf("event is not a valid envelope: %w", err)
	}
	fmt.Printf("event   : type=%s entityId=%s (from %s)\n", env.Type, env.EntityID, path)
	return registry.Handle(ctx, eventbus.Record{
		Topic:          "dryrun",
		Key:            []byte(env.EntityID),
		Value:          data,
		IsFinalAttempt: true,
		NoMoreRetries:  true,
	})
}

// report prints what the flow tried to publish, which is the whole output of a
// dry run: the recipients are the answer to "would this have notified anyone?"
func report(pub *capturingProducer, out string) {
	if len(pub.published) == 0 {
		fmt.Println("\nRESULT: the flow published nothing.")
		fmt.Println("That is a real outcome, not a failure — the flow's trigger condition did not")
		fmt.Println("match, or it matched and resolved no recipients (an empty approval group, a")
		fmt.Println("project with no contacts). Both are silent by design.")
		return
	}
	for i, p := range pub.published {
		fmt.Printf("\nRESULT %d/%d: the flow WOULD publish this event.\n", i+1, len(pub.published))
		var env events.Envelope
		if err := json.Unmarshal(p, &env); err == nil {
			fmt.Printf("  type   : %s\n", env.Type)
			var notice events.CRApprovalRequestedPayload
			if json.Unmarshal(env.Payload, &notice) == nil && notice.Subject != "" {
				fmt.Printf("  subject: %s\n", notice.Subject)
				fmt.Printf("  to     : %s (%d)\n", strings.Join(notice.Recipients, ", "), len(notice.Recipients))
			}
		}
		fmt.Printf("  json   : %s\n", string(p))
	}
	fmt.Println("\nNothing was published and no mail was sent.")

	if out != "" {
		if err := os.WriteFile(out, append(pub.published[0], '\n'), 0o600); err != nil {
			fail("write %s: %v", out, err)
		}
		fmt.Printf("Wrote the event to %s — csm-notification-service's cmd/preview renders it.\n", out)
	}
}

// capturingProducer stands in for the bus: it records rather than publishes,
// which is what makes this a dry run.
type capturingProducer struct{ published [][]byte }

func (p *capturingProducer) Publish(_ context.Context, _, value []byte) error {
	p.published = append(p.published, value)
	return nil
}

// preClaimed replays rows already claimed above, so the drainer's dispatch path
// runs verbatim without a second claim.
type preClaimed []store.Change

func (c preClaimed) ClaimChanges(context.Context, []string, int) ([]store.Change, error) {
	return []store.Change(c), nil
}

func isEnabled(key string) bool {
	for _, f := range flows.All() {
		if f.Key() == key {
			return true
		}
	}
	return false
}

func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func joinIDs(ids []int64) string {
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, fmt.Sprintf("%d", id))
	}
	return strings.Join(parts, ", ")
}

func compact(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(b)
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "dryrun: "+format+"\n", args...)
	os.Exit(1)
}
