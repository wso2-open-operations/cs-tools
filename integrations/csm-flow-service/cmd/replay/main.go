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

// Command replay is a DEV-ONLY local harness for exercising flows without a
// broker or any deployed dependency. It reads one events.Envelope as JSON (from
// a file argument or stdin), decodes it exactly as the consumer would, and
// reports which registered flows match it. It is not deployed (no Choreo
// component points at it) and does no I/O of its own.
//
//	# report which flows match a crafted event
//	go run ./cmd/replay cmd/replay/testdata/comment_added.json
//	echo '{"type":"case.comment_added","entityId":"c1","payload":{}}' | go run ./cmd/replay
//
//	# also execute the matching flows (Deps are empty here, so a flow that calls
//	# entity-service or publishes will error — that's expected without config):
//	go run ./cmd/replay -run cmd/replay/testdata/comment_added.json
//
// For real assertions, prefer a Go unit test over this harness — Match is pure
// and Run takes fake Deps (see internal/flows/registry_test.go).
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-flow-service/internal/eventbus"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-flow-service/internal/events"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-flow-service/internal/flows"
)

func main() {
	run := flag.Bool("run", false, "execute matching flows (Deps are empty; a flow needing entity/producer will error)")
	flag.Parse()

	data, err := readInput(flag.Args())
	if err != nil {
		fmt.Fprintf(os.Stderr, "replay: read input: %v\n", err)
		os.Exit(1)
	}

	var env events.Envelope
	if err := json.Unmarshal(data, &env); err != nil {
		fmt.Fprintf(os.Stderr, "replay: input is not a valid event envelope: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("event   : type=%s entityId=%s (knownType=%v)\n", env.Type, env.EntityID, env.Type.IsKnown())
	fmt.Printf("payload : %s\n\n", compact(env.Payload))

	// Catalogue, not All: a flow is worth replaying before it is enabled —
	// that is when its trigger condition is least proven.
	registry := flows.NewRegistry(flows.Deps{}, flows.Catalogue()...)
	evt := flows.Event{
		Envelope: env,
		Record:   eventbus.Record{Topic: "cs-events", Value: data},
	}

	registered := registry.Flows()
	if len(registered) == 0 {
		fmt.Println("no flows ported yet (flows.Catalogue() is empty).")
	} else {
		matches := 0
		fmt.Println("flow matching (enabled=false means ported but not yet live — see flows.All):")
		for _, f := range registered {
			m := f.Match(evt)
			if m {
				matches++
			}
			fmt.Printf("  %-30s match=%v enabled=%v\n", f.Key(), m, enabled(f.Key()))
		}
		fmt.Printf("\n%d/%d ported flows match this event\n", matches, len(registered))
	}

	if *run {
		fmt.Println("\n-run: executing matching flows with empty Deps ...")
		if err := registry.Handle(context.Background(), evt.Record); err != nil {
			fmt.Printf("Handle returned error (would be retried on the bus): %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Handle completed without error")
	}
}

func readInput(args []string) ([]byte, error) {
	if len(args) > 0 {
		return os.ReadFile(args[0])
	}
	return io.ReadAll(os.Stdin)
}

// compact re-marshals raw JSON onto one line for a tidy print; falls back to
// the raw bytes if it isn't valid JSON.
func compact(raw json.RawMessage) string {
	if len(raw) == 0 {
		return "(none)"
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return string(raw)
	}
	b, err := json.Marshal(v)
	if err != nil {
		return string(raw)
	}
	return string(b)
}

// enabled reports whether a ported flow is also live in production.
func enabled(key string) bool {
	for _, f := range flows.All() {
		if f.Key() == key {
			return true
		}
	}
	return false
}
