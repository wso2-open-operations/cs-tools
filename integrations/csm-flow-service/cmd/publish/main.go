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

// Command publish is a DEV-ONLY tool for putting one crafted event onto the
// real Event Hub topic, so a flow can be exercised end to end against a live
// broker rather than only in unit tests or cmd/replay's in-process harness.
//
// It is the producing half of local testing; cmd/replay is the consuming half
// but never touches a broker. Neither is deployed — no Choreo component points
// at either.
//
//	# publish one fixture to the configured topic
//	EVENT_HUB_BROKER=… EVENT_HUB_CONNECTION_STRING=… EVENT_HUB_TOPIC=… \
//	  go run ./cmd/publish cmd/replay/testdata/cr_approval_assess.json
//
//	# then, in another shell, run the consumer and watch it react
//	go run ./cmd/consumer
//
// USE A NON-PRODUCTION NAMESPACE. This writes a real record to a real topic,
// and every consumer group on that topic sees it — including
// csm-notification-service, which will ignore an event type it does not know
// but will still read it.
//
// The record is keyed by the envelope's entityId, matching what entity-service
// does, so a hand-published event lands on the same partition a real one would.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-flow-service/internal/config"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-flow-service/internal/eventbus"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-flow-service/internal/events"
)

// publishTimeout bounds the whole publish so a misconfigured broker fails
// visibly instead of hanging a developer's terminal.
const publishTimeout = 30 * time.Second

func main() {
	data, err := readInput(os.Args[1:])
	if err != nil {
		fail("read input: %v", err)
	}

	// Decode and re-encode rather than shipping the file bytes verbatim: it
	// rejects a malformed fixture here, where the error is obvious, instead of
	// at the consumer where it looks like a flow bug.
	var env events.Envelope
	if err := json.Unmarshal(data, &env); err != nil {
		fail("input is not a valid event envelope: %v", err)
	}
	if env.Type == "" {
		fail("envelope has no \"type\"")
	}
	if env.EntityID == "" {
		fail("envelope has no \"entityId\" — it is the partition key, so publishing without one would break per-entity ordering")
	}
	value, err := json.Marshal(env)
	if err != nil {
		fail("re-encode envelope: %v", err)
	}

	config.LoadDotEnv(".env")

	cfg := eventbus.Config{
		Broker:           mustEnv("EVENT_HUB_BROKER"),
		ConnectionString: mustEnv("EVENT_HUB_CONNECTION_STRING"),
		Topic:            mustEnv("EVENT_HUB_TOPIC"),
	}

	producer := eventbus.NewProducer(cfg)
	defer producer.Close()

	ctx, cancel := context.WithTimeout(context.Background(), publishTimeout)
	defer cancel()

	if err := producer.Publish(ctx, []byte(env.EntityID), value); err != nil {
		fail("publish to %s: %v", cfg.Topic, err)
	}

	fmt.Printf("published type=%s entityId=%s to topic=%s (%d bytes)\n",
		env.Type, env.EntityID, cfg.Topic, len(value))
	fmt.Println("run ./cmd/consumer against the same topic to see which flows react")
}

// readInput reads the envelope from a file argument, or stdin when none given.
func readInput(args []string) ([]byte, error) {
	if len(args) > 0 && args[0] != "-" {
		return os.ReadFile(args[0])
	}
	return io.ReadAll(os.Stdin)
}

func mustEnv(key string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		fail("missing required env var %s", key)
	}
	return v
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "publish: "+format+"\n", args...)
	os.Exit(1)
}
